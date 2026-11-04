package core

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

//go:embed mihomo-release.json
var pinnedReleaseJSON []byte

type releaseAsset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Name   string `json:"name"`
	Format string `json:"format"`
	SHA256 string `json:"sha256"`
}

type pinnedRelease struct {
	Version string         `json:"version"`
	Assets  []releaseAsset `json:"assets"`
}

var releaseVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func parsePinnedRelease(data []byte) (pinnedRelease, error) {
	var release pinnedRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return release, fmt.Errorf("decode pinned Mihomo release: %w", err)
	}
	if !releaseVersionPattern.MatchString(release.Version) || len(release.Assets) != 6 {
		return release, errors.New("invalid pinned Mihomo release version or platform count")
	}
	seen := make(map[string]bool)
	for _, asset := range release.Assets {
		key := asset.OS + "/" + asset.Arch
		if (asset.OS != "linux" && asset.OS != "darwin" && asset.OS != "windows") ||
			(asset.Arch != "amd64" && asset.Arch != "arm64") || seen[key] {
			return release, errors.New("invalid or duplicate pinned Mihomo platform")
		}
		seen[key] = true
		format := ".gz"
		if asset.OS == "windows" {
			format = ".zip"
		}
		prefix := "mihomo-" + asset.OS + "-" + asset.Arch + "-"
		ordinary := prefix + release.Version + format
		baseline := prefix + "v1-" + release.Version + format
		if asset.Format != format || (asset.Name != ordinary && (asset.Arch != "amd64" || asset.Name != baseline)) {
			return release, errors.New("invalid pinned Mihomo archive filename or format")
		}
		digest, err := hex.DecodeString(asset.SHA256)
		if err != nil || len(digest) != 32 || asset.SHA256 != strings.ToLower(asset.SHA256) {
			return release, errors.New("invalid pinned Mihomo SHA-256")
		}
	}
	return release, nil
}

func (release pinnedRelease) asset(goos, goarch string) (releaseAsset, error) {
	for _, asset := range release.Assets {
		if asset.OS == goos && asset.Arch == goarch {
			return asset, nil
		}
	}
	return releaseAsset{}, fmt.Errorf("Mihomo is not supported on %s/%s", goos, goarch)
}

func PinnedMihomoVersion() (string, error) {
	release, err := parsePinnedRelease(pinnedReleaseJSON)
	return release.Version, err
}
