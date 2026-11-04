package core

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
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

func TestMihomoAssetName(t *testing.T) {
	tests := []struct {
		goos      string
		goarch    string
		wantName  string
		wantExt   string
		wantError bool
	}{
		{goos: "linux", goarch: "amd64", wantName: "mihomo-linux-amd64-v1.2.3.gz", wantExt: ".gz"},
		{goos: "linux", goarch: "arm64", wantName: "mihomo-linux-arm64-v1.2.3.gz", wantExt: ".gz"},
		{goos: "darwin", goarch: "amd64", wantName: "mihomo-darwin-amd64-v1.2.3.gz", wantExt: ".gz"},
		{goos: "darwin", goarch: "arm64", wantName: "mihomo-darwin-arm64-v1.2.3.gz", wantExt: ".gz"},
		{goos: "windows", goarch: "amd64", wantName: "mihomo-windows-amd64-v1.2.3.zip", wantExt: ".zip"},
		{goos: "windows", goarch: "arm64", wantName: "mihomo-windows-arm64-v1.2.3.zip", wantExt: ".zip"},
		{goos: "freebsd", goarch: "amd64", wantError: true},
		{goos: "linux", goarch: "386", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.goos+"/"+test.goarch, func(t *testing.T) {
			name, extension, err := mihomoAssetName(test.goos, test.goarch, "v1.2.3")
			if test.wantError {
				if err == nil {
					t.Fatal("mihomoAssetName() succeeded for unsupported target")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name != test.wantName || extension != test.wantExt {
				t.Fatalf("mihomoAssetName() = (%q, %q), want (%q, %q)", name, extension, test.wantName, test.wantExt)
			}
		})
	}
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
