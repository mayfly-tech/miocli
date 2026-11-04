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
	"fmt"
	"io"
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
	maxReleaseBytes  = 4 << 20
	maxBinaryBytes   = 256 << 20
	maxDownloadBytes = 128 << 20
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Digest             string `json:"digest"`
	} `json:"assets"`
}

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
	name := "mihomo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(binaryDir, name)
	release, err := latestMihomoRelease(ctx)
	if err != nil {
		return "", err
	}
	assetName, archiveExt, err := mihomoAssetName(runtime.GOOS, runtime.GOARCH, release.TagName)
	if err != nil {
		return "", err
	}
	var assetURL string
	var assetDigest string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			assetURL = asset.BrowserDownloadURL
			assetDigest = asset.Digest
			break
		}
	}
	if assetURL == "" {
		return "", fmt.Errorf("Mihomo release %s has no asset for %s/%s", release.TagName, runtime.GOOS, runtime.GOARCH)
	}
	archive, err := downloadMihomoAsset(ctx, assetURL)
	if err != nil {
		return "", err
	}
	if err := verifyMihomoAsset(archive, assetDigest); err != nil {
		return "", err
	}
	binary, err := unpackMihomo(archive, archiveExt)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(binaryDir, 0700); err != nil {
		return "", fmt.Errorf("create Mihomo binary directory: %w", err)
	}
	if err := writeBinary(path, binary); err != nil {
		return "", err
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

func mihomoAssetName(goos, goarch, version string) (string, string, error) {
	if goos != "linux" && goos != "darwin" && goos != "windows" {
		return "", "", fmt.Errorf("Mihomo is not supported on %s", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", "", fmt.Errorf("Mihomo is not supported on %s/%s", goos, goarch)
	}
	extension := ".gz"
	if goos == "windows" {
		extension = ".zip"
	}
	return fmt.Sprintf("mihomo-%s-%s-%s%s", goos, goarch, version, extension), extension, nil
}

func latestMihomoRelease(ctx context.Context) (githubRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/"+mihomoRepo+"/releases/latest", nil)
	if err != nil {
		return githubRelease{}, errors.New("create Mihomo release request")
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "miocli")
	client := githubHTTPClient(30 * time.Second)
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return githubRelease{}, ctx.Err()
		}
		return githubRelease{}, errors.New("could not check the latest Mihomo release")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("Mihomo release lookup returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return githubRelease{}, ctx.Err()
		}
		return githubRelease{}, errors.New("could not read Mihomo release metadata")
	}
	if len(data) > maxReleaseBytes {
		return githubRelease{}, errors.New("Mihomo release metadata exceeds 4 MiB")
	}
	var release githubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return githubRelease{}, fmt.Errorf("decode Mihomo release metadata: %w", err)
	}
	if release.TagName == "" {
		return githubRelease{}, errors.New("Mihomo release metadata has no version")
	}
	return release, nil
}

func githubHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			host := strings.ToLower(request.URL.Hostname())
			allowedHost := host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
			if len(via) >= 5 || request.URL.Scheme != "https" || !allowedHost {
				return errors.New("Mihomo download redirect is not allowed")
			}
			return nil
		},
	}
}

func downloadMihomoAsset(ctx context.Context, assetURL string) ([]byte, error) {
	parsed, err := url.Parse(assetURL)
	if err != nil || parsed.Scheme != "https" || strings.ToLower(parsed.Hostname()) != "github.com" {
		return nil, errors.New("Mihomo release contains an invalid binary URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New("create Mihomo binary request")
	}
	request.Header.Set("User-Agent", "miocli")
	response, err := githubHTTPClient(2 * time.Minute).Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("Mihomo binary download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mihomo binary download returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDownloadBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
