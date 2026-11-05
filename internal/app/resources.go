package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
	"miocli/internal/theme"
)

type resourcePage uint8

const (
	pageGroups resourcePage = iota
	pageNodes
	pageProviders
	pageStatus
)

type Runtime struct {
	ConfigPath string
}

type Option func(*Model)

func WithRuntime(runtime Runtime) Option {
	return func(m *Model) { m.runtime = runtime }
}

type resourceRow struct {
	id      string
	cells   []string
	details []string
}

type providersMsg struct {
	items      map[string]mihomo.Provider
	err        error
	writeErr   error
	updated    bool
	generation uint64
}

func (m *Model) loadProviders() tea.Cmd {
	if m.providerBusy {
		return nil
	}
	client, ok := m.client.(mihomo.ProviderClient)
	if !ok {
		m.providerErr = "Provider API unavailable for this client"
		return nil
	}
	m.providerBusy = true
	ctx, generation := m.ctx, m.generation
	return func() tea.Msg {
		items, err := client.Providers(ctx)
		return providersMsg{items: items, err: err, generation: generation}
	}
}

func (m *Model) updateProvider() tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	rows := m.resourceRows()
	if len(rows) == 0 {
		m.notice = "No provider to update"
		return nil
	}
	client, ok := m.client.(mihomo.ProviderClient)
	if !ok {
		m.lastError = "Provider API unavailable for this client"
		return nil
	}
	name := rows[m.resourceIndex(rows)].id
	m.cancelLatency()
	m.generation++
	m.operation = "Updating provider"
	m.lastError = ""
	m.notice = ""
	ctx, generation := m.ctx, m.generation
	return func() tea.Msg {
		writeErr := client.UpdateProvider(ctx, name)
		items, err := client.Providers(ctx)
		return providersMsg{items: items, err: err, writeErr: writeErr, updated: true, generation: generation}
	}
}

func (m *Model) changePage(page resourcePage) tea.Cmd {
	m.page = page
	m.detail = false
	m.detailOffset = 0
	m.focusRight = false
	m.groupPath = nil
	m.parentRows = nil
	m.search.SetValue("")
	m.searching = false
	m.search.Blur()
	m.filterType, m.filterSource = "", ""
	if page == pageProviders || page == pageNodes {
		return m.loadProviders()
	}
	return nil
}

