package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
)

type providerFake struct {
	*fakeClient
	items    map[string]mihomo.Provider
	writeErr error
	readErr  error
	writes   int
}

func (f *providerFake) Providers(context.Context) (map[string]mihomo.Provider, error) {
	return f.items, f.readErr
}

func (f *providerFake) UpdateProvider(_ context.Context, name string) error {
	f.writes++
	if f.writeErr != nil {
		return f.writeErr
	}
	f.items = map[string]mihomo.Provider{
		name: {Name: name, Proxies: []mihomo.Proxy{{Name: "New Node", Type: "Socks5"}}},
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.proxies.Items["New Node"] = mihomo.Proxy{Name: "New Node", Type: "Socks5"}
	group := f.proxies.Items["Manual"]
	group.All = append(group.All, "New Node")
	f.proxies.Items["Manual"] = group
	return nil
}

func TestProviderUpdateRefreshesNodesAndGroupsAndRejectsOldReads(t *testing.T) {
	client := &providerFake{fakeClient: newFakeClient()}
	m := newReadyModel(t, client)
	m.providers = map[string]mihomo.Provider{"Fixture": {Name: "Fixture"}}
	m.changePage(pageProviders)
	oldProxies := m.proxies
	m.polling, m.pollGeneration = true, m.generation
	oldGeneration := m.generation
	cmd := m.updateProvider()
	if cmd == nil || m.generation == oldGeneration {
		t.Fatal("update did not invalidate in-flight reads")
	}
	_, refresh := m.Update(cmd())
	if client.writes != 1 || m.lastError != "" || refresh == nil {
		t.Fatal("provider update not confirmed or proxy refresh missing")
	}
	_, _ = m.Update(refreshMsg{generation: oldGeneration, proxies: oldProxies})
	if !m.polling {
		t.Fatal("old refresh interrupted the post-update read")
	}
	applyRefreshCommands(t, m, refresh)
	if !contains(m.proxies.Items["Manual"].All, "New Node") {
		t.Fatal("group members were not refreshed after provider update")
	}
	m.changePage(pageNodes)
	found := false
	for _, node := range m.nodes() {
		if node.proxy.Name == "New Node" && contains(node.sources, "Fixture") {
			found = true
		}
	}
	if !found || client.writes != 1 {
		t.Fatal("node inventory lost provider source or repeated a write")
	}
}

func applyRefreshCommands(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, command := range msg {
			applyRefreshCommands(t, m, command)
		}
	case refreshMsg:
		_, _ = m.Update(msg)
	default:
		t.Fatalf("unexpected refresh message %T", msg)
	}
}

func TestProviderUpdateErrorsAreExplicitAndNeverReplayed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		writeErr error
		readErr  error
		want     string
	}{
		{"write timeout", &mihomo.APIError{Kind: mihomo.ErrorTimeout}, nil, "Provider update unconfirmed"},
		{"readback failed", nil, errors.New("read failed"), "provider readback failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &providerFake{fakeClient: newFakeClient(), writeErr: tc.writeErr, readErr: tc.readErr}
			m := newReadyModel(t, client)
			m.providers = map[string]mihomo.Provider{"Fixture": {Name: "Fixture"}}
			m.notice = "Previous success"
			m.changePage(pageProviders)
			_, refresh := m.Update(m.updateProvider()())
			if !strings.Contains(m.lastError, tc.want) || m.notice != "" || m.operation != "" {
				t.Fatalf("failed update = %q, notice %q, operation %q", m.lastError, m.notice, m.operation)
			}
			applyRefreshCommands(t, m, refresh)
			if client.writes != 1 || !strings.Contains(m.lastError, tc.want) {
				t.Fatal("refresh replayed the write or cleared its error")
			}
			if tc.readErr != nil && len(m.providers["Fixture"].Proxies) != 0 {
				t.Fatal("failed readback replaced last known provider data")
			}
		})
	}
}

func TestOldProviderReadCannotReleaseNewRequest(t *testing.T) {
	client := &providerFake{fakeClient: newFakeClient(), items: map[string]mihomo.Provider{"Fixture": {Name: "Fixture"}}}
	m := newReadyModel(t, client)
	m.page, m.providers = pageProviders, client.items
	old := m.loadProviders()()
	_, _ = m.Update(m.updateProvider()())
	current := m.loadProviders()
	if current == nil {
		t.Fatal("new provider read did not start")
	}
	_, _ = m.Update(old)
	if !m.providerBusy {
		t.Fatal("old provider response released a newer request")
	}
	_, _ = m.Update(current())
	if m.providerBusy || len(m.providers["Fixture"].Proxies) != 1 {
		t.Fatal("new provider response was lost")
	}
}
