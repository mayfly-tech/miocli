package systemproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"miocli/internal/config"
)

type Endpoint struct {
	Host      string
	HTTPPort  int
	SOCKSPort int
}

type snapshot struct {
	Platform string            `json:"platform"`
	Service  string            `json:"service,omitempty"`
	Values   map[string]string `json:"values"`
}

func validateEndpoint(endpoint Endpoint) error {
	if endpoint.Host != "127.0.0.1" && endpoint.Host != "::1" {
		return errors.New("system proxy host must be a loopback address")
	}
	if endpoint.HTTPPort < 1 || endpoint.HTTPPort > 65535 {
		return errors.New("Mihomo must expose an HTTP or mixed proxy port between 1 and 65535")
	}
	if endpoint.SOCKSPort < 0 || endpoint.SOCKSPort > 65535 {
		return errors.New("Mihomo SOCKS port must be between 0 and 65535")
	}
	return nil
}

func readSnapshot(path, platform, service string) (snapshot, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot{}, nil
	}
	if err != nil {
		return snapshot{}, fmt.Errorf("inspect system proxy state: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return snapshot{}, errors.New("system proxy state must be a regular file")
	}
	if err := config.ProtectFile(path); err != nil {
		return snapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot{}, fmt.Errorf("read system proxy state: %w", err)
	}
	var saved snapshot
	if err := json.Unmarshal(data, &saved); err != nil {
		return snapshot{}, fmt.Errorf("parse system proxy state: %w", err)
	}
	if saved.Platform == "" || saved.Values == nil {
		return snapshot{}, errors.New("system proxy state is incomplete")
	}
	if saved.Platform != platform || saved.Service != service {
		return snapshot{}, errors.New("system proxy state belongs to another platform or network service")
	}
	return saved, nil
}

func writeSnapshot(path, platform, service string, values map[string]string) error {
	data, err := json.Marshal(snapshot{Platform: platform, Service: service, Values: values})
	if err != nil {
		return fmt.Errorf("encode system proxy state: %w", err)
	}
	return config.WritePrivate(path, data)
}

func removeSnapshot(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove system proxy state: %w", err)
	}
	return nil
}

func statePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "system-proxy-state.json")
}
