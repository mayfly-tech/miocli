package mihomo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc, secret string) *HTTPClient {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewHTTPClient(server.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestHTTPClientAuthAndProxies(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/proxies" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"proxies":{"Auto":{"type":"URLTest","now":"Node A","all":["Node A","Node B"]},"Manual":{"type":"Selector","now":"Node A","all":["Node A","Auto"]}}}`))
	}, "test-secret")

	got, err := client.Proxies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Items["Manual"].Now != "Node A" || got.Items["Manual"].Type != "Selector" {
		t.Fatalf("unexpected manual proxy: %#v", got.Items["Manual"])
	}
	if len(got.Items["Manual"].All) != 2 || got.Items["Manual"].All[1] != "Auto" {
		t.Fatalf("proxy member order was not preserved: %#v", got.Items["Manual"].All)
	}
}

func TestHTTPClientOmitsEmptyAuth(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("unexpected Authorization header %q", got)
		}
		_, _ = w.Write([]byte(`{"version":"test"}`))
	}, "")
	if _, err := client.Version(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNewHTTPClientEnforcesRemoteTLS(t *testing.T) {
	for _, server := range []string{"http://192.0.2.10:9090", "http://mihomo.example:9090"} {
		if _, err := NewHTTPClient(server, ""); err == nil {
			t.Errorf("NewHTTPClient(%q) unexpectedly accepted remote HTTP", server)
		}
	}
	if _, err := NewHTTPClient("https://mihomo.example:9090", ""); err != nil {
		t.Fatalf("NewHTTPClient rejected HTTPS server: %v", err)
	}
}

func TestHTTPClientEncodesProxyPathAndDelayQuery(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/proxies/group%2Fname%20%25/delay" {
			t.Errorf("escaped path = %q", got)
		}
		if got := r.URL.Query().Get("url"); got != "https://www.gstatic.com/generate_204" {
			t.Errorf("test URL = %q", got)
		}
		if got := r.URL.Query().Get("timeout"); got != "5000" {
			t.Errorf("timeout = %q", got)
		}
		_, _ = w.Write([]byte(`{"delay":42}`))
	}, "")

	delay, err := client.Delay(context.Background(), "group/name %", "https://www.gstatic.com/generate_204", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if delay != 42 {
		t.Fatalf("Delay() = %d, want 42", delay)
	}
}

func TestHTTPClientWriteRequests(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/proxies/Manual":
			if r.Method != http.MethodPut {
				t.Errorf("method = %q", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["name"] != "Node B" {
				t.Errorf("body = %#v", body)
			}
		case "/configs":
			if r.Method != http.MethodPatch {
				t.Errorf("method = %q", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["mode"] != "global" {
				t.Errorf("body = %#v", body)
			}
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}, "")

	if err := client.Select(context.Background(), "Manual", "Node B"); err != nil {
		t.Fatal(err)
	}
	if err := client.SetMode(context.Background(), ModeGlobal); err != nil {
		t.Fatal(err)
	}
	if err := client.SetMode(context.Background(), Mode("script")); err == nil {
		t.Fatal("SetMode accepted unsupported mode")
	}
	if requests != 2 {
		t.Fatalf("received %d requests, want 2", requests)
	}
}

func TestHTTPClientClassifiesErrors(t *testing.T) {
	tests := []struct {
		status int
		kind   ErrorKind
	}{
		{status: http.StatusUnauthorized, kind: ErrorAuth},
		{status: http.StatusNotFound, kind: ErrorNotFound},
		{status: http.StatusInternalServerError, kind: ErrorAPI},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
			}, "")
			_, err := client.Version(context.Background())
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Kind != test.kind {
				t.Fatalf("Version() error = %#v, want kind %q", err, test.kind)
			}
		})
	}
}

func TestHTTPClientRejectsMalformedAndOversizedResponses(t *testing.T) {
	t.Run("missing version", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}, "")
		_, err := client.Version(context.Background())
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Kind != ErrorDecode {
			t.Fatalf("Version() error = %#v, want missing-version decode error", err)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("{"))
		}, "")
		_, err := client.Version(context.Background())
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Kind != ErrorDecode {
			t.Fatalf("Version() error = %#v, want decode error", err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"version":"` + strings.Repeat("x", maxResponseBytes) + `"}`))
		}, "")
		_, err := client.Version(context.Background())
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Kind != ErrorDecode {
			t.Fatalf("Version() error = %#v, want response-size error", err)
		}
	})
}

func TestHTTPClientHonorsContextDeadline(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(100 * time.Millisecond):
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case <-r.Context().Done():
		}
	}, "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := client.Version(ctx)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != ErrorTimeout {
		t.Fatalf("Version() error = %#v, want timeout", err)
	}
}
