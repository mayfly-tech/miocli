package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"miocli/internal/theme"
)

func (m *Model) screenSize() (int, int) {
	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, height
}

func (m *Model) View() string {
	width, height := m.screenSize()
	if width < 50 || height < 12 {
		return fitScreen([]string{"miocli · Window too small", "Resize to at least 50 columns x 12 rows", "q / Ctrl+C quit"}, width, height)
	}
	styles := theme.New(m.noColor)
	mode := strings.ToUpper(unknown(string(m.mode)))
	connection := "Connected"
	switch {
	case m.profilePending:
		connection = "Recovery pending"
	case m.state == stateLoading:
		connection = "Connecting"
	case m.state == stateError:
		connection = "Disconnected"
	case m.refreshError != "":
		connection = "Stale"
	case m.modeError != "" || m.connectionsError != "":
		connection = "Partial"
	}
	connectionStyle := styles.Success
	switch connection {
	case "Connecting":
		connectionStyle = styles.Highlight
	case "Recovery pending", "Stale", "Partial":
		connectionStyle = styles.Warning
	case "Disconnected":
		connectionStyle = styles.Error
	}
	header := joinSides(
		styles.Brand.Render("miocli")+" "+styles.Muted.Render("mihomo "+unknown(m.version.Version)),
		styles.Highlight.Render(mode)+"  "+connectionStyle.Render("● "+connection),
		width,
	)
	nav := []string{"1 Subscriptions", "2 Groups", "3 Nodes", "4 Status"}
	if width < 70 {
		nav[0] = "1 Subs"
	}
	active := 1
	switch m.page {
	case pageProfiles, pageProviders:
		active = 0
	case pageNodes:
		active = 2
	case pageStatus:
		active = 3
	}
	for i := range nav {
		if i == active {
			nav[i] = styles.ActiveTab.Render("[" + nav[i] + "]")
		} else {
			nav[i] = styles.Tab.Render(" " + nav[i] + " ")
		}
	}
	lines := []string{
		header,
		strings.Join(nav, "  "),
		styles.Muted.Render("◆ " + m.selectedNodeLineAtWidth(width-4)),
		styles.Divider.Render(strings.Repeat("─", width)),
	}
	contentHeight := m.contentHeight()
	var content []string
	switch {
	case m.help:
		content = m.scrolled(m.helpLines(), width, contentHeight)
	case m.state == stateLoading:
		content = []string{"Connecting to Mihomo...", "", "q / Ctrl+C to cancel"}
	case m.state == stateError:
		content = m.scrolled(m.failureLines(), width, contentHeight)
	case m.page == pageStatus:
		content = m.statusView(width, contentHeight, styles)
	default:
		content = m.tableView(width, contentHeight, styles)
	}
	for len(content) < contentHeight {
		content = append(content, "")
	}
	lines = append(lines, content[:min(contentHeight, len(content))]...)
	for _, line := range m.alertLines() {
		lines = append(lines, styles.Error.Render(line))
	}
	lines = append(lines, m.footerLines(styles)...)
	if m.dialog != nil {
		lines = m.dialogView(width, height, styles)
	}
	return fitScreen(lines, width, height)
}

func fitScreen(lines []string, width, height int) string {
	result := make([]string, 0, min(len(lines), height))
	for _, line := range lines[:min(len(lines), height)] {
		result = append(result, ansi.Truncate(line, width, "…"))
	}
	return strings.Join(result, "\n")
}

func joinSides(left, right string, width int) string {
	space := width - lipgloss.Width(left) - lipgloss.Width(right)
	if space < 2 {
		left = ansi.Truncate(left, max(0, width-lipgloss.Width(right)-2), "…")
		space = max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	}
	return left + strings.Repeat(" ", space) + right
}

func wrappedLines(lines []string, width int) []string {
	var result []string
	for _, line := range lines {
		result = append(result, strings.Split(ansi.Wrap(safeText(line), max(1, width), ""), "\n")...)
	}
	return result
}