func (m *Model) resourceKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if m.help {
		if msg.String() == "?" || msg.Type == tea.KeyEsc {
			m.help = false
		} else if msg.String() == "q" {
			m.shutdown()
			return true, tea.Quit
		} else {
			m.scrollDetails(msg.String())
		}
		return true, nil
	}
	if m.commanding {
		switch msg.Type {
		case tea.KeyEsc:
			m.commanding = false
			m.command.Blur()
			return true, nil
		case tea.KeyEnter:
			value := m.command.Value()
			m.command.SetValue("")
			m.commanding = false
			m.command.Blur()
			return true, m.executeCommand(value)
		}
		var cmd tea.Cmd
		m.command, cmd = m.command.Update(msg)
		return true, cmd
	}
	if m.searching {
		return false, nil
	}
	if m.state != stateReady {
		switch msg.String() {
		case "q", "r", "?":
			return false, nil
		}
		if m.state == stateError {
			m.scrollDetails(msg.String())
		}
		return true, nil
	}
	key := msg.String()
	switch key {
	case "c":
		if m.latencyStop != nil {
			m.latencyStop()
			m.latencyStop = nil
		}
		m.latencyCh = nil
		m.latencyID++
		m.notice = fmt.Sprintf("Test cancelled: %d/%d complete; results kept", m.latencyDone, m.latencySize)
		return true, nil
	case ":":
		m.commanding = true
		m.command.Focus()
		return true, nil
	case "1":
		return true, m.changePage(pageGroups)
	case "2":
		return true, m.changePage(pageNodes)
	case "3":
		return true, m.changePage(pageProviders)
	case "4":
		return true, m.changePage(pageStatus)
	case "r":
		if m.state == stateReady && !m.polling && m.operation == "" {
			m.pollRetries = 0
			return true, tea.Batch(m.startRefresh(), m.loadProviders())
		}
	case "s":
		if m.page == pageNodes {
			m.rememberResource()
			m.sortOrder = (m.sortOrder + 1) % 3
			return true, nil
		}
	case "t", "f":
		if m.page == pageNodes {
			m.cycleFilter(key == "t")
			return true, nil
		}
	}
	if m.page == pageGroups {
		if key == "v" {
			m.detail = !m.detail
			m.detailOffset = 0
			return true, nil
		}
		if m.detail {
			switch key {
			case "esc", "enter", "tab", "shift+tab", "left", "h":
				m.detail = false
			case "q", "?":
				return false, nil
			default:
				m.scrollDetails(key)
			}
			return true, nil
		}
		switch key {
		case "home", "g":
			if m.focusRight {
				m.rowIndex = 0
			} else {
				m.moveSelection(-len(m.groups))
			}
			return true, nil
		case "end", "G":
			if m.focusRight {
				m.rowIndex = max(0, len(m.visibleRows())-1)
			} else {
				m.moveSelection(len(m.groups))
			}
			return true, nil
		case "pgup", "pgdown":
			delta := m.listHeight()
			if key == "pgup" {
				delta = -delta
			}
			m.moveSelection(delta)
			return true, nil
		case "right", "l":
			if !m.focusRight {
				if len(m.visibleGroups()) == 0 {
					m.notice = "No matching groups"
					return true, nil
				}
				m.focusRight = true
				m.search.SetValue("")
				return true, nil
			} else {
				rows := m.visibleRows()
				if len(rows) > 0 {
					if proxy := m.proxies.Items[rows[clamp(m.rowIndex, 0, len(rows)-1)]]; isGroupType(proxy.Type) {
						return true, m.activateSelection()
					}
				}
			}
		case "left", "h":
			if len(m.groupPath) > 0 {
				m.leaveGroup()
				m.search.SetValue("")
			} else {
				m.focusRight = false
			}
			return true, nil
		case "tab", "shift+tab":
			if !m.focusRight && len(m.visibleGroups()) == 0 {
				m.notice = "No matching groups"
				return true, nil
			}
			m.focusRight = !m.focusRight
			if !m.focusRight {
				for len(m.groupPath) > 0 {
					m.leaveGroup()
				}
			}
			m.search.SetValue("")
			return true, nil
		case " ":
			if !m.focusRight && len(m.visibleGroups()) == 0 {
				m.notice = "No matching groups"
				return true, nil
			}
		case "enter":
			if m.focusRight {
				return true, m.selectMember()
			}
			if len(m.visibleGroups()) == 0 {
				m.notice = "No matching groups"
				return true, nil
			}
		}
		return false, nil
	}
	if m.page == pageStatus {
		switch key {
		case "q", "?", "m":
			return false, nil
		}
		m.scrollDetails(key)
		return true, nil
	}
	if m.detail && (key == "pgup" || key == "pgdown" || key == "up" || key == "down" || key == "j" || key == "k" || key == "home" || key == "end" || key == "g" || key == "G") {
		m.scrollDetails(key)
		return true, nil
	}
	switch key {
	case "up", "k":
		m.moveResource(-1)
	case "down", "j":
		m.moveResource(1)
	case "pgup":
		m.moveResource(-m.listHeight())
	case "pgdown":
		m.moveResource(m.listHeight())
	case "home", "g":
		m.moveResource(-int(^uint(0) >> 1))
	case "end", "G":
		m.moveResource(int(^uint(0) >> 1))
	case "enter", "right", "l", "tab", "shift+tab":
		m.detail = !m.detail
		m.detailOffset = 0
	case "esc", "left", "h":
		if m.detail {
			m.detail = false
		} else {
			m.search.SetValue("")
			m.filterType, m.filterSource = "", ""
		}
	case "u":
		if m.page == pageProviders {
			return true, m.updateProvider()
		}
	case " ":
		if m.page == pageNodes {
			return true, m.testNodes(false)
		}
	case "T":
		if m.page == pageNodes {
			return true, m.testNodes(true)
		}
	default:
		switch key {
		case "q", "?", "/", "m":
			return false, nil
		}
	}
	return true, nil
}

