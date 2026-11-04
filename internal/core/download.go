package core

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	mihomoRepo       = "MetaCubeX/mihomo"
	maxBinaryBytes   = 256 << 20
	maxDownloadBytes = 128 << 20
)

func FindMihomo(binaryDir, explicit string) (string, error) {
	if explicit != "" {
		path, err := filepath.Abs(explicit)
		if err != nil {
			return "", fmt.Errorf("resolve Mihomo binary: %w", err)
		}
		return inspectBinary(path)
	}
	if binary, err := exec.LookPath("mihomo"); err == nil {
		return inspectBinary(binary)
	} else if !errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("find Mihomo in PATH: %w", err)
	}
	name := "mihomo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(binaryDir, name)
	return inspectBinary(path)
}

func inspectBinary(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect Mihomo binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("Mihomo binary path is not a regular file: %s", path)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("Mihomo binary is not executable: %s", path)
	}
	return path, nil
}

func BinaryVersion(ctx context.Context, binary string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "-v").Output()
	if err != nil {
		return "", fmt.Errorf("check Mihomo binary version: %w", err)
	}
	version := strings.TrimSpace(string(output))
	if !strings.HasPrefix(version, "Mihomo ") || len(version) > 4096 {
		return "", errors.New("executable did not identify itself as Mihomo")
	}
	return strings.SplitN(version, "\n", 2)[0], nil
}

func InstallMihomo(ctx context.Context, binaryDir string) (string, error) {
	return InstallMihomoWithProgress(ctx, binaryDir, nil)
}

type DownloadProgress struct {
	Source string
}

func InstallMihomoWithProgress(ctx context.Context, binaryDir string, progress func(DownloadProgress)) (string, error) {
	return installMihomo(ctx, binaryDir, runtime.GOOS, runtime.GOARCH, pinnedReleaseJSON, downloadMihomoAsset, progress)
}

func installMihomo(ctx context.Context, binaryDir, goos, goarch string, manifest []byte,
	download func(context.Context, string, time.Duration) ([]byte, error), progress func(DownloadProgress)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	name := "mihomo"
	if goos == "windows" {
		name += ".exe"
	}
	path := filepath.Join(binaryDir, name)
	release, err := parsePinnedRelease(manifest)
	if err != nil {
		return "", err
	}
	asset, err := release.asset(goos, goarch)
	if err != nil {
		return "", err
	}
	githubURL := "https://github.com/" + mihomoRepo + "/releases/download/" + release.Version + "/" + asset.Name
	archive, err := download(ctx, githubURL, 30*time.Second)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := verifyMihomoAsset(archive, "sha256:"+asset.SHA256); err != nil {
		return "", err
	}
	binary, err := unpackMihomo(archive, asset.Format)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(binaryDir, 0700); err != nil {
		return "", fmt.Errorf("create Mihomo binary directory: %w", err)
	}
	if err := writeBinary(path, binary); err != nil {
		return "", err
	}
	if progress != nil {
		progress(DownloadProgress{Source: "GitHub"})
	}
	return path, nil
}

func verifyMihomoAsset(data []byte, digest string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		return errors.New("Mihomo release asset has no SHA-256 digest")
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(digest, prefix))
	if err != nil || len(expected) != sha256.Size {
		return errors.New("Mihomo release asset has an invalid SHA-256 digest")
	}
	actual := sha256.Sum256(data)
	if !bytes.Equal(actual[:], expected) {
		return errors.New("Mihomo binary checksum verification failed")
	}
	return nil
}

type sourceUnavailable struct{ reason string }

func (err *sourceUnavailable) Error() string { return err.reason }

