package mihomo

import (
	"context"
	"net/http"
	"testing"
)

func TestProvidersDecodeAndUpdate(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			if r.URL.EscapedPath() != "/providers/proxies/sub%2Fname%20%25" {
				t.Errorf("path = %s", r.URL.EscapedPath())
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path != "/providers/proxies" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"providers":{"sub/name %":{"type":"Proxy","vehicleType":"HTTP","proxies":[{"name":"A","type":"Shadowsocks"}],"subscriptionInfo":{"Total":1024,"Expire":0}}}}`))
	}, "")
	providers, err := client.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	p := providers["sub/name %"]
	if p.Name != "sub/name %" || len(p.Proxies) != 1 || p.SubscriptionInfo.Upload != nil || *p.SubscriptionInfo.Total != 1024 {
		t.Fatalf("provider = %#v", p)
	}
	if err := client.UpdateProvider(context.Background(), p.Name); err != nil {
		t.Fatal(err)
	}
}

func TestProvidersMissingObjectIsNotEmptySuccess(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }, "")
	if _, err := client.Providers(context.Background()); err == nil {
		t.Fatal("missing providers object was accepted")
	}
}

func TestProviderDelayEscapesBothPathSegments(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/providers/proxies/sub%2Fname/node%20%25/healthcheck" || r.URL.Query().Get("timeout") != "5000" {
			t.Errorf("URL = %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"delay":12}`))
	}, "")
	if delay, err := client.DelayProvider(context.Background(), "sub/name", "node %", "https://www.gstatic.com/generate_204", 5000); err != nil || delay != 12 {
		t.Fatalf("delay=%d %v", delay, err)
	}
}

func TestMissingDelayIsNotZeroLatency(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }, "")
	if _, err := client.Delay(context.Background(), "node", "https://example.test", 5000); err == nil {
		t.Fatal("missing delay became zero latency")
	}
	if _, err := client.DelayProvider(context.Background(), "sub", "node", "https://example.test", 5000); err == nil {
		t.Fatal("missing provider delay became zero latency")
	}
}