func (m *Model) scrolled(lines []string, width, height int) []string {
	lines = wrappedLines(lines, width)
	offset := clamp(m.detailOffset, 0, max(0, len(lines)-height))
	return lines[offset:min(len(lines), offset+height)]
}

func (m *Model) alertLines() []string {
	width, _ := m.screenSize()
	var errors []string
	if m.profilePending {
		errors = append(errors, "Recovery pending: quit and restart the owned core; writes blocked")
	}
	if m.lastError != "" {
		errors = append(errors, m.lastError)
	}
	if m.refreshError != "" {
		errors = append(errors, "Stale data: "+m.refreshError)
	}
	if m.modeError != "" {
		errors = append(errors, "Mode: "+m.modeError)
	}
	if (m.page == pageProviders || m.page == pageNodes) && m.providerErr != "" {
		errors = append(errors, "Providers: "+m.providerErr)
	}
	if m.profileService != nil && m.profileErr != "" {
		errors = append(errors, "Profiles: "+m.profileErr)
	}
	if len(errors) == 0 {
		return nil
	}
	summary := "Error: " + strings.Join(errors, " · ")
	lines := wrappedLines([]string{summary}, width)
	if len(lines) > 2 {
		lines = lines[:2]
		suffix := " · 4 Status for details"
		if m.state == stateError {
			suffix = " · j/k scroll for details"
		}
		lines[1] = ansi.Truncate(lines[1], max(1, width-lipgloss.Width(suffix)), "…") + suffix
	}
	return lines
}

func (m *Model) footerLines(styles theme.Styles) []string {
	width, _ := m.screenSize()
	var lines []string
	status := ""
	switch {
	case m.operation != "":
		status = m.operation + "..."
	case m.latencyStop != nil:
		status = fmt.Sprintf("Testing %d/%d · c cancel", m.latencyDone, m.latencySize)
	case m.notice != "":
		status = safeText(m.notice)
	}
	if status != "" {
		lines = wrappedLines([]string{status}, width)
		if len(lines) > 2 {
			lines = []string{ansi.Truncate(safeText(status), width, "…")}
		}
		statusStyle := styles.Highlight
		if m.notice != "" {
			statusStyle = styles.Success
		}
		for i := range lines {
			lines[i] = statusStyle.Render("● " + ansi.Truncate(lines[i], max(1, width-2), "…"))
		}
	}
	switch {
	case m.searching:
		lines = append(lines, styles.Highlight.Render("SEARCH")+" "+m.search.View()+"  Enter keep · Esc clear")
	case m.commanding:
		lines = append(lines, styles.Highlight.Render("COMMAND")+" "+m.command.View())
	default:
		hints := m.keyHints()
		if m.help || m.detail {
			hints = "j/k scroll  PgUp/PgDn page  g/G ends  Esc back  q quit"
		}
		if status == "" {
			state := "Ready"
			if len(m.alertLines()) > 0 {
				state = "Attention"
			}
			hints = joinSides(ansi.Truncate(hints, max(1, width-len(state)-2), "…"), state, width)
		}
		lines = append(lines, styles.Footer.Render(hints))
	}
	return lines
}

func (m *Model) contentHeight() int {
	_, height := m.screenSize()
	return max(1, height-4-len(m.alertLines())-len(m.footerLines(theme.Styles{})))
}

func (m *Model) listHeight() int { return max(1, m.contentHeight()-2) }

