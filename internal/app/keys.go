package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
	"miocli/internal/theme"
)

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
			m.cancelLatency()
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
	m.cancelLatency()
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
			return m.setMode(mode)
		}
	}
	m.lastError = "Unknown command or arguments; press ? for available commands"
	return nil
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
