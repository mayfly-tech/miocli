package app

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"miocli/internal/config"
	"miocli/internal/mihomo"
	"miocli/internal/profile"
	"miocli/internal/theme"
)

func populatedModel(t *testing.T) *Model {
	t.Helper()
	m := newReadyModel(t, newFakeClient())
	m.providers = make(map[string]mihomo.Provider)
	items := map[string]mihomo.Proxy{}
	var names []string
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("fixture-node-%02d", i)
		names = append(names, name)
		items[name] = mihomo.Proxy{Name: name, Type: "Shadowsocks"}
		m.providers[name] = mihomo.Provider{Name: name}
		m.profileIndex.Entries = append(m.profileIndex.Entries, profile.Entry{ID: name, Name: name, Kind: "https"})
	}
	items["Manual"] = mihomo.Proxy{Name: "Manual", Type: "Selector", Now: names[0], All: names}
	m.applyProxies(mihomo.Proxies{Items: items})
	m.focusRight = true
	return m
}

func assertFits(t *testing.T, view string, width, height int) {
	t.Helper()
	if got := len(strings.Split(view, "\n")); got > height {
		t.Fatalf("height %d > %d:\n%s", got, height, view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("width %d > %d: %q", got, width, line)
		}
	}
}

func TestHealthyListDensity(t *testing.T) {
	for _, size := range []struct{ width, height, minimum int }{{80, 24, 16}, {120, 30, 22}} {
		for _, page := range []resourcePage{pageGroups, pageProfiles, pageProviders, pageNodes} {
			m := populatedModel(t)
			m.width, m.height, m.page = size.width, size.height, page
			view := m.View()
			assertFits(t, view, size.width, size.height)
			count := 0
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "fixture-node-") {
					count++
				}
			}
			if count < size.minimum {
				t.Fatalf("%dx%d page %d: %d rows < %d:\n%s", size.width, size.height, page, count, size.minimum, view)
			}
		}
	}
}

func TestConsoleShellKeepsContextAndFocusVisibleWithoutColor(t *testing.T) {
	m := populatedModel(t)
	m.version = mihomo.Version{Version: "test"}
	m.width, m.height, m.rowIndex = 120, 30, 1
	view := m.View()
	plain := ansi.Strip(view)
	for _, want := range []string{"miocli", "mihomo test", "[2 Groups]", "● Connected", "fixture-node-01", "Space test"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("console view does not show %q:\n%s", want, plain)
		}
	}

	m.noColor = true
	view = m.View()
	assertFits(t, view, m.width, m.height)
	if strings.Contains(view, "\x1b") {
		t.Fatal("no-color console emitted ANSI styles")
	}
	if !strings.Contains(view, "[2 Groups]") || !strings.Contains(view, ">   fixture-node-01") {
		t.Fatalf("no-color view lost focus indicators:\n%s", view)
	}

	m.width, m.height = 50, 12
	view = m.View()
	assertFits(t, view, m.width, m.height)
	if !strings.Contains(view, "[2 Groups]") || !strings.Contains(view, "3 Nodes") || !strings.Contains(view, "4 Status") {
		t.Fatalf("compact navigation hid a destination:\n%s", view)
	}
}

func TestResponsiveViewsAndInputStates(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {100, 24}, {80, 24}, {60, 20}, {50, 12}, {30, 8}} {
		for _, noColor := range []bool{false, true} {
			for _, state := range []string{"normal", "help", "details", "search", "command", "error", "recovery", "dialog"} {
				m := populatedModel(t)
				m.width, m.height, m.noColor = size[0], size[1], noColor
				m.search, m.command = theme.Input(noColor), theme.Input(noColor)
				switch state {
				case "help":
					m.help = true
				case "details":
					m.page, m.detail = pageNodes, true
				case "search":
					m.searching = true
					m.search.Focus()
				case "command":
					m.commanding = true
					m.command.Focus()
				case "error":
					m.lastError = strings.Repeat("Very long error 中文👩‍💻 ", 40)
				case "recovery":
					m.profilePending = true
				case "dialog":
					m.dialog = &profileDialog{action: "apply", confirming: true, warning: "This hot reload may affect existing connections. No automatic restart."}
				}
				view := m.View()
				assertFits(t, view, size[0], size[1])
				if noColor && strings.Contains(view, "\x1b") {
					t.Fatalf("no-color %s emitted styles: %q", state, view)
				}
			}
		}
	}
}

