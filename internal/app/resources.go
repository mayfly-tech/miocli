package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

type resourceRow struct {
	id      string
	cells   []string
	details []string
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
