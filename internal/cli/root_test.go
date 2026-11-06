package cli

import (
	"bytes"
	"context"
	"encoding/json"
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
	for _, mode := range []string{"connect", "address-override", "dev"} {
		t.Run(mode, func(t *testing.T) {
			path := isolatedConfig(t)
			cfg := config.Config{Server: config.DefaultServer}
			args := []string{"--config", path}
			if mode == "address-override" {
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
			deps.probe = func(context.Context, config.Config) error { return nil }
			called := false
			deps.ui = func(_ context.Context, _ config.Config, _ bool, runtime app.Runtime) error {
				if runtime.ConfigPath != path {
					t.Fatal("incorrect runtime configuration")
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

func TestDevReadsLocalFileAndRejectsUnknownFields(t *testing.T) {
	path := isolatedConfig(t)
	if err := config.Save(path, config.Config{Server: "https://user.example"}); err != nil {
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
	if err := os.WriteFile(filepath.Join(dir, "config.local.yaml"), []byte("unknown-setting: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, deps, "dev"); err == nil || !strings.Contains(err.Error(), "field unknown-setting not found") {
		t.Fatalf("dev configuration error = %v", err)
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
	answers := []string{"1", "http://127.0.0.1:19090", "entered-secret", "y"}
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
	if err != nil || cfg.Secret != "entered-secret" || len(answers) != 0 {
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

func TestSetupImportsOnlyConnectionAndKeepsSourceReadOnly(t *testing.T) {
	path := isolatedConfig(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "source.yaml")
	data := []byte("external-controller: 127.0.0.1:19090\nsecret: fixture-secret\nmode: rule\ntun:\n  enable: true\n")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	answers := []string{"2", source, "http://127.0.0.1:19090", "fixture-secret", "y"}
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
	deps.probe = func(_ context.Context, cfg config.Config) error {
		if cfg.Server != "http://127.0.0.1:19090" || cfg.Secret != "fixture-secret" {
			t.Fatal("import lost connection details")
		}
		return nil
	}
	output, err := commandTest(t, deps, "setup")
	if err != nil || len(answers) != 0 || strings.Contains(output, "fixture-secret") {
		t.Fatalf("setup = %q, %v, unused answers %v", output, err, answers)
	}
	after, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("setup changed source configuration")
	}
	saved, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(saved), "tun:") || strings.Contains(string(saved), "mode:") || strings.Contains(string(saved), "core:") {
		t.Fatalf("setup saved more than connection details: %v", err)
	}
}

func TestExplicitServerFlagOverridesConnection(t *testing.T) {
	path := isolatedConfig(t)
	if err := config.Save(path, config.Config{Server: config.DefaultServer}); err != nil {
		t.Fatal(err)
	}
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.probe = func(_ context.Context, cfg config.Config) error {
		if cfg.Server != "http://127.0.0.1:19090" {
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
	if err := config.Save(path, config.Config{Server: config.DefaultServer, Secret: "never-print-me"}); err != nil {
		t.Fatal(err)
	}
	deps := defaultDependencies()
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
	for _, section := range []string{"Configuration", "Controller", "OK: /version"} {
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

func TestProfileUseReloadsOnlyLocalController(t *testing.T) {
	path := isolatedConfig(t)
	profileDir := filepath.Join(filepath.Dir(path), "profiles")
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(profileDir, "home.yaml")
	if err := os.WriteFile(profilePath, []byte("mode: rule\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "profiles.yaml"), []byte("- name: home\n  url: https://subscriptions.example/home\n  updated-at: 2026-01-01T00:00:00Z\n"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/configs" || r.URL.Query().Get("force") != "true" {
			t.Errorf("reload request = %s %s", r.Method, r.URL.String())
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if filepath.Dir(body["path"]) != profileDir || body["path"] == profilePath {
			t.Errorf("reload must use a separate effective profile: %q", body["path"])
		}
		data, err := os.ReadFile(body["path"])
		if err != nil || !strings.Contains(string(data), "external-controller:") {
			t.Errorf("effective profile missing controller: %q, %v", data, err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := config.Save(path, config.Config{Server: server.URL}); err != nil {
		t.Fatal(err)
	}
	output, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home")
	if err != nil || !strings.Contains(output, "reload requested") {
		t.Fatalf("profile use = %q, %v", output, err)
	}

	if err := config.Save(path, config.Config{Server: "https://controller.example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "profile", "use", "home"); err == nil || !strings.Contains(err.Error(), "local Mihomo controller") {
		t.Fatalf("remote profile use error = %v", err)
	}
}

func TestSystemProxyRejectsRemoteControllerBeforeReadingRuntime(t *testing.T) {
	path := isolatedConfig(t)
	if err := config.Save(path, config.Config{Server: "https://controller.example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := commandTest(t, defaultDependencies(), "--config", path, "system-proxy", "enable"); err == nil || !strings.Contains(err.Error(), "local Mihomo controller") {
		t.Fatalf("remote system proxy error = %v", err)
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
