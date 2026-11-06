package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"

	"gopkg.in/yaml.v3"
)

// ProfileWithController keeps subscription network policy but not its management listeners.
func ProfileWithController(data []byte, cfg Config) ([]byte, error) {
	var fields map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, errors.New("profile must be a valid YAML mapping without duplicate keys")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("profile must contain exactly one YAML document")
	}
	server, err := ValidateServer(cfg.Server)
	if err != nil {
		return nil, err
	}
	target, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parse profile controller: %w", err)
	}
	// Disable other listener types so a subscription cannot expose an additional controller.
	for _, key := range []string{"external-controller", "external-controller-tls", "external-controller-unix", "external-controller-pipe"} {
		fields[key] = ""
	}
	switch target.Scheme {
	case "unix":
		fields["external-controller-unix"] = target.Path
	case "http", "https":
		port := target.Port()
		if port == "" {
			port = "80"
			if target.Scheme == "https" {
				port = "443"
			}
		}
		key := "external-controller"
		if target.Scheme == "https" {
			key = "external-controller-tls"
		}
		fields[key] = net.JoinHostPort(target.Hostname(), port)
	}
	fields["secret"] = cfg.Secret
	result, err := yaml.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode effective profile: %w", err)
	}
	return result, nil
}