func downloadHTTPClient(timeout time.Duration) *http.Client {
	dialTimeout := 8 * time.Second
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: dialTimeout}).DialContext
	transport.TLSHandshakeTimeout = dialTimeout
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			host := strings.ToLower(request.URL.Hostname())
			allowedHost := host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
			if len(via) >= 5 || request.URL.Scheme != "https" || !allowedHost || request.URL.User != nil ||
				(request.URL.Port() != "" && request.URL.Port() != "443") {
				return errors.New("Mihomo download redirect is not allowed")
			}
			return nil
		},
	}
}

func downloadMihomoAsset(ctx context.Context, assetURL string, timeout time.Duration) ([]byte, error) {
	parsed, err := url.Parse(assetURL)
	if err != nil {
		return nil, errors.New("Mihomo release contains an invalid binary URL")
	}
	if parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Port() != "" && parsed.Port() != "443") ||
		parsed.Hostname() != "github.com" {
		return nil, errors.New("Mihomo release contains an invalid binary URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New("create Mihomo binary request")
	}
	request.Header.Set("User-Agent", "miocli")
	client := downloadHTTPClient(timeout)
	defer client.CloseIdleConnections()
	return readMihomoResponse(ctx, client, request)
}

func readMihomoResponse(ctx context.Context, client *http.Client, request *http.Request) ([]byte, error) {
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Redirect and TLS trust failures must not be disguised as availability failures.
		var trust *tls.CertificateVerificationError
		if errors.As(err, &trust) {
			return nil, errors.New("Mihomo binary TLS certificate verification failed")
		}
		var network *net.OpError
		var urlErr *url.Error
		if errors.As(err, &network) || (errors.As(err, &urlErr) && urlErr.Timeout()) ||
			errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, &sourceUnavailable{reason: "Mihomo binary transport failed or timed out"}
		}
		return nil, errors.New("Mihomo binary request rejected (TLS or redirect policy)")
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return nil, &sourceUnavailable{reason: fmt.Sprintf("Mihomo binary download returned HTTP %d", response.StatusCode)}
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("Mihomo binary response must be HTTP 200")
	}
	if response.ContentLength > maxDownloadBytes {
		return nil, errors.New("Mihomo binary archive exceeds 128 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDownloadBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, &sourceUnavailable{reason: "Mihomo binary transfer was interrupted"}
		}
		var network net.Error
		if errors.As(err, &network) {
			return nil, &sourceUnavailable{reason: "Mihomo binary transfer failed or timed out"}
		}
		return nil, errors.New("could not read Mihomo binary archive")
	}
	if len(data) > maxDownloadBytes {
		return nil, errors.New("Mihomo binary archive exceeds 128 MiB")
	}
	return data, nil
}

func unpackMihomo(archive []byte, extension string) ([]byte, error) {
	switch extension {
	case ".gz":
		reader, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, fmt.Errorf("open Mihomo gzip archive: %w", err)
		}
		defer reader.Close()
		return readLimited(reader, maxBinaryBytes)
	case ".zip":
		reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("open Mihomo zip archive: %w", err)
		}
		for _, file := range reader.File {
			name := strings.ToLower(filepath.Base(file.Name))
			if name == "mihomo.exe" || strings.HasPrefix(name, "mihomo-") && strings.HasSuffix(name, ".exe") {
				stream, err := file.Open()
				if err != nil {
					return nil, fmt.Errorf("open Mihomo executable in archive: %w", err)
				}
				defer stream.Close()
				return readLimited(stream, maxBinaryBytes)
			}
		}
		return nil, errors.New("Mihomo zip archive does not contain mihomo.exe")
	default:
		return nil, errors.New("unsupported Mihomo archive format")
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read Mihomo executable: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, errors.New("Mihomo executable exceeds 256 MiB")
	}
	return data, nil
}

func writeBinary(path string, data []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".mihomo-*")
	if err != nil {
		return fmt.Errorf("create temporary Mihomo binary: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0700); err != nil {
		file.Close()
		return fmt.Errorf("set Mihomo binary permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write Mihomo binary: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync Mihomo binary: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close Mihomo binary: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("install Mihomo binary: %w", err)
	}
	return nil
}