func (m *Model) groupRows(members bool) ([]resourceRow, int) {
	var rows []resourceRow
	index := 0
	if members {
		names := m.visibleRows()
		nodes := m.nodes()
		if !m.focusRight {
			names = m.proxies.Items[m.currentGroup()].All
		}
		for _, name := range names {
			proxy := m.proxies.Items[name]
			marker, status := "  ", ""
			if m.proxies.Items[m.currentGroup()].Now == name {
				marker, status = "* ", "Current"
			}
			if isGroupType(proxy.Type) {
				status += " Group >"
			}
			label := marker + name
			if isGroupType(proxy.Type) {
				label += " >"
			}
			sources := []string{"Unknown"}
			for _, node := range nodes {
				if node.proxy.Name == name {
					sources = node.sources
					break
				}
			}
			rows = append(rows, resourceRow{name, []string{label, proxy.Type, m.delayLabel(name), status}, []string{
				"Name: " + name, "Type: " + proxy.Type, "Latency: " + m.delayLabel(name),
				"Source: " + strings.Join(sources, ", "),
				"State: " + unknown(strings.TrimSpace(status)), "Group: " + m.currentGroup(),
				"Enter selects members only in Selector groups; Right enters child groups",
			}})
		}
		index = clamp(m.rowIndex, 0, max(0, len(rows)-1))
	} else {
		for _, name := range m.visibleGroups() {
			proxy := m.proxies.Items[name]
			rows = append(rows, resourceRow{name, []string{name, proxy.Type, unknown(proxy.Now), fmt.Sprint(len(proxy.All))}, []string{
				"Name: " + name, "Type: " + proxy.Type, "Current: " + unknown(proxy.Now),
				fmt.Sprintf("Direct members: %d", len(proxy.All)), "Only Selector groups allow manual selection",
			}})
			if name == m.rootGroup() {
				index = len(rows) - 1
			}
		}
	}
	return rows, index
}

func (m *Model) rootGroup() string {
	if m.groupIndex >= 0 && m.groupIndex < len(m.groups) {
		return m.groups[m.groupIndex]
	}
	return ""
}

