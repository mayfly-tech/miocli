package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
	"miocli/internal/theme"
)

const (
	refreshInterval = 5 * time.Second
	delayTestURL    = "https://www.gstatic.com/generate_204"
	delayTimeoutMS  = 5000
	delayWorkers    = 10
)

type dataState uint8

const (
	stateLoading dataState = iota
	stateReady
	stateError
)

type Model struct {
	client mihomo.Client
	ctx    context.Context
	cancel context.CancelFunc
	server string

	state             dataState
	version           mihomo.Version
	proxies           mihomo.Proxies
	groups            []string
	groupIndex        int
	groupPath         []string
	selectedGroupPath []string
	parentRows        []int
	rowIndex          int
	focusRight        bool
	mode              mihomo.Mode
	modeError         string
	lastError         string
	refreshError      string
	notice            string
	operation         string
	help              bool
	noColor           bool
	width             int
	height            int
	polling           bool
	pollRetries       int
	pollSerial        uint64
	pollGeneration    uint64
	runtime           Runtime
	page              resourcePage
	command           textinput.Model
	commanding        bool
	providers         map[string]mihomo.Provider
	providerErr       string
	providerBusy      bool
	connections       []mihomo.Connection
	connectionsLoaded bool
	connectionsBusy   bool
	connectionsError  string
	filterType        string
	filterSource      string
	sortOrder         int
	resourceIDs       map[resourcePage]string
	detail            bool
	detailOffset      int
	lastRefresh       time.Time
	generation        uint64

	search      textinput.Model
	searching   bool
	latencyID   uint64
	latencyStop context.CancelFunc
	latencyCh   <-chan delayResult
	latency     map[string]latencyValue
	latencyDone int
	latencySize int
}

type latencyValue struct {
	delay int
	err   string
}

type delayResult struct {
	name  string
	delay int
	err   error
}

type bootstrapMsg struct {
	version    mihomo.Version
	proxies    mihomo.Proxies
	mode       mihomo.Mode
	versionErr error
	proxiesErr error
	modeErr    error
}

type refreshMsg struct {
	proxies    mihomo.Proxies
	mode       mihomo.Mode
	proxiesErr error
	modeErr    error
	generation uint64
}

type pollTickMsg struct{ serial uint64 }

type selectMsg struct {
	group     string
	name      string
	groupPath []string
	proxies   mihomo.Proxies
	writeErr  error
	readErr   error
}

type setModeMsg struct {
	mode     mihomo.Mode
	current  mihomo.Mode
	writeErr error
	readErr  error
}

type latencyStartedMsg struct {
	id     uint64
	cancel context.CancelFunc
	ch     <-chan delayResult
	total  int
}

type latencyResultMsg struct {
	id     uint64
	result delayResult
	ok     bool
}

type latencyFinishedMsg struct {
	id uint64
}

