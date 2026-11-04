package mihomo

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestConnectionsDecode(t *testing.T) {
	for _, test := range []struct {
		name, body string
		count      int
		valid      bool
	}{
		{"active", `{"connections":[{"id":"one","chains":["Node","Auto","Manual"],"metadata":{"host":"not-retained.example"}}]}`, 1, true},
		{"empty", `{"connections":[]}`, 0, true},
		{"idle", `{"connections":null}`, 0, true},
		{"missing", `{}`, 0, false},
		{"invalid", `{"connections":{}}`, 0, false},
		{"invalid-chains", `{"connections":[{"chains":42}]}`, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/connections" {
					t.Errorf("%s %s", r.Method, r.URL)
				}
				_, _ = w.Write([]byte(test.body))
			}, "")
			items, err := client.Connections(context.Background())
			if (err == nil) != test.valid || len(items) != test.count {
				t.Fatalf("%#v %v", items, err)
			}
			if len(items) > 0 && (len(items[0].Chains) != 3 || items[0].Chains[0] != "Node") {
				t.Fatalf("%#v", items)
			}
		})
	}
}

func TestConnectionsAuthAndErrors(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		kind   ErrorKind
	}{
		{"auth", http.StatusUnauthorized, ErrorAuth},
		{"unsupported", http.StatusNotFound, ErrorNotFound},
		{"server", http.StatusServiceUnavailable, ErrorAPI},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer synthetic-test" {
					t.Errorf("authorization = %q", got)
				}
				w.WriteHeader(test.status)
			}, "synthetic-test")
			items, err := client.Connections(context.Background())
			var apiErr *APIError
			if len(items) != 0 || !errors.As(err, &apiErr) || apiErr.Kind != test.kind {
				t.Fatalf("%v %v", items, err)
			}
		})
	}
	t.Run("timeout", func(t *testing.T) {
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, "")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := client.Connections(ctx)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Kind != ErrorTimeout {
			t.Fatalf("%v", err)
		}
	})
}
