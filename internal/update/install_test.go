package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tarGzWith(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	writer := tar.NewWriter(gz)
	for _, entry := range []struct {
		name string
		body []byte
	}{{"LICENSE", []byte("license")}, {name, content}} {
		if err := writer.WriteHeader(&tar.Header{Name: entry.name, Mode: 0o755, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeReleaseHost serves an archive plus the checksums.txt GoReleaser publishes.
func fakeReleaseHost(t *testing.T, asset string, archive []byte, checksum string) string {
	t.Helper()
	if checksum == "" {
		sum := sha256.Sum256(archive)
		checksum = hex.EncodeToString(sum[:])
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", checksum, asset)
	})
	mux.HandleFunc("/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

func TestBinaryVerifiesTheChecksum(t *testing.T) {
	asset := "pyahu_Darwin_arm64.tar.gz"
	archive := tarGzWith(t, "pyahu", []byte("the real binary"))
	downloader := &Downloader{HTTP: http.DefaultClient, BaseURL: fakeReleaseHost(t, asset, archive, "")}

	release, err := ReleaseFor("0.7.0", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := downloader.Binary(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if string(binary) != "the real binary" {
		t.Fatalf("binary = %q", binary)
	}
}

func TestBinaryRejectsATamperedArchive(t *testing.T) {
	asset := "pyahu_Darwin_arm64.tar.gz"
	archive := tarGzWith(t, "pyahu", []byte("tampered"))
	// checksums.txt advertises a different artifact than the one served.
	host := fakeReleaseHost(t, asset, archive, strings.Repeat("a", 64))
	downloader := &Downloader{HTTP: http.DefaultClient, BaseURL: host}

	release, _ := ReleaseFor("0.7.0", "darwin", "arm64")
	_, err := downloader.Binary(context.Background(), release)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("a tampered archive must be refused, got %v", err)
	}
}

func TestBinaryRejectsAnArchiveWithoutPyahu(t *testing.T) {
	asset := "pyahu_Linux_x86_64.tar.gz"
	archive := tarGzWith(t, "something-else", []byte("nope"))
	downloader := &Downloader{HTTP: http.DefaultClient, BaseURL: fakeReleaseHost(t, asset, archive, "")}

	release, _ := ReleaseFor("0.7.0", "linux", "amd64")
	_, err := downloader.Binary(context.Background(), release)
	if err == nil || !strings.Contains(err.Error(), "does not contain a pyahu binary") {
		t.Fatalf("error = %v", err)
	}
}

func TestReleaseForBuildsTheGoReleaserURLs(t *testing.T) {
	cases := []struct {
		goos, goarch, asset string
	}{
		{"darwin", "arm64", "pyahu_Darwin_arm64.tar.gz"},
		{"darwin", "amd64", "pyahu_Darwin_x86_64.tar.gz"},
		{"linux", "arm64", "pyahu_Linux_arm64.tar.gz"},
		{"linux", "amd64", "pyahu_Linux_x86_64.tar.gz"},
	}
	for _, tc := range cases {
		release, err := ReleaseFor("v0.7.0", tc.goos, tc.goarch)
		if err != nil {
			t.Fatalf("%s/%s: %v", tc.goos, tc.goarch, err)
		}
		if release.Asset != tc.asset {
			t.Fatalf("%s/%s asset = %q", tc.goos, tc.goarch, release.Asset)
		}
		wantURL := "https://github.com/pyahu/cli/releases/download/v0.7.0/" + tc.asset
		if release.AssetURL != wantURL {
			t.Fatalf("asset URL = %q", release.AssetURL)
		}
		if release.ChecksumsURL != "https://github.com/pyahu/cli/releases/download/v0.7.0/checksums.txt" {
			t.Fatalf("checksums URL = %q", release.ChecksumsURL)
		}
	}
}

func TestUnsupportedPlatformsAreRefused(t *testing.T) {
	for _, tc := range []struct{ goos, goarch string }{
		{"windows", "amd64"}, // cannot replace a running .exe
		{"linux", "386"},
		{"plan9", "amd64"},
	} {
		if _, err := AssetName(tc.goos, tc.goarch); !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("%s/%s: err = %v", tc.goos, tc.goarch, err)
		}
	}
}

func TestReplaceSwapsTheBinaryAndKeepsItExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("content = %q", content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("binary is not executable: %v", info.Mode())
	}
	// The staged file must not survive next to the binary.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("leftover files in the install dir: %d", len(entries))
	}
}

func TestReplaceRefusesAnEmptyBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Replace(path, nil); err == nil {
		t.Fatal("an empty binary must be refused")
	}
	// The existing binary must survive a refused install.
	content, _ := os.ReadFile(path)
	if string(content) != "old" {
		t.Fatalf("the previous binary was clobbered: %q", content)
	}
}

func TestReplaceReportsAnUnwritableDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := Replace(path, []byte("new"))
	if err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("error = %v", err)
	}
}

func TestChecksumForReadsTheGoReleaserFormat(t *testing.T) {
	checksums := "aaa  pyahu_Linux_x86_64.tar.gz\nbbb  pyahu_Darwin_arm64.tar.gz\n"
	if got, ok := checksumFor(checksums, "pyahu_Darwin_arm64.tar.gz"); !ok || got != "bbb" {
		t.Fatalf("got %q ok=%t", got, ok)
	}
	if _, ok := checksumFor(checksums, "pyahu_Windows_arm64.zip"); ok {
		t.Fatal("an unlisted asset must not resolve")
	}
}

func TestManagerDecidesWhoUpgrades(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "/home/dev/go")

	cases := []struct {
		executable  string
		manager     Manager
		selfManaged bool
		command     string
	}{
		{"/usr/local/bin/pyahu", ManagerStandalone, true, "pyahu upgrade"},
		{"/home/dev/.local/share/mise/installs/github-pyahu-cli/0.4.0/pyahu", ManagerMise, false, "mise use github:pyahu/cli@0.7.0"},
		{"/home/dev/go/bin/pyahu", ManagerGo, false, "go install github.com/pyahu/cli/cmd/pyahu@latest"},
	}
	for _, tc := range cases {
		manager := DetectManager(tc.executable)
		if manager != tc.manager || manager.SelfManaged() != tc.selfManaged {
			t.Fatalf("%s: manager=%v selfManaged=%t", tc.executable, manager, manager.SelfManaged())
		}
		if got := manager.UpgradeCommand("v0.7.0"); got != tc.command {
			t.Fatalf("%s: command = %q", tc.executable, got)
		}
	}
}