func New(ctx context.Context, client mihomo.Client, server string, noColor bool, options ...Option) *Model {
	root, cancel := context.WithCancel(ctx)
	search := theme.Input(noColor)
	search.Prompt = "/ "
	search.Placeholder = "Search names"
	search.CharLimit = 128
	search.Width = 32
	command := theme.Input(noColor)
	command.Prompt = ": "
	command.Placeholder = "groups · nodes · providers · status · help"
	command.CharLimit = 128
	model := &Model{
		client:      client,
		ctx:         root,
		cancel:      cancel,
		server:      server,
		state:       stateLoading,
		search:      search,
		noColor:     noColor,
		latency:     make(map[string]latencyValue),
		command:     command,
		resourceIDs: make(map[resourcePage]string),
	}
	for _, option := range options {
		option(model)
	}
	return model
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(bootstrapCmd(m.ctx, m.client), m.loadProviders(), m.loadConnections())
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case connectionsMsg:
		m.connectionsBusy = false
		if msg.generation != m.generation {
			return m, m.loadConnections()
		}
		if msg.err != nil {
			m.connectionsError = messageForError(msg.err)
		} else {
			m.connections, m.connectionsLoaded, m.connectionsError = msg.items, true, ""
		}
		return m, nil
	case providersMsg:
		m.providerBusy = false
		if msg.generation != m.generation {
			return m, nil
		}
		if msg.err != nil {
			m.providerErr = messageForError(msg.err)
		} else {
			m.providers = msg.items
			m.providerErr = ""
		}
		if msg.updated {
			m.operation = ""
			if msg.writeErr != nil {
				m.lastError = "Provider update unconfirmed: " + messageForError(msg.writeErr)
			} else if msg.err != nil {
				m.lastError = "Update sent, but provider readback failed: " + m.providerErr
			} else {
				m.notice = "Provider update completed; status refreshed"
				m.lastError = ""
			}
			m.pollRetries = 0
			return m, m.startRefresh()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.command.Width = max(1, msg.Width-14)
		if m.width > 4 {
			m.search.Width = max(1, m.width-35)
		}
		return m, nil
	case bootstrapMsg:
		if msg.versionErr != nil {
			m.state = stateError
			m.lastError = messageForError(msg.versionErr)
			return m, nil
		}
		m.version = msg.version
		if msg.proxiesErr != nil {
			m.state = stateError
			m.lastError = messageForError(msg.proxiesErr)
			return m, nil
		}
		m.applyProxies(msg.proxies)
		m.mode = msg.mode
		m.modeError = ""
		if msg.modeErr != nil {
			m.modeError = messageForError(msg.modeErr)
		}
		m.state = stateReady
		m.lastError = ""
		m.refreshError = ""
		m.lastRefresh = time.Now()
		return m, m.schedulePoll(refreshInterval)
	case refreshMsg:
		if m.polling && msg.generation != m.pollGeneration {
			return m, nil
		}
		m.polling = false
		if msg.generation != m.generation {
			return m, m.schedulePoll(refreshInterval)
		}
		if msg.proxiesErr == nil {
			m.applyProxies(msg.proxies)
			if m.refreshError != "" {
				m.notice = "Connection restored"
			}
			m.refreshError = ""
			m.pollRetries = 0
			m.lastRefresh = time.Now()
		} else {
			m.refreshError = messageForError(msg.proxiesErr)
			if transientError(msg.proxiesErr) {
				if m.pollRetries < 3 {
					delay := time.Duration(1<<m.pollRetries) * time.Second
					m.pollRetries++
					return m, m.schedulePoll(delay)
				}
			} else {
				m.pollRetries = 3
			}
		}
		if msg.modeErr == nil {
			m.mode = msg.mode
			m.modeError = ""
		} else {
			m.modeError = messageForError(msg.modeErr)
		}
		if m.pollRetries >= 3 && msg.proxiesErr != nil {
			m.notice = "Reconnect paused; press r to retry"
			return m, nil
		}
		return m, m.schedulePoll(refreshInterval)
	case pollTickMsg:
		if msg.serial != m.pollSerial {
			return m, nil
		}
		if m.operation != "" {
			return m, m.schedulePoll(refreshInterval)
		}
		if !m.polling && m.state == stateReady {
			return m, tea.Batch(m.startRefresh(), m.loadProviders())
		}
		return m, nil
	case selectMsg:
		m.operation = ""
		if msg.readErr == nil {
			m.applyProxies(msg.proxies)
		}
		if msg.readErr == nil && msg.proxies.Items[msg.group].Now == msg.name {
			m.selectedGroupPath = append([]string(nil), msg.groupPath...)
			m.notice = "Selection confirmed: " + msg.name
			m.lastError = ""
		} else if msg.writeErr != nil {
			m.lastError = messageForError(msg.writeErr)
		} else if msg.readErr != nil {
			m.lastError = "Selection sent, but readback failed: " + messageForError(msg.readErr)
		} else {
			m.lastError = "Selection not confirmed by Mihomo; current state refreshed"
		}
		return m, nil
	case setModeMsg:
		m.operation = ""
		if msg.readErr == nil {
			m.mode = msg.current
		}
		if msg.readErr == nil && msg.current == msg.mode {
			m.notice = "Mode confirmed: " + strings.ToUpper(string(msg.mode))
			m.lastError = ""
		} else if msg.writeErr != nil {
			m.lastError = messageForError(msg.writeErr)
		} else if msg.readErr != nil {
			m.lastError = "Mode change sent, but readback failed: " + messageForError(msg.readErr)
		} else {
			m.lastError = "Mode change not confirmed by Mihomo"
		}
		return m, nil
	case latencyStartedMsg:
		if msg.id != m.latencyID {
			msg.cancel()
			return m, nil
		}
		m.latencyStop = msg.cancel
		m.latencyCh = msg.ch
		m.latencySize = msg.total
		m.latencyDone = 0
		m.latency = make(map[string]latencyValue, msg.total)
		if msg.total == 0 {
			msg.cancel()
			m.latencyStop = nil
			m.notice = "This group has no direct nodes to test"
			return m, nil
		}
		m.notice = "Testing: 0/" + fmt.Sprint(msg.total)
		return m, nextLatencyCmd(msg.id, msg.ch)
	case latencyResultMsg:
		if msg.id != m.latencyID {
			return m, nil
		}
		if !msg.ok {
			if m.latencyStop != nil {
				m.latencyStop()
				m.latencyStop = nil
			}
			m.latencyCh = nil
			m.notice = fmt.Sprintf("Test complete: %d/%d", m.latencyDone, m.latencySize)
			return m, nil
		}
		m.latencyDone++
		value := latencyValue{delay: msg.result.delay}
		if msg.result.err != nil {
			value.err = latencyErrorLabel(msg.result.err)
		}
		m.latency[msg.result.name] = value
		m.notice = fmt.Sprintf("Testing: %d/%d", m.latencyDone, m.latencySize)
		return m, nextLatencyCmd(msg.id, m.latencyCh)
	case latencyFinishedMsg:
		if msg.id != m.latencyID {
			return m, nil
		}
		if m.latencyStop != nil {
			m.latencyStop()
			m.latencyStop = nil
		}
		m.latencyCh = nil
		m.notice = fmt.Sprintf("Test complete: %d/%d", m.latencyDone, m.latencySize)
		return m, nil
	case tea.KeyMsg:
		return m, m.updateKey(msg)
	}
	return m, nil
}

