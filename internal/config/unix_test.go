//go:build !windows

package config

import "testing"

func TestImportUnixController(t *testing.T) {
	cfg, err := ConnectionFromYAML([]byte("external-controller: ''\nexternal-controller-unix: /tmp/control.sock\nsecret: fixture\n"))
	if err != nil || cfg.Server != "unix:///tmp/control.sock" || cfg.Secret != "fixture" {
		t.Fatalf("Unix connection = %#v, %v", cfg, err)
	}
	for _, source := range []string{
		"external-controller-unix: relative.sock\n",
		"external-controller-unix: [/tmp/control.sock]\n",
	} {
		if _, err := ConnectionFromYAML([]byte(source)); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
}