func (m *Model) executeCommand(value string) tea.Cmd {
	parts := strings.Fields(strings.ToLower(value))
	if len(parts) == 0 {
		m.lastError = "Enter a command; press ? for help"
		return nil
	}
	if len(parts) == 1 {
		switch parts[0] {
		case "groups":
			return m.changePage(pageGroups)
		case "providers":
			return m.changePage(pageProviders)
		case "nodes":
			return m.changePage(pageNodes)
		case "status":
			return m.changePage(pageStatus)
		case "help":
			m.help = true
			return nil
		case "quit", "q":
			m.shutdown()
			return tea.Quit
		case "refresh":
			_, cmd := m.resourceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
			return cmd
		}
	}
	if len(parts) == 2 && parts[0] == "mode" {
		mode := mihomo.Mode(parts[1])
		if mode == mihomo.ModeRule || mode == mihomo.ModeGlobal || mode == mihomo.ModeDirect {
			if m.operation != "" {
				m.lastError = "Wait for the current operation to finish"
				return nil
			}
			m.cancelLatency()
			m.generation++
			m.operation = "Changing mode"
			return setModeCmd(m.ctx, m.client, mode)
		}
	}
	m.lastError = "Unknown command or arguments; press ? for available commands"
	return nil
}

func (m *Model) selectMember() tea.Cmd {
	rows := m.visibleRows()
	if len(rows) == 0 || m.operation != "" {
		return nil
	}
	name := rows[clamp(m.rowIndex, 0, len(rows)-1)]
	group := m.proxies.Items[m.currentGroup()]
	if group.Type != "Selector" {
		m.notice = "Automatic group; manual selection is unavailable"
		return nil
	}
	if !contains(group.All, name) {
		m.lastError = "Group members changed; press r to refresh"
		return nil
	}
	m.cancelLatency()
	m.generation++
	m.operation = "Switching node"
	m.lastError = ""
	return selectCmd(m.ctx, m.client, m.currentGroup(), name, m.currentGroupPath())
}