func (m *Model) updateKey(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlC {
		m.shutdown()
		return tea.Quit
	}
	if m.width > 0 && m.height > 0 && (m.width < 50 || m.height < 12) {
		if msg.String() == "q" {
			m.shutdown()
			return tea.Quit
		}
		return nil
	}
	if handled, cmd := m.resourceKey(msg); handled {
		return cmd
	}
	if m.searching {
		switch msg.Type {
		case tea.KeyEsc:
			m.searching = false
			m.search.Blur()
			m.search.SetValue("")
			m.rowIndex = 0
			return nil
		case tea.KeyEnter:
			m.searching = false
			m.search.Blur()
			m.rowIndex = 0
			return nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.rowIndex = 0
		m.ensureGroupVisible()
		return cmd
	}
	switch msg.Type {
	case tea.KeyEsc:
		if m.help {
			m.help = false
			return nil
		}
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.rowIndex = 0
			return nil
		}
		if len(m.groupPath) > 0 {
			m.leaveGroup()
			m.clearLatency()
			return nil
		}
		m.focusRight = false
		m.detail = false
		return nil
	case tea.KeyUp:
		m.moveSelection(-1)
	case tea.KeyDown:
		m.moveSelection(1)
	case tea.KeyLeft:
		m.focusRight = false
	case tea.KeyRight:
		m.focusRight = true
	case tea.KeyTab:
		m.focusRight = !m.focusRight
	case tea.KeyShiftTab:
		m.focusRight = !m.focusRight
	case tea.KeyEnter:
		return m.activateSelection()
	case tea.KeySpace:
		return m.startLatency()
	case tea.KeyRunes:
		return m.updateRuneKey(string(msg.Runes))
	}
	return nil
}

func (m *Model) updateRuneKey(key string) tea.Cmd {
	switch key {
	case "q":
		m.shutdown()
		return tea.Quit
	case "?":
		m.help = !m.help
		m.detailOffset = 0
	case "/":
		m.searching = true
		m.search.Focus()
	case "h":
		m.focusRight = false
	case "l":
		m.focusRight = true
	case "j":
		m.moveSelection(1)
	case "k":
		m.moveSelection(-1)
	case "m":
		return m.changeMode()
	case "r":
		if m.state == stateError {
			m.state = stateLoading
			return bootstrapCmd(m.ctx, m.client)
		}
		if m.pollRetries >= 3 && !m.polling {
			m.pollRetries = 0
			m.notice = "Reconnecting"
			return m.startRefresh()
		}
	}
	return nil
}

