//go:build windows

package systemproxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const internetSettingsPath = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
const internetSettingsPolicyPath = `Software\Policies\Microsoft\Windows\CurrentVersion\Internet Settings`

var internetSetOption = syscall.NewLazyDLL("wininet.dll").NewProc("InternetSetOptionW")

func Enable(endpoint Endpoint, _ string, configPath string) error {
	if err := validateEndpoint(endpoint); err != nil {
		return err
	}
	path := statePath(configPath)
	if saved, err := readSnapshot(path, "windows", ""); err != nil {
		return err
	} else if saved.Platform != "" {
		return errors.New("miocli already has a system proxy enabled; disable it before enabling again")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open current-user Internet Settings: %w", err)
	}
	defer key.Close()
	enable, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil && err != syscall.Errno(2) {
		return fmt.Errorf("read current proxy state: %w", err)
	}
	enableBefore := strconv.FormatUint(enable, 10)
	if err == syscall.Errno(2) {
		enableBefore = "\x00"
	}
	if enable != 0 {
		return errors.New("a Windows system proxy is already enabled; miocli will not overwrite it")
	}
	if autoConfig, _, err := key.GetStringValue("AutoConfigURL"); err == nil && autoConfig != "" {
		return errors.New("a Windows proxy auto-configuration URL is set; miocli will not override it")
	} else if err != nil && err != syscall.Errno(2) {
		return fmt.Errorf("read proxy auto-configuration URL: %w", err)
	}
	if autoDetect, _, err := key.GetIntegerValue("AutoDetect"); err == nil && autoDetect != 0 {
		return errors.New("Windows proxy auto-detection is enabled; miocli will not override it")
	} else if err != nil && err != syscall.Errno(2) {
		return fmt.Errorf("read proxy auto-detection state: %w", err)
	}
	if err := rejectWindowsAutoProxy(); err != nil {
		return err
	}
	if err := rejectMachineProxyPolicy(); err != nil {
		return err
	}
	oldServer, err := readRegistryString(key, "ProxyServer")
	if err != nil {
		return err
	}
	values := map[string]string{
		"ProxyEnable": enableBefore,
		"ProxyServer": oldServer,
	}
	if err := writeSnapshot(path, "windows", "", values); err != nil {
		return err
	}
	parts := []string{
		"http=" + endpoint.Host + ":" + strconv.Itoa(endpoint.HTTPPort),
		"https=" + endpoint.Host + ":" + strconv.Itoa(endpoint.HTTPPort),
	}
	if endpoint.SOCKSPort > 0 {
		parts = append(parts, "socks="+endpoint.Host+":"+strconv.Itoa(endpoint.SOCKSPort))
	}
	if err := key.SetStringValue("ProxyServer", strings.Join(parts, ";")); err != nil {
		if cleanupErr := removeSnapshot(path); cleanupErr != nil {
			return fmt.Errorf("set current-user proxy server: %w; saved state cleanup failed: %v", err, cleanupErr)
		}
		return fmt.Errorf("set current-user proxy server: %w", err)
	}
	if err := key.SetDWordValue("ProxyEnable", 1); err != nil {
		if restoreErr := restoreProxyServer(key, values["ProxyServer"]); restoreErr != nil {
			return fmt.Errorf("enable current-user proxy: %w; restore previous proxy server failed: %v (saved state retained)", err, restoreErr)
		}
		if cleanupErr := removeSnapshot(path); cleanupErr != nil {
			return fmt.Errorf("enable current-user proxy: %w; restored previous server but could not remove saved state: %v", err, cleanupErr)
		}
		return fmt.Errorf("enable current-user proxy: %w", err)
	}
	values["miocli.ProxyEnable"] = "1"
	values["miocli.ProxyServer"] = strings.Join(parts, ";")
	if err := writeSnapshot(path, "windows", "", values); err != nil {
		if rollbackErr := key.SetDWordValue("ProxyEnable", 0); rollbackErr != nil {
			return fmt.Errorf("save active proxy state: %w; disable rollback failed: %v (saved state retained)", err, rollbackErr)
		}
		if rollbackErr := restoreProxyServer(key, values["ProxyServer"]); rollbackErr != nil {
			return fmt.Errorf("save active proxy state: %w; restore rollback failed: %v (saved state retained)", err, rollbackErr)
		}
		if cleanupErr := removeSnapshot(path); cleanupErr != nil {
			return fmt.Errorf("save active proxy state: %w; settings restored but saved state cleanup failed: %v", err, cleanupErr)
		}
		return fmt.Errorf("save active proxy state: %w", err)
	}
	if err := refreshInternetSettings(); err != nil {
		return fmt.Errorf("proxy settings were changed but application notification failed; saved state is retained for restoration: %w", err)
	}
	return nil
}

