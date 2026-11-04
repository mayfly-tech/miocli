package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"miocli/internal/mihomo"
)

func TestSelectedNodeVisibleOnEveryResourcePage(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.width, m.height = 100, 24
	m.connectionsLoaded = true
	m.connections = []mihomo.Connection{{Chains: []string{"Node A", "Manual"}}}
	for _, page := range []resourcePage{pageGroups, pageProviders, pageProfiles, pageNodes, pageStatus} {
		m.changePage(page)
		view := m.View()
		if !strings.Contains(view, "Selected: Manual → Node A") {
			t.Fatalf("page %d hides the current group selection:\n%s", page, view)
		}
		if page == pageStatus && !strings.Contains(view, "Connections: Manual → Node A") {
			t.Fatalf("status page hides active connection routes:\n%s", view)
		}
		if page != pageStatus && strings.Contains(strings.Split(view, "\n")[2], "Connections:") {
			t.Fatalf("resource page shows connection routes instead of the selected node:\n%s", view)
		}
	}
}

func TestConnectionsAreIndependentOfCursorAndProxySelection(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.connectionsLoaded = true
	m.connections = []mihomo.Connection{{Chains: []string{"Node A", "Manual"}}}
	before := m.connectionLine()
	selected := m.selectedNodeLine()
	m.focusRight, m.rowIndex = true, 1
	if got := m.cursorLine(); got != "Cursor: Groups / Manual / Node B" {
		t.Fatal(got)
	}
	if m.selectedNodeLine() != selected {
		t.Fatal("moving the cursor changed the displayed selected node")
	}
	m.focusRight = false
	m.moveSelection(1)
	if m.selectedNodeLine() != selected {
		t.Fatal("moving the group cursor changed the displayed selected node")
	}
	m.groupIndex = 0
	m.focusRight = true
	m.rowIndex = 1
	if m.connectionLine() != before {
		t.Fatal("moving cursor changed current connections")
	}
	cmd := m.selectMember()
	if cmd == nil {
		t.Fatal("selection did not issue write")
	}
	_, _ = m.Update(cmd())
	if m.connectionLine() != before {
		t.Fatal("proxy selection changed existing connection route")
	}
	if got := m.selectedNodeLine(); got != "Selected: Manual → Node B" {
		t.Fatalf("selected node header = %q", got)
	}
	_, _ = m.Update(connectionsMsg{generation: m.generation, items: []mihomo.Connection{{Chains: []string{"Node B", "Manual"}}}})
	if got := m.connectionLine(); got != "Connections: Manual → Node B" {
		t.Fatal(got)
	}
	m.focusRight = false
	m.moveSelection(1)
	if got := m.cursorLine(); got != "Cursor: Groups / Nested" {
		t.Fatal(got)
	}
}

func TestSelectedNodeFollowsNestedGroupSelection(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.selectedGroupPath = []string{"Manual", "Nested"}
	if got := m.selectedNodeLine(); got != "Selected: Manual / Nested → Node C" {
		t.Fatal(got)
	}
}

func TestConnectionsNeverInventRouteFromModeOrNow(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	if got := m.connectionLine(); got != "Connections: loading" {
		t.Fatal(got)
	}
	m.connectionsLoaded = true
	for _, mode := range []mihomo.Mode{mihomo.ModeRule, mihomo.ModeGlobal, mihomo.ModeDirect} {
		m.mode = mode
		if got := m.connectionLine(); got != "Connections: no active connections" {
			t.Fatal(got)
		}
	}
	m.connections = []mihomo.Connection{{Chains: []string{"DIRECT"}}}
	if got := m.connectionLine(); got != "Connections: DIRECT" {
		t.Fatal(got)
	}
	m.connectionsError = "Authentication failed"
	if got := m.connectionLine(); !strings.Contains(got, "read failed") || strings.Contains(got, "DIRECT") {
		t.Fatal(got)
	}
	if status := strings.Join(m.statusLines(), "\n"); strings.Contains(status, "Active routes:") {
		t.Fatal("status page shows stale routes", status)
	}
}

func TestCursorFollowsGroupMembersAndFiltering(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.changePage(pageGroups)
	m.focusRight = true
	for _, index := range []int{0, 1} {
		m.rowIndex = index
		rows := m.visibleRows()
		want := "Cursor: Groups / Manual / " + rows[m.rowIndex]
		if got := m.cursorLine(); got != want {
			t.Fatalf("%q != %q", got, want)
		}
	}
	m.changePage(pageProviders)
	m.providers = map[string]mihomo.Provider{"Example": {}}
	m.changePage(pageNodes)
	if got := m.cursorLine(); !strings.HasPrefix(got, "Cursor: Nodes / ") {
		t.Fatal(got)
	}
	m.changePage(pageProviders)
	if got := m.cursorLine(); got != "Cursor: Providers / Example" {
		t.Fatal(got)
	}
	m.search.SetValue("no matching resource")
	if got := m.cursorLine(); got != "Cursor: Providers / no matches" {
		t.Fatal(got)
	}
	m.changePage(pageProfiles)
	if got := m.cursorLine(); got != "Cursor: Profiles / no matches" {
		t.Fatal(got)
	}
}

func TestConnectionRoutesReverseDeduplicateAndFit(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.connectionsLoaded = true
	m.connections = []mihomo.Connection{
		{Chains: []string{"Node C", "Nested", "Manual"}},
		{Chains: []string{"Node C", "Nested", "Manual"}},
		{Chains: []string{"DIRECT"}},
	}
	routes := m.connectionRoutes()
	if len(routes) != 2 || routes[1] != "Manual → Nested → Node C" {
		t.Fatalf("%#v", routes)
	}
	m.width = 50
	m.connections = append(m.connections, mihomo.Connection{Chains: []string{strings.Repeat("中文👩‍💻", 30), "Another"}})
	if line := m.connectionLine(); lipgloss.Width(line) > 50 || !strings.Contains(line, "routes") {
		t.Fatal(line)
	}
}

type connectionFake struct {
	*fakeClient
	items []mihomo.Connection
	err   error
}

func (c *connectionFake) Connections(context.Context) ([]mihomo.Connection, error) {
	return c.items, c.err
}

func TestConnectionPollingFailureAndRecovery(t *testing.T) {
	client := &connectionFake{fakeClient: newFakeClient(), items: []mihomo.Connection{{Chains: []string{"Node A", "Manual"}}}}
	m := New(context.Background(), client, "http://127.0.0.1:9090", true)
	t.Cleanup(m.shutdown)
	cmd := m.loadConnections()
	if cmd == nil || !m.connectionsBusy || m.loadConnections() != nil {
		t.Fatal("connection polling is not serialized")
	}
	_, _ = m.Update(cmd())
	if m.connectionsBusy || !m.connectionsLoaded || m.connectionLine() != "Connections: Manual → Node A" {
		t.Fatal(m.connectionLine())
	}
	client.err = &mihomo.APIError{Kind: mihomo.ErrorAuth}
	_, _ = m.Update(m.loadConnections()())
	if !strings.Contains(m.connectionLine(), "read failed") {
		t.Fatal(m.connectionLine())
	}
	client.err, client.items = nil, nil
	_, _ = m.Update(m.loadConnections()())
	if got := m.connectionLine(); got != "Connections: no active connections" {
		t.Fatal(got)
	}
	m.generation = 1
	client.err = errors.New("network error")
	_, next := m.Update(connectionsMsg{generation: 0, items: []mihomo.Connection{{Chains: []string{"stale"}}}})
	if next == nil || len(m.connections) != 0 {
		t.Fatal("stale connection result was applied or not refreshed")
	}
}
