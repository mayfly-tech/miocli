package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"miocli/internal/mihomo"
)

func TestReadConnectionConfigIsBoundedAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mihomo.yaml")
	want := []byte("external-controller: 127.0.0.1:9090\nsecret: fixture\n")
	if err := os.WriteFile(path, want, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadConfigFile(path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("read = %q, %v", got, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, want) {
		t.Fatal("read modified source")
	}
	if _, err := ReadConfigFile(dir); err == nil {
		t.Fatal("directory accepted as source")
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxConfigBytes), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadConfigFile(path); err != nil || len(data) != maxConfigBytes {
		t.Fatalf("16 MiB source was rejected: %v", err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), maxConfigBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadConfigFile(path); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("oversized source error = %v", err)
	}
}

func TestConnectionFromYAML(t *testing.T) {
	for _, tc := range []struct {
		name, source, server, secret, wantError string
	}{
		{"loopback", "external-controller: 127.0.0.1:9090\nsecret: fixture\nmixed-port: 7890\ntun:\n  enable: true\n", DefaultServer, "fixture", ""},
		{"IPv6", "external-controller: '[::1]:9090'\n", "http://[::1]:9090", "", ""},
		{"missing", "mixed-port: 7890\n", "", "", "no HTTP external-controller"},
		{"wildcard", "external-controller: 0.0.0.0:9090\n", "", "", "not a supported client target"},
		{"remote HTTP", "external-controller: remote.example:9090\n", "", "", "not a supported client target"},
		{"URL", "external-controller: http://127.0.0.1:9090\n", "", "", "not a URL"},
		{"port", "external-controller: 127.0.0.1\n", "", "", "host and port"},
		{"secret mapping", "external-controller: 127.0.0.1:9090\nsecret: {token: fixture}\n", "", "", "scalar"},
		{"controller sequence", "external-controller: [127.0.0.1:9090]\n", "", "", "scalar"},
		{"duplicates", "external-controller: 127.0.0.1:9090\nexternal-controller: 127.0.0.1:9091\n", "", "", "duplicate"},
		{"multiple documents", "external-controller: 127.0.0.1:9090\n---\nsecret: fixture\n", "", "", "exactly one"},
		{"sequence", "- external-controller\n", "", "", "single YAML mapping"},
		{"invalid", "external-controller: [\n", "", "", "valid YAML"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(tc.source)
			before := append([]byte(nil), data...)
			cfg, err := ConnectionFromYAML(data)
			if !bytes.Equal(data, before) {
				t.Fatal("import mutated input")
			}
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
			} else if err != nil || cfg.Server != tc.server || cfg.Secret != tc.secret {
				t.Fatalf("connection = %#v, %v", cfg, err)
			}
		})
	}
}

func TestConnectionHints(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&mihomo.APIError{Kind: mihomo.ErrorAuth}, "secret"},
		{&mihomo.APIError{Kind: mihomo.ErrorTimeout}, "timed out"},
		{&mihomo.APIError{Kind: mihomo.ErrorDecode}, "compatible Mihomo"},
		{&mihomo.APIError{Kind: mihomo.ErrorNotFound}, "proxy port"},
		{syscall.ECONNREFUSED, "start the owning app"},
		{errors.New("offline"), "miocli doctor"},
	} {
		if hint := ConnectionHint(tc.err); !strings.Contains(hint, tc.want) {
			t.Fatalf("hint = %q, want %q", hint, tc.want)
		}
	}
}
