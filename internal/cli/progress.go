package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"miocli/internal/theme"
)

type workDone struct{ err error }

type workModel struct {
	label      string
	spinner    spinner.Model
	results    <-chan error
	cancel     context.CancelFunc
	cancelling bool
	done       bool
	err        error
	width      int
}

func (m workModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, func() tea.Msg { return workDone{<-m.results} })
}

func (m workModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case workDone:
		m.done, m.err = true, msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.cancel()
			m.cancelling = true
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m workModel) View() string {
	if m.done {
		return ""
	}
	if m.cancelling {
		return "Cancelling...\n"
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	const hint = " · Ctrl+C cancel"
	line := ansi.Truncate(m.spinner.View()+" "+theme.SafeText(m.label), max(1, width-len(hint)), "…") + hint
	return ansi.Truncate(line, width, "…") + "\n"
}

func runWork(cmd *cobra.Command, label string, work func(context.Context) error) error {
	noColor, err := promptNoColor(cmd)
	if err != nil {
		return err
	}
	if noColor || !isTerminal(cmd) {
		fmt.Fprintln(cmd.OutOrStdout(), label+"...")
		return work(cmd.Context())
	}
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	results := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		results <- work(ctx)
	}()
	s := spinner.New(spinner.WithSpinner(spinner.Line), spinner.WithStyle(theme.New(false).Title))
	model, runErr := tea.NewProgram(workModel{label: label, spinner: s, results: results, cancel: cancel},
		tea.WithContext(cmd.Context()), tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()), tea.WithoutSignalHandler()).Run()
	cancel()
	<-done
	if runErr != nil {
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		return fmt.Errorf("show setup progress: %w", runErr)
	}
	result, ok := model.(workModel)
	if !ok || !result.done {
		return fmt.Errorf("setup progress ended before completion")
	}
	if result.cancelling {
		return context.Canceled
	}
	return result.err
}
