package app

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"miocli/internal/mihomo"
	"miocli/internal/theme"
)

type dataState uint8

const (
	stateLoading dataState = iota
	stateReady
	stateError
)

type Model struct {
	client mihomo.Client
	ctx    context.Context
	cancel context.CancelFunc
	server string

	state             dataState
	version           mihomo.Version
	proxies           mihomo.Proxies
	groups            []string
	groupIndex        int
	groupPath         []string
	selectedGroupPath []string
	parentRows        []int
	rowIndex          int
	focusRight        bool
	mode              mihomo.Mode
	modeError         string
	lastError         string
	refreshError      string
	notice            string
	operation         string
	help              bool
	noColor           bool
	width             int
	height            int
	polling           bool
	pollRetries       int
	pollSerial        uint64
	pollGeneration    uint64
	runtime           Runtime
	page              resourcePage
	command           textinput.Model
	commanding        bool
	providers         map[string]mihomo.Provider
	providerErr       string
	providerBusy      bool
	providerSerial    uint64
	connections       []mihomo.Connection
	connectionsLoaded bool
	connectionsBusy   bool
	connectionsError  string
	filterType        string
	filterSource      string
	sortOrder         int
	resourceIDs       map[resourcePage]string
	detail            bool
	detailOffset      int
	lastRefresh       time.Time
	generation        uint64

	search      textinput.Model
	searching   bool
	latencyID   uint64
	latencyStop context.CancelFunc
	latencyCh   <-chan delayResult
	latency     map[string]latencyValue
	latencyDone int
	latencySize int
}

func New(ctx context.Context, client mihomo.Client, server string, noColor bool, options ...Option) *Model {
	root, cancel := context.WithCancel(ctx)
	search := theme.Input(noColor)
	search.Prompt = "/ "
	search.Placeholder = "Search names"
	search.CharLimit = 128
	search.Width = 32
	command := theme.Input(noColor)
	command.Prompt = ": "
	command.Placeholder = "groups · nodes · providers · status · help"
	command.CharLimit = 128
	model := &Model{
		client:      client,
		ctx:         root,
		cancel:      cancel,
		server:      server,
		state:       stateLoading,
		search:      search,
		noColor:     noColor,
		latency:     make(map[string]latencyValue),
		command:     command,
		resourceIDs: make(map[resourcePage]string),
	}
	for _, option := range options {
		option(model)
	}
	return model
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(bootstrapCmd(m.ctx, m.client), m.loadProviders(), m.loadConnections())
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.command.Width = max(1, msg.Width-14)
		if m.width > 4 {
			m.search.Width = max(1, m.width-35)
		}
		return m, nil
	case connectionsMsg, providersMsg:
		return m, m.updateResources(message)
	case bootstrapMsg, refreshMsg, pollTickMsg:
		return m, m.updateRefresh(message)
	case selectMsg, setModeMsg:
		return m, m.updateOperation(message)
	case latencyStartedMsg, latencyResultMsg, latencyFinishedMsg:
		return m, m.updateLatency(message)
	case tea.KeyMsg:
		return m, m.updateKey(msg)
	}
	return m, nil
}

func (m *Model) shutdown() {
	m.cancelLatency()
	m.cancel()
}

type Runtime struct {
	ConfigPath string
}

type Option func(*Model)

func WithRuntime(runtime Runtime) Option {
	return func(m *Model) { m.runtime = runtime }
}
