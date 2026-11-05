package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
)

type fakeClient struct {
	mu          sync.Mutex
	proxies     mihomo.Proxies
	mode        mihomo.Mode
	active      atomic.Int32
	maxActive   atomic.Int32
	delayWait   time.Duration
	selectError error
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		mode: mihomo.ModeRule,
		proxies: mihomo.Proxies{Items: map[string]mihomo.Proxy{
			"Manual": {Name: "Manual", Type: "Selector", Now: "Node A", All: []string{"Node A", "Node B", "Nested"}},
			"Nested": {Name: "Nested", Type: "Selector", Now: "Node C", All: []string{"Node C"}},
			"Node A": {Name: "Node A", Type: "Shadowsocks"},
			"Node B": {Name: "Node B", Type: "Shadowsocks"},
			"Node C": {Name: "Node C", Type: "Shadowsocks"},
		}},
	}
}

func (f *fakeClient) Version(context.Context) (mihomo.Version, error) {
	return mihomo.Version{Version: "test"}, nil
}

func (f *fakeClient) Proxies(context.Context) (mihomo.Proxies, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := mihomo.Proxies{Items: make(map[string]mihomo.Proxy, len(f.proxies.Items))}
	for name, proxy := range f.proxies.Items {
		proxy.All = append([]string(nil), proxy.All...)
		result.Items[name] = proxy
	}
	return result, nil
}

func (f *fakeClient) Mode(context.Context) (mihomo.Mode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mode, nil
}

func (f *fakeClient) Select(_ context.Context, group, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.selectError != nil {
		return f.selectError
	}
	proxy := f.proxies.Items[group]
	proxy.Now = name
	f.proxies.Items[group] = proxy
	return nil
}

func (f *fakeClient) SetMode(_ context.Context, mode mihomo.Mode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
	return nil
}

