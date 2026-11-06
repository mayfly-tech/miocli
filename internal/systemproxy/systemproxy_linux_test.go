//go:build linux

package systemproxy

import "testing"

func TestGSettingsPathUsesGNOMESubschemas(t *testing.T) {
	tests := []struct {
		key, schema, setting string
	}{
		{"mode", "org.gnome.system.proxy", "mode"},
		{"http host", "org.gnome.system.proxy.http", "host"},
		{"https port", "org.gnome.system.proxy.https", "port"},
		{"socks host", "org.gnome.system.proxy.socks", "host"},
	}
	for _, test := range tests {
		schema, setting := gsettingsPath(test.key)
		if schema != test.schema || setting != test.setting {
			t.Errorf("gsettingsPath(%q) = %q, %q", test.key, schema, setting)
		}
	}
}
