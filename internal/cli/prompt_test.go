package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
	"miocli/internal/theme"
)

func TestChooserNavigationAndSafeConfirmation(t *testing.T) {
	c := chooser{label: "Confirm write?", choices: []choice{{"n", "Cancel"}, {"y", "Confirm"}}, noColor: true}
	model, _ := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model.(chooser).choices[model.(chooser).index].value != "n" {
		t.Fatal("default confirmation authorizes a write")
	}
	model, _ = c.Update(tea.KeyMsg{Type: tea.KeyRight})
	c = model.(chooser)
	model, _ = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !model.(chooser).done || model.(chooser).index != 1 {
		t.Fatal("explicit confirmation not selected")
	}
	model, _ = c.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !errors.Is(model.(chooser).err, errBack) {
		t.Fatal("escape did not return")
	}
	model, _ = c.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !errors.Is(model.(chooser).err, context.Canceled) {
		t.Fatal("ctrl+c did not cancel")
	}
}

func TestChooserResizingNeverAllowsHiddenConfirmation(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {50, 12}, {40, 8}, {20, 4}} {
		c := chooser{label: strings.Repeat("Important authorization warning. ", 8), choices: []choice{{"n", "Cancel"}, {"y", "Confirm"}}, index: 1, noColor: true, width: size[0], height: size[1]}
		view := c.View()
		if len(strings.Split(strings.TrimSuffix(view, "\n"), "\n")) > size[1] {
			t.Fatal("prompt exceeds height")
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("prompt exceeds width: %q", line)
			}
		}
		model, _ := c.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if !c.fits() && model.(chooser).done {
			t.Fatal("hidden confirmation was accepted")
		}
	}
}

func TestPromptNoColorAndSecretCompletion(t *testing.T) {
	input := theme.Input(true)
	input.EchoMode = textinput.EchoPassword
	input.SetValue("never-print-secret")
	input.Focus()
	q := question{label: "Secret", input: input, noColor: true, width: 50, height: 12}
	for _, done := range []bool{false, true} {
		q.done = done
		if view := q.View(); strings.Contains(view, "never-print-secret") || strings.Contains(view, "\x1b") {
			t.Fatal("secret or style leaked", view)
		}
	}
	q.done = false
	q.label = strings.Repeat("Long field explanation ", 80)
	if !strings.Contains(q.View(), "****") {
		t.Fatal("long label hid the input field")
	}
}

func TestStaticWorkPropagatesFailureAndProgressCancellation(t *testing.T) {
	cmd := newCommand(defaultDependencies())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	if err := cmd.ParseFlags(nil); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("fixture failure")
	err := runWork(cmd, "Checking fixture", func(context.Context) error { return failure })
	if !errors.Is(err, failure) || !strings.Contains(out.String(), "Checking fixture...") || strings.Contains(out.String(), "\x1b") {
		t.Fatal("static progress hid failure or emitted animation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := workModel{spinner: spinner.New(), cancel: cancel}
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if ctx.Err() == nil || !model.(workModel).cancelling {
		t.Fatal("progress did not cancel work")
	}
	model, _ = model.Update(workDone{err: failure})
	if !model.(workModel).done || !errors.Is(model.(workModel).err, failure) || model.View() != "" {
		t.Fatal("progress did not finish cleanly")
	}
	resized, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if lipgloss.Width(strings.TrimSuffix(resized.View(), "\n")) > 40 {
		t.Fatal("progress exceeds the window")
	}
}

func TestHelpAndVersionRemainPlainText(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"setup", "--help"}, {"doctor", "--help"}, {"dev", "--help"}, {"version"}} {
		output, err := commandTest(t, defaultDependencies(), args...)
		if err != nil || strings.Contains(output, "\x1b") {
			t.Fatalf("%v: %v, %q", args, err, output)
		}
		if args[len(args)-1] == "--help" && (!strings.Contains(output, "Usage:") || !strings.Contains(output, "Examples:")) {
			t.Fatalf("help lacks usage/examples: %s", output)
		}
	}
}

func TestColorPolicyReachesPanel(t *testing.T) {
	for _, tc := range []struct {
		env, term string
		flag      bool
	}{
		{"", "xterm", false}, {"1", "xterm", false}, {"", "dumb", false}, {"", "xterm", true},
	} {
		path := isolatedConfig(t)
		t.Setenv("NO_COLOR", tc.env)
		t.Setenv("TERM", tc.term)
		if err := config.Save(path, config.Config{Server: config.DefaultServer, Core: config.Core{Mode: config.Connect}}); err != nil {
			t.Fatal(err)
		}
		deps := defaultDependencies()
		deps.terminal = func(*cobra.Command) bool { return true }
		deps.probe = func(context.Context, config.Config) error { return nil }
		called := false
		deps.ui = func(_ context.Context, _ config.Config, noColor bool, _ app.Runtime) error {
			called = true
			if noColor != (tc.flag || tc.env != "" || tc.term == "dumb") {
				t.Fatal("color policy not passed to panel")
			}
			return nil
		}
		args := []string{"--config", path}
		if tc.flag {
			args = append(args, "--no-color")
		}
		if _, err := commandTest(t, deps, args...); err != nil || !called {
			t.Fatalf("panel: %v", err)
		}
	}
}
