package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
)

type providersMsg struct {
	items      map[string]mihomo.Provider
	err        error
	writeErr   error
	updated    bool
	generation uint64
	serial     uint64
}

func (m *Model) loadProviders() tea.Cmd {
	if m.providerBusy || m.operation != "" {
		return nil
	}
	client, ok := m.client.(mihomo.ProviderClient)
	if !ok {
		m.providerErr = "Provider API unavailable for this client"
		return nil
	}
	m.providerBusy = true
	m.providerSerial++
	ctx, generation, serial := m.ctx, m.generation, m.providerSerial
	return func() tea.Msg {
		items, err := client.Providers(ctx)
		return providersMsg{items: items, err: err, generation: generation, serial: serial}
	}
}

func (m *Model) updateProvider() tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	rows := m.resourceRows()
	if len(rows) == 0 {
		m.notice = "No provider to update"
		return nil
	}
	client, ok := m.client.(mihomo.ProviderClient)
	if !ok {
		m.lastError = "Provider API unavailable for this client"
		return nil
	}
	name := rows[m.resourceIndex(rows)].id
	m.beginOperation("Updating provider")
	m.providerBusy = true
	m.providerSerial++
	ctx, generation, serial := m.ctx, m.generation, m.providerSerial
	return func() tea.Msg {
		writeErr := client.UpdateProvider(ctx, name)
		items, err := client.Providers(ctx)
		return providersMsg{items: items, err: err, writeErr: writeErr, updated: true, generation: generation, serial: serial}
	}
}

func (m *Model) updateResources(message tea.Msg) tea.Cmd {
	switch msg := message.(type) {
	case connectionsMsg:
		m.connectionsBusy = false
		if msg.generation != m.generation {
			return m.loadConnections()
		}
		if msg.err != nil {
			m.connectionsError = messageForError(msg.err)
		} else {
			m.connections, m.connectionsLoaded, m.connectionsError = msg.items, true, ""
		}
		return nil
	case providersMsg:
		if msg.serial != m.providerSerial {
			return nil
		}
		m.providerBusy = false
		if msg.generation != m.generation {
			return m.loadProviders()
		}
		if msg.err != nil {
			m.providerErr = messageForError(msg.err)
		} else {
			m.providers = msg.items
			m.providerErr = ""
		}
		if msg.updated {
			m.operation = ""
			if msg.writeErr != nil {
				m.lastError = "Provider update unconfirmed: " + messageForError(msg.writeErr)
			} else if msg.err != nil {
				m.lastError = "Update sent, but provider readback failed: " + m.providerErr
			} else {
				m.notice = "Provider update completed; status refreshed"
				m.lastError = ""
			}
			m.pollRetries = 0
			return m.startRefresh()
		}
		return nil

	}
	return nil
}
