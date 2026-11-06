//go:build darwin

package systemproxy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fakeNetworkSetup(t *testing.T) map[string]map[string]string {
	t.Helper()
	original := networkSetup
	t.Cleanup(func() { networkSetup = original })
	state := make(map[string]map[string]string)
	for _, kind := range proxyTypes {
		state[kind] = map[string]string{"Enabled": "No", "Server": "old.example", "Port": "8080"}
	}
	networkSetup = func(args ...string) ([]byte, error) {
		flag := args[0]
		if flag == "-getautoproxyurl" || flag == "-getproxyautodiscovery" {
			return []byte("Enabled: No\n"), nil
		}
		for _, kind := range proxyTypes {
			switch flag {
			case "-get" + kind:
				s := state[kind]
				return []byte(fmt.Sprintf("Enabled: %s\nServer: %s\nPort: %s\n", s["Enabled"], s["Server"], s["Port"])), nil
			case "-set" + kind:
				state[kind]["Server"], state[kind]["Port"] = args[2], args[3]
				return nil, nil
			case "-set" + kind + "state":
				state[kind]["Enabled"] = map[string]string{"on": "Yes", "off": "No"}[args[2]]
				return nil, nil
			}
		}
		return nil, fmt.Errorf("unexpected command: %v", args)
	}
	return state
}

func TestProxyRestoresSnapshotAndProtectsExternalChanges(t *testing.T) {
	state := fakeNetworkSetup(t)
	settings := filepath.Join(t.TempDir(), "config.yaml")
	endpoint := Endpoint{Host: "127.0.0.1", HTTPPort: 7890, SOCKSPort: 7891}
	if err := Enable(endpoint, "Wi-Fi", settings); err != nil {
		t.Fatal(err)
	}
	if err := Enable(endpoint, "Wi-Fi", settings); err == nil {
		t.Fatal("repeated enable replaced the snapshot")
	}
	state["webproxy"]["Server"] = "new-owner.example"
	if err := Disable("Wi-Fi", settings); err == nil || !strings.Contains(err.Error(), "newer value") {
		t.Fatalf("external change overwritten: %v", err)
	}
	if _, err := os.Stat(statePath(settings)); err != nil {
		t.Fatal("conflict lost snapshot")
	}
	state["webproxy"]["Server"] = "127.0.0.1"
	if err := Disable("Wi-Fi", settings); err != nil {
		t.Fatal(err)
	}
	for kind, values := range state {
		if !reflect.DeepEqual(values, map[string]string{"Enabled": "No", "Server": "old.example", "Port": "8080"}) {
			t.Fatalf("%s not restored: %v", kind, values)
		}
	}
	if _, err := os.Stat(statePath(settings)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot retained after restoration: %v", err)
	}
}

func TestProxyEnableRollsBackPartialFailure(t *testing.T) {
	state := fakeNetworkSetup(t)
	run := networkSetup
	failed := false
	networkSetup = func(args ...string) ([]byte, error) {
		if args[0] == "-setsecurewebproxy" && !failed {
			failed = true
			return nil, errors.New("fixture failure")
		}
		return run(args...)
	}
	settings := filepath.Join(t.TempDir(), "config.yaml")
	err := Enable(Endpoint{Host: "127.0.0.1", HTTPPort: 7890}, "Wi-Fi", settings)
	if err == nil || !strings.Contains(err.Error(), "fixture failure") {
		t.Fatalf("failure hidden: %v", err)
	}
	for kind, values := range state {
		if values["Server"] != "old.example" || values["Port"] != "8080" || values["Enabled"] != "No" {
			t.Fatalf("%s not rolled back: %v", kind, values)
		}
	}
	if _, err := os.Stat(statePath(settings)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("snapshot not cleaned up: %v", err)
	}
}
