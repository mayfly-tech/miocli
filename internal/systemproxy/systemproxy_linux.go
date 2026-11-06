//go:build linux

package systemproxy

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const gnomeProxy = "org.gnome.system.proxy"

var proxyKeys = []string{
	"mode",
	"http host", "http port",
	"https host", "https port",
	"socks host", "socks port",
}

func Enable(endpoint Endpoint, _ string, configPath string) error {
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}
	path := statePath(configPath)
	if saved, err := readSnapshot(path, "linux", ""); err != nil {
		return err
	} else if saved.Platform != "" {
		return errors.New("miocli already has a system proxy enabled; disable it before enabling again")
	}
	current := make(map[string]string, len(proxyKeys))
	for _, key := range proxyKeys {
		value, err := gsettingsGet(key)
		if err != nil {
			return err
		}
		current[key] = value
	}
	if strings.Trim(current["mode"], "'") != "none" {
		return errors.New("GNOME system proxy is already configured; miocli will not overwrite it")
	}
	updates := map[string]string{
		"http host":  gvariantString(endpoint.Host),
		"http port":  strconv.Itoa(endpoint.HTTPPort),
		"https host": gvariantString(endpoint.Host),
		"https port": strconv.Itoa(endpoint.HTTPPort),
		"socks host": gvariantString(endpoint.Host),
		"socks port": strconv.Itoa(endpoint.SOCKSPort),
		"mode":       "'manual'",
	}
	if endpoint.SOCKSPort == 0 {
		updates["socks host"] = "''"
	}
	for key, value := range updates {
		current["miocli-"+key] = value
	}
	if err := writeSnapshot(path, "linux", "", current); err != nil {
		return err
	}
	for _, key := range []string{"http host", "http port", "https host", "https port", "socks host", "socks port", "mode"} {
		if err := gsettingsSet(key, updates[key]); err != nil {
			if restoreErr := restore(current); restoreErr != nil {
				return fmt.Errorf("%w; restoring previous GNOME settings failed: %v (saved state retained)", err, restoreErr)
			}
			if cleanupErr := removeSnapshot(path); cleanupErr != nil {
				return fmt.Errorf("%w; restored settings but could not remove saved state: %v", err, cleanupErr)
			}
			return err
		}
	}
	return nil
}

func Disable(_ string, configPath string) error {
	path := statePath(configPath)
	saved, err := readSnapshot(path, "linux", "")
	if err != nil {
		return err
	}
	if saved.Platform == "" {
		return errors.New("miocli has no saved GNOME proxy settings to restore")
	}
	changed := false
	for _, key := range proxyKeys {
		current, err := gsettingsGet(key)
		if err != nil {
			return err
		}
		managed, original := saved.Values["miocli-"+key], saved.Values[key]
		if current != managed && current != original {
			return fmt.Errorf("GNOME proxy setting %q changed since miocli enabled it; refusing to overwrite the newer value", key)
		}
		changed = changed || current != original
	}
	if changed {
		if err := restore(saved.Values); err != nil {
			return err
		}
	}
	if err := removeSnapshot(path); err != nil {
		return err
	}
	return nil
}

func Status(_ string) (string, error) {
	mode, err := gsettingsGet("mode")
	if err != nil {
		return "", err
	}
	return "GNOME proxy mode: " + strings.Trim(mode, "'"), nil
}

func restore(values map[string]string) error {
	var restoreErrors []error
	for _, key := range []string{"http host", "http port", "https host", "https port", "socks host", "socks port", "mode"} {
		if err := gsettingsSet(key, values[key]); err != nil {
			restoreErrors = append(restoreErrors, err)
		}
	}
	return errors.Join(restoreErrors...)
}

func gsettingsGet(key string) (string, error) {
	schema, setting := gsettingsPath(key)
	output, err := exec.Command("gsettings", "get", schema, setting).Output()
	if err != nil {
		return "", fmt.Errorf("read GNOME proxy setting %q (gsettings required): %w", key, err)
	}
	return strings.TrimSpace(string(output)), nil
}

func gsettingsSet(key, value string) error {
	schema, setting := gsettingsPath(key)
	cmd := exec.Command("gsettings", "set", schema, setting, value)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set GNOME proxy setting %q: %s", key, strings.TrimSpace(string(output)))
	}
	return nil
}

func gsettingsPath(key string) (string, string) {
	if key == "mode" {
		return gnomeProxy, key
	}
	schema, setting, _ := strings.Cut(key, " ")
	return gnomeProxy + "." + schema, setting
}

func gvariantString(value string) string { return "'" + strings.ReplaceAll(value, "'", "\\'") + "'" }
