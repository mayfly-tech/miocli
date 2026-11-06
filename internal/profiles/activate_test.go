package profiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"miocli/internal/config"
)

type reloadFunc func(context.Context, string) error

func (f reloadFunc) ReloadConfig(ctx context.Context, path string) error { return f(ctx, path) }

func TestActivationRetainsUncertainReloadAndPreservesRestartFile(t *testing.T) {
	for _, failure := range []error{errors.New("reload rejected"), context.Canceled, context.DeadlineExceeded} {
		t.Run(failure.Error(), func(t *testing.T) {
			settings := filepath.Join(t.TempDir(), "settings.yaml")
			manager := New(settings)
			if err := manager.writeProfile("home", []byte("mode: rule\n")); err != nil {
				t.Fatal(err)
			}
			if err := manager.write([]Info{{Name: "home", URL: "https://example.test/profile"}}); err != nil {
				t.Fatal(err)
			}
			restart := filepath.Join(t.TempDir(), "restart.yaml")
			original := []byte("mode: direct\n")
			if err := config.WritePrivate(restart, original); err != nil {
				t.Fatal(err)
			}
			var effective string
			cfg := config.Config{Server: config.DefaultServer, Secret: "private", ActiveProfilePath: restart}
			err := manager.Activate(context.Background(), "home", settings, cfg, reloadFunc(func(_ context.Context, path string) error {
				effective = path
				data, err := os.ReadFile(path)
				if err != nil || !strings.Contains(string(data), "secret: private") {
					t.Fatalf("effective file = %q, %v", data, err)
				}
				return failure
			}))
			if !errors.Is(err, failure) || !strings.Contains(err.Error(), "effective profile retained") {
				t.Fatalf("activation error = %v", err)
			}
			if _, err := os.Stat(effective); err != nil {
				t.Fatalf("uncertain reload lost file: %v", err)
			}
			saved, err := os.ReadFile(restart)
			if err != nil || string(saved) != string(original) {
				t.Fatalf("restart file changed: %q, %v", saved, err)
			}
		})
	}
}