func (f *fakeClient) Delay(ctx context.Context, _ string, _ string, _ int) (int, error) {
	active := f.active.Add(1)
	defer f.active.Add(-1)
	for {
		maximum := f.maxActive.Load()
		if active <= maximum || f.maxActive.CompareAndSwap(maximum, active) {
			break
		}
	}
	if f.delayWait > 0 {
		select {
		case <-time.After(f.delayWait):
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	return 21, nil
}

func TestSearchAndNestedGroupNavigation(t *testing.T) {
	client := newFakeClient()
	model := newReadyModel(t, client)
	model.search.SetValue("node b")
	if got := model.visibleRows(); len(got) != 1 || got[0] != "Node B" {
		t.Fatalf("visibleRows() = %#v, want Node B", got)
	}

	model.search.SetValue("")
	model.focusRight = true
	model.rowIndex = 2
	if cmd := model.activateSelection(); cmd != nil {
		t.Fatal("entering a nested group should not start a command")
	}
	if model.currentGroup() != "Nested" || len(model.groupPath) != 1 {
		t.Fatalf("current group = %q, path = %#v", model.currentGroup(), model.groupPath)
	}

	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if model.currentGroup() != "Manual" || len(model.groupPath) != 0 {
		t.Fatalf("escape did not return to parent group: %q", model.currentGroup())
	}
}

func TestSelectingNodeConfirmsStateFromAPI(t *testing.T) {
	client := newFakeClient()
	model := newReadyModel(t, client)
	model.focusRight = true
	model.rowIndex = 1
	cmd := model.activateSelection()
	if cmd == nil {
		t.Fatal("selecting a leaf node should issue a request")
	}
	if model.operation == "" {
		t.Fatal("selection should show an in-progress state")
	}
	message := cmd()
	_, _ = model.Update(message)
	if model.operation != "" {
		t.Fatalf("operation remains in progress: %q", model.operation)
	}
	if model.proxies.Items["Manual"].Now != "Node B" {
		t.Fatalf("selected node = %q, want Node B", model.proxies.Items["Manual"].Now)
	}
	if model.notice != "Selection confirmed: Node B" {
		t.Fatalf("notice = %q", model.notice)
	}
	if got := model.selectedNodeLine(); got != "Selected: Manual → Node B" {
		t.Fatalf("selected node header = %q", got)
	}
}

func TestGroupCursorDoesNotChangeTopSelectionUntilConfirmed(t *testing.T) {
	client := newFakeClient()
	model := newReadyModel(t, client)
	before := model.selectedNodeLine()

	model.moveSelection(1)
	if got := model.selectedNodeLine(); got != before {
		t.Fatalf("moving group cursor changed selection: %q != %q", got, before)
	}

	model.groupIndex = 0
	model.focusRight = true
	model.rowIndex = 1
	if got := model.selectedNodeLine(); got != before {
		t.Fatalf("moving member cursor changed selection: %q != %q", got, before)
	}
	cmd := model.selectMember()
	if cmd == nil {
		t.Fatal("selecting a member should issue a request")
	}
	if got := model.selectedNodeLine(); got != before {
		t.Fatalf("pending selection changed header before confirmation: %q != %q", got, before)
	}
	_, _ = model.Update(cmd())
	if got := model.selectedNodeLine(); got != "Selected: Manual → Node B" {
		t.Fatalf("confirmed selection header = %q", got)
	}
}

func TestSelectingNonSelectorDoesNotSendWrite(t *testing.T) {
	client := newFakeClient()
	proxies := client.proxies
	group := proxies.Items["Nested"]
	group.Type = "URLTest"
	proxies.Items["Nested"] = group
	client.proxies = proxies
	model := newReadyModel(t, client)
	model.groupIndex = 1
	model.focusRight = true
	model.rowIndex = 0
	cmd := model.activateSelection()
	if cmd != nil {
		t.Fatal("automatically managed group must not issue a switch request")
	}
	if model.notice == "" {
		t.Fatal("expected an explanation for non-selector groups")
	}
}

func TestLatencyWorkerPoolIsBoundedAndReportsResults(t *testing.T) {
	client := newFakeClient()
	client.proxies.Items["Manual"] = mihomo.Proxy{
		Name: "Manual", Type: "Selector", Now: "Node 0",
	}
	names := make([]string, 35)
	for i := range names {
		name := "Node " + string(rune('A'+i))
		names[i] = name
		client.proxies.Items[name] = mihomo.Proxy{Name: name, Type: "Shadowsocks"}
		client.proxies.Items["Manual"] = appendProxyName(client.proxies.Items["Manual"], name)
	}
	client.delayWait = time.Millisecond
	model := newReadyModel(t, client)
	start := startLatencyCmd(model.ctx, client, 7, names)()
	started, ok := start.(latencyStartedMsg)
	if !ok {
		t.Fatalf("start command returned %T", start)
	}
	model.latencyID = 7
	_, command := model.Update(started)
	for command != nil {
		message := command()
		_, command = model.Update(message)
	}
	if got := client.maxActive.Load(); got > delayWorkers {
		t.Fatalf("maximum concurrent delay requests = %d, limit %d", got, delayWorkers)
	}
	if model.latencyDone != len(names) || len(model.latency) != len(names) {
		t.Fatalf("recorded %d/%d delay results", model.latencyDone, len(names))
	}
}

func TestModeChangeIsReadBack(t *testing.T) {
	client := newFakeClient()
	model := newReadyModel(t, client)
	cmd := model.changeMode()
	if cmd == nil {
		t.Fatal("mode change should issue a request")
	}
	_, _ = model.Update(cmd())
	if model.mode != mihomo.ModeGlobal || model.notice != "Mode confirmed: GLOBAL" {
		t.Fatalf("mode=%q notice=%q", model.mode, model.notice)
	}
}

func TestBootstrapLoadsVersionAndState(t *testing.T) {
	client := newFakeClient()
	message := bootstrapCmd(context.Background(), client)()
	loaded, ok := message.(bootstrapMsg)
	if !ok {
		t.Fatalf("bootstrap command returned %T", message)
	}
	model := newReadyModel(t, client)
	model.state = stateLoading
	_, next := model.Update(loaded)
	if model.state != stateReady || model.version.Version != "test" || model.mode != mihomo.ModeRule {
		t.Fatalf("bootstrap state: state=%d version=%q mode=%q", model.state, model.version.Version, model.mode)
	}
	if next == nil {
		t.Fatal("successful bootstrap did not schedule polling")
	}
}

func TestPollingStopsAfterThreeTransientRetries(t *testing.T) {
	model := newReadyModel(t, newFakeClient())
	for i := 0; i < 3; i++ {
		model.polling = true
		_, next := model.Update(refreshMsg{
			proxiesErr: &mihomo.APIError{Kind: mihomo.ErrorNetwork, Err: errors.New("offline")},
		})
		if next == nil {
			t.Fatalf("retry %d was not scheduled", i+1)
		}
	}
	model.polling = true
	_, next := model.Update(refreshMsg{
		proxiesErr: &mihomo.APIError{Kind: mihomo.ErrorNetwork, Err: errors.New("offline")},
	})
	if next != nil || model.pollRetries != 3 {
		t.Fatalf("retry limit not enforced: retries=%d cmd=%v", model.pollRetries, next != nil)
	}

	_, retry := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if retry == nil || !model.polling {
		t.Fatal("manual retry did not restart polling")
	}
}

func TestPollingPausesImmediatelyOnAuthenticationError(t *testing.T) {
	model := newReadyModel(t, newFakeClient())
	model.polling = true
	_, next := model.Update(refreshMsg{
		proxiesErr: &mihomo.APIError{Kind: mihomo.ErrorAuth, Status: 401},
	})
	if next != nil || model.pollRetries != 3 {
		t.Fatalf("authentication error should pause polling: retries=%d cmd=%v", model.pollRetries, next != nil)
	}
	if !strings.Contains(model.notice, "press r to retry") {
		t.Fatalf("notice = %q, want manual retry instruction", model.notice)
	}
}

func TestCtrlCQuitsWhileSearchIsFocused(t *testing.T) {
	model := newReadyModel(t, newFakeClient())
	model.searching = true
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C should quit while search is focused")
	}
	if model.ctx.Err() == nil {
		t.Fatal("Ctrl+C did not cancel application context")
	}
}

func TestTabChangesFocusedPanel(t *testing.T) {
	model := newReadyModel(t, newFakeClient())
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !model.focusRight {
		t.Fatal("Tab did not focus the node panel")
	}
	_, _ = model.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if model.focusRight {
		t.Fatal("Shift+Tab did not return focus to the group panel")
	}
}

func TestViewMarksCurrentlySelectedProxy(t *testing.T) {
	model := newReadyModel(t, newFakeClient())
	model.width = 100
	model.height = 24
	model.focusRight = true
	if got := model.View(); !strings.Contains(got, "* Node A") {
		t.Fatalf("view does not mark the selected proxy:\n%s", got)
	}
}

func TestMessageForAPIErrorDoesNotExposeRawSecret(t *testing.T) {
	err := &mihomo.APIError{Kind: mihomo.ErrorAuth, Err: errors.New("secret=do-not-show")}
	if got := messageForError(err); got != "Authentication failed; check the Mihomo secret" {
		t.Fatalf("messageForError() = %q", got)
	}
}

func newReadyModel(t *testing.T, client mihomo.Client) *Model {
	t.Helper()
	model := New(context.Background(), client, "http://127.0.0.1:9090", true)
	proxies, err := client.Proxies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	model.applyProxies(proxies)
	model.mode, err = client.Mode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	model.state = stateReady
	t.Cleanup(model.shutdown)
	return model
}

func appendProxyName(proxy mihomo.Proxy, name string) mihomo.Proxy {
	proxy.All = append(proxy.All, name)
	return proxy
}
