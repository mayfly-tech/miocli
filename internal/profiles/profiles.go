package profiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"miocli/internal/config"
)

const maxProfileBytes = 16 << 20

var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Info struct {
	Name      string    `yaml:"name"`
	URL       string    `yaml:"url"`
	UpdatedAt time.Time `yaml:"updated-at"`
}

type Manager struct {
	dir    string
	client *http.Client
}

func New(configPath string) *Manager {
	return &Manager{
		dir: filepath.Join(filepath.Dir(configPath), "profiles"),
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy: nil,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 5 * time.Second,
			},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (m *Manager) List() ([]Info, error) {
	return m.read()
}

func (m *Manager) Add(ctx context.Context, name, sourceURL string) error {
	if err := validateName(name); err != nil {
		return err
	}
	sourceURL, err := validateURL(sourceURL)
	if err != nil {
		return err
	}
	entries, err := m.read()
	if err != nil {
		return err
	}
	if find(entries, name) >= 0 {
		return fmt.Errorf("profile %q already exists", name)
	}
	data, err := m.download(ctx, sourceURL)
	if err != nil {
		return err
	}
	if err := m.writeProfile(name, data); err != nil {
		return err
	}
	entries = append(entries, Info{Name: name, URL: sourceURL, UpdatedAt: time.Now().UTC()})
	if err := m.write(entries); err != nil {
		_ = os.Remove(m.profilePath(name))
		return err
	}
	return nil
}

func (m *Manager) Update(ctx context.Context, name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	entries, err := m.read()
	if err != nil {
		return err
	}
	index := find(entries, name)
	if index < 0 {
		return fmt.Errorf("profile %q does not exist", name)
	}
	data, err := m.download(ctx, entries[index].URL)
	if err != nil {
		return err
	}
	if err := m.writeProfile(name, data); err != nil {
		return err
	}
	entries[index].UpdatedAt = time.Now().UTC()
	return m.write(entries)
}

func (m *Manager) Remove(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	entries, err := m.read()
	if err != nil {
		return err
	}
	index := find(entries, name)
	if index < 0 {
		return fmt.Errorf("profile %q does not exist", name)
	}
	entries = append(entries[:index], entries[index+1:]...)
	if err := m.write(entries); err != nil {
		return err
	}
	if err := os.Remove(m.profilePath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove profile file: %w", err)
	}
	return nil
}

func (m *Manager) ConfigPath(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	entries, err := m.read()
	if err != nil {
		return "", err
	}
	if find(entries, name) < 0 {
		return "", fmt.Errorf("profile %q does not exist", name)
	}
	path, err := filepath.Abs(m.profilePath(name))
	if err != nil {
		return "", fmt.Errorf("resolve profile path: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect profile file: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("profile path is not a regular file")
	}
	return path, nil
}

func (m *Manager) read() ([]Info, error) {
	path := filepath.Join(m.dir, "profiles.yaml")
	if err := m.ensureDir(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect profiles: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("profile registry must be a regular file")
	}
	if err := config.ProtectFile(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profiles: %w", err)
	}
	var entries []Info
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if err := validateName(entry.Name); err != nil || entry.URL == "" || seen[entry.Name] {
			return nil, errors.New("profile registry contains invalid or duplicate entries")
		}
		if _, err := validateURL(entry.URL); err != nil {
			return nil, errors.New("profile registry contains an invalid source URL")
		}
		seen[entry.Name] = true
	}
	return entries, nil
}

func (m *Manager) write(entries []Info) error {
	data, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("encode profiles: %w", err)
	}
	if err := m.ensureDir(); err != nil {
		return err
	}
	return config.WritePrivate(filepath.Join(m.dir, "profiles.yaml"), data)
}

func (m *Manager) writeProfile(name string, data []byte) error {
	if err := m.ensureDir(); err != nil {
		return err
	}
	return config.WritePrivate(m.profilePath(name), data)
}

func (m *Manager) ensureDir() error {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}
	if err := os.Chmod(m.dir, 0700); err != nil {
		return fmt.Errorf("protect profiles directory: %w", err)
	}
	return nil
}

func (m *Manager) profilePath(name string) string {
	return filepath.Join(m.dir, name+".yaml")
}

func (m *Manager) download(ctx context.Context, sourceURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, errors.New("create profile download request")
	}
	request.Header.Set("Accept", "application/yaml, application/x-yaml, text/yaml, text/plain")
	response, err := m.client.Do(request)
	if err != nil {
		return nil, errors.New("download profile failed; check the URL, network and TLS certificate")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("profile server returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxProfileBytes+1))
	if err != nil {
		return nil, errors.New("read profile response failed")
	}
	if len(data) == 0 || len(data) > maxProfileBytes {
		return nil, errors.New("profile response must be between 1 byte and 16 MiB")
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("profile response is not valid YAML")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("profile must contain exactly one YAML document")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("profile must be a YAML mapping")
	}
	var fields map[string]any
	if err := yaml.Unmarshal(data, &fields); err != nil {
		return nil, errors.New("profile contains duplicate keys or invalid YAML values")
	}
	if len(fields) == 0 {
		return nil, errors.New("profile must not be empty")
	}
	return data, nil
}

func validateName(name string) error {
	if strings.EqualFold(name, "profiles") {
		return errors.New("profile name profiles is reserved for the registry")
	}
	if !validName.MatchString(name) {
		return errors.New("profile name must be 1-64 letters, digits, dots, underscores or hyphens, and start with a letter or digit")
	}
	return nil
}

func validateURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("profile URL must be HTTPS, include a host, and not contain credentials or a fragment")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", errors.New("profile URL has an invalid port")
		}
	}
	return parsed.String(), nil
}

func find(entries []Info, name string) int {
	for i := range entries {
		if entries[i].Name == name {
			return i
		}
	}
	return -1
}
