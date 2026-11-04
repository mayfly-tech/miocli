package core

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"miocli/internal/config"
)

func TestUnpackMihomoGzip(t *testing.T) {
	var archive bytes.Buffer
	writer := gzip.NewWriter(&archive)
	if _, err := writer.Write([]byte("binary-data")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := unpackMihomo(archive.Bytes(), ".gz")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary-data" {
		t.Fatalf("unpackMihomo() = %q, want binary-data", got)
	}
}

func TestUnpackMihomoZip(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("mihomo-windows-amd64.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "binary-data"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := unpackMihomo(archive.Bytes(), ".zip")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary-data" {
		t.Fatalf("unpackMihomo() = %q, want binary-data", got)
	}
}

func TestUnpackMihomoRejectsMissingExecutable(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	if _, err := writer.Create("README.txt"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := unpackMihomo(archive.Bytes(), ".zip"); err == nil {
		t.Fatal("unpackMihomo() succeeded without executable")
	}
}

func TestPinnedMihomoPlatforms(t *testing.T) {
	release, err := parsePinnedRelease(pinnedReleaseJSON)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		goos      string
		goarch    string
		wantName  string
		wantExt   string
		wantError bool
	}{
		{goos: "linux", goarch: "amd64", wantName: "mihomo-linux-amd64-v1-v1.19.33.gz", wantExt: ".gz"},
		{goos: "linux", goarch: "arm64", wantName: "mihomo-linux-arm64-v1.19.33.gz", wantExt: ".gz"},
		{goos: "darwin", goarch: "amd64", wantName: "mihomo-darwin-amd64-v1-v1.19.33.gz", wantExt: ".gz"},
		{goos: "darwin", goarch: "arm64", wantName: "mihomo-darwin-arm64-v1.19.33.gz", wantExt: ".gz"},
		{goos: "windows", goarch: "amd64", wantName: "mihomo-windows-amd64-v1-v1.19.33.zip", wantExt: ".zip"},
		{goos: "windows", goarch: "arm64", wantName: "mihomo-windows-arm64-v1.19.33.zip", wantExt: ".zip"},
		{goos: "freebsd", goarch: "amd64", wantError: true},
		{goos: "linux", goarch: "386", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			asset, err := release.asset(test.goos, test.goarch)
			if test.wantError {
				if err == nil {
					t.Fatal("asset() succeeded for unsupported target")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if asset.Name != test.wantName || asset.Format != test.wantExt {
				t.Fatalf("asset() = %#v, want (%q, %q)", asset, test.wantName, test.wantExt)
			}
		})
	}
}

func TestPinnedReleaseRejectsInvalidManifest(t *testing.T) {
	for _, mutate := range []func(*pinnedRelease){
		func(r *pinnedRelease) { r.Version = "../latest" },
		func(r *pinnedRelease) { r.Assets = r.Assets[:5] },
		func(r *pinnedRelease) { r.Assets[1] = r.Assets[0] },
		func(r *pinnedRelease) { r.Assets[0].Name = "../mihomo.gz" },
		func(r *pinnedRelease) { r.Assets[0].SHA256 = "" },
		func(r *pinnedRelease) { r.Assets[0].Format = ".zip" },
	} {
		release, err := parsePinnedRelease(pinnedReleaseJSON)
		if err != nil {
			t.Fatal(err)
		}
		mutate(&release)
		data, err := json.Marshal(release)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parsePinnedRelease(data); err == nil {
			t.Fatalf("accepted invalid manifest: %s", data)
		}
	}
}

func TestInstallMihomoSourcePolicy(t *testing.T) {
	var archive bytes.Buffer
	writer := gzip.NewWriter(&archive)
	_, _ = writer.Write([]byte("verified-core"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	release, err := parsePinnedRelease(pinnedReleaseJSON)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(archive.Bytes())
	release.Assets[0].SHA256 = hex.EncodeToString(digest[:])
	manifest, err := json.Marshal(release)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name         string
		firstErr     error
		secondErr    error
		corrupt      bool
		badArchive   bool
		cancel       bool
		wantRequests int
		wantSuccess  bool
	}{
		{name: "github", wantRequests: 1, wantSuccess: true},
		{name: "fallback", firstErr: &sourceUnavailable{"HTTP 503"}, wantRequests: 2, wantSuccess: true},
		{name: "both unavailable", firstErr: &sourceUnavailable{"HTTP 503"}, secondErr: &sourceUnavailable{"HTTP 404"}, wantRequests: 2},
		{name: "policy rejected", firstErr: errors.New("invalid URL"), wantRequests: 1},
		{name: "checksum", corrupt: true, wantRequests: 1},
		{name: "corrupt archive", badArchive: true, wantRequests: 1},
		{name: "cancelled", cancel: true, wantRequests: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			testManifest := manifest
			if test.badArchive {
				broken := release
				broken.Assets = append([]releaseAsset(nil), release.Assets...)
				digest := sha256.Sum256([]byte("invalid gzip"))
				broken.Assets[0].SHA256 = hex.EncodeToString(digest[:])
				var err error
				testManifest, err = json.Marshal(broken)
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dir := t.TempDir()
			path := filepath.Join(dir, "mihomo")
			if err := os.WriteFile(path, []byte("old-core"), 0700); err != nil {
				t.Fatal(err)
			}
			var requests []string
			var events []DownloadProgress
			download := func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
				requests = append(requests, url)
				if len(requests) == 1 {
					if !strings.HasPrefix(url, "https://github.com/") || timeout != 30*time.Second {
						t.Fatalf("primary request = %s, %s", url, timeout)
					}
					if test.cancel {
						cancel()
						return nil, &sourceUnavailable{"timed out"}
					}
					if test.firstErr != nil {
						return nil, test.firstErr
					}
				} else {
					if url != distributionRoot+"/mihomo/releases/"+release.Version+"/"+release.Assets[0].Name || timeout != 180*time.Second {
						t.Fatalf("mirror request = %s, %s", url, timeout)
					}
					if test.secondErr != nil {
						return nil, test.secondErr
					}
				}
				if test.corrupt {
					return []byte("wrong archive"), nil
				}
				if test.badArchive {
					return []byte("invalid gzip"), nil
				}
				return archive.Bytes(), nil
			}
			_, err := installMihomo(ctx, dir, "linux", "amd64", testManifest, download, func(event DownloadProgress) {
				events = append(events, event)
			})
			if (err == nil) != test.wantSuccess || len(requests) != test.wantRequests {
				t.Fatalf("error=%v requests=%v", err, requests)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			want := "old-core"
			if test.wantSuccess {
				want = "verified-core"
				if len(events) == 0 || events[len(events)-1].Fallback {
					t.Fatal("missing verified source notification")
				}
			}
			if string(got) != want {
				t.Fatalf("core = %q, want %q", got, want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMihomoHTTPFailureClassification(t *testing.T) {
	for _, test := range []struct {
		name        string
		status      int
		err         error
		unavailable bool
	}{
		{name: "http", status: 503, unavailable: true},
		{name: "unexpected success status", status: 204},
		{name: "unexpected redirect status", status: 302},
		{name: "network", err: &net.OpError{Op: "dial", Err: errors.New("refused")}, unavailable: true},
		{name: "closed connection", err: io.EOF, unavailable: true},
		{name: "policy", err: errors.New("redirect rejected")},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				if test.err != nil {
					return nil, test.err
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}
			request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://github.com/asset", nil)
			_, err := readMihomoResponse(request.Context(), client, request)
			var unavailable *sourceUnavailable
			if errors.As(err, &unavailable) != test.unavailable {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDownloadMihomoRejectsUntrustedDestinations(t *testing.T) {
	for _, url := range []string{"http://github.com/file", "https://evil.test/file", "https://s3.usemayfly.tech/private/file",
		"https://user:password@github.com/file", "https://github.com:8443/file"} {
		if _, err := downloadMihomoAsset(context.Background(), url, time.Second); err == nil {
			t.Fatalf("accepted %s", url)
		}
	}
	for _, mirror := range []bool{false, true} {
		client := downloadHTTPClient(30*time.Second, mirror)
		request, _ := http.NewRequest(http.MethodGet, "https://evil.test/file", nil)
		if err := client.CheckRedirect(request, nil); err == nil {
			t.Fatal("accepted foreign redirect")
		}
		request, _ = http.NewRequest(http.MethodGet, distributionRoot+"/mihomo/releases/v1.19.33/file", nil)
		if err := client.CheckRedirect(request, nil); (err == nil) != mirror {
			t.Fatalf("mirror redirect policy: %v", err)
		}
		client.CloseIdleConnections()
	}
}

func TestMihomoReadLimitsAndCancellation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/large" {
			w.Header().Set("Content-Length", "134217729")
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if _, err := readMihomoResponse(ctx, server.Client(), request); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
	request, _ = http.NewRequest(http.MethodGet, server.URL+"/large", nil)
	if _, err := readMihomoResponse(request.Context(), server.Client(), request); err == nil || !strings.Contains(err.Error(), "128 MiB") {
		t.Fatalf("limit = %v", err)
	}
}

func TestIsolatedS3Bootstrap(t *testing.T) {
	archivePath := os.Getenv("MIOCLI_TEST_ARCHIVE")
	if archivePath == "" {
		t.Skip("set MIOCLI_TEST_ARCHIVE to the verified pinned archive for this platform")
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	release, err := parsePinnedRelease(pinnedReleaseJSON)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := release.asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyMihomoAsset(archive, "sha256:"+asset.SHA256); err != nil {
		t.Fatal(err)
	}
	var requests []string
	mirror := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		_, _ = w.Write(archive)
	}))
	defer mirror.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	download := func(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
		if strings.HasPrefix(url, "https://github.com/") {
			return nil, &sourceUnavailable{"GitHub deliberately blocked"}
		}
		expected := distributionRoot + "/mihomo/releases/" + release.Version + "/" + asset.Name
		if url != expected {
			t.Fatalf("mirror URL = %s, want %s", url, expected)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet,
			mirror.URL+"/public/mihomo/releases/"+release.Version+"/"+asset.Name, nil)
		if err != nil {
			return nil, err
		}
		return readMihomoResponse(ctx, mirror.Client(), request)
	}
	dir := t.TempDir()
	binary, err := installMihomo(ctx, filepath.Join(dir, "bin"), runtime.GOOS, runtime.GOARCH, pinnedReleaseJSON, download, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("mirror requests = %v", requests)
	}
	version, err := BinaryVersion(ctx, binary)
	if err != nil || !strings.Contains(version, release.Version) {
		t.Fatalf("binary version = %q, %v", version, err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{Server: "http://" + address, Core: config.Core{Mode: config.Managed}}
	cfg.Secret, archive, err = PrepareManagedConfig([]byte("log-level: silent\nmode: direct\nrules: ['MATCH,DIRECT']\n"), cfg.Server, "")
	if err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(dir, "config")
	if err := os.Mkdir(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	if err := CheckManagedConfig(ctx, binary, configDir, archive, cfg); err != nil {
		t.Fatal(err)
	}
	manager, err := startMihomo(ctx, binary, configDir, cfg.Server, cfg.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Verified pinned %s bootstrap and child cleanup with GitHub blocked.", release.Version)
}
func TestVerifyMihomoAsset(t *testing.T) {
	data := []byte("mihomo-binary")
	digest := sha256.Sum256(data)
	if err := verifyMihomoAsset(data, "sha256:"+hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
	if err := verifyMihomoAsset(data, "sha256:"+hex.EncodeToString(make([]byte, sha256.Size))); err == nil {
		t.Fatal("verifyMihomoAsset() accepted a mismatched digest")
	}
	if err := verifyMihomoAsset(data, ""); err == nil {
		t.Fatal("verifyMihomoAsset() accepted a missing digest")
	}
}
