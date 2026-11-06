package systemproxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		wantErr  string
	}{
		{"valid HTTP and SOCKS", Endpoint{Host: "127.0.0.1", HTTPPort: 7890, SOCKSPort: 7891}, ""},
		{"valid mixed port", Endpoint{Host: "127.0.0.1", HTTPPort: 7890, SOCKSPort: 7890}, ""},
		{"non-loopback", Endpoint{Host: "192.0.2.1", HTTPPort: 7890}, "loopback"},
		{"missing HTTP port", Endpoint{Host: "127.0.0.1"}, "HTTP or mixed"},
		{"invalid SOCKS port", Endpoint{Host: "127.0.0.1", HTTPPort: 7890, SOCKSPort: 65536}, "SOCKS"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateEndpoint(test.endpoint)
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestSystemProxySnapshotIsPrivateAndPlatformBound(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Unix permission bits are not portable to Windows")
	}
	path := filepath.Join(t.TempDir(), "system-proxy-state.json")
	if err := writeSnapshot(path, "test", "", map[string]string{"mode": "none"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("snapshot permissions = %v, %v", info, err)
	}
	saved, err := readSnapshot(path, "test", "")
	if err != nil || saved.Values["mode"] != "none" {
		t.Fatalf("snapshot = %#v, %v", saved, err)
	}
	if _, err := readSnapshot(path, "test", "another-service"); err == nil {
		t.Fatal("snapshot accepted a different service")
	}
}