func (m *Model) moveSelection(delta int) {
	if m.focusRight {
		m.rowIndex = clamp(m.rowIndex+delta, 0, max(0, len(m.visibleRows())-1))
		return
	}
	groups := m.visibleGroups()
	if len(groups) == 0 {
		return
	}
	index := 0
	for i, name := range groups {
		if name == m.currentGroup() {
			index = i
			break
		}
	}
	name := groups[clamp(index+delta, 0, len(groups)-1)]
	for i, group := range m.groups {
		if group == name {
			m.groupIndex = i
			break
		}
	}
	m.groupPath = nil
	m.parentRows = nil
	m.rowIndex = 0
	m.clearLatency()
}

func (m *Model) activateSelection() tea.Cmd {
	if !m.focusRight {
		m.focusRight = true
		m.search.SetValue("")
		return nil
	}
	rows := m.visibleRows()
	if len(rows) == 0 || m.operation != "" {
		return nil
	}
	row := rows[clamp(m.rowIndex, 0, len(rows)-1)]
	item, ok := m.proxies.Items[row]
	if ok && isGroupType(item.Type) {
		if row == m.rootGroup() || row == m.currentGroup() || contains(m.groupPath, row) {
			m.lastError = "Group cycle detected; cannot enter this group"
			return nil
		}
		m.parentRows = append(m.parentRows, m.rowIndex)
		m.groupPath = append(m.groupPath, row)
		m.rowIndex = 0
		m.search.SetValue("")
		m.clearLatency()
		return nil
	}
	group, ok := m.proxies.Items[m.currentGroup()]
	if !ok || group.Type != "Selector" {
		m.notice = "Automatic group; manual selection is unavailable"
		return nil
	}
	if !contains(group.All, row) {
		m.lastError = "Member no longer belongs to this group; press r to refresh"
		return nil
	}
	m.cancelLatency()
	m.operation = "Switching node"
	m.notice = ""
	m.lastError = ""
	return selectCmd(m.ctx, m.client, m.currentGroup(), row, m.currentGroupPath())
}

func (m *Model) changeMode() tea.Cmd {
	if m.operation != "" {
		return nil
	}
	next := mihomo.ModeRule
	switch m.mode {
	case mihomo.ModeRule:
		next = mihomo.ModeGlobal
	case mihomo.ModeGlobal:
		next = mihomo.ModeDirect
	}
	m.cancelLatency()
	m.generation++
	m.operation = "Changing mode"
	m.notice = ""
	m.lastError = ""
	return setModeCmd(m.ctx, m.client, next)
}

func (m *Model) startLatency() tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	groupName := m.currentGroup()
	group, ok := m.proxies.Items[groupName]
	if !ok {
		m.notice = "Select a group first"
		return nil
	}
	names := make([]string, 0, len(group.All))
	for _, name := range group.All {
		if child, found := m.proxies.Items[name]; found && isGroupType(child.Type) {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		m.notice = "This group has no direct nodes to test"
		return nil
	}
	m.cancelLatency()
	m.latencyID++
	m.notice = "Starting latency test"
	m.lastError = ""
	return startLatencyCmd(m.ctx, m.client, m.latencyID, names)
}

func (m *Model) cancelLatency() {
	if m.latencyStop != nil {
		m.latencyStop()
		m.latencyStop = nil
	}
	m.latencyCh = nil
	m.latencyID++
	m.latency = make(map[string]latencyValue)
	m.latencyDone = 0
	m.latencySize = 0
}

func (m *Model) clearLatency() {
	m.cancelLatency()
}

func (m *Model) shutdown() {
	m.cancelLatency()
	m.cancel()
}

