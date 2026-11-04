package cli

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
	"miocli/internal/core"
)

func isolatedConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	t.Setenv("MIOCLI_SERVER", "")
	t.Setenv("MIOCLI_SECRET", "")
	path, err := config.DefaultPath(os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func commandTest(t *testing.T, deps dependencies, args ...string) (string, error) {
	t.Helper()
	var output bytes.Buffer
	deps.choose = func(cmd *cobra.Command, label, initial string, _ []choice) (string, error) {
		return deps.ask(cmd, label, initial, false)
	}
	cmd := newCommand(deps)
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return output.String(), err
}

func TestConnectionEntrypointsNeverManageCore(t *testing.T) {
	for _, mode := range []string{"connect", "managed-override", "dev"} {
		t.Run(mode, func(t *testing.T) {
			path := isolatedConfig(t)
			cfg := config.Config{Server: config.DefaultServer, Core: config.Core{Mode: config.Connect}}
			args := []string{"--config", path}
			if mode == "managed-override" {
				cfg.Core.Mode = config.Managed
				t.Setenv("MIOCLI_SERVER", cfg.Server)
			}
			if mode == "dev" {
				args = append([]string{"dev"}, args...)
			}
			if err := config.Save(path, cfg); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(path)
			deps := defaultDependencies()
			deps.terminal = func(*cobra.Command) bool { return true }
			deps.start = func(context.Context, config.Config, core.Paths) (*core.Manager, error) {
				t.Fatal("connect path started a core")
				return nil, nil
			}
			deps.probe = func(context.Context, config.Config) error { return nil }
			called := false
			deps.ui = func(_ context.Context, _ config.Config, _ bool, runtime app.Runtime) error {
				if runtime.Owned {
					t.Fatal("connection mode acquired core ownership")
				}
				called = true
				return nil
			}
			if _, err := commandTest(t, deps, args...); err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(path)
			if !called || !bytes.Equal(before, after) {
				t.Fatal("UI not called or connection config changed")
			}
			deps.probe = func(context.Context, config.Config) error {
				return errors.New("connection failed")
			}
			if _, err := commandTest(t, deps, args...); err == nil {
				t.Fatal("connection failure was hidden")
			}
		})
	}
}

func TestDevReadsLocalFileAndRejectsManaged(t *testing.T) {
	path := isolatedConfig(t)
	if err := config.Save(path, config.Config{Server: "https://user.example", Core: config.Core{Mode: config.Managed}}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod": "module miocli\n", "config.local.example.yaml": "secret: \"\"\n",
		"config.local.yaml": "server: http://127.0.0.1:19090\nsecret: local\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.probe = func(_ context.Context, cfg config.Config) error {
		if cfg.Server != "http://127.0.0.1:19090" || cfg.Secret != "local" {
			t.Fatalf("dev loaded user config: %#v", cfg)
		}
		return nil
	}
	deps.ui = func(context.Context, config.Config, bool, app.Runtime) error { return nil }
	if _, err := commandTest(t, deps, "dev"); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(filepath.Join(dir, "config.local.yaml"), config.Config{Server: config.DefaultServer, Core: config.Core{Mode: config.Managed}}); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, deps, "dev"); err == nil || !strings.Contains(err.Error(), "dev only connects") {
		t.Fatalf("dev managed error = %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "config.local.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, deps, "dev"); err == nil || !strings.Contains(err.Error(), "copy config.local.example.yaml") {
		t.Fatalf("missing dev config error = %v", err)
	}
}

func TestSetupSavesEnteredValuesNotEnvironment(t *testing.T) {
	path := isolatedConfig(t)
	t.Setenv("MIOCLI_SERVER", "https://override.example")
	t.Setenv("MIOCLI_SECRET", "environment-secret")
	answers := []string{"1", "1", "http://127.0.0.1:19090", "entered-secret", "y"}
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.ask = func(_ *cobra.Command, label, initial string, secret bool) (string, error) {
		if initial == "environment-secret" || initial == "https://override.example" {
			t.Fatal("environment override used as setup default")
		}
		if len(answers) == 0 {
			t.Fatalf("unexpected question: %s", label)
		}
		if strings.Contains(label, "Controller secret") && !secret {
			t.Fatal("secret input is not hidden")
		}
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	deps.probe = func(_ context.Context, cfg config.Config) error {
		if cfg.Secret != "entered-secret" {
			t.Fatal("setup validated environment rather than entered secret")
		}
		return nil
	}
	output, err := commandTest(t, deps, "setup")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(config.Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil || cfg.Secret != "entered-secret" || cfg.Core.Mode != config.Connect || len(answers) != 0 {
		t.Fatalf("saved config = %#v, %v, answers %v", cfg, err, answers)
	}
	if strings.Contains(output, "entered-secret") || strings.Contains(output, "environment-secret") {
		t.Fatal("setup output leaked secrets")
	}
}

func TestSetupNonInteractiveAndCancellationDoNotWrite(t *testing.T) {
	path := isolatedConfig(t)
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return false }
	if _, err := commandTest(t, deps, "setup"); err == nil {
		t.Fatal("noninteractive setup succeeded")
	}
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.ask = func(*cobra.Command, string, string, bool) (string, error) { return "c", nil }
	if _, err := commandTest(t, deps, "setup"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancel wrote config: %v", err)
	}
}

func TestManagedDownloadDeclinedDoesNotInstallOrSave(t *testing.T) {
	path := isolatedConfig(t)
	dir := t.TempDir()
	t.Setenv("PATH", filepath.Join(dir, "empty"))
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	source := filepath.Join(dir, "source.yaml")
	if err := os.WriteFile(source, []byte("mode: rule\n"), 0600); err != nil {
		t.Fatal(err)
	}
	answers := []string{"2", "y", "http://127.0.0.1:19090", "1", source, "", "n", "c"}
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.ask = func(_ *cobra.Command, label, _ string, _ bool) (string, error) {
		if len(answers) == 0 {
			t.Fatalf("unexpected question: %s", label)
		}
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	if _, err := commandTest(t, deps, "setup"); !errors.Is(err, context.Canceled) {
		t.Fatalf("decline error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("declining download wrote app config")
	}
	paths, err := core.PathsFor(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.BinaryDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("declining download created binary directory")
	}
	if len(answers) != 0 {
		t.Fatalf("unused answers: %v", answers)
	}
}

func TestExplicitServerFlagDisablesManagedStartup(t *testing.T) {
	path := isolatedConfig(t)
	if err := config.Save(path, config.Config{Server: config.DefaultServer, Core: config.Core{Mode: config.Managed}}); err != nil {
		t.Fatal(err)
	}
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.start = func(context.Context, config.Config, core.Paths) (*core.Manager, error) {
		t.Fatal("explicit address started managed core")
		return nil, nil
	}
	deps.probe = func(_ context.Context, cfg config.Config) error {
		if cfg.Server != "http://127.0.0.1:19090" || cfg.Core.Mode != config.Connect {
			t.Fatalf("effective override = %#v", cfg)
		}
		return nil
	}
	deps.ui = func(context.Context, config.Config, bool, app.Runtime) error { return nil }
	if _, err := commandTest(t, deps, "--server", "http://127.0.0.1:19090"); err != nil {
		t.Fatal(err)
	}
}

func TestDoctorIsReadOnlyAndRedacted(t *testing.T) {
	path := isolatedConfig(t)
	if err := config.Save(path, config.Config{Server: config.DefaultServer, Secret: "never-print-me", Core: config.Core{Mode: config.Connect}}); err != nil {
		t.Fatal(err)
	}
	deps := defaultDependencies()
	deps.start = func(context.Context, config.Config, core.Paths) (*core.Manager, error) {
		t.Fatal("doctor started a core")
		return nil, nil
	}
	deps.ui = func(context.Context, config.Config, bool, app.Runtime) error {
		t.Fatal("doctor entered TUI")
		return nil
	}
	deps.probe = func(context.Context, config.Config) error { return nil }
	before, _ := os.ReadFile(path)
	output, err := commandTest(t, deps, "doctor")
	if err != nil || !strings.Contains(output, "source:") || strings.Contains(output, "never-print-me") {
		t.Fatalf("doctor = %s, %v", output, err)
	}
	for _, section := range []string{"Configuration", "Core", "Controller", "OK: /version"} {
		if !strings.Contains(output, section) {
			t.Fatalf("doctor lacks section %q", section)
		}
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("doctor changed config")
	}
	deps.probe = func(context.Context, config.Config) error { return errors.New("offline") }
	if _, err := commandTest(t, deps, "doctor"); err == nil {
		t.Fatal("doctor hid failure")
	}
}

func TestProbeUsesOnlyRequiredReadAPIs(t *testing.T) {
	for _, broken := range []string{"", "/version", "/proxies", "/configs", "auth"} {
		t.Run(broken, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Error("probe sent a write request")
				}
				paths = append(paths, r.URL.Path)
				if broken == "auth" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if r.URL.Path == broken {
					fmtBody := `{}`
					_, _ = w.Write([]byte(fmtBody))
					return
				}
				switch r.URL.Path {
				case "/version":
					_, _ = w.Write([]byte(`{"version":"test","meta":true}`))
				case "/proxies":
					_, _ = w.Write([]byte(`{"proxies":{}}`))
				case "/configs":
					_, _ = w.Write([]byte(`{"mode":"rule"}`))
				default:
					t.Error("unexpected API")
				}
			}))
			defer server.Close()
			err := probeConnection(context.Background(), config.Config{Server: server.URL})
			if (err != nil) != (broken != "") {
				t.Fatalf("probe error = %v", err)
			}
			if broken == "" && len(paths) != 3 {
				t.Fatalf("probe paths = %v", paths)
			}
		})
	}
}

func TestQuestionHidesSecretsAndSupportsBackAndCancel(t *testing.T) {
	input := textinput.New()
	input.EchoMode = textinput.EchoPassword
	input.SetValue("hidden-value")
	q := question{label: "Secret", input: input}
	if strings.Contains(q.View(), "hidden-value") {
		t.Fatal("input leaked secret")
	}
	model, _ := q.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if strings.Contains(model.View(), "hidden-value") {
		t.Fatal("completed input leaked secret")
	}
	model, _ = q.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !errors.Is(model.(question).err, errBack) {
		t.Fatal("escape did not go back")
	}
	model, _ = q.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !errors.Is(model.(question).err, context.Canceled) {
		t.Fatal("ctrl+c did not cancel")
	}
}
