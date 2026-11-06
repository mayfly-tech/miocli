package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "loopback IPv4", input: "http://127.0.0.1:9090/", want: "http://127.0.0.1:9090"},
		{name: "loopback IPv6", input: "http://[::1]:9090", want: "http://[::1]:9090"},
		{name: "localhost", input: "http://localhost:9090", want: "http://localhost:9090"},
		{name: "remote TLS", input: "https://mihomo.example:9090/", want: "https://mihomo.example:9090"},
		{name: "remote HTTP", input: "http://mihomo.example:9090", wantErr: "require HTTPS"},
		{name: "userinfo", input: "https://user:pass@example.com", wantErr: "credentials"},
		{name: "query", input: "https://example.com?token=x", wantErr: "query"},
		{name: "path", input: "https://example.com/api", wantErr: "path"},
		{name: "invalid port", input: "https://example.com:70000", wantErr: "port"},
		{name: "scheme", input: "ftp://example.com", wantErr: "scheme"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateServer(test.input)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("ValidateServer() error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateServer() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("ValidateServer() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLoadRejectsReadableSecretFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("secret: private\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
	if err == nil || !strings.Contains(err.Error(), "permissions 0600") {
		t.Fatalf("Load() error = %v, want permission error", err)
	}
}

func TestLoadPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("server: https://yaml.example:9090/\nsecret: yaml-secret\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"MIOCLI_SERVER": "https://env.example:9090",
		"MIOCLI_SECRET": "env-secret",
	}
	lookup := func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}

	cfg, err := Load(Overrides{
		ConfigPath: path,
		Server:     "https://flag.example:9090",
		ServerSet:  true,
		LookupEnv:  lookup,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "https://flag.example:9090" || cfg.Secret != "env-secret" {
		t.Fatalf("unexpected config: %#v", cfg)
	}

	cfg, err = Load(Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "https://yaml.example:9090" || cfg.Secret != "yaml-secret" {
		t.Fatalf("unexpected YAML config: %#v", cfg)
	}
}

func TestLoadRejectsUnknownFieldsAndMissingExplicitFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("server: http://127.0.0.1:9090\nsecrett: typo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
	if err == nil || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("Load() error = %v, want strict YAML parse error", err)
	}

	_, err = Load(Overrides{
		ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"),
		LookupEnv:  func(string) (string, bool) { return "", false },
	})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() error = %v, want os.ErrNotExist", err)
	}
}

func TestSaveWritesPrivateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	want := Config{Server: DefaultServer, Secret: "private"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(Overrides{
		ConfigPath: path,
		LookupEnv:  func(string) (string, bool) { return "", false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("config permissions = %04o, want 0600", info.Mode().Perm())
		}
	}
}

func TestDefaultPathRejectsRelativeXDGConfigHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses APPDATA")
	}

	_, err := DefaultPath(func(key string) (string, bool) {
		if key == "XDG_CONFIG_HOME" {
			return "relative/path", true
		}
		return "", false
	})
	if err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("DefaultPath() error = %v, want absolute-path error", err)
	}
}

func TestUnixServerAndPersistentProfileConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		if _, err := ValidateServer("unix:///tmp/control.sock"); err == nil {
			t.Fatal("Windows accepted Unix controller")
		}
		return
	}
	server := "unix:///tmp/control.sock"
	if got, err := ValidateServer(server); err != nil || got != server {
		t.Fatalf("Unix server = %q, %v", got, err)
	}
	if got, err := ValidateServer("unix:/tmp/control.sock"); err != nil || got != server {
		t.Fatalf("canonical Unix server = %q, %v", got, err)
	}
	for _, value := range []string{"unix://host/tmp/c.sock", "unix:relative", "unix:///", "unix:///tmp/../c.sock", "unix:///tmp/c.sock?", "unix:///tmp/%00.sock"} {
		if _, err := ValidateServer(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	path := filepath.Join(t.TempDir(), "miocli.yaml")
	want := Config{Server: server, ActiveProfilePath: "/etc/mihomo/config.yaml"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(Overrides{ConfigPath: path, LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil || cfg != want {
		t.Fatalf("persistent settings = %#v, %v", cfg, err)
	}
	want.ActiveProfilePath = "relative.yaml"
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(Overrides{ConfigPath: path}); err == nil {
		t.Fatal("accepted relative active-profile-path")
	}
}
