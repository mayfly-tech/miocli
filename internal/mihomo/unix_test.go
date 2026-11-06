//go:build !windows

package mihomo

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

func TestUnixController(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing socket authentication")
		}
		switch r.URL.Path {
		case "/version":
			_, _ = w.Write([]byte(`{"version":"unix-test"}`))
		case "/proxies/group/name %":
			if r.Method != http.MethodPut || r.URL.EscapedPath() != "/proxies/group%2Fname%20%25" {
				t.Errorf("unexpected selection request: %s %s", r.Method, r.URL)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected path: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	client, err := NewHTTPClient((&url.URL{Scheme: "unix", Path: path}).String(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	version, err := client.Version(context.Background())
	if err != nil || version.Version != "unix-test" {
		t.Fatalf("Version = %#v, %v", version, err)
	}
	if err := client.Select(context.Background(), "group/name %", "Node A"); err != nil {
		t.Fatal(err)
	}
}

func TestUnixControllerRejectsInvalidAddresses(t *testing.T) {
	for _, address := range []string{
		"unix://host/tmp/control.sock", "unix:relative.sock", "unix:///",
		"unix:///tmp/../control.sock", "unix:///tmp/control.sock?secret=x",
		"unix:///tmp/control.sock?", "unix:///tmp/control.sock#fragment", "unix:///tmp/%00.sock",
	} {
		if _, err := NewHTTPClient(address, ""); err == nil {
			t.Errorf("accepted %q", address)
		}
	}
}