func (m *Model) applyProxies(proxies mihomo.Proxies) {
	oldRow := ""
	if rows := m.visibleRows(); m.rowIndex >= 0 && m.rowIndex < len(rows) {
		oldRow = rows[m.rowIndex]
	}
	old := ""
	if m.groupIndex >= 0 && m.groupIndex < len(m.groups) {
		old = m.groups[m.groupIndex]
	}
	m.proxies = proxies
	m.groups = m.groups[:0]
	for name, proxy := range proxies.Items {
		if isGroupType(proxy.Type) {
			m.groups = append(m.groups, name)
		}
	}
	sort.Strings(m.groups)
	if len(m.groups) == 0 {
		m.groupIndex = 0
		m.groupPath = nil
		m.parentRows = nil
		return
	}
	m.groupIndex = 0
	for index, name := range m.groups {
		if name == old {
			m.groupIndex = index
			break
		}
	}
	if len(m.selectedGroupPath) == 0 {
		m.selectedGroupPath = []string{m.rootGroup()}
	}
	if len(m.groupPath) > 0 {
		for _, name := range m.groupPath {
			proxy, exists := proxies.Items[name]
			if !exists || !isGroupType(proxy.Type) {
				m.groupPath = nil
				m.parentRows = nil
				m.rowIndex = 0
				break
			}
		}
	}
	m.rowIndex = clamp(m.rowIndex, 0, max(0, len(m.visibleRows())-1))
	for index, name := range m.visibleRows() {
		if name == oldRow {
			m.rowIndex = index
			break
		}
	}
}

func (m *Model) currentGroup() string {
	if len(m.groupPath) > 0 {
		return m.groupPath[len(m.groupPath)-1]
	}
	if m.groupIndex < 0 || m.groupIndex >= len(m.groups) {
		return ""
	}
	return m.groups[m.groupIndex]
}

func (m *Model) currentGroupPath() []string {
	return append([]string{m.rootGroup()}, m.groupPath...)
}

func (m *Model) visibleRows() []string {
	group, ok := m.proxies.Items[m.currentGroup()]
	if !ok {
		return nil
	}
	query := strings.ToLower(m.search.Value())
	rows := make([]string, 0, len(group.All))
	for _, name := range group.All {
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			rows = append(rows, name)
		}
	}
	return rows
}

func bootstrapCmd(ctx context.Context, client mihomo.Client) tea.Cmd {
	return func() tea.Msg {
		version, err := client.Version(ctx)
		if err != nil {
			return bootstrapMsg{versionErr: err}
		}
		proxies, mode, proxiesErr, modeErr := readState(ctx, client)
		return bootstrapMsg{
			version:    version,
			proxies:    proxies,
			mode:       mode,
			proxiesErr: proxiesErr,
			modeErr:    modeErr,
		}
	}
}

func refreshCmd(ctx context.Context, client mihomo.Client) tea.Cmd {
	return refreshGenerationCmd(ctx, client, 0)
}

func refreshGenerationCmd(ctx context.Context, client mihomo.Client, generation uint64) tea.Cmd {
	return func() tea.Msg {
		proxies, mode, proxiesErr, modeErr := readState(ctx, client)
		return refreshMsg{
			proxies:    proxies,
			mode:       mode,
			proxiesErr: proxiesErr,
			modeErr:    modeErr,
			generation: generation,
		}
	}
}

func readState(ctx context.Context, client mihomo.Client) (mihomo.Proxies, mihomo.Mode, error, error) {
	var (
		proxies    mihomo.Proxies
		mode       mihomo.Mode
		proxiesErr error
		modeErr    error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		proxies, proxiesErr = client.Proxies(ctx)
	}()
	go func() {
		defer wg.Done()
		mode, modeErr = client.Mode(ctx)
	}()
	wg.Wait()
	return proxies, mode, proxiesErr, modeErr
}

func selectCmd(ctx context.Context, client mihomo.Client, group, name string, groupPath []string) tea.Cmd {
	return func() tea.Msg {
		writeErr := client.Select(ctx, group, name)
		proxies, readErr := client.Proxies(ctx)
		return selectMsg{
			group:     group,
			name:      name,
			groupPath: append([]string(nil), groupPath...),
			proxies:   proxies,
			writeErr:  writeErr,
			readErr:   readErr,
		}
	}
}

func setModeCmd(ctx context.Context, client mihomo.Client, mode mihomo.Mode) tea.Cmd {
	return func() tea.Msg {
		writeErr := client.SetMode(ctx, mode)
		current, readErr := client.Mode(ctx)
		return setModeMsg{mode: mode, current: current, writeErr: writeErr, readErr: readErr}
	}
}

