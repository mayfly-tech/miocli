package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"

	"miocli/internal/mihomo"
)

const maxConfigBytes = 16 << 20

func ReadConfigFile(path string) ([]byte, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect source configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("source configuration must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source configuration: %w", err)
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect source configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("source configuration must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read source configuration: %w", err)
	}
	if len(data) > maxConfigBytes {
		return nil, errors.New("source configuration exceeds 16 MiB")
	}
	return data, nil
}

func ConnectionFromYAML(data []byte) (Config, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&document); err != nil {
		return Config{}, errors.New("configuration must be a valid YAML mapping")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Config{}, errors.New("configuration must contain exactly one YAML document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return Config{}, errors.New("configuration must be a single YAML mapping")
	}
	var fields map[string]any
	if err := document.Decode(&fields); err != nil {
		return Config{}, errors.New("configuration has duplicate keys or invalid values")
	}
	var address, secret string
	root := document.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i].Value, root.Content[i+1]
		switch key {
		case "external-controller":
			if value.Kind != yaml.ScalarNode {
				return Config{}, errors.New("Mihomo external-controller must be a scalar value")
			}
			address = value.Value
		case "secret":
			if value.Kind != yaml.ScalarNode {
				return Config{}, errors.New("Mihomo secret must be a scalar value")
			}
			secret = value.Value
		}
	}
	if address == "" {
		return Config{}, errors.New("no HTTP external-controller; enable a loopback HTTP controller in the owning app (socket/pipe is not supported)")
	}
	if strings.Contains(address, "://") {
		return Config{}, errors.New("external-controller must be a host:port listener, not a URL")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return Config{}, errors.New("external-controller must contain a host and port; enter the actual controller URL manually")
	}
	server, err := ValidateServer("http://" + address)
	if err != nil {
		return Config{}, fmt.Errorf("controller is not a supported client target; enter a loopback address manually, or use HTTPS for a remote controller: %w", err)
	}
	return Config{Server: server, Secret: secret}, nil
}

func ConnectionHint(err error) string {
	var apiErr *mihomo.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case mihomo.ErrorAuth:
			return "Authentication/permission denied: check the owning app's controller secret and MIOCLI_SECRET. miocli never starts cores."
		case mihomo.ErrorTimeout:
			return "Controller timed out: check the owning app and endpoint. miocli never starts cores."
		case mihomo.ErrorDecode, mihomo.ErrorNotFound:
			return "The endpoint is not a compatible Mihomo control API; check that it is not the proxy port."
		}
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "Connection refused: start the owning app or check its controller port. Run miocli setup to configure the connection."
	}
	return "Check the control API (not the proxy port), secret, TLS certificate and owning app; use miocli setup or miocli doctor. miocli never starts cores."
}
