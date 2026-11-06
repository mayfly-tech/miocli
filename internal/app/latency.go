package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
)

const (
	delayTestURL   = "https://www.gstatic.com/generate_204"
	delayTimeoutMS = 5000
	delayWorkers   = 10
)

type latencyValue struct {
	delay int
	err   string
}

type delayResult struct {
	name  string
	delay int
	err   error
}

type latencyStartedMsg struct {
	id     uint64
	cancel context.CancelFunc
	ch     <-chan delayResult
	total  int
}

type latencyResultMsg struct {
	id     uint64
	result delayResult
}

type latencyFinishedMsg struct {
	id uint64
}

func (m *Model) startLatency() tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	groupName := m.currentGroup()
	group, ok := m.proxies.Items[groupName]
	if !ok {
		m.notice = "Select a group first"
		return nil
	}
	names := make([]string, 0, len(group.All))
	for _, name := range group.All {
		if child, found := m.proxies.Items[name]; found && isGroupType(child.Type) {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		m.notice = "This group has no direct nodes to test"
		return nil
	}
	m.cancelLatency()
	m.notice = "Starting latency test"
	m.lastError = ""
	return startLatencyCmd(m.ctx, m.client, m.latencyID, names)
}

func (m *Model) cancelLatency() {
	if m.latencyStop != nil {
		m.latencyStop()
		m.latencyStop = nil
	}
	m.latencyCh = nil
	m.latencyID++
	m.latency = make(map[string]latencyValue)
	m.latencyDone = 0
	m.latencySize = 0
}

func startLatencyCmd(ctx context.Context, client mihomo.Client, id uint64, names []string) tea.Cmd {
	targets := make([]delayTarget, len(names))
	for i, name := range names {
		targets[i] = delayTarget{key: name, name: name}
	}
	return startTargetLatencyCmd(ctx, client, id, targets)
}

type delayTarget struct{ key, name, provider string }

func startTargetLatencyCmd(ctx context.Context, client mihomo.Client, id uint64, targets []delayTarget) tea.Cmd {
	return func() tea.Msg {
		taskCtx, cancel := context.WithCancel(ctx)
		results := make(chan delayResult, delayWorkers)
		jobs := make(chan delayTarget)
		var workers sync.WaitGroup
		workers.Add(min(delayWorkers, len(targets)))
		for range min(delayWorkers, len(targets)) {
			go func() {
				defer workers.Done()
				for target := range jobs {
					requestCtx, stop := context.WithTimeout(taskCtx, 5*time.Second)
					var delay int
					var err error
					if target.provider != "" {
						if providerClient, ok := client.(mihomo.ProviderDelayClient); ok {
							delay, err = providerClient.DelayProvider(requestCtx, target.provider, target.name, delayTestURL, delayTimeoutMS)
						} else {
							err = &mihomo.APIError{Kind: mihomo.ErrorNotFound, Op: "provider delay"}
						}
					} else {
						delay, err = client.Delay(requestCtx, target.name, delayTestURL, delayTimeoutMS)
					}
					stop()
					select {
					case results <- delayResult{name: target.key, delay: delay, err: err}:
					case <-taskCtx.Done():
						return
					}
				}
			}()
		}
		go func() {
			defer close(jobs)
			for _, target := range targets {
				select {
				case jobs <- target:
				case <-taskCtx.Done():
					return
				}
			}
		}()
		go func() {
			workers.Wait()
			close(results)
		}()
		return latencyStartedMsg{id: id, cancel: cancel, ch: results, total: len(targets)}
	}
}

func nextLatencyCmd(id uint64, ch <-chan delayResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-ch
		if !ok {
			return latencyFinishedMsg{id: id}
		}
		return latencyResultMsg{id: id, result: result}
	}
}

func latencyErrorLabel(err error) string {
	var apiErr *mihomo.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case mihomo.ErrorTimeout:
			return "Timeout"
		case mihomo.ErrorNotFound:
			return "No API"
		case mihomo.ErrorAuth:
			return "Auth error"
		}
	}
	return "Failed"
}

func (m *Model) updateLatency(message tea.Msg) tea.Cmd {
	switch msg := message.(type) {
	case latencyStartedMsg:
		if msg.id != m.latencyID {
			msg.cancel()
			return nil
		}
		m.latencyStop = msg.cancel
		m.latencyCh = msg.ch
		m.latencySize = msg.total
		m.latencyDone = 0
		m.latency = make(map[string]latencyValue, msg.total)
		if msg.total == 0 {
			msg.cancel()
			m.latencyStop = nil
			m.notice = "This group has no direct nodes to test"
			return nil
		}
		m.notice = "Testing: 0/" + fmt.Sprint(msg.total)
		return nextLatencyCmd(msg.id, msg.ch)
	case latencyResultMsg:
		if msg.id != m.latencyID {
			return nil
		}
		m.latencyDone++
		value := latencyValue{delay: msg.result.delay}
		if msg.result.err != nil {
			value.err = latencyErrorLabel(msg.result.err)
		}
		m.latency[msg.result.name] = value
		m.notice = fmt.Sprintf("Testing: %d/%d", m.latencyDone, m.latencySize)
		return nextLatencyCmd(msg.id, m.latencyCh)
	case latencyFinishedMsg:
		if msg.id != m.latencyID {
			return nil
		}
		if m.latencyStop != nil {
			m.latencyStop()
			m.latencyStop = nil
		}
		m.latencyCh = nil
		m.notice = fmt.Sprintf("Test complete: %d/%d", m.latencyDone, m.latencySize)
		return nil

	}
	return nil
}

func (m *Model) testNodes(all bool) tea.Cmd {
	if m.operation != "" {
		m.notice = "Wait for the current operation to finish"
		return nil
	}
	rows := m.resourceRows()
	if len(rows) == 0 {
		m.notice = "No nodes to test"
		return nil
	}
	if !all {
		rows = rows[m.resourceIndex(rows) : m.resourceIndex(rows)+1]
	}
	targets := make([]delayTarget, 0, len(rows))
	nodes := m.nodes()
	for _, row := range rows {
		for _, node := range nodes {
			if row.id != node.id {
				continue
			}
			target := delayTarget{key: node.id, name: node.proxy.Name}
			if _, global := m.proxies.Items[node.proxy.Name]; !global && len(node.sources) == 1 {
				target.provider = node.sources[0]
			}
			targets = append(targets, target)
			break
		}
	}
	if len(targets) == 0 {
		m.lastError = "No nodes to test"
		return nil
	}
	m.cancelLatency()
	return startTargetLatencyCmd(m.ctx, m.client, m.latencyID, targets)
}
