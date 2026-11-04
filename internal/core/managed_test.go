package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"miocli/internal/config"
)

func TestMain(m *testing.M) {
	if os.Getenv("MIOCLI_FAKE_CORE") == "1" {
		os.Exit(fakeCore())
	}
	os.Exit(m.Run())
}

func fakeCore() int {
	if len(os.Args) > 1 && os.Args[1] == "-v" {
		fmt.Println("Mihomo test")
		return 0
	}
	dir := ""
	for i := 1; i+1 < len(os.Args); i++ {
		if os.Args[i] == "-d" {
			dir = os.Args[i+1]
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return 2
	}
	if strings.Contains(string(data), "fail-check: true") {
		fmt.Fprintln(os.Stderr, "test configuration failure")
		return 2
	}
	if len(os.Args) > 1 && os.Args[1] == "-t" {
		return 0
	}
	if strings.Contains(string(data), "fail-start: true") {
		fmt.Fprintln(os.Stderr, "test startup failure")
		return 2
	}
	var cfg struct {
		Controller string `yaml:"external-controller"`
		Secret     string `yaml:"secret"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return 2
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+cfg.Secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/version":
			fmt.Fprint(w, `{"meta":true,"version":"test"}`)
		case "/proxies":
			fmt.Fprint(w, `{"proxies":{}}`)
		case "/configs":
			fmt.Fprint(w, `{"mode":"rule"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if err := http.ListenAndServe(cfg.Controller, handler); err != nil {
		return 2
	}
	return 0
}

func availableServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return "http://" + address
}

func TestConnectionImportIsLimitedAndReadOnly(t *testing.T) {
	for _, test := range []struct {
		data string
		ok   bool
	}{
		{"external-controller: 127.0.0.1:19090\nsecret: chosen\nmixed-port: 7890\n", true},
		{"external-controller: 0.0.0.0:9090\n", false},
		{"external-controller-unix: /tmp/ignored.sock\n", false},
		{"external-controller: http://127.0.0.1:9090\n", false},
		{"external-controller: 127.0.0.1\n", false},
		{"external-controller: 127.0.0.1:9090\nsecret: {invalid: value}\n", false},
		{"external-controller: 127.0.0.1:9090\n---\nsecret: bad\n", false},
		{"secret: a\nsecret: b\n", false},
	} {
		t.Run(test.data, func(t *testing.T) {
			before := test.data
			cfg, err := ConnectionFromYAML([]byte(test.data))
			if (err == nil) != test.ok {
				t.Fatalf("import = %#v, %v", cfg, err)
			}
			if test.ok && (cfg.Core.Mode != config.Connect || cfg.Secret != "chosen" || cfg.Server != "http://127.0.0.1:19090") {
				t.Fatalf("connection = %#v", cfg)
			}
			if test.data != before {
				t.Fatal("source changed")
			}
		})
	}
}

func TestManagedCopyUsesChosenControllerAndRemovesExtraControlListeners(t *testing.T) {
	source := []byte("external-controller: 0.0.0.0:9090\nexternal-controller-unix: /tmp/old\nexternal-controller-tls: 0.0.0.0:9443\nsecret: old\nmixed-port: 7890\n")
	secret, data, err := PrepareManagedConfig(source, "http://127.0.0.1:19090", "new")
	if err != nil {
		t.Fatal(err)
	}
	if secret != "new" || !strings.Contains(string(data), "127.0.0.1:19090") || strings.Contains(string(data), "external-controller-unix") || strings.Contains(string(data), "external-controller-tls") {
		t.Fatalf("managed config = %s", data)
	}
	if !strings.Contains(string(source), "0.0.0.0:9090") {
		t.Fatal("modified source")
	}
	for _, address := range []string{"https://127.0.0.1:9090", "http://localhost:9090", "http://192.0.2.1:9090", "http://127.0.0.1"} {
		if err := ValidateManagedServer(address); err == nil {
			t.Fatalf("accepted managed address %s", address)
		}
	}
}

func TestManagedNeverStartsForAnOccupiedController(t *testing.T) {
	for _, response := range []string{"auth", "wrong-service", "valid"} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if response == "auth" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				if response == "wrong-service" {
					fmt.Fprint(w, `{"version":"not-mihomo"}`)
					return
				}
				switch r.URL.Path {
				case "/version":
					fmt.Fprint(w, `{"meta":true,"version":"test"}`)
				case "/proxies":
					fmt.Fprint(w, `{"proxies":{}}`)
				case "/configs":
					fmt.Fprint(w, `{"mode":"rule"}`)
				}
			}))
			defer server.Close()
			manager, err := EnsureRunning(context.Background(), config.Config{
				Server: server.URL, Core: config.Core{Mode: config.Managed, Binary: "/must-not-execute"},
			}, Paths{ConfigDir: "/must-not-read"})
			if manager != nil || (err == nil) != (response == "valid") {
				t.Fatalf("EnsureRunning = %v, %v", manager, err)
			}
		})
	}
}