func (m *Model) resourceRows() []resourceRow {
	var rows []resourceRow
	switch m.page {
	case pageProviders:
		for name, p := range m.providers {
			info := []string{"Name: " + name, "Type: " + p.Type, "Source: " + p.VehicleType, "Updated: " + unknown(p.UpdatedAt)}
			if s := p.SubscriptionInfo; s != nil {
				if s.Upload != nil && s.Download != nil {
					info = append(info, fmt.Sprintf("Used: upload %d / download %d bytes", *s.Upload, *s.Download))
				}
				if s.Total != nil {
					info = append(info, fmt.Sprintf("Total: %d bytes", *s.Total))
				}
				if s.Expire != nil && *s.Expire > 0 {
					info = append(info, "Expires: "+time.Unix(*s.Expire, 0).Format("2006-01-02"))
				} else {
					info = append(info, "Expires: unknown")
				}
			}
			info = append(info, "Members:")
			for _, proxy := range p.Proxies {
				info = append(info, "  "+proxy.Name+" ["+proxy.Type+"]")
			}
			rows = append(rows, resourceRow{name, []string{name, p.VehicleType, fmt.Sprint(len(p.Proxies)), unknown(p.UpdatedAt)}, info})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	case pageNodes:
		memberships := make(map[string][]string)
		for _, name := range m.groups {
			group := m.proxies.Items[name]
			for _, member := range group.All {
				memberships[member] = append(memberships[member], "  "+name+" ["+group.Type+"]")
			}
		}
		for _, node := range m.nodes() {
			if m.filterType != "" && node.proxy.Type != m.filterType || m.filterSource != "" && !contains(node.sources, m.filterSource) {
				continue
			}
			details := []string{"Name: " + node.proxy.Name, "Protocol: " + node.proxy.Type, "Source: " + strings.Join(node.sources, ", "), "Latency: " + m.delayLabel(node.id), "Groups:"}
			details = append(details, memberships[node.proxy.Name]...)
			details = append(details, "Choose a Selector in Groups to switch nodes")
			rows = append(rows, resourceRow{node.id, []string{node.proxy.Name, node.proxy.Type, strings.Join(node.sources, ", "), m.delayLabel(node.id)}, details})
		}
		if m.sortOrder != 0 {
			sort.SliceStable(rows, func(i, j int) bool {
				if m.sortOrder == 2 {
					a, aok := m.latency[rows[i].id]
					b, bok := m.latency[rows[j].id]
					av, bv := int(^uint(0)>>1), int(^uint(0)>>1)
					if aok && a.err == "" {
						av = a.delay
					}
					if bok && b.err == "" {
						bv = b.delay
					}
					if av != bv {
						return av < bv
					}
				}
				return rows[i].id < rows[j].id
			})
		}
	}
	query := strings.ToLower(m.search.Value())
	if query != "" {
		filtered := rows[:0]
		for _, row := range rows {
			if strings.Contains(strings.ToLower(row.cells[0]), query) {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	return rows
}

type nodeRow struct {
	id      string
	proxy   mihomo.Proxy
	sources []string
}

func (m *Model) nodes() []nodeRow {
	nodes := make(map[string]nodeRow)
	order := make([]string, 0)
	add := func(p mihomo.Proxy, source string) {
		if p.Name == "" || isGroupType(p.Type) {
			return
		}
		key := p.Name
		if source != "" {
			if _, global := m.proxies.Items[p.Name]; !global {
				key = source + "\x00" + p.Name
			}
		}
		n, exists := nodes[key]
		if !exists {
			n.id = key
			n.proxy = p
			order = append(order, key)
		}
		if source != "" && !contains(n.sources, source) {
			n.sources = append(n.sources, source)
		}
		nodes[key] = n
	}
	for _, group := range m.groups {
		for _, name := range m.proxies.Items[group].All {
			if p, ok := m.proxies.Items[name]; ok {
				add(p, "")
			}
		}
	}
	names := make([]string, 0, len(m.proxies.Items))
	for name := range m.proxies.Items {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		add(m.proxies.Items[name], "")
	}
	names = names[:0]
	for name := range m.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, p := range m.providers[name].Proxies {
			add(p, name)
		}
	}
	result := make([]nodeRow, 0, len(order))
	for _, name := range order {
		n := nodes[name]
		if len(n.sources) == 0 {
			source := "Config"
			if name == "DIRECT" || name == "REJECT" || name == "REJECT-DROP" || name == "PASS" {
				source = "Built-in"
			}
			n.sources = []string{source}
		}
		result = append(result, n)
	}
	return result
}

func (m *Model) resourceIndex(rows []resourceRow) int {
	for i, row := range rows {
		if row.id == m.resourceIDs[m.page] {
			return i
		}
	}
	return 0
}

func (m *Model) moveResource(delta int) {
	rows := m.resourceRows()
	if len(rows) == 0 {
		return
	}
	index := m.resourceIndex(rows)
	if delta > len(rows) {
		index = len(rows) - 1
	} else if delta < -len(rows) {
		index = 0
	} else {
		index = clamp(index+delta, 0, len(rows)-1)
	}
	m.resourceIDs[m.page] = rows[index].id
}

func (m *Model) cycleFilter(protocol bool) {
	m.rememberResource()
	values := []string{""}
	for _, node := range m.nodes() {
		candidates := node.sources
		if protocol {
			candidates = []string{node.proxy.Type}
		}
		for _, value := range candidates {
			if !contains(values, value) {
				values = append(values, value)
			}
		}
	}
	sort.Strings(values)
	current := &m.filterSource
	if protocol {
		current = &m.filterType
	}
	for i, value := range values {
		if value == *current {
			*current = values[(i+1)%len(values)]
			return
		}
	}
	*current = ""
}

func (m *Model) testNodes(all bool) tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	rows := m.resourceRows()
	if len(rows) == 0 {
		m.notice = "No nodes to test"
		return nil
	}
	if !all {
		rows = rows[m.resourceIndex(rows) : m.resourceIndex(rows)+1]
	}
	targets := make([]delayTarget, 0, len(rows))
	nodes := m.nodes()
	for _, row := range rows {
		for _, node := range nodes {
			if row.id != node.id {
				continue
			}
			target := delayTarget{key: node.id, name: node.proxy.Name}
			if _, global := m.proxies.Items[node.proxy.Name]; !global && len(node.sources) == 1 {
				target.provider = node.sources[0]
			}
			targets = append(targets, target)
			break
		}
	}
	if len(targets) == 0 {
		m.lastError = "No nodes to test"
		return nil
	}
	m.cancelLatency()
	m.latencyID++
	return startTargetLatencyCmd(m.ctx, m.client, m.latencyID, targets)
}

func (m *Model) rememberResource() {
	rows := m.resourceRows()
	if len(rows) > 0 {
		m.resourceIDs[m.page] = rows[m.resourceIndex(rows)].id
	}
}

func (m *Model) scrollDetails(key string) {
	width, _ := m.screenSize()
	lines := m.helpLines()
	height := m.contentHeight()
	if m.page == pageProviders {
		height -= len(m.providerHeader(width, theme.New(m.noColor)))
	}
	if m.page == pageStatus && !m.help {
		lines = m.statusLines()
	}
	if m.state == stateError && !m.help {
		lines = m.failureLines()
	}
	if m.detail && !m.help && m.page != pageStatus {
		rows := m.resourceRows()
		index := m.resourceIndex(rows)
		if m.page == pageGroups {
			rows, index = m.groupRows(m.focusRight)
		}
		if len(rows) > 0 {
			lines = rows[index].details
		}
		if width >= 100 && m.page != pageGroups {
			width = max(32, width/3)
		}
		height--
	}
	total := len(wrappedLines(lines, width))
	delta := 0
	switch key {
	case "pgdown":
		delta = max(1, height)
	case "pgup":
		delta = -max(1, height)
	case "j", "down":
		delta = 1
	case "k", "up":
		delta = -1
	case "home", "g":
		m.detailOffset = 0
		return
	case "end", "G":
		m.detailOffset = max(0, total-height)
		return
	}
	m.detailOffset = clamp(m.detailOffset+delta, 0, max(0, total-height))
}

func (m *Model) visibleGroups() []string {
	query := ""
	if !m.focusRight {
		query = strings.ToLower(m.search.Value())
	}
	groups := make([]string, 0, len(m.groups))
	for _, name := range m.groups {
		if query == "" || strings.Contains(strings.ToLower(name), query) {
			groups = append(groups, name)
		}
	}
	return groups
}

func (m *Model) ensureGroupVisible() {
	if m.page != pageGroups || m.focusRight {
		return
	}
	groups := m.visibleGroups()
	if len(groups) == 0 || contains(groups, m.currentGroup()) {
		return
	}
	for i, name := range m.groups {
		if name == groups[0] {
			m.groupIndex = i
			break
		}
	}
}

func (m *Model) delayLabel(name string) string {
	if value, ok := m.latency[name]; ok {
		if value.err != "" {
			return value.err
		}
		return fmt.Sprintf("%d ms", value.delay)
	}
	return "-"
}

func unknown(value string) string {
	if value == "" || strings.HasPrefix(value, "0001-") {
		return "Unknown"
	}
	return value
}

func safeText(value string) string {
	return theme.SafeText(value)
}