func Disable(_ string, configPath string) error {
	path := statePath(configPath)
	saved, err := readSnapshot(path, "windows", "")
	if err != nil {
		return err
	}
	if saved.Platform == "" {
		return errors.New("miocli has no saved Windows proxy settings to restore")
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open current-user Internet Settings: %w", err)
	}
	defer key.Close()
	currentEnabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil && err != syscall.Errno(2) {
		return fmt.Errorf("read current proxy state: %w", err)
	}
	currentServer, err := readRegistryString(key, "ProxyServer")
	if err != nil {
		return err
	}
	currentEnableValue := strconv.FormatUint(currentEnabled, 10)
	previousEnableValue := saved.Values["ProxyEnable"]
	if previousEnableValue == "\x00" {
		previousEnableValue = "0"
	}
	if (currentEnableValue != saved.Values["miocli.ProxyEnable"] && currentEnableValue != previousEnableValue) ||
		(currentServer != saved.Values["miocli.ProxyServer"] && currentServer != saved.Values["ProxyServer"]) {
		return errors.New("Windows proxy settings changed since miocli enabled them; refusing to overwrite the newer values")
	}
	changed := currentEnableValue != previousEnableValue || currentServer != saved.Values["ProxyServer"]
	if changed {
		if previousEnableValue == "0" {
			if err := key.SetDWordValue("ProxyEnable", 0); err != nil {
				return fmt.Errorf("disable current-user proxy: %w", err)
			}
		}
		if saved.Values["ProxyEnable"] == "\x00" {
			if err := key.DeleteValue("ProxyEnable"); err != nil && err != syscall.Errno(2) {
				return fmt.Errorf("restore previous proxy state: %w", err)
			}
		}
		if err := restoreProxyServer(key, saved.Values["ProxyServer"]); err != nil {
			return fmt.Errorf("restore previous proxy server: %w", err)
		}
	}
	if err := refreshInternetSettings(); err != nil {
		return err
	}
	return removeSnapshot(path)
}

func Status(_ string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("open current-user Internet Settings: %w", err)
	}
	defer key.Close()
	enable, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil && err != syscall.Errno(2) {
		return "", fmt.Errorf("read current proxy state: %w", err)
	}
	if enable != 0 {
		return "Windows current-user proxy is enabled", nil
	}
	return "Windows current-user proxy is disabled", nil
}

func readRegistryString(key registry.Key, name string) (string, error) {
	value, _, err := key.GetStringValue(name)
	if err == syscall.Errno(2) {
		return "\x00", nil
	}
	if err != nil {
		return "", fmt.Errorf("read current proxy server: %w", err)
	}
	return value, nil
}

func restoreProxyServer(key registry.Key, value string) error {
	if value == "\x00" {
		if err := key.DeleteValue("ProxyServer"); err != nil && err != syscall.Errno(2) {
			return fmt.Errorf("remove proxy server setting: %w", err)
		}
		return nil
	}
	if err := key.SetStringValue("ProxyServer", value); err != nil {
		return fmt.Errorf("write proxy server setting: %w", err)
	}
	return nil
}

func rejectWindowsAutoProxy() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, internetSettingsPath+`\Connections`, registry.QUERY_VALUE)
	if err == syscall.Errno(2) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open Windows proxy connection settings: %w", err)
	}
	defer key.Close()
	data, _, err := key.GetBinaryValue("DefaultConnectionSettings")
	if err == syscall.Errno(2) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Windows automatic proxy settings: %w", err)
	}
	if len(data) < 12 {
		return errors.New("Windows automatic proxy settings have an unexpected format; refusing to change proxy configuration")
	}
	flags := binary.LittleEndian.Uint32(data[8:12])
	if flags&0x0c != 0 {
		return errors.New("Windows automatic proxy detection or PAC is configured; miocli will not override it")
	}
	return nil
}

func rejectMachineProxyPolicy() error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, internetSettingsPolicyPath, registry.QUERY_VALUE)
	if err == syscall.Errno(2) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect Windows proxy policy: %w", err)
	}
	defer key.Close()
	perUser, _, err := key.GetIntegerValue("ProxySettingsPerUser")
	if err == syscall.Errno(2) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Windows proxy policy: %w", err)
	}
	if perUser == 0 {
		return errors.New("Windows policy requires machine-wide proxy settings; miocli will not attempt to elevate or modify them")
	}
	return nil
}

func refreshInternetSettings() error {
	const (
		optionSettingsChanged = 39
		optionRefresh         = 37
	)
	for _, option := range []uintptr{optionSettingsChanged, optionRefresh} {
		result, _, err := internetSetOption.Call(0, option, 0, 0)
		if result == 0 {
			if err == nil {
				return errors.New("notify Windows applications of proxy changes failed")
			}
			return fmt.Errorf("notify Windows applications of proxy changes: %w", err)
		}
	}
	return nil
}
