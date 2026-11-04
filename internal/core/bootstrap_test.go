package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPrepareManagedConfigGeneratesSecretAndKeepsUserSettings(t *testing.T) {
	source := []byte("mixed-port: 7890\nproxies:\n  - name: example\n    type: direct\n")
	secret, managed, err := PrepareManagedConfig(source, "http://127.0.0.1:9090", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 64 {
		t.Fatalf("secret length = %d, want 64 hexadecimal characters", len(secret))
	}
	if _, err := hex.DecodeString(secret); err != nil {
		t.Fatalf("secret is not hexadecimal: %v", err)
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(managed, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["external-controller"] != "127.0.0.1:9090" || decoded["secret"] != secret {
		t.Fatalf("controller settings = %#v", decoded)
	}
	if decoded["mixed-port"] != 7890 {
		t.Fatalf("user setting was not preserved: %#v", decoded["mixed-port"])
	}
	if strings.Contains(string(source), "external-controller") {
		t.Fatal("source configuration was modified")
	}
}

func TestPrepareManagedConfigOverridesControllerOnlyInManagedCopy(t *testing.T) {
	source := []byte("external-controller: 0.0.0.0:9090\nsecret: existing-secret\nmode: rule\n")
	secret, managed, err := PrepareManagedConfig(source, "http://127.0.0.1:9090", "")
	if err != nil {
		t.Fatal(err)
	}
	if secret != "existing-secret" {
		t.Fatalf("secret = %q, want existing secret", secret)
	}
	if !strings.Contains(string(managed), "external-controller: 127.0.0.1:9090") {
		t.Fatalf("managed controller was not restricted to loopback: %s", managed)
	}
	if !strings.Contains(string(source), "0.0.0.0:9090") {
		t.Fatal("source configuration was modified")
	}
}

func TestPrepareManagedConfigRejectsDuplicateControllerKeys(t *testing.T) {
	_, _, err := PrepareManagedConfig([]byte("external-controller: 127.0.0.1:9090\nexternal-controller: 0.0.0.0:9090\n"), "http://127.0.0.1:9090", "")
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("prepareManagedConfig() error = %v, want duplicate-key error", err)
	}
}

func TestReadConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mihomo.yaml")
	want := []byte("mixed-port: 7890\n")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("promptConfig() = %q, want %q", got, want)
	}
}

func TestDownloadConfigRequiresHTTPSAndHidesURLInErrors(t *testing.T) {
	if _, err := DownloadConfig(context.Background(), "http://example.test/config.yaml"); err == nil {
		t.Fatal("downloadConfig() accepted HTTP URL")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "not available", http.StatusBadGateway)
	}))
	defer server.Close()
	_, err := downloadConfigWithClient(context.Background(), server.URL+"/config?token=secret", server.Client())
	if err == nil || strings.Contains(err.Error(), "token=secret") {
		t.Fatalf("downloadConfigWithClient() error = %v, want a sanitized HTTP error", err)
	}
}

func TestDownloadConfigWithClientEnforcesSizeLimit(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", maxConfigBytes+1)))
	}))
	defer server.Close()
	_, err := downloadConfigWithClient(context.Background(), server.URL, server.Client())
	if err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("downloadConfigWithClient() error = %v, want size limit error", err)
	}
}

func TestWriteManagedConfigRestrictsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mihomo", "config.yaml")
	if err := writeManagedConfig(path, []byte("secret: private\n")); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("managed config permissions = %04o, want 0600", info.Mode().Perm())
		}
	}
}