func (m *Model) tableView(width, height int, styles theme.Styles) []string {
	if m.page == pageGroups {
		groups, groupIndex := m.groupRows(false)
		members, memberIndex := m.groupRows(true)
		if m.detail {
			rows, index := groups, groupIndex
			if m.focusRight {
				rows, index = members, memberIndex
			}
			if len(rows) > 0 {
				return append([]string{styles.Highlight.Render("Details · Esc back")}, m.scrolled(rows[index].details, width, height-1)...)
			}
		}
		if width >= 100 {
			leftWidth := width / 3
			left := renderTable("Groups", []string{"Name", "Type", "Current", "Members"}, groups, groupIndex, leftWidth, height, !m.focusRight, []int{0, 2}, styles)
			rightWidth := width - leftWidth - 3
			right := renderTable(m.breadcrumb(), []string{"Name", "Type", "Delay", "State"}, members, memberIndex, rightWidth, height, m.focusRight, []int{0, 1, 2}, styles)
			var lines []string
			for i := 0; i < max(len(left), len(right)); i++ {
				a, b := "", ""
				if i < len(left) {
					a = left[i]
				}
				if i < len(right) {
					b = right[i]
				}
				lines = append(lines, a+strings.Repeat(" ", max(0, leftWidth-lipgloss.Width(a)))+" │ "+b)
			}
			return lines
		}
		if m.focusRight {
			indices := []int{0, 1, 2}
			if width < 65 {
				indices = []int{0, 2}
			}
			return renderTable(m.breadcrumb(), []string{"Name", "Type", "Delay", "State"}, members, memberIndex, width, height, true, indices, styles)
		}
		return renderTable("Groups", []string{"Name", "Type", "Current", "Members"}, groups, groupIndex, width, height, true, []int{0, 2}, styles)
	}
	rows := m.resourceRows()
	index := m.resourceIndex(rows)
	headers := []string{"Name", "Protocol", "Source", "Delay"}
	title := ""
	switch m.page {
	case pageProfiles:
		title, headers = "Profiles · p Providers", []string{"Name", "Source", "State", "Updated"}
	case pageProviders:
		title, headers = "Providers · p Profiles", []string{"Name", "Source", "Nodes", "Updated"}
	}
	if m.search.Value() != "" {
		title += " · /" + safeText(m.search.Value())
	}
	if m.page == pageNodes {
		title = "Nodes · " + []string{"API order", "Name", "Latency"}[m.sortOrder]
		if m.filterType != "" {
			title += " · " + m.filterType
		}
		if m.filterSource != "" {
			title += " · " + m.filterSource
		}
		if m.search.Value() != "" {
			title += " · /" + safeText(m.search.Value())
		}
	}
	if len(rows) == 0 {
		message := "No entries available"
		switch {
		case m.search.Value() != "" || m.filterType != "" || m.filterSource != "":
			message = "No matches. Press Esc to clear filters"
		case m.page == pageProfiles:
			message = m.profileEmptyText()
		case (m.page == pageProviders || m.page == pageNodes) && m.providerErr != "":
			message = "Provider read failed. Press r to retry; 4 Status for details"
		}
		var header []string
		if m.page == pageProfiles || m.page == pageProviders {
			header = m.subscriptionHeader(width, styles)
		}
		return append(header,
			append([]string{styles.Highlight.Render(ansi.Truncate(title, width, "…")), ""}, wrappedLines([]string{message}, width)...)...)
	}
	var header []string
	if m.page == pageProfiles || m.page == pageProviders {
		header = m.subscriptionHeader(width, styles)
	}
	height = max(1, height-len(header))
	tableWidth := width
	detailWidth := width
	if m.detail {
		if width < 100 {
			return append(header, append([]string{styles.Highlight.Render("Details · Esc back")}, m.scrolled(rows[index].details, width, height-1)...)...)
		}
		detailWidth = max(32, width/3)
		tableWidth = width - detailWidth - 3
	}
	indices := []int{0, 1, 2, 3}
	if tableWidth < 85 {
		indices = []int{0, 1, 2}
		if m.page == pageNodes {
			indices = []int{0, 1, 3}
		}
	}
	if tableWidth < 65 {
		indices = []int{0, 2}
		if m.page == pageNodes {
			indices = []int{0, 3}
		}
	}
	lines := renderTable(title, headers, rows, index, tableWidth, height, !m.detail, indices, styles)
	if m.detail {
		details := append([]string{styles.Highlight.Render("Details · Esc back")}, m.scrolled(rows[index].details, detailWidth, height-1)...)
		for len(lines) < height {
			lines = append(lines, "")
		}
		for i := range lines {
			lines[i] += strings.Repeat(" ", max(0, tableWidth-lipgloss.Width(lines[i]))) + " │ "
			if i < len(details) {
				lines[i] += details[i]
			}
		}
	}
	return append(header, lines...)
}

