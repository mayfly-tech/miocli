package app

import (
	"context"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
)

const refreshInterval = 5 * time.Second

type bootstrapMsg struct {
	version    mihomo.Version
	proxies    mihomo.Proxies
	mode       mihomo.Mode
	versionErr error
	proxiesErr error
	modeErr    error
}

type refreshMsg struct {
	proxies    mihomo.Proxies
	mode       mihomo.Mode
	proxiesErr error
	modeErr    error
	generation uint64
}

type pollTickMsg struct{ serial uint64 }

func bootstrapCmd(ctx context.Context, client mihomo.Client) tea.Cmd {
	return func() tea.Msg {
		version, err := client.Version(ctx)
		if err != nil {
			return bootstrapMsg{versionErr: err}
		}
		proxies, mode, proxiesErr, modeErr := readState(ctx, client)
		return bootstrapMsg{
			version:    version,
			proxies:    proxies,
			mode:       mode,
			proxiesErr: proxiesErr,
			modeErr:    modeErr,
		}
	}
}

func refreshGenerationCmd(ctx context.Context, client mihomo.Client, generation uint64) tea.Cmd {
	return func() tea.Msg {
		proxies, mode, proxiesErr, modeErr := readState(ctx, client)
		return refreshMsg{
			proxies:    proxies,
			mode:       mode,
			proxiesErr: proxiesErr,
			modeErr:    modeErr,
			generation: generation,
		}
	}
}

func readState(ctx context.Context, client mihomo.Client) (mihomo.Proxies, mihomo.Mode, error, error) {
	var (
		proxies    mihomo.Proxies
		mode       mihomo.Mode
		proxiesErr error
		modeErr    error
		wg         sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		proxies, proxiesErr = client.Proxies(ctx)
	}()
	go func() {
		defer wg.Done()
		mode, modeErr = client.Mode(ctx)
	}()
	wg.Wait()
	return proxies, mode, proxiesErr, modeErr
}

func (m *Model) schedulePoll(delay time.Duration) tea.Cmd {
	m.pollSerial++
	serial := m.pollSerial
	return tea.Tick(delay, func(time.Time) tea.Msg { return pollTickMsg{serial: serial} })
}

func (m *Model) startRefresh() tea.Cmd {
	if m.polling && m.pollGeneration == m.generation {
		return nil
	}
	m.polling = true
	m.pollGeneration = m.generation
	return tea.Batch(refreshGenerationCmd(m.ctx, m.client, m.generation), m.loadConnections())
}

func (m *Model) updateRefresh(message tea.Msg) tea.Cmd {
	switch msg := message.(type) {
	case bootstrapMsg:
		if msg.versionErr != nil {
			m.state = stateError
			m.lastError = messageForError(msg.versionErr)
			return nil
		}
		m.version = msg.version
		if msg.proxiesErr != nil {
			m.state = stateError
			m.lastError = messageForError(msg.proxiesErr)
			return nil
		}
		m.applyProxies(msg.proxies)
		m.mode = msg.mode
		m.modeError = ""
		if msg.modeErr != nil {
			m.modeError = messageForError(msg.modeErr)
		}
		m.state = stateReady
		m.lastError = ""
		m.refreshError = ""
		m.lastRefresh = time.Now()
		return m.schedulePoll(refreshInterval)
	case refreshMsg:
		if m.polling && msg.generation != m.pollGeneration {
			return nil
		}
		m.polling = false
		if msg.generation != m.generation {
			return m.schedulePoll(refreshInterval)
		}
		if msg.modeErr == nil {
			m.mode = msg.mode
			m.modeError = ""
		} else {
			m.modeError = messageForError(msg.modeErr)
		}
		if msg.proxiesErr == nil {
			m.applyProxies(msg.proxies)
			if m.refreshError != "" {
				m.notice = "Connection restored"
			}
			m.refreshError = ""
			m.pollRetries = 0
			m.lastRefresh = time.Now()
		} else {
			m.refreshError = messageForError(msg.proxiesErr)
			if transientError(msg.proxiesErr) {
				if m.pollRetries < 3 {
					delay := time.Duration(1<<m.pollRetries) * time.Second
					m.pollRetries++
					return m.schedulePoll(delay)
				}
			} else {
				m.pollRetries = 3
			}
		}
		if m.pollRetries >= 3 && msg.proxiesErr != nil {
			m.notice = "Reconnect paused; press r to retry"
			return nil
		}
		return m.schedulePoll(refreshInterval)
	case pollTickMsg:
		if msg.serial != m.pollSerial {
			return nil
		}
		if m.operation != "" {
			return m.schedulePoll(refreshInterval)
		}
		if !m.polling && m.state == stateReady {
			return tea.Batch(m.startRefresh(), m.loadProviders())
		}
		return nil

	}
	return nil
}
