package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"miocli/internal/mihomo"
)

type connectionsMsg struct {
	items      []mihomo.Connection
	err        error
	generation uint64
}

func (m *Model) loadConnections() tea.Cmd {
	if m.connectionsBusy {
		return nil
	}
	client, ok := m.client.(mihomo.ConnectionClient)
	if !ok {
		m.connectionsError = "Connections API unavailable for this client"
		return nil
	}
	m.connectionsBusy = true
	ctx, generation := m.ctx, m.generation
	return func() tea.Msg {
		items, err := client.Connections(ctx)
		return connectionsMsg{items: items, err: err, generation: generation}
	}
}

func (m *Model) connectionRoutes() []string {
	routes := make(map[string]bool)
	for _, connection := range m.connections {
		path := make([]string, len(connection.Chains))
		// Mihomo reports chains from the final node back to the outer group.
		for i, name := range connection.Chains {
			path[len(path)-1-i] = safeText(name)
		}
		route := strings.Join(path, " → ")
		if route == "" {
			route = "Unknown route"
		}
		routes[route] = true
	}
	result := make([]string, 0, len(routes))
	for route := range routes {
		result = append(result, route)
	}
	sort.Strings(result)
	return result
}

func (m *Model) selectionLines() []string {
	return []string{m.selectedNodeLine(), m.cursorLine()}
}

func (m *Model) selectedNodeLine() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	return m.selectedNodeLineAtWidth(width)
}

func (m *Model) selectedNodeLineAtWidth(width int) string {
	const prefix = "Selected: "
	if m.state != stateReady {
		state := "unavailable"
		if m.state == stateLoading {
			state = "loading"
		}
		return prefix + state
	}
	if len(m.selectedGroupPath) == 0 {
		return prefix + "unavailable"
	}
	groups := make([]string, 0, len(m.selectedGroupPath))
	seen := make(map[string]bool, len(m.selectedGroupPath))
	for _, name := range m.selectedGroupPath {
		proxy, ok := m.proxies.Items[name]
		if !ok || !isGroupType(proxy.Type) {
			return prefix + "selection unavailable"
		}
		if seen[name] {
			return prefix + "selection unavailable"
		}
		seen[name] = true
		groups = append(groups, safeText(name))
	}
	path := []string{strings.Join(groups, " / ")}
	for current := m.selectedGroupPath[len(m.selectedGroupPath)-1]; ; {
		proxy, ok := m.proxies.Items[current]
		if !ok || proxy.Now == "" {
			path = append(path, "no selected member")
			break
		}
		selected := safeText(proxy.Now)
		path = append(path, selected)
		child, ok := m.proxies.Items[proxy.Now]
		if !ok || !isGroupType(child.Type) {
			break
		}
		if seen[proxy.Now] {
			path = append(path, "cycle")
			break
		}
		seen[proxy.Now] = true
		current = proxy.Now
	}
	if width <= 0 {
		width = 80
	}
	return ansi.Truncate(prefix+strings.Join(path, " → "), width, "…")
}

func (m *Model) connectionLine() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	return m.connectionLineAtWidth(width)
}

func (m *Model) connectionLineAtWidth(width int) string {
	const prefix = "Connections: "
	if m.connectionsError != "" {
		return prefix + "read failed (" + safeText(m.connectionsError) + ")"
	}
	if !m.connectionsLoaded {
		return prefix + "loading"
	}
	routes := m.connectionRoutes()
	if len(routes) == 0 {
		return prefix + "no active connections"
	}
	if width <= 0 {
		width = 80
	}
	for count := 1; count <= len(routes); count++ {
		suffix := ""
		if count < len(routes) {
			suffix = fmt.Sprintf(" · +%d routes", len(routes)-count)
		}
		content := strings.Join(routes[:count], " · ")
		available := max(1, width-lipgloss.Width(prefix+suffix))
		if count == 1 && lipgloss.Width(content) > available {
			return prefix + ansi.Truncate(content, available, "…") + suffix
		}
		if count < len(routes) && lipgloss.Width(content+" · "+routes[count]) > available {
			return prefix + content + suffix
		}
		if count == len(routes) {
			return prefix + content
		}
	}
	return prefix + "unknown route"
}

func (m *Model) cursorLine() string {
	const prefix = "Cursor: "
	if m.state != stateReady {
		return prefix + "no selection"
	}
	switch m.page {
	case pageGroups:
		group := m.currentGroup()
		if group == "" || !m.focusRight && len(m.visibleGroups()) == 0 {
			return prefix + "Groups / no matches"
		}
		position := prefix + "Groups / " + safeText(group)
		if m.focusRight {
			rows := m.visibleRows()
			if len(rows) == 0 {
				return position + " / no matching members"
			}
			position += " / " + safeText(rows[clamp(m.rowIndex, 0, len(rows)-1)])
		}
		return position
	case pageStatus:
		return prefix + "Status"
	case pageNodes:
		label := "Nodes"
		rows := m.resourceRows()
		if len(rows) == 0 {
			return prefix + label + " / no matches"
		}
		return prefix + label + " / " + safeText(rows[m.resourceIndex(rows)].cells[0])
	default:
		label := "Resources"
		if m.page == pageProfiles {
			label = "Profiles"
		}
		if m.page == pageProviders {
			label = "Providers"
		}
		rows := m.resourceRows()
		if len(rows) == 0 {
			return prefix + label + " / no matches"
		}
		return prefix + label + " / " + safeText(rows[m.resourceIndex(rows)].cells[0])
	}
}