func renderTable(title string, headers []string, rows []resourceRow, index, width, height int, focused bool, indices []int, styles theme.Styles) []string {
	title = fmt.Sprintf("%s [%d]", title, len(rows))
	if focused {
		title = "> " + title
	}
	widths := make([]int, len(indices))
	available := width - 2 - (len(indices)-1)*2
	for i := 1; i < len(indices); i++ {
		if headers[indices[i]] == "Current" {
			widths[i] = max(1, available*2/5)
			available -= widths[i]
			continue
		}
		target := 12
		if headers[indices[i]] == "Delay" {
			target = 9
		}
		if headers[indices[i]] == "Updated" {
			target = 12
		}
		widths[i] = min(target, max(1, available/(len(indices)+1)))
		available -= widths[i]
	}
	widths[0] = max(1, available)
	render := func(cells []string) string {
		parts := make([]string, len(indices))
		for i, column := range indices {
			value := ""
			if column < len(cells) {
				value = safeText(cells[column])
			}
			value = ansi.Truncate(value, widths[i], "…")
			parts[i] = value + strings.Repeat(" ", max(0, widths[i]-lipgloss.Width(value)))
		}
		return strings.Join(parts, "  ")
	}
	titlePrefix := "  "
	if focused {
		titlePrefix = "▸ "
	}
	lines := []string{
		styles.Highlight.Render(ansi.Truncate(titlePrefix+title, width, "…")),
		styles.TableHead.Render("  " + render(headers)),
	}
	if len(rows) == 0 {
		return append(lines, "  No matches")
	}
	count := max(1, height-2)
	start := max(0, index-count+1)
	for i := start; i < min(len(rows), start+count); i++ {
		prefix := "  "
		if i == index {
			prefix = "> "
		}
		line := prefix + render(rows[i].cells)
		if i == index && focused {
			line = styles.Selected.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m *Model) breadcrumb() string {
	if len(m.groupPath) > 0 {
		return "Members · " + m.rootGroup() + " / " + strings.Join(m.groupPath, " / ")
	}
	return "Members · " + m.currentGroup()
}

func (m *Model) keyHints() string {
	width, _ := m.screenSize()
	if width < 70 {
		switch m.page {
		case pageGroups:
			if m.focusRight {
				return "Enter select  Space/T test  s/t/f filter/sort  ?"
			}
			return "Enter members  Space test  ? help"
		case pageProfiles:
			if m.runtime.Owned {
				return "a add  Space activate  ? help"
			}
			return "p Providers  Enter details  ? help"
		case pageProviders:
			return "u update  p Profiles  ? help"
		default:
			return "j/k scroll  ? help  q quit"
		}
	}
	switch m.page {
	case pageProfiles:
		if !m.runtime.Owned {
			return "p Providers  Enter details  ? help  q quit"
		}
		return "a add  e edit  u update  Space activate  d delete  ? help"
	case pageProviders:
		return "u update  p Profiles  Enter details  / search  ? help"
	case pageNodes:
		return "Space test  T test filtered  / search  ? help  q quit"
	case pageGroups:
		if m.focusRight {
			return "Enter select  Space test  v details  / search  ? help"
		}
		return "Enter members  Space test  v details  / search  ? help"
	default:
		return "j/k scroll  r refresh  ? help  q quit"
	}
}

func (m *Model) subscriptionHeader(width int, styles theme.Styles) []string {
	section, summary := "PROFILES", ""
	if m.page == pageProfiles {
		active := 0
		for _, entry := range m.profileIndex.Entries {
			if entry.ID == m.profileIndex.Active {
				active++
			}
		}
		access := "Managed profiles unavailable"
		if m.runtime.Owned {
			access = "Managed profiles enabled"
		}
		summary = fmt.Sprintf("%d saved · %d active · %s", len(m.profileIndex.Entries), active, access)
	} else {
		section = "API PROVIDERS"
		total := 0
		for _, provider := range m.providers {
			total += len(provider.Proxies)
		}
		summary = fmt.Sprintf("%d providers · %d nodes · updates are manual", len(m.providers), total)
	}
	left := styles.Brand.Render("SUBSCRIPTIONS") + " " + styles.ActiveTab.Render(section)
	right := styles.Muted.Render(summary) + "  " + styles.Tab.Render("p switch")
	if width < 100 {
		return []string{ansi.Truncate(left+"  "+right, width, "…")}
	}
	return []string{joinSides(left, right, width)}
}

func (m *Model) statusView(width, height int, styles theme.Styles) []string {
	lines := m.scrolled(m.statusLines(), width, height)
	for i, line := range lines {
		if strings.HasSuffix(line, ":") {
			lines[i] = styles.ActiveTab.Render(strings.TrimSuffix(line, ":"))
			continue
		}
		if label, value, ok := strings.Cut(line, ": "); ok {
			lines[i] = styles.Muted.Render(label+": ") + safeText(value)
		}
	}
	return lines
}

func (m *Model) statusLines() []string {
	owned := "No; full profile writes unavailable"
	if m.runtime.Owned {
		owned = "Yes; only this child is stopped on exit"
	}
	lines := []string{
		"INSTANCE:",
		"Controller: " + m.server,
		"Mihomo version: " + unknown(m.version.Version),
		"Proxy mode: " + strings.ToUpper(string(m.mode)),
		"Core mode: " + unknown(m.runtime.CoreMode),
		"Owned core: " + owned,
		"Configuration: " + unknown(m.runtime.ConfigPath),
		"Credentials: secret hidden",
		"",
		"CONNECTIONS:",
		"Active route summary: " + m.connectionLine(),
		"",
		"GROUP SELECTION:",
		m.selectedNodeLine(),
	}
	if m.connectionsLoaded && m.connectionsError == "" {
		lines = append(lines, fmt.Sprintf("Active routes (%d):", len(m.connectionRoutes())))
		lines = append(lines, m.connectionRoutes()...)
	}
	lines = append(lines, "", "RESOURCES:")
	lines = append(lines, fmt.Sprintf("Groups: %d", len(m.groups)))
	lines = append(lines, fmt.Sprintf("Nodes: %d", len(m.nodes())))
	lines = append(lines, fmt.Sprintf("Providers: %d", len(m.providers)))
	if !m.lastRefresh.IsZero() {
		lines = append(lines, "Last successful read: "+m.lastRefresh.Format("2006-01-02 15:04:05"))
	}
	lines = append(lines, "", "DIAGNOSTICS:")
	if m.lastError != "" {
		lines = append(lines, "Error: "+m.lastError)
	}
	if m.refreshError != "" {
		lines = append(lines, "Stale data: "+m.refreshError)
	}
	if m.profilePending {
		lines = append(lines, "Profile recovery pending: quit and restart the owned core")
	}
	if m.modeError != "" {
		lines = append(lines, "Mode error: "+m.modeError)
	}
	if m.providerErr != "" {
		lines = append(lines, "Providers: "+m.providerErr)
	}
	if m.connectionsError != "" {
		lines = append(lines, "Connections error: "+m.connectionsError)
	}
	if m.profileErr != "" {
		lines = append(lines, "Profiles: "+m.profileErr)
	}
	if m.notice != "" {
		lines = append(lines, "Last result: "+m.notice)
	}
	if strings.HasSuffix(lines[len(lines)-1], ":") {
		lines = append(lines, "No current errors")
	}
	return lines
}

func (m *Model) failureLines() []string {
	return []string{"Connection failed", "r retry · j/k scroll · ? help · q quit",
		"Quit and run miocli doctor / miocli setup to check the connection.",
		"No second core is installed or started automatically.", "", m.lastError}
}

func (m *Model) helpLines() []string {
	return []string{
		"miocli · Help (? / Esc close)", "",
		"Navigate",
		"1 Subscriptions · 2 Groups · 3 Nodes · 4 Status",
		"Up/Down or j/k move · PgUp/PgDn page · g/G first/last",
		"Groups: Tab focus group/member · Enter enter/select",
		"v group/member details · Esc close details",
		"Right/l enter nested group · Left/h or Esc return",
		"Subscriptions: Enter or Tab details · Esc close",
		"/ search names · Enter keep filter · Esc clear",
		": commands · m cycle Rule/Global/Direct · r refresh",
		"? help · q / Ctrl+C quit · Esc at root does not quit", "",
		"Actions",
		"Profiles: a add · e edit · u refresh · Space activate · d delete",
		"p switch Profiles / Providers · Providers: u update",
		"Nodes: Space test one · T test filtered · c cancel",
		"s sort nodes · t protocol filter · f source filter",
		"Groups: Space test direct nodes",
		"* marks the confirmed member; > marks the cursor",
		"Automatic groups cannot be selected manually", "",
		"Commands (not a shell)",
		"profiles | providers | groups | nodes (members) | status | help",
		"refresh | mode rule|global|direct | quit", "",
		"Full profiles require a managed child started by this run.",
		"Sources stay read-only. Timed-out writes are not replayed.",
		"Connections show active routes, not the current cursor.",
	}
}
