//go:build darwin

package systemproxy

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// networkSetup is replaceable in tests so restoration never touches host settings.
var networkSetup = func(args ...string) ([]byte, error) {
	return exec.Command("/usr/sbin/networksetup", args...).CombinedOutput()
}

var proxyTypes = []string{"webproxy", "securewebproxy", "socksfirewallproxy"}

func Enable(endpoint Endpoint, service, configPath string) error {
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}
	if strings.TrimSpace(service) == "" {
		return errors.New("macOS system proxy requires --service; use `networksetup -listnetworkserviceorder` to find the active service")
	}
	path := statePath(configPath)
	if saved, err := readSnapshot(path, "darwin", service); err != nil {
		return err
	} else if saved.Platform != "" {
		return errors.New("miocli already has a system proxy enabled; disable it before enabling again")
	}
	for _, option := range []string{"-getautoproxyurl", "-getproxyautodiscovery"} {
		output, err := networkSetup(option, service)
		if err != nil {
			return fmt.Errorf("read macOS automatic proxy settings for %q: %w", service, err)
		}
		if strings.Contains(string(output), "Enabled: Yes") {
			return fmt.Errorf("network service %q has automatic proxy configuration enabled; refusing to overwrite it", service)
		}
	}
	values := make(map[string]string)
	for _, proxyType := range proxyTypes {
		settings, err := getProxy(service, proxyType)
		if err != nil {
			return err
		}
		if settings["Enabled"] == "Yes" || settings["Authenticated Proxy Enabled"] == "1" {
			return fmt.Errorf("network service %q already has an enabled or authenticated %s; refusing to overwrite it", service, proxyType)
		}
		for _, key := range []string{"Server", "Port", "Enabled"} {
			values[proxyType+"."+key] = settings[key]
		}
	}
	managed := map[string]map[string]string{
		"webproxy":           {"Server": endpoint.Host, "Port": strconv.Itoa(endpoint.HTTPPort), "Enabled": "Yes"},
		"securewebproxy":     {"Server": endpoint.Host, "Port": strconv.Itoa(endpoint.HTTPPort), "Enabled": "Yes"},
		"socksfirewallproxy": {"Server": endpoint.Host, "Port": strconv.Itoa(endpoint.SOCKSPort), "Enabled": "Yes"},
	}
	for proxyType, settings := range managed {
		if proxyType == "socksfirewallproxy" && endpoint.SOCKSPort == 0 {
			for _, key := range []string{"Server", "Port", "Enabled"} {
				settings[key] = values[proxyType+"."+key]
			}
		}
		for key, value := range settings {
			values["miocli."+proxyType+"."+key] = value
		}
	}
	if err := writeSnapshot(path, "darwin", service, values); err != nil {
		return err
	}
	configs := map[string][]string{
		"webproxy":           {"-setwebproxy", service, endpoint.Host, strconv.Itoa(endpoint.HTTPPort)},
		"securewebproxy":     {"-setsecurewebproxy", service, endpoint.Host, strconv.Itoa(endpoint.HTTPPort)},
		"socksfirewallproxy": {"-setsocksfirewallproxy", service, endpoint.Host, strconv.Itoa(endpoint.SOCKSPort)},
	}
	for _, proxyType := range proxyTypes {
		if proxyType == "socksfirewallproxy" && endpoint.SOCKSPort == 0 {
			continue
		}
		if err := runNetworkSetup(configs[proxyType]...); err != nil {
			return rollbackError(service, values, path, err)
		}
	}
	for _, args := range [][]string{
		{"-setwebproxystate", service, "on"},
		{"-setsecurewebproxystate", service, "on"},
	} {
		if err := runNetworkSetup(args...); err != nil {
			return rollbackError(service, values, path, err)
		}
	}
	if endpoint.SOCKSPort > 0 {
		if err := runNetworkSetup("-setsocksfirewallproxystate", service, "on"); err != nil {
			return rollbackError(service, values, path, err)
		}
	}
	return nil
}

func Disable(service, configPath string) error {
	path := statePath(configPath)
	saved, err := readSnapshot(path, "darwin", service)
	if err != nil {
		return err
	}
	if saved.Platform == "" {
		return errors.New("miocli has no saved macOS proxy settings to restore")
	}
	changed := false
	for _, proxyType := range proxyTypes {
		settings, err := getProxy(service, proxyType)
		if err != nil {
			return err
		}
		for _, key := range []string{"Server", "Port", "Enabled"} {
			managed, original := saved.Values["miocli."+proxyType+"."+key], saved.Values[proxyType+"."+key]
			if settings[key] != managed && settings[key] != original {
				return fmt.Errorf("macOS %s setting %q changed since miocli enabled it; refusing to overwrite the newer value", proxyType, key)
			}
			changed = changed || settings[key] != original
		}
	}
	if changed {
		if err := restore(service, saved.Values); err != nil {
			return err
		}
	}
	return removeSnapshot(path)
}

func Status(service string) (string, error) {
	if strings.TrimSpace(service) == "" {
		return "", errors.New("macOS system proxy status requires --service")
	}
	var enabled []string
	for _, proxyType := range proxyTypes {
		settings, err := getProxy(service, proxyType)
		if err != nil {
			return "", err
		}
		if settings["Enabled"] == "Yes" {
			enabled = append(enabled, proxyType)
		}
	}
	if len(enabled) == 0 {
		return "System proxy is disabled for " + service, nil
	}
	return fmt.Sprintf("Enabled for %s: %s", service, strings.Join(enabled, ", ")), nil
}

func getProxy(service, proxyType string) (map[string]string, error) {
	output, err := networkSetup("-get"+proxyType, service)
	if err != nil {
		return nil, fmt.Errorf("read macOS proxy settings for %q: %w", service, err)
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			result[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	if result["Enabled"] == "" {
		return nil, fmt.Errorf("could not parse macOS %s settings for %q", proxyType, service)
	}
	return result, nil
}

func restore(service string, values map[string]string) error {
	for _, proxyType := range proxyTypes {
		server := values[proxyType+".Server"]
		port, _ := strconv.Atoi(values[proxyType+".Port"])
		if server != "" && server != "(null)" && port > 0 {
			var args []string
			switch proxyType {
			case "webproxy":
				args = []string{"-setwebproxy", service, server, strconv.Itoa(port)}
			case "securewebproxy":
				args = []string{"-setsecurewebproxy", service, server, strconv.Itoa(port)}
			default:
				args = []string{"-setsocksfirewallproxy", service, server, strconv.Itoa(port)}
			}
			if err := runNetworkSetup(args...); err != nil {
				return err
			}
		}
		state := "off"
		if values[proxyType+".Enabled"] == "Yes" {
			state = "on"
		}
		flag := "-set" + proxyType + "state"
		if err := runNetworkSetup(flag, service, state); err != nil {
			return err
		}
	}
	return nil
}

func runNetworkSetup(args ...string) error {
	output, err := networkSetup(args...)
	if err != nil {
		return fmt.Errorf("change macOS proxy settings (networksetup may require administrator authorization): %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func rollbackError(service string, values map[string]string, path string, cause error) error {
	if err := restore(service, values); err != nil {
		return fmt.Errorf("%w; restoring previous macOS settings failed: %v (saved state retained)", cause, err)
	}
	if err := removeSnapshot(path); err != nil {
		return fmt.Errorf("%w; restored settings but could not remove saved state: %v", cause, err)
	}
	return cause
}