func TestWideGroupsAndOnDemandDetails(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.width, m.height = 120, 30
	view := m.View()
	if !strings.Contains(view, "Groups") || !strings.Contains(view, "Members · Manual") || !strings.Contains(view, "Node B") {
		t.Fatal("wide groups do not show both lists", view)
	}
	if strings.Contains(m.View(), "Details ·") {
		t.Fatal("details consume space before opening")
	}
	m.detail = true
	if !strings.Contains(m.View(), "Details · Esc back") {
		t.Fatal("requested details are hidden")
	}
	m.width = 60
	if strings.Contains(m.View(), "│") {
		t.Fatal("narrow details still use a sidebar")
	}
	m.changePage(pageGroups)
	m.width = 120
	key(m, "v")
	if !m.detail || !strings.Contains(m.View(), "Type: Selector") {
		t.Fatal("group details are unavailable")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.detail {
		t.Fatal("escape did not close group details")
	}
}

func TestNestedReturnAndSmallWindowSafety(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.focusRight, m.rowIndex = true, 2
	m.activateSelection()
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.currentGroup() != "Manual" || m.rowIndex != 2 {
		t.Fatal("return lost parent member")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.focusRight || m.rowIndex != 2 {
		t.Fatal("reenter lost member position")
	}
	m.width, m.height = 30, 8
	m.dialog = &profileDialog{action: "apply", confirming: true, yes: true}
	if cmd := key(m, "y"); cmd != nil || m.operation != "" {
		t.Fatal("hidden confirmation executed a write")
	}
}

func TestDetailEndsScrollWithoutChangingResource(t *testing.T) {
	m := populatedModel(t)
	m.width, m.height, m.page, m.detail = 80, 24, pageProviders, true
	var proxies []mihomo.Proxy
	for i := 0; i < 50; i++ {
		proxies = append(proxies, mihomo.Proxy{Name: fmt.Sprintf("member-%02d", i), Type: "Shadowsocks"})
	}
	m.providers = map[string]mihomo.Provider{"Fixture": {Name: "Fixture", Proxies: proxies}}
	m.resourceIDs[pageProviders] = "Fixture"
	key(m, "G")
	if !strings.Contains(m.View(), "member-49") || m.detailOffset == 0 || m.resourceIDs[pageProviders] != "Fixture" {
		t.Fatal("detail end did not scroll the current resource", m.View())
	}
	key(m, "g")
	if m.detailOffset != 0 || !strings.Contains(m.View(), "Name: Fixture") {
		t.Fatal("detail start did not restore the top")
	}
}

func TestSubscriptionKeyColumnsRemainVisible(t *testing.T) {
	for _, width := range []int{50, 80, 120} {
		m := populatedModel(t)
		m.width, m.height, m.page = width, 24, pageProfiles
		m.profileIndex.Active = m.profileIndex.Entries[0].ID
		if !strings.Contains(m.View(), "Active") {
			t.Fatalf("width %d hides the active profile", width)
		}
		m.page = pageProviders
		if !strings.Contains(m.View(), "Nodes") {
			t.Fatalf("width %d hides the provider node count", width)
		}
	}
}

func TestSubscriptionAndStatusDashboardsShowSummaries(t *testing.T) {
	m := populatedModel(t)
	m.width, m.height = 120, 30
	m.profileIndex.Active = m.profileIndex.Entries[0].ID

	m.page = pageProfiles
	profiles := ansi.Strip(m.View())
	for _, want := range []string{"SUBSCRIPTIONS", "PROFILES", "40 saved", "1 active"} {
		if !strings.Contains(profiles, want) {
			t.Fatalf("profile dashboard missing %q:\n%s", want, profiles)
		}
	}

	m.page = pageProviders
	providers := ansi.Strip(m.View())
	for _, want := range []string{"API PROVIDERS", "40 providers", "0 nodes"} {
		if !strings.Contains(providers, want) {
			t.Fatalf("provider dashboard missing %q:\n%s", want, providers)
		}
	}

	m.page = pageStatus
	status := ansi.Strip(m.View())
	for _, want := range []string{"INSTANCE", "CONNECTIONS", "RESOURCES", "DIAGNOSTICS", "No current errors"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status dashboard missing %q:\n%s", want, status)
		}
	}
	if !strings.Contains(status, "4 Status") {
		t.Fatalf("status navigation does not show the fourth panel:\n%s", status)
	}
}

func TestEnglishProductCopyAndCurrentMarkers(t *testing.T) {
	m := populatedModel(t)
	m.width, m.height = 120, 30
	m.rowIndex = 1
	view := m.View()
	if !strings.Contains(view, "* fixture-node-00") || !strings.Contains(view, ">   fixture-node-01") {
		t.Fatal("current member and cursor are not distinct", view)
	}
	for _, page := range []resourcePage{pageGroups, pageProfiles, pageProviders, pageNodes, pageStatus} {
		m.page = page
		for _, r := range ansi.Strip(m.View() + strings.Join(m.helpLines(), "\n")) {
			if unicode.Is(unicode.Han, r) {
				t.Fatalf("untranslated product copy on page %d", page)
			}
		}
	}
}

func TestLongErrorsRemainReadableAndNeverReady(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.width, m.height = 50, 12
	m.lastError = strings.Repeat("failure ", 100) + "FINAL-CAUSE"
	if strings.Contains(m.View(), "Ready") || !strings.Contains(m.View(), "Status for details") {
		t.Fatal("error not discoverable", m.View())
	}
	key(m, "4")
	key(m, "G")
	if !strings.Contains(strings.ReplaceAll(m.View(), "\n", ""), "FINAL-CAUSE") {
		t.Fatal("long error cannot be read to the end", m.View())
	}
	m.state = stateError
	m.detailOffset = 0
	if !strings.Contains(m.View(), "r retry") {
		t.Fatal("connection error lacks recovery keys")
	}
	key(m, "G")
	if !strings.Contains(strings.ReplaceAll(m.View(), "\n", ""), "FINAL-CAUSE") {
		t.Fatal("startup error is not scrollable")
	}
}

func TestProfileFormSelectsSourceAndKeepsValidationFocus(t *testing.T) {
	m := newReadyModel(t, newFakeClient())
	m.runtime.Owned = true
	m.profileService = profile.New(t.TempDir()+"/config.yaml", config.Config{}, true, nil)
	m.profileKey("a")
	m.dialogKey(tea.KeyMsg{Type: tea.KeyTab})
	m.dialogKey(tea.KeyMsg{Type: tea.KeyRight})
	if m.dialog.fields[1].Value() != "file" {
		t.Fatal("source picker did not select Local YAML")
	}
	m.dialogKey(tea.KeyMsg{Type: tea.KeyTab})
	m.dialog.fields[2].SetValue("/fixture.yaml")
	m.dialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.dialog.field != 0 || m.dialog.err == "" || m.dialog.confirming {
		t.Fatal("invalid name was not focused")
	}
	m.dialog.fields[0].SetValue("Fixture")
	m.dialogKey(tea.KeyMsg{Type: tea.KeyTab})
	m.dialogKey(tea.KeyMsg{Type: tea.KeyTab})
	m.dialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.dialog.confirming || m.dialog.yes {
		t.Fatal("confirmation is not safe by default")
	}
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 12})
	assertFits(t, m.View(), 50, 12)
	if !strings.Contains(m.View(), "[Cancel]") || strings.Contains(m.View(), "/fixture.yaml") {
		t.Fatal("confirmation hides cancel or leaks source")
	}
}
