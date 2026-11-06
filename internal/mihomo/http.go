package mihomo

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes = 1 << 20
	requestTimeout   = 5 * time.Second
	reloadTimeout    = 120 * time.Second
)

type HTTPClient struct {
	base   *url.URL
	secret string
	client *http.Client
}

func NewHTTPClient(server, secret string) (*HTTPClient, error) {
	base, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parse Mihomo server URL: %w", err)
	}
	unixSocket := ""
	if base.Scheme == "unix" {
		if runtime.GOOS == "windows" || base.Host != "" || base.User != nil ||
			base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || base.Opaque != "" ||
			!filepath.IsAbs(base.Path) || base.Path == "/" ||
			strings.ContainsRune(base.Path, '\x00') || filepath.Clean(base.Path) != base.Path {
			return nil, errors.New("Unix controller requires an absolute normalized socket path without a host, query or fragment")
		}
		unixSocket = base.Path
		base = &url.URL{Scheme: "http", Host: "localhost"}
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, errors.New("Mihomo server URL scheme must be http or https")
	}
	if base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("invalid Mihomo server URL")
	}
	if port := base.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, errors.New("Mihomo server URL port must be between 1 and 65535")
		}
	}
	if base.Path != "" && base.Path != "/" {
		return nil, errors.New("Mihomo server URL must not contain a path")
	}
	if base.Scheme == "http" && !isLoopbackHost(base.Hostname()) {
		return nil, errors.New("non-loopback Mihomo servers require HTTPS")
	}
	base.Path = ""
	base.RawPath = ""

	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 3 * time.Second,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}
	if unixSocket != "" {
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", unixSocket)
		}
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return &HTTPClient{base: base, secret: secret, client: client}, nil
}

func (c *HTTPClient) Version(ctx context.Context) (Version, error) {
	var version Version
	if err := c.doJSON(ctx, "GET", "/version", nil, &version); err != nil {
		return Version{}, err
	}
	if version.Version == "" {
		return Version{}, &APIError{Kind: ErrorDecode, Op: "decode version", Err: errors.New("missing version")}
	}
	return version, nil
}

func (c *HTTPClient) Proxies(ctx context.Context) (Proxies, error) {
	var response struct {
		Proxies map[string]proxyDTO `json:"proxies"`
	}
	if err := c.doJSON(ctx, "GET", "/proxies", nil, &response); err != nil {
		return Proxies{}, err
	}
	if response.Proxies == nil {
		return Proxies{}, &APIError{Kind: ErrorDecode, Op: "decode proxies", Err: errors.New("missing proxies object")}
	}

	result := Proxies{Items: make(map[string]Proxy, len(response.Proxies))}
	for name, item := range response.Proxies {
		result.Items[name] = Proxy{
			Name:    name,
			Type:    item.Type,
			Now:     item.Now,
			All:     append([]string(nil), item.All...),
			History: append([]DelayHistory(nil), item.History...),
		}
	}
	return result, nil
}

func (c *HTTPClient) Mode(ctx context.Context) (Mode, error) {
	var response struct {
		Mode string `json:"mode"`
	}
	if err := c.doJSON(ctx, "GET", "/configs", nil, &response); err != nil {
		return "", err
	}
	return Mode(strings.ToLower(response.Mode)), nil
}

func (c *HTTPClient) RuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	var runtimeConfig RuntimeConfig
	if err := c.doJSON(ctx, http.MethodGet, "/configs", nil, &runtimeConfig); err != nil {
		return RuntimeConfig{}, err
	}
	if runtimeConfig.Mode == "" {
		return RuntimeConfig{}, &APIError{Kind: ErrorDecode, Op: "decode configs", Err: errors.New("missing mode")}
	}
	return runtimeConfig, nil
}

func (c *HTTPClient) ReloadConfig(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("Mihomo configuration path must not be empty")
	}
	// Reload can synchronously initialize providers and download Geo databases.
	client := *c.client
	client.Timeout = reloadTimeout
	reloader := *c
	reloader.client = &client
	return reloader.doJSON(ctx, http.MethodPut, "/configs?force=true", map[string]string{"path": path}, nil)
}

func (c *HTTPClient) Select(ctx context.Context, group, name string) error {
	return c.doJSON(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), map[string]string{"name": name}, nil)
}

func (c *HTTPClient) SetMode(ctx context.Context, mode Mode) error {
	if !validMode(mode) {
		return fmt.Errorf("unsupported proxy mode %q", mode)
	}
	return c.doJSON(ctx, http.MethodPatch, "/configs", map[string]Mode{"mode": mode}, nil)
}

