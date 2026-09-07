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
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	downloadBase    = "https://github.com/pyahu/cli/releases/download"
	downloadTimeout = 2 * time.Minute

	// maxAssetSize bounds what a redirect or a wrong URL can make the CLI read
	// into memory. The release tarball is a few MB.
	maxAssetSize = 128 << 20
)

// ErrUnsupportedPlatform is returned for a platform that cannot be upgraded in
// place. Windows cannot rename over a running executable.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// Release locates one published build.
type Release struct {
	Version      string
	Asset        string
	AssetURL     string
	ChecksumsURL string
}

// ReleaseFor builds the download locations for a version and platform. The
// asset names mirror the GoReleaser archive template, which is what the install
// script downloads too.
func ReleaseFor(version string, goos string, goarch string) (Release, error) {
	asset, err := AssetName(goos, goarch)
	if err != nil {
		return Release{}, err
	}
	tag := "v" + normalize(version)
	return Release{
		Version:      normalize(version),
		Asset:        asset,
		AssetURL:     fmt.Sprintf("%s/%s/%s", downloadBase, tag, asset),
		ChecksumsURL: fmt.Sprintf("%s/%s/checksums.txt", downloadBase, tag),
	}, nil
}

// AssetName returns the release archive for a platform, or ErrUnsupportedPlatform.
func AssetName(goos string, goarch string) (string, error) {
	var arch string
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("%w: architecture %s", ErrUnsupportedPlatform, goarch)
	}
	switch goos {
	case "darwin":
		return "pyahu_Darwin_" + arch + ".tar.gz", nil
	case "linux":
		return "pyahu_Linux_" + arch + ".tar.gz", nil
	case "windows":
		// The Windows build ships a zip, and Windows will not let a running
		// executable be replaced under it. Download it from the releases page.
		return "", fmt.Errorf("%w: %s cannot be upgraded in place", ErrUnsupportedPlatform, goos)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPlatform, goos)
	}
}

// Downloader fetches and verifies a release archive.
type Downloader struct {
	HTTP *http.Client
	// BaseURL overrides the release host in tests.
	BaseURL string
}

func NewDownloader() *Downloader {
	return &Downloader{HTTP: &http.Client{Timeout: downloadTimeout}}
}

// Binary downloads the release archive, checks its SHA-256 against the
// published checksums.txt, and returns the `pyahu` executable inside it.
//
// The hash is not optional. The CLI refuses unverified plugin downloads for the
// same reason, and this one replaces the CLI itself.
func (d *Downloader) Binary(ctx context.Context, release Release) ([]byte, error) {
	assetURL, checksumsURL := release.AssetURL, release.ChecksumsURL
	if d.BaseURL != "" {
		assetURL = d.BaseURL + "/" + release.Asset
		checksumsURL = d.BaseURL + "/checksums.txt"
	}

	checksums, err := d.get(ctx, checksumsURL)
	if err != nil {
		return nil, fmt.Errorf("download checksums for v%s: %w", release.Version, err)
	}
	want, ok := checksumFor(string(checksums), release.Asset)
	if !ok {
		return nil, fmt.Errorf("checksums.txt for v%s does not list %s", release.Version, release.Asset)
	}

	archive, err := d.get(ctx, assetURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", release.Asset, err)
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != want {
		return nil, fmt.Errorf("checksum mismatch for %s: got %s, expected %s", release.Asset, got, want)
	}
	return binaryFromTarGz(archive)
}

func (d *Downloader) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxAssetSize))
}

// checksumFor reads the "<sha256>  <file>" lines GoReleaser publishes.
func checksumFor(checksums string, asset string) (string, bool) {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], true
		}
	}
	return "", false
}

func binaryFromTarGz(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read release archive: %w", err)
	}
	defer gz.Close()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != "pyahu" {
			continue
		}
		binary, err := io.ReadAll(io.LimitReader(reader, maxAssetSize))
		if err != nil {
			return nil, fmt.Errorf("read pyahu from the release archive: %w", err)
		}
		return binary, nil
	}
	return nil, errors.New("the release archive does not contain a pyahu binary")
}

// Replace swaps the executable at path with binary.
//
// The new file is written next to the target and renamed over it, so the swap is
// atomic on the same filesystem and a failed download never leaves a truncated
// binary behind. Unix keeps the running process on the old inode, so the command
// doing the upgrade finishes normally.
func Replace(path string, binary []byte) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("%w: windows cannot replace a running executable", ErrUnsupportedPlatform)
	}
	if len(binary) == 0 {
		return errors.New("refusing to install an empty binary")
	}
	dir := filepath.Dir(path)
	staged, err := os.CreateTemp(dir, ".pyahu-upgrade-*")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			// The raw error names the temporary file, which tells the user
			// nothing; the directory is what they have to fix.
			return fmt.Errorf("%w: %s is not writable", fs.ErrPermission, dir)
		}
		return err
	}
	stagedPath := staged.Name()
	defer os.Remove(stagedPath)

	if _, err := staged.Write(binary); err != nil {
		staged.Close()
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	mode := os.FileMode(0o755)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(stagedPath, mode); err != nil {
		return err
	}
	return os.Rename(stagedPath, path)
}
