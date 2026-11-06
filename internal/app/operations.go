package app

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
)

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
		m.cancelLatency()
		return nil
	}
	return m.selectMember()
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
	return m.setMode(next)
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

func (m *Model) updateOperation(message tea.Msg) tea.Cmd {
	switch msg := message.(type) {
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
		return nil
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
		return nil

	}
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
	m.beginOperation("Switching node")
	return selectCmd(m.ctx, m.client, m.currentGroup(), name, m.currentGroupPath())
}

// beginOperation invalidates reads started before a write and resets transient UI state.
func (m *Model) beginOperation(label string) {
	m.cancelLatency()
	m.generation++
	m.operation = label
	m.notice, m.lastError = "", ""
}

func (m *Model) setMode(mode mihomo.Mode) tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	m.beginOperation("Changing mode")
	return setModeCmd(m.ctx, m.client, mode)
}
