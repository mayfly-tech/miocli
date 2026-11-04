package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"

	"miocli/internal/theme"
)

var errBack = errors.New("return to setup menu")

type question struct {
	label         string
	input         textinput.Model
	done          bool
	err           error
	noColor       bool
	width, height int
}

func (q question) Init() tea.Cmd { return textinput.Blink }

func (q question) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		q.width, q.height = size.Width, size.Height
		q.input.Width = max(1, size.Width-4)
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.Type {
		case tea.KeyCtrlC:
			q.err = context.Canceled
			return q, tea.Quit
		case tea.KeyEsc:
			q.err = errBack
			return q, tea.Quit
		case tea.KeyEnter:
			if q.width > 0 && q.height > 0 && (q.width < 40 || q.height < 8) {
				return q, nil
			}
			q.done = true
			return q, tea.Quit
		}
	}
	var cmd tea.Cmd
	q.input, cmd = q.input.Update(msg)
	return q, cmd
}

func (q question) View() string {
	if q.err != nil {
		return ""
	}
	if q.done {
		value := theme.SafeText(q.input.Value())
		if q.input.EchoMode == textinput.EchoPassword {
			value = "[hidden]"
		}
		return theme.SafeText(q.label) + ": " + value + "\n"
	}
	styles := theme.New(q.noColor)
	width, height := q.width, q.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	labels := strings.Split(ansi.Wrap(theme.SafeText(q.label), width, ""), "\n")
	labels = labels[:min(len(labels), max(1, height-4))]
	for i := range labels {
		labels[i] = styles.Title.Render(labels[i])
	}
	lines := append(labels, q.input.View(), "", styles.Muted.Render("Enter continue · Esc menu · Ctrl+C cancel"))
	return promptView(lines, q.width, q.height)
}

func askTerminal(cmd *cobra.Command, label, initial string, secret bool) (string, error) {
	noColor, err := promptNoColor(cmd)
	if err != nil {
		return "", err
	}
	input := theme.Input(noColor)
	input.CharLimit = 64 << 10
	input.Width = 72
	input.SetValue(initial)
	if secret {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '*'
	}
	input.Focus()
	model, err := tea.NewProgram(question{label: label, input: input, noColor: noColor},
		tea.WithContext(cmd.Context()), tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()),
		tea.WithoutSignalHandler()).Run()
	if err != nil {
		if cmd.Context().Err() != nil {
			return "", cmd.Context().Err()
		}
		return "", fmt.Errorf("read setup input: %w", err)
	}
	result, ok := model.(question)
	if !ok {
		return "", errors.New("unexpected setup input state")
	}
	if result.err != nil {
		return "", result.err
	}
	if !result.done {
		return "", errors.New("setup input ended before confirmation")
	}
	if secret {
		return result.input.Value(), nil
	}
	return strings.TrimSpace(result.input.Value()), nil
}

type choice struct{ value, label string }

type chooser struct {
	label         string
	choices       []choice
	index         int
	done          bool
	err           error
	noColor       bool
	width, height int
}

func (c chooser) Init() tea.Cmd { return nil }

func (c chooser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			c.err = context.Canceled
			return c, tea.Quit
		case "esc":
			c.err = errBack
			return c, tea.Quit
		case "up", "left", "k", "h", "shift+tab":
			c.index = (c.index + len(c.choices) - 1) % len(c.choices)
		case "down", "right", "j", "l", "tab":
			c.index = (c.index + 1) % len(c.choices)
		case "enter":
			if !c.fits() {
				return c, nil
			}
			c.done = true
			return c, tea.Quit
		default:
			for i, option := range c.choices {
				if strings.EqualFold(msg.String(), option.value) {
					c.index = i
				}
			}
		}
	}
	return c, nil
}

func (c chooser) View() string {
	if c.err != nil {
		return ""
	}
	if c.done {
		return theme.SafeText(c.label) + ": " + theme.SafeText(c.choices[c.index].label) + "\n"
	}
	if !c.fits() {
		return promptView([]string{"Resize to review all options", "Esc menu · Ctrl+C cancel"}, c.width, c.height)
	}
	styles := theme.New(c.noColor)
	lines := []string{styles.Title.Render(theme.SafeText(c.label)), ""}
	for i, option := range c.choices {
		prefix := "  "
		if i == c.index {
			prefix = "> "
		}
		line := prefix + option.value + "  " + theme.SafeText(option.label)
		if i == c.index {
			line = styles.Selected.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", styles.Muted.Render("Up/Down choose · Enter confirm · Esc menu · Ctrl+C cancel"))
	return promptView(lines, c.width, c.height)
}

func (c chooser) fits() bool {
	width, height := c.width, c.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	if width < 40 || height < 8 {
		return false
	}
	count := len(strings.Split(ansi.Wrap(theme.SafeText(c.label), width, ""), "\n")) + 2
	for _, option := range c.choices {
		count += len(strings.Split(ansi.Wrap("  "+option.value+"  "+theme.SafeText(option.label), width, ""), "\n"))
	}
	count += len(strings.Split(ansi.Wrap("Up/Down choose · Enter confirm · Esc menu · Ctrl+C cancel", width, ""), "\n"))
	return count <= height
}

func promptView(lines []string, width, height int) string {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	if width < 40 || height < 8 {
		lines = []string{"Resize to at least 40 x 8", "Esc menu · Ctrl+C cancel"}
	}
	var result []string
	for _, line := range lines {
		result = append(result, strings.Split(ansi.Wrap(line, width, ""), "\n")...)
	}
	if len(result) > height {
		footer := result[len(result)-1]
		result = append(result[:max(0, height-1)], footer)
	}
	return strings.Join(result, "\n") + "\n"
}

func promptNoColor(cmd *cobra.Command) (bool, error) {
	flag, err := cmd.Flags().GetBool("no-color")
	return theme.NoColor(flag), err
}

func chooseTerminal(cmd *cobra.Command, label, initial string, choices []choice) (string, error) {
	if len(choices) == 0 {
		return "", errors.New("setup choice has no options")
	}
	noColor, err := promptNoColor(cmd)
	if err != nil {
		return "", err
	}
	model := chooser{label: label, choices: choices, noColor: noColor}
	for i, option := range choices {
		if option.value == initial {
			model.index = i
		}
	}
	result, err := tea.NewProgram(model, tea.WithContext(cmd.Context()), tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()), tea.WithoutSignalHandler()).Run()
	if err != nil {
		if cmd.Context().Err() != nil {
			return "", cmd.Context().Err()
		}
		return "", fmt.Errorf("read setup choice: %w", err)
	}
	selected, ok := result.(chooser)
	if !ok {
		return "", errors.New("unexpected setup choice state")
	}
	if selected.err != nil {
		return "", selected.err
	}
	if !selected.done {
		return "", errors.New("setup choice ended before confirmation")
	}
	return selected.choices[selected.index].value, nil
}
