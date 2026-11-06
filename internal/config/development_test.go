package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevelopmentPathFindsOnlyProjectRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module miocli\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.local.example.yaml"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "internal", "cli")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	got, err := DevelopmentPath(nested)
	if err != nil || got != filepath.Join(dir, "config.local.yaml") {
		t.Fatalf("DevelopmentPath() = %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module unrelated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DevelopmentPath(nested); err == nil {
		t.Fatal("accepted an unrelated checkout")
	}
}

func TestResolvePolicyAndSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server: http://127.0.0.1:19090\nsecret: file-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Resolve(Overrides{ConfigPath: path, LookupEnv: func(key string) (string, bool) {
		if key == "MIOCLI_SERVER" {
			return DefaultServer, true
		}
		return "", true
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Exists || !cfg.ExplicitServer || cfg.ServerSource != "MIOCLI_SERVER" || cfg.SecretSource != path || cfg.Secret != "file-secret" {
		t.Fatalf("unexpected resolution: %#v", cfg)
	}
}

func TestLoadDefaultAndInvalidPolicies(t *testing.T) {
	for _, test := range []struct {
		data string
		err  string
	}{
		{data: "server: http://127.0.0.1:9090\n"},
		{data: "unknown: true\n", err: "field unknown"},
		{data: "server: http://127.0.0.1:9090\n---\nsecret: other\n", err: "exactly one"},
	} {
		t.Run(test.data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.data), 0600); err != nil {
				t.Fatal(err)
			}

			cfg, err := Load(Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
			if test.err != "" {
				if err == nil || !strings.Contains(err.Error(), test.err) {
					t.Fatalf("error = %v, want %s", err, test.err)
				}
			} else if err != nil || cfg.Server != DefaultServer {
				t.Fatalf("default policy config = %#v, %v", cfg, err)
			}
		})
	}
}

func TestMissingFieldsKeepDefaultSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Resolve(Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerSource != "default" || cfg.SecretSource != "not set" {
		t.Fatalf("missing fields reported as file overrides: %#v", cfg)
	}
}