func TestManagedSetupCommitsOnlyOnSuccess(t *testing.T) {
	t.Setenv("MIOCLI_FAKE_CORE", "1")
	for _, failure := range []string{"", "fail-check: true\n", "fail-start: true\n", "save", "profile"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			paths := Paths{AppConfig: filepath.Join(dir, "config.yaml"), ConfigDir: filepath.Join(dir, "mihomo")}
			if err := os.MkdirAll(paths.ConfigDir, 0700); err != nil {
				t.Fatal(err)
			}
			old := []byte("previous managed config\n")
			if err := os.WriteFile(filepath.Join(paths.ConfigDir, "config.yaml"), old, 0600); err != nil {
				t.Fatal(err)
			}
			oldApp := config.Config{Server: config.DefaultServer, Core: config.Core{Mode: config.Connect}}
			if err := config.Save(paths.AppConfig, oldApp); err != nil {
				t.Fatal(err)
			}
			cfg := config.Config{Server: availableServer(t), Core: config.Core{Mode: config.Managed, Binary: os.Args[0]}}
			source := "mode: rule\n"
			if failure != "save" && failure != "profile" {
				source += failure
			}
			secret, data, err := PrepareManagedConfig([]byte(source), cfg.Server, "")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Secret = secret
			if failure == "save" {
				paths.AppConfig = paths.ConfigDir
			}
			if failure == "profile" {
				err = SetupManaged(context.Background(), cfg, paths, data, func() error { return errors.New("profile commit failed") })
			} else {
				err = SetupManaged(context.Background(), cfg, paths, data)
			}
			if (err != nil) != (failure != "") {
				t.Fatalf("SetupManaged error = %v", err)
			}
			contents, readErr := os.ReadFile(filepath.Join(paths.ConfigDir, "config.yaml"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if failure != "" && string(contents) != string(old) {
				t.Fatalf("failed setup lost old config: %s", contents)
			}
			if failure == "profile" {
				saved, err := config.Load(config.Overrides{ConfigPath: paths.AppConfig, LookupEnv: func(string) (string, bool) { return "", false }})
				if err != nil || saved != oldApp {
					t.Fatalf("profile failure lost connection config: %#v %v", saved, err)
				}
			}
			if failure == "" {
				saved, err := config.Load(config.Overrides{ConfigPath: paths.AppConfig, LookupEnv: func(string) (string, bool) { return "", false }})
				if err != nil || saved != cfg {
					t.Fatalf("saved = %#v, %v", saved, err)
				}
				// The temporary validation child must already be stopped.
				address := strings.TrimPrefix(cfg.Server, "http://")
				conn, err := net.Dial("tcp", address)
				if err == nil {
					conn.Close()
					t.Fatal("setup left a child running")
				}
			}
		})
	}
}

func TestManagedPreflightRejectsPortsAndPrivilegeRequirements(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	for _, source := range []string{
		"tun:\n  enable: true\n",
		fmt.Sprintf("mixed-port: %d\n", port),
		"listeners:\n  - name: special\n    port: 7890\n",
	} {
		cfg := config.Config{Server: availableServer(t)}
		secret, data, err := PrepareManagedConfig([]byte(source), cfg.Server, "")
		if err != nil {
			t.Fatal(err)
		}
		cfg.Secret = secret
		if err := CheckManagedConfig(context.Background(), "/must-not-execute", t.TempDir(), data, cfg); err == nil {
			t.Fatalf("accepted privileged/conflicting config: %s", source)
		}
	}
}

func TestFindMihomoDoesNotDownloadOrHideInvalidBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := FindMihomo(t.TempDir(), ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing core error = %v", err)
	}
	if _, err := FindMihomo(t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory binary error = %v", err)
	}
}