func startLatencyCmd(ctx context.Context, client mihomo.Client, id uint64, names []string) tea.Cmd {
	targets := make([]delayTarget, len(names))
	for i, name := range names {
		targets[i] = delayTarget{key: name, name: name}
	}
	return startTargetLatencyCmd(ctx, client, id, targets)
}

type delayTarget struct{ key, name, provider string }

func startTargetLatencyCmd(ctx context.Context, client mihomo.Client, id uint64, targets []delayTarget) tea.Cmd {
	return func() tea.Msg {
		taskCtx, cancel := context.WithCancel(ctx)
		results := make(chan delayResult, delayWorkers)
		jobs := make(chan delayTarget)
		var workers sync.WaitGroup
		workers.Add(min(delayWorkers, len(targets)))
		for range min(delayWorkers, len(targets)) {
			go func() {
				defer workers.Done()
				for target := range jobs {
					requestCtx, stop := context.WithTimeout(taskCtx, 5*time.Second)
					var delay int
					var err error
					if target.provider != "" {
						if providerClient, ok := client.(mihomo.ProviderDelayClient); ok {
							delay, err = providerClient.DelayProvider(requestCtx, target.provider, target.name, delayTestURL, delayTimeoutMS)
						} else {
							err = &mihomo.APIError{Kind: mihomo.ErrorNotFound, Op: "provider delay"}
						}
					} else {
						delay, err = client.Delay(requestCtx, target.name, delayTestURL, delayTimeoutMS)
					}
					stop()
					select {
					case results <- delayResult{name: target.key, delay: delay, err: err}:
					case <-taskCtx.Done():
						return
					}
				}
			}()
		}
		go func() {
			defer close(jobs)
			for _, target := range targets {
				select {
				case jobs <- target:
				case <-taskCtx.Done():
					return
				}
			}
		}()
		go func() {
			workers.Wait()
			close(results)
		}()
		return latencyStartedMsg{id: id, cancel: cancel, ch: results, total: len(targets)}
	}
}

func nextLatencyCmd(id uint64, ch <-chan delayResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-ch
		if !ok {
			return latencyFinishedMsg{id: id}
		}
		return latencyResultMsg{id: id, result: result, ok: true}
	}
}

func (m *Model) schedulePoll(delay time.Duration) tea.Cmd {
	m.pollSerial++
	serial := m.pollSerial
	return tea.Tick(delay, func(time.Time) tea.Msg { return pollTickMsg{serial: serial} })
}

func (m *Model) startRefresh() tea.Cmd {
	if m.polling && m.pollGeneration == m.generation {
		return nil
	}
	m.polling = true
	m.pollGeneration = m.generation
	return tea.Batch(refreshGenerationCmd(m.ctx, m.client, m.generation), m.loadConnections())
}

func messageForError(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *mihomo.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case mihomo.ErrorAuth:
			return "Authentication failed; check the Mihomo secret"
		case mihomo.ErrorTimeout:
			return "Request timed out; check that Mihomo is reachable"
		case mihomo.ErrorNotFound:
			return "API unsupported or resource not found"
		case mihomo.ErrorDecode:
			return "Mihomo returned an unrecognized response"
		}
	}
	return err.Error()
}

func transientError(err error) bool {
	var apiErr *mihomo.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Kind == mihomo.ErrorNetwork ||
		apiErr.Kind == mihomo.ErrorTimeout ||
		(apiErr.Kind == mihomo.ErrorAPI && apiErr.Status >= 500)
}

func latencyErrorLabel(err error) string {
	var apiErr *mihomo.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case mihomo.ErrorTimeout:
			return "Timeout"
		case mihomo.ErrorNotFound:
			return "No API"
		case mihomo.ErrorAuth:
			return "Auth error"
		}
	}
	return "Failed"
}

func (m *Model) leaveGroup() {
	m.groupPath = m.groupPath[:len(m.groupPath)-1]
	m.rowIndex = 0
	if len(m.parentRows) > 0 {
		m.rowIndex = m.parentRows[len(m.parentRows)-1]
		m.parentRows = m.parentRows[:len(m.parentRows)-1]
	}
}

func isGroupType(value string) bool {
	switch strings.ToLower(value) {
	case "selector", "urltest", "fallback", "loadbalance", "relay", "smart":
		return true
	default:
		return false
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
