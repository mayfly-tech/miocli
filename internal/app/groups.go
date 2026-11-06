package app

import (
	"sort"
	"strings"

	"miocli/internal/mihomo"
)

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
