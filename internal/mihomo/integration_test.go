package mihomo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestMihomoIntegration(t *testing.T) {
	serverURL := os.Getenv("MIOCLI_INTEGRATION_URL")
	if serverURL == "" {
		t.Skip("set MIOCLI_INTEGRATION_URL to run the Mihomo integration test")
	}
	secret := os.Getenv("MIOCLI_INTEGRATION_SECRET")
	client, err := NewHTTPClient(serverURL, secret)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	version, err := client.Version(ctx)
	if err != nil || version.Version == "" {
		t.Fatalf("Version() = %#v, %v", version, err)
	}
	proxies, err := client.Proxies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	group, ok := proxies.Items["MIOCLI-Selector"]
	if !ok || group.Type != "Selector" {
		t.Fatalf("integration selector missing or has unexpected type: %#v", group)
	}
	if !containsName(group.All, "REJECT") || !containsName(group.All, "DIRECT") {
		t.Fatalf("integration group members = %#v", group.All)
	}

	if err := client.Select(ctx, "MIOCLI-Selector", "REJECT"); err != nil {
		t.Fatal(err)
	}
	proxies, err = client.Proxies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := proxies.Items["MIOCLI-Selector"].Now; got != "REJECT" {
		t.Fatalf("selected proxy = %q, want REJECT", got)
	}

	if err := client.SetMode(ctx, ModeGlobal); err != nil {
		t.Fatal(err)
	}
	mode, err := client.Mode(ctx)
	if err != nil || mode != ModeGlobal {
		t.Fatalf("Mode() = %q, %v; want global", mode, err)
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Mihomo rejects zero-millisecond results; loopback responses can otherwise round down to zero.
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer testServer.Close()
	delay, err := client.Delay(ctx, "DIRECT", testServer.URL, 5000)
	if err != nil {
		t.Fatalf("Delay(DIRECT) failed: %v", err)
	}
	if delay <= 0 {
		t.Fatalf("Delay() returned non-positive latency: %d", delay)
	}
	providers, err := client.Providers(ctx)
	if err != nil || len(providers["MIOCLI-Provider"].Proxies) != 1 {
		t.Fatalf("Providers() = %#v, %v", providers, err)
	}
	if err := client.UpdateProvider(ctx, "MIOCLI-Provider"); err != nil {
		t.Fatalf("UpdateProvider() failed: %v", err)
	}
	if path := os.Getenv("MIOCLI_INTEGRATION_CONFIG"); path != "" {
		if err := client.Reload(ctx, path); err != nil {
			t.Fatalf("Reload() failed: %v", err)
		}
		if mode, err := client.Mode(ctx); err != nil || mode != ModeRule {
			t.Fatalf("reloaded mode = %q, %v", mode, err)
		}
	}
}

func containsName(names []string, name string) bool {
	for _, item := range names {
		if item == name {
			return true
		}
	}
	return false
}