func (c *HTTPClient) Providers(ctx context.Context) (map[string]Provider, error) {
	var response struct {
		Providers map[string]Provider `json:"providers"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/providers/proxies", nil, &response); err != nil {
		return nil, err
	}
	if response.Providers == nil {
		return nil, &APIError{Kind: ErrorDecode, Op: "decode providers", Err: errors.New("missing providers object")}
	}
	for name, provider := range response.Providers {
		provider.Name = name
		response.Providers[name] = provider
	}
	return response.Providers, nil
}

func (c *HTTPClient) UpdateProvider(ctx context.Context, name string) error {
	return c.doJSON(ctx, http.MethodPut, "/providers/proxies/"+url.PathEscape(name), nil, nil)
}

func (c *HTTPClient) Connections(ctx context.Context) ([]Connection, error) {
	var response struct {
		Connections json.RawMessage `json:"connections"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/connections", nil, &response); err != nil {
		return nil, err
	}
	if len(response.Connections) == 0 {
		return nil, &APIError{Kind: ErrorDecode, Op: "decode connections", Err: errors.New("missing connections")}
	}
	var connections []Connection
	if err := json.Unmarshal(response.Connections, &connections); err != nil {
		return nil, &APIError{Kind: ErrorDecode, Op: "decode connections", Err: errors.New("invalid connections")}
	}
	return connections, nil
}

func (c *HTTPClient) Delay(ctx context.Context, name, testURL string, timeoutMS int) (int, error) {
	query := url.Values{}
	query.Set("url", testURL)
	query.Set("timeout", strconv.Itoa(timeoutMS))
	endpoint := "/proxies/" + url.PathEscape(name) + "/delay?" + query.Encode()
	var response struct {
		Delay *int `json:"delay"`
	}

	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return 0, err
	}
	if response.Delay == nil || *response.Delay < 0 {
		return 0, &APIError{Kind: ErrorDecode, Op: "decode delay", Err: errors.New("missing or negative delay")}
	}
	return *response.Delay, nil
}

func (c *HTTPClient) DelayProvider(ctx context.Context, provider, name, testURL string, timeoutMS int) (int, error) {
	query := url.Values{"url": {testURL}, "timeout": {strconv.Itoa(timeoutMS)}}
	endpoint := "/providers/proxies/" + url.PathEscape(provider) + "/" + url.PathEscape(name) + "/healthcheck?" + query.Encode()
	var response struct {
		Delay *int `json:"delay"`
	}
	if err := c.doJSON(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return 0, err
	}
	if response.Delay == nil || *response.Delay < 0 {
		return 0, &APIError{Kind: ErrorDecode, Op: "decode provider delay", Err: errors.New("missing or negative delay")}
	}
	return *response.Delay, nil
}

type proxyDTO struct {
	Type    string         `json:"type"`
	Now     string         `json:"now"`
	All     []string       `json:"all"`
	History []DelayHistory `json:"history"`
}

func (c *HTTPClient) doJSON(ctx context.Context, method, endpoint string, body any, dst any) error {
	requestURL, err := c.endpoint(endpoint)
	if err != nil {
		return &APIError{Kind: ErrorNetwork, Op: method + " " + endpoint, Err: err}
	}

	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return &APIError{Kind: ErrorDecode, Op: method + " " + endpoint, Err: err}
		}
		requestBody = bytes.NewReader(data)
	}

	request, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return &APIError{Kind: ErrorNetwork, Op: method + " " + endpoint, Err: err}
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		request.Header.Set("Authorization", "Bearer "+c.secret)
	}

	response, err := c.client.Do(request)
	if err != nil {
		kind := ErrorNetwork
		if errors.Is(err, context.DeadlineExceeded) {
			kind = ErrorTimeout
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			kind = ErrorTimeout
		}
		return &APIError{Kind: kind, Op: method + " " + endpoint, Err: err}
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return &APIError{Kind: ErrorNetwork, Op: method + " " + endpoint, Err: err}
	}
	if len(data) > maxResponseBytes {
		return &APIError{Kind: ErrorDecode, Op: method + " " + endpoint, Err: errors.New("response exceeds 1 MiB")}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		kind := ErrorAPI
		switch response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			kind = ErrorAuth
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			kind = ErrorNotFound
		}
		return &APIError{Kind: kind, Status: response.StatusCode, Op: method + " " + endpoint}
	}
	if dst == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return &APIError{Kind: ErrorDecode, Op: method + " " + endpoint, Err: err}
	}
	return nil
}

func (c *HTTPClient) endpoint(endpoint string) (string, error) {
	copyURL := *c.base
	path, query, _ := strings.Cut(endpoint, "?")
	rawPath := strings.TrimRight(copyURL.EscapedPath(), "/") + path
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil {
		return "", err
	}
	copyURL.Path = decodedPath
	copyURL.RawPath = rawPath
	copyURL.RawQuery = query
	return copyURL.String(), nil
}

func validMode(mode Mode) bool {
	return mode == ModeRule || mode == ModeGlobal || mode == ModeDirect
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func httpStatusText(status int) string {
	return strconv.Itoa(status)
}
