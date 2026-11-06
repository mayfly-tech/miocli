package config

import (
	"reflect"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfileWithController(t *testing.T) {
	raw := []byte(`defaults: &defaults
  external-controller: 0.0.0.0:9090
  secret: subscription-secret
<<: *defaults
external-controller-tls: 0.0.0.0:9443
external-controller-unix: /tmp/untrusted.sock
external-controller-pipe: \\.\pipe\untrusted
mode: rule
dns:
  enable: true
tun:
  enable: false
proxies:
  - name: node
    type: ss
    server: example.test
rules:
  - MATCH,DIRECT
`)
	for _, server := range []string{"http://127.0.0.1:19090", "http://localhost", "https://[::1]", "unix:///tmp/miocli.sock"} {
		t.Run(server, func(t *testing.T) {
			if runtime.GOOS == "windows" && server == "unix:///tmp/miocli.sock" {
				t.Skip("Unix sockets are unsupported on Windows")
			}
			cfg := Config{Server: server, Secret: "local-secret"}
			data, err := ProfileWithController(raw, cfg)
			if err != nil {
				t.Fatal(err)
			}
			var before, after map[string]any
			if err := yaml.Unmarshal(raw, &before); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(data, &after); err != nil {
				t.Fatal(err)
			}
			expectedKey, expectedAddress := "external-controller", "127.0.0.1:19090"
			switch server {
			case "http://localhost":
				expectedAddress = "localhost:80"
			case "https://[::1]":
				expectedKey, expectedAddress = "external-controller-tls", "[::1]:443"
			case "unix:///tmp/miocli.sock":
				expectedKey, expectedAddress = "external-controller-unix", "/tmp/miocli.sock"
			}
			for _, key := range []string{"external-controller", "external-controller-tls", "external-controller-unix", "external-controller-pipe"} {
				want := ""
				if key == expectedKey {
					want = expectedAddress
				}
				if after[key] != want {
					t.Errorf("%s = %v, want %q", key, after[key], want)
				}
				delete(before, key)
				delete(after, key)
			}
			if after["secret"] != cfg.Secret {
				t.Fatal("subscription secret was not replaced")
			}
			delete(before, "secret")
			delete(after, "secret")
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("subscription policy changed: %#v != %#v", before, after)
			}
		})
	}
}

func TestProfileWithControllerRejectsInvalidYAML(t *testing.T) {
	for _, raw := range []string{"", "[]", "mode: rule\nmode: direct\n", "mode: rule\n---\nmode: direct\n", "dns: [", "dns:\n  enable: true\n  enable: false\n"} {
		if _, err := ProfileWithController([]byte(raw), Config{Server: DefaultServer}); err == nil {
			t.Errorf("invalid profile accepted: %q", raw)
		}
	}
	data, err := ProfileWithController([]byte("mode: rule\n"), Config{Server: DefaultServer})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := ConnectionFromYAML(data)
	if err != nil || connection.Server != DefaultServer || connection.Secret != "" {
		t.Fatalf("missing controller adaptation failed: %#v, %v", connection, err)
	}
}
