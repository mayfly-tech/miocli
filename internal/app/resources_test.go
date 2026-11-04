package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"miocli/internal/mihomo"
)

func key(m *Model, value string) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
	return cmd
}

func TestResourceNavigationSearchAndCommands(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.executeCommand("nodes")
	if m.page != pageNodes {
		t.Fatal("nodes command did not navigate to the Nodes panel")
	}
	key(m, "/")
	key(m, "q")
	if !m.searching || m.search.Value() != "q" || m.ctx.Err() != nil {
		t.Fatal("search leaked q to global quit")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.searching || m.search.Value() != "" {
		t.Fatal("escape did not clear search")
	}
	key(m, ":")
	key(m, "status")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.page != pageStatus || m.commanding {
		t.Fatal("resource command failed")
	}
	key(m, "2")
	if m.page != pageGroups {
		t.Fatal("Groups shortcut did not navigate")
	}
	key(m, "3")
	if m.page != pageNodes {
		t.Fatal("Nodes shortcut did not navigate")
	}
	key(m, "4")
	if m.page != pageStatus {
		t.Fatal("Status shortcut did not navigate")
	}
	m.executeCommand("mode nonsense")
	if m.lastError == "" {
		t.Fatal("invalid command had no error")
	}
	m.executeCommand("groups")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || m.ctx.Err() != nil {
		t.Fatal("root Escape quit")
	}
}

func TestNestedMemberSelectionIsSeparateFromNavigation(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.focusRight, m.rowIndex = true, 2
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("nested member was not selectable")
	}
	_, _ = m.Update(cmd())
	if m.proxies.Items["Manual"].Now != "Nested" || len(m.groupPath) != 0 {
		t.Fatal("Enter drilled into a group rather than selecting it")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd != nil || m.currentGroup() != "Nested" {
		t.Fatal("Right did not drill into nested group")
	}
}

func TestNodeSortingFiltersAndStableSelection(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.changePage(pageNodes)
	m.resourceIDs[pageNodes] = "Node B"
	m.latency["Node B"] = latencyValue{delay: 20}
	m.latency["Node A"] = latencyValue{delay: 100}
	m.sortOrder = 2
	rows := m.resourceRows()
	if rows[0].id != "Node B" || rows[1].id != "Node A" || rows[m.resourceIndex(rows)].id != "Node B" {
		t.Fatalf("sort or selection: %#v", rows)
	}
	key(m, "s")
	key(m, "s")
	m.filterType = "NoSuchProtocol"
	if len(m.resourceRows()) != 0 {
		t.Fatal("protocol filter ignored")
	}
	m.filterType = ""
	m.providers = map[string]mihomo.Provider{"One": {Proxies: []mihomo.Proxy{{Name: "ProviderOnly", Type: "Socks5"}}}, "Two": {Proxies: []mihomo.Proxy{{Name: "ProviderOnly", Type: "Socks5"}}}}
	m.filterSource = "One"
	rows = m.resourceRows()
	if len(rows) != 1 || rows[0].id != "One\x00ProviderOnly" {
		t.Fatalf("source identity lost: %#v", rows)
	}
}

func TestNodeTestTargetsSelectedOrFilteredNodes(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.changePage(pageNodes)
	m.resourceIDs[pageNodes] = "Node B"
	if cmd := m.testNodes(false); cmd == nil {
		t.Fatal("selected node did not start a latency test")
	}
	m.filterType = "Shadowsocks"
	if got := m.resourceRows(); len(got) != 3 {
		t.Fatalf("filtered nodes = %#v", got)
	}
	if cmd := m.testNodes(true); cmd == nil {
		t.Fatal("filtered nodes did not start a latency test")
	}
}

func TestGroupsSearchAndSelectionSurviveRefresh(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	key(m, "/")
	key(m, "Nested")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.visibleGroups()) != 1 || m.currentGroup() != "Nested" {
		t.Fatal("group search not applied")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.focusRight || len(m.visibleRows()) != 1 {
		t.Fatal("group search leaked into member filter")
	}
	m.search.SetValue("")
	m.groupIndex, m.focusRight, m.rowIndex = 0, true, 1
	proxies := mihomo.Proxies{Items: map[string]mihomo.Proxy{}}
	for name, proxy := range m.proxies.Items {
		proxies.Items[name] = proxy
	}
	group := proxies.Items["Manual"]
	group.All = []string{"Node B", "Node A", "Nested"}
	proxies.Items["Manual"] = group
	m.applyProxies(proxies)
	if m.rowIndex != 0 {
		t.Fatal("refresh moved cursor to a different member")
	}
}

func TestHelpAndDialogOwnKeyboardFocus(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	key(m, "?")
	key(m, "m")
	if m.operation != "" {
		t.Fatal("help leaked mode key to backend")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.dialog = &profileDialog{action: "delete", confirming: true}
	key(m, "m")
	if m.operation != "" {
		t.Fatal("confirmation leaked key to backend")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.dialog != nil || m.operation != "" {
		t.Fatal("default confirmation was destructive")
	}
	m.profilePending = true
	if cmd := key(m, "m"); cmd != nil || m.operation != "" {
		t.Fatal("pending transaction allowed writes")
	}
}

func TestViewFitsWindowAndRetainsNoColorLayout(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {100, 30}, {80, 24}, {60, 20}, {40, 12}, {10, 4}} {
		for _, page := range []resourcePage{pageGroups, pageProfiles, pageProviders, pageNodes, pageStatus} {
			m := newReadyModel(t, newFakeClient())
			m.width, m.height = size[0], size[1]
			m.page = page
			m.lastError = strings.Repeat("长错误👩‍💻", 80)
			m.providers = map[string]mihomo.Provider{"长订阅": {Name: "长订阅", Proxies: []mihomo.Proxy{{Name: strings.Repeat("节点👩‍💻", 40), Type: "Shadowsocks"}}}}
			for _, line := range strings.Split(m.View(), "\n") {
				if got := lipgloss.Width(line); got > m.width {
					t.Fatalf("%v page %d: width %d: %q", size, page, got, line)
				}
				if strings.Contains(line, "\x1b") {
					t.Fatal("no-color emitted ANSI")
				}
			}
			if len(strings.Split(m.View(), "\n")) > m.height {
				t.Fatalf("%v page %d exceeded height", size, page)
			}
		}
	}
}

func TestUnsafeTerminalCharactersAndStaleResults(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	if text := safeText("node\x1b]52;clipboard\a\r\n"); strings.ContainsAny(text, "\x1b\a\r\n") {
		t.Fatal("terminal controls survived")
	}
	before := m.proxies
	m.generation = 2
	_, _ = m.Update(refreshMsg{generation: 1, proxies: mihomo.Proxies{Items: map[string]mihomo.Proxy{}}})
	if len(m.proxies.Items) != len(before.Items) {
		t.Fatal("stale refresh overwrote new configuration")
	}
}

func TestPollingDoesNotEraseWriteErrorsOrDuplicateTimers(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.lastError = "配置加载结果未确认"
	m.profilePending = true
	_, _ = m.Update(refreshMsg{proxies: m.proxies, mode: m.mode})
	if m.lastError != "配置加载结果未确认" {
		t.Fatal("read success erased write error")
	}
	oldSerial := m.pollSerial
	m.schedulePoll(refreshInterval)
	_, cmd := m.Update(pollTickMsg{serial: oldSerial})
	if cmd != nil || m.polling {
		t.Fatal("superseded timer issued another poll")
	}
	if !strings.Contains(m.View(), "Recovery pending") {
		t.Fatal("pending transaction warning is invisible")
	}
}

func TestEscapeClearsRetainedGroupSearchBeforeGoingBack(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.focusRight = true
	m.groupPath = []string{"Nested"}
	m.search.SetValue("Node C")
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.search.Value() != "" || m.currentGroup() != "Nested" {
		t.Fatal("Escape skipped clearing retained filter")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.currentGroup() != "Manual" {
		t.Fatal("second Escape did not return to parent")
	}
}

func TestCancelLatencyKeepsCompletedResultsAndRejectsLateMessages(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	task, stop := context.WithCancel(m.ctx)
	m.latencyID, m.latencySize, m.latencyDone = 5, 2, 1
	m.latencyStop = stop
	m.latency["Node A"] = latencyValue{delay: 12}
	key(m, "c")
	if task.Err() == nil || m.ctx.Err() != nil || m.latency["Node A"].delay != 12 || m.latencyStop != nil {
		t.Fatal("cancellation lost results or stopped application")
	}
	_, _ = m.Update(latencyResultMsg{id: 5, ok: true, result: delayResult{name: "Node B", delay: 99}})
	if _, exists := m.latency["Node B"]; exists {
		t.Fatal("late canceled result was applied")
	}
}
