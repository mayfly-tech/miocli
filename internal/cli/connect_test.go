package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"miocli/internal/app"
	"miocli/internal/config"
)

func TestLegacyCoreRejectedWithoutTouchingData(t *testing.T) {
	for _, args := range [][]string{nil, {"doctor"}, {"setup"}, {"--server", "http://127.0.0.1:19090"}, {"dev"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			path := isolatedConfig(t)
			data := []byte("server: http://127.0.0.1:9090\ncore:\n  mode: managed\n  binary: /existing/mihomo\n")
			if err := config.WritePrivate(path, data); err != nil {
				t.Fatal(err)
			}
			oldData := filepath.Join(filepath.Dir(path), "mihomo", "config.yaml")
			if err := config.WritePrivate(oldData, []byte("secret: old-fixture\n")); err != nil {
				t.Fatal(err)
			}
			deps := defaultDependencies()
			deps.probe = func(context.Context, config.Config) error {
				t.Fatal("legacy config reached the API")
				return nil
			}
			deps.ui = func(context.Context, config.Config, bool, app.Runtime) error {
				t.Fatal("legacy config entered UI")
				return nil
			}
			deps.terminal = func(*cobra.Command) bool { return true }
			args = append(args, "--config", path)
			if _, err := commandTest(t, deps, args...); err == nil || !strings.Contains(err.Error(), "remove the core field") {
				t.Fatalf("legacy config error = %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, after) {
				t.Fatal("legacy config was rewritten")
			}
			if _, err := os.Stat(oldData); err != nil {
				t.Fatal("historical core data was deleted")
			}
		})
	}
}

func TestSetupDeclineAndFailureDoNotSave(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "decline", true: "offline"}[failure], func(t *testing.T) {
			path := isolatedConfig(t)
			answers := []string{"1", config.DefaultServer, "", "n", "c"}
			if failure {
				answers = []string{"1", config.DefaultServer, "", "c"}
			}

			deps := defaultDependencies()
			deps.terminal = func(*cobra.Command) bool { return true }
			deps.ask = func(_ *cobra.Command, label, _ string, _ bool) (string, error) {
				if len(answers) == 0 {
					t.Fatalf("unexpected setup question: %s", label)
				}
				value := answers[0]
				answers = answers[1:]
				return value, nil
			}
			deps.probe = func(context.Context, config.Config) error {
				if failure {
					return errors.New("offline")
				}
				return nil
			}
			output, err := commandTest(t, deps, "setup")
			if !errors.Is(err, context.Canceled) || len(answers) != 0 {
				t.Fatalf("cancel = %v, unused answers %v", err, answers)
			}
			if failure && !strings.Contains(output, "Connection failed: offline") {
				t.Fatal("setup hid failed connection")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("declined/failed setup wrote config")
			}
		})
	}
}

func TestUnavailableDefaultControllerOffersOnlyConnectionSetup(t *testing.T) {
	path := isolatedConfig(t)
	deps := defaultDependencies()
	deps.terminal = func(*cobra.Command) bool { return true }
	deps.probe = func(context.Context, config.Config) error { return errors.New("offline") }
	deps.ask = func(_ *cobra.Command, label, _ string, _ bool) (string, error) {
		if label != "Connection details" {
			t.Fatalf("unexpected startup setup question %q", label)
		}
		return "c", nil
	}
	deps.ui = func(context.Context, config.Config, bool, app.Runtime) error {
		t.Fatal("offline startup entered UI")
		return nil
	}
	output, err := commandTest(t, deps)
	if !errors.Is(err, context.Canceled) || !strings.Contains(output, "Controller unavailable: offline") {
		t.Fatalf("startup = %q, %v", output, err)
	}
	if !strings.Contains(output, "never installs, starts or stops cores") {
		t.Fatal("connection setup does not explain the lifecycle boundary")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled startup created settings")
	}
}
