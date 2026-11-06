package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultServer = "http://127.0.0.1:9090"

type Config struct {
	Server            string `yaml:"server"`
	Secret            string `yaml:"secret"`
	ActiveProfilePath string `yaml:"active-profile-path,omitempty"`
}

type Resolved struct {
	Config
	Path           string
	Exists         bool
	ServerSource   string
	SecretSource   string
	ExplicitServer bool
}

type Overrides struct {
	ConfigPath string
	Server     string
	ServerSet  bool
	LookupEnv  func(string) (string, bool)
}

func Load(overrides Overrides) (Config, error) {
	resolved, err := Resolve(overrides)
	return resolved.Config, err
}

func Resolve(overrides Overrides) (Resolved, error) {
	lookup := overrides.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}

	path := overrides.ConfigPath
	if path == "" {
		var err error
		path, err = DefaultPath(lookup)
		if err != nil {
			return Resolved{}, fmt.Errorf("resolve config path: %w", err)
		}
	}

	cfg := Config{Server: DefaultServer}
	result := Resolved{Path: path, ServerSource: "default", SecretSource: "not set"}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Resolved{}, fmt.Errorf("read config file %s: %w", path, err)
	}
	if errors.Is(err, os.ErrNotExist) && overrides.ConfigPath != "" {
		return Resolved{}, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err == nil {
		result.Exists = true
		var fileConfig struct {
			Server            *string `yaml:"server"`
			Secret            *string `yaml:"secret"`
			ActiveProfilePath string  `yaml:"active-profile-path"`
		}
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&fileConfig); err != nil {
			return Resolved{}, fmt.Errorf("parse config file %s: %w", path, err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return Resolved{}, errors.New("config file must contain exactly one YAML document")
		}
		if fileConfig.Server != nil && *fileConfig.Server != "" {
			cfg.Server = *fileConfig.Server
			result.ServerSource = path
		}
		if fileConfig.Secret != nil {
			cfg.Secret = *fileConfig.Secret
		}
		cfg.ActiveProfilePath = fileConfig.ActiveProfilePath
		if cfg.Secret != "" {
			result.SecretSource = path
		}
		if runtime.GOOS != "windows" && cfg.Secret != "" {
			info, err := os.Stat(path)
			if err != nil {
				return Resolved{}, fmt.Errorf("inspect config file permissions: %w", err)
			}
			if info.Mode().Perm()&0077 != 0 {
				return Resolved{}, fmt.Errorf("config file contains a secret and must have Unix permissions 0600: chmod 600 %q", path)
			}
		}
	}
	if cfg.Server == "" {
		cfg.Server = DefaultServer
		result.ServerSource = "default"
	}
	if value, ok := lookup("MIOCLI_SECRET"); ok && value != "" {
		cfg.Secret = value
		result.SecretSource = "MIOCLI_SECRET"
	}
	if value, ok := lookup("MIOCLI_SERVER"); ok && value != "" {
		cfg.Server = value
		result.ServerSource = "MIOCLI_SERVER"
		result.ExplicitServer = true
	}
	if overrides.ServerSet {
		cfg.Server = overrides.Server
		result.ServerSource = "--server"
		result.ExplicitServer = true
	}

	server, err := ValidateServer(cfg.Server)
	if err != nil {
		return Resolved{}, err
	}
	cfg.Server = server
	if cfg.ActiveProfilePath != "" && (!filepath.IsAbs(cfg.ActiveProfilePath) || strings.ContainsRune(cfg.ActiveProfilePath, '\x00')) {
		return Resolved{}, errors.New("active-profile-path must be an absolute path without NUL bytes")
	}
	result.Config = cfg
	return result, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config file: %w", err)
	}
	return WritePrivate(path, data)
}

func WritePrivate(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	file, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := ProtectFile(tempPath); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write config file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync config file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}

func DefaultPath(lookup func(string) (string, bool)) (string, error) {
	if runtime.GOOS == "windows" {
		if appData, ok := lookup("APPDATA"); ok && appData != "" {
			return filepath.Join(appData, "miocli", "config.yaml"), nil
		}
		return "", errors.New("APPDATA is not set")
	}
	if xdg, ok := lookup("XDG_CONFIG_HOME"); ok && xdg != "" {
		if !filepath.IsAbs(xdg) {
			return "", errors.New("XDG_CONFIG_HOME must be an absolute path")
		}
		return filepath.Join(xdg, "miocli", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "miocli", "config.yaml"), nil
}

func ValidateServer(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid server URL: %w", err)
	}
	if parsed.Scheme == "unix" {
		if runtime.GOOS == "windows" || parsed.Host != "" || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
			parsed.Opaque != "" || !filepath.IsAbs(parsed.Path) || parsed.Path == "/" ||
			strings.ContainsRune(parsed.Path, '\x00') || filepath.Clean(parsed.Path) != parsed.Path {
			return "", errors.New("Unix controller requires an absolute normalized socket path without a host, query or fragment")
		}
		return (&url.URL{Scheme: "unix", Path: parsed.Path}).String(), nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("server URL scheme must be http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL must contain a host and must not contain credentials, query, or fragment")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", errors.New("server URL port must be between 1 and 65535")
		}
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("server URL must not contain a path")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return "", errors.New("non-loopback Mihomo servers require HTTPS; use an SSH tunnel for HTTP")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsLocalController reports whether a controller refers to this host.
func IsLocalController(server string) bool {
	parsed, err := url.Parse(server)
	if err != nil {
		return false
	}
	if parsed.Scheme == "unix" {
		return true
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
