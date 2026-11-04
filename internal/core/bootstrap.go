package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"miocli/internal/config"
	"miocli/internal/mihomo"
)

const maxConfigBytes = 16 << 20

type Paths struct {
	AppConfig string
	ConfigDir string
	BinaryDir string
}

func PathsFor(appConfig string) (Paths, error) {
	var err error
	appConfig, err = filepath.Abs(appConfig)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve managed config path: %w", err)
	}
	var dataRoot string
	if runtime.GOOS == "windows" {
		dataRoot = os.Getenv("LOCALAPPDATA")
		if dataRoot == "" {
			return Paths{}, errors.New("LOCALAPPDATA is not set")
		}
	} else {
		dataRoot = os.Getenv("XDG_DATA_HOME")
		if dataRoot == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, fmt.Errorf("resolve home directory: %w", err)
			}
			dataRoot = filepath.Join(home, ".local", "share")
		}
		if !filepath.IsAbs(dataRoot) {
			return Paths{}, errors.New("XDG_DATA_HOME must be an absolute path")
		}
	}
	return Paths{
		AppConfig: appConfig,
		ConfigDir: filepath.Join(filepath.Dir(appConfig), "mihomo"),
		BinaryDir: filepath.Join(dataRoot, "miocli", "bin"),
	}, nil
}

func EnsureRunning(ctx context.Context, cfg config.Config, paths Paths) (*Manager, error) {
	if cfg.Core.Mode != config.Managed {
		return nil, errors.New("starting a core requires core.mode: managed")
	}
	if err := ValidateManagedServer(cfg.Server); err != nil {
		return nil, err
	}
	client, err := mihomo.NewHTTPClient(cfg.Server, cfg.Secret)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	probeErr := mihomo.CheckConnection(probeCtx, client)
	cancel()
	if probeErr == nil {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !connectionRefused(probeErr) {
		return nil, fmt.Errorf("controller probe failed; refusing to start another core: %w", probeErr)
	}
	data, err := ReadConfigFile(filepath.Join(paths.ConfigDir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("managed configuration unavailable; run miocli setup: %w", err)
	}
	binary, err := FindMihomo(paths.BinaryDir, cfg.Core.Binary)
	if err != nil {
		return nil, fmt.Errorf("managed core unavailable; run miocli setup (no automatic download): %w", err)
	}
	if err := CheckManagedConfig(ctx, binary, paths.ConfigDir, data, cfg); err != nil {
		return nil, err
	}
	return startMihomo(ctx, binary, paths.ConfigDir, cfg.Server, cfg.Secret)
}

func ExpandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func ReadConfigFile(path string) ([]byte, error) {
	path, err := ExpandHome(path)
	if err != nil {
		return nil, err
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

func DownloadConfig(ctx context.Context, rawURL string) ([]byte, error) {
	client := &http.Client{
		Timeout: 45 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 || request.URL.Scheme != "https" {
				return errors.New("subscription redirect is not allowed")
			}
			return nil
		},
	}
	return downloadConfigWithClient(ctx, rawURL, client)
}

func downloadConfigWithClient(ctx context.Context, rawURL string, client *http.Client) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return nil, errors.New("subscription URL must be a valid HTTPS URL without embedded credentials")
	}
	if client == nil {
		return nil, errors.New("subscription HTTP client is required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New("could not create subscription request")
	}
	request.Header.Set("Accept", "application/yaml, text/yaml, application/octet-stream, */*")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("subscription download failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("subscription download returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxConfigBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("could not read subscription response")
	}
	if len(data) > maxConfigBytes {
		return nil, errors.New("subscription configuration exceeds 16 MiB")
	}
	return data, nil
}

func yamlMapping(data []byte) (*yaml.Node, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("configuration must be a valid YAML mapping")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("configuration must contain exactly one YAML document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("configuration must be a single YAML mapping")
	}
	var check map[string]any
	if err := document.Decode(&check); err != nil {
		return nil, errors.New("configuration has duplicate keys or invalid values")
	}
	return &document, nil
}

func ConnectionFromYAML(data []byte) (config.Config, error) {
	document, err := yamlMapping(data)
	if err != nil {
		return config.Config{}, err
	}
	root := document.Content[0]
	address := mappingScalar(root, "external-controller")
	if address == "" {
		return config.Config{}, errors.New("no HTTP external-controller; enable a loopback HTTP controller in the owning app (socket/pipe is not supported)")
	}
	if strings.Contains(address, "://") {
		return config.Config{}, errors.New("external-controller must be a host:port listener, not a URL")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return config.Config{}, errors.New("external-controller must contain a host and port; enter the actual controller URL manually")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "secret" && root.Content[i+1].Kind != yaml.ScalarNode {
			return config.Config{}, errors.New("Mihomo secret must be a scalar value")
		}
	}
	server, err := config.ValidateServer("http://" + address)
	if err != nil {
		return config.Config{}, fmt.Errorf("controller is not a supported client target; enter a loopback address manually, or use HTTPS for a remote controller: %w", err)
	}
	return config.Config{Server: server, Secret: mappingScalar(root, "secret"), Core: config.Core{Mode: config.Connect}}, nil
}

func PrepareManagedConfig(data []byte, server, secret string) (string, []byte, error) {
	if err := ValidateManagedServer(server); err != nil {
		return "", nil, err
	}
	document, err := yamlMapping(data)
	if err != nil {
		return "", nil, err
	}
	root := document.Content[0]
	if secret == "" {
		secret = mappingScalar(root, "secret")
	}
	if secret == "" {
		secretBytes := make([]byte, 32)
		if _, err := rand.Read(secretBytes); err != nil {
			return "", nil, fmt.Errorf("generate controller secret: %w", err)
		}
		secret = hex.EncodeToString(secretBytes)
	}
	parsed, err := url.Parse(server)
	if err != nil {
		return "", nil, err
	}
	setMappingScalar(root, "external-controller", parsed.Host)
	setMappingScalar(root, "secret", secret)
	// A managed copy must not expose inherited secondary control listeners.
	for _, key := range []string{"external-controller-tls", "external-controller-unix", "external-controller-pipe", "external-doh-server"} {
		removeMappingKey(root, key)
	}
	encoded, err := yaml.Marshal(document)
	if err != nil {
		return "", nil, fmt.Errorf("encode YAML: %w", err)
	}
	return secret, encoded, nil
}

func mappingScalar(mapping *yaml.Node, key string) string {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key && mapping.Content[i+1].Kind == yaml.ScalarNode {
			return mapping.Content[i+1].Value
		}
	}
	return ""
}

func setMappingScalar(mapping *yaml.Node, key, value string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

func removeMappingKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

func writeManagedConfig(path string, data []byte) error {
	return config.WritePrivate(path, data)
}
