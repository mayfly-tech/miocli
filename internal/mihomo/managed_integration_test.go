package mihomo

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsolatedManagedAPI(t *testing.T) {
	binary := os.Getenv("MIOCLI_TEST_BINARY")
	if binary == "" {
		t.Skip("set MIOCLI_TEST_BINARY to test an isolated local core")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	proxyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	proxyAddress := proxyListener.Addr().String()
	proxyPort := proxyListener.Addr().(*net.TCPAddr).Port
	if err := proxyListener.Close(); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	data := fmt.Sprintf("log-level: silent\nexternal-controller: %s\nmixed-port: %d\nsecret: isolated-test\nmode: rule\nproxy-providers:\n  MIOCLI-Provider:\n    type: file\n    path: ./provider.yaml\nproxy-groups:\n  - name: MIOCLI-Selector\n    type: select\n    proxies: [DIRECT, REJECT]\n    use: [MIOCLI-Provider]\nrules: ['MATCH,MIOCLI-Selector']\n", address, proxyPort)
	providerPath := filepath.Join(dir, "provider.yaml")
	if err := os.WriteFile(providerPath, []byte("proxies:\n  - name: Local\n    type: socks5\n    server: 127.0.0.1\n    port: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-d", dir)
	log, err := os.Create(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = log.Close()
	})
	client, err := NewHTTPClient("http://"+address, "isolated-test")
	if err != nil {
		t.Fatal(err)
	}
	ready := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if _, err := client.Version(ctx); err == nil {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		output, _ := os.ReadFile(filepath.Join(dir, "test.log"))
		t.Fatalf("core did not start: %s", output)
	}
	if err := client.SetMode(ctx, ModeDirect); err != nil {
		t.Fatal(err)
	}
	if mode, err := client.Mode(ctx); err != nil || mode != ModeDirect {
		t.Fatalf("mode = %q, %v", mode, err)
	}
	providers, err := client.Providers(ctx)
	if err != nil || len(providers["MIOCLI-Provider"].Proxies) != 1 {
		t.Fatalf("providers = %#v, %v", providers, err)
	}
	if err := client.UpdateProvider(ctx, "MIOCLI-Provider"); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(dir, "candidate.yaml")
	data += "proxies: []\n"
	if err := os.WriteFile(candidate, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := client.Reload(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	if mode, err := client.Mode(ctx); err != nil || mode != ModeRule {
		t.Fatalf("reload mode = %q, %v", mode, err)
	}
	if proxies, err := client.Proxies(ctx); err != nil || proxies.Items["MIOCLI-Selector"].Type != "Selector" {
		t.Fatalf("reload proxies = %#v, %v", proxies, err)
	}
	t.Run("active-connection-chain", func(t *testing.T) {
		target, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer target.Close()
		tunnel, err := net.DialTimeout("tcp", proxyAddress, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer tunnel.Close()
		if err := tunnel.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(tunnel, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target.Addr(), target.Addr()); err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(bufio.NewReader(tunnel), nil)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("CONNECT: %s", response.Status)
		}
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			connections, err := client.Connections(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, connection := range connections {
				if strings.Join(connection.Chains, ",") == "DIRECT,MIOCLI-Selector" {
					return
				}
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("live connection did not expose node-first group chain")
	})
	t.Setenv("MIOCLI_INTEGRATION_URL", "http://"+address)
	t.Setenv("MIOCLI_INTEGRATION_SECRET", "isolated-test")
	t.Setenv("MIOCLI_INTEGRATION_CONFIG", candidate)
	t.Run("release-fixture-contract", TestMihomoIntegration)
}
