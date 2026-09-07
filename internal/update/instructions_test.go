package update

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstructionsFollowTheInstallMethod(t *testing.T) {
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "/home/dev/go")

	cases := []struct {
		name       string
		executable string
		want       []string
	}{
		// A binary another tool owns gets that tool's command, and only it:
		// `pyahu upgrade` would be undone by the next `mise install`.
		{"mise", "/home/dev/.local/share/mise/installs/github-pyahu-cli/0.4.0/pyahu", []string{"mise use github:pyahu/cli@0.6.1"}},
		{"mise shim", "/home/dev/.local/share/mise/shims/pyahu", []string{"mise use github:pyahu/cli@0.6.1"}},
		{"go install", "/home/dev/go/bin/pyahu", []string{"go install github.com/pyahu/cli/cmd/pyahu@latest"}},
		// A standalone binary can upgrade itself; the install script stays as the
		// fallback for when the install directory is not writable.
		{"install script", "/usr/local/bin/pyahu", []string{"pyahu upgrade", "curl -fsSL https://cli.pyahu.io/install.sh | sh"}},
		{"unknown", "", []string{"pyahu upgrade", "curl -fsSL https://cli.pyahu.io/install.sh | sh"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Instructions(tc.executable, "v0.6.1")
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("Instructions(%q) = %#v, want %#v", tc.executable, got, tc.want)
			}
		})
	}
}

func TestGoBinIsDetectedThroughGOBIN(t *testing.T) {
	t.Setenv("GOBIN", filepath.FromSlash("/opt/gobin"))
	t.Setenv("GOPATH", "")

	got := Instructions(filepath.FromSlash("/opt/gobin/pyahu"), "0.6.1")
	if len(got) != 1 || !strings.HasPrefix(got[0], "go install") {
		t.Fatalf("Instructions = %#v", got)
	}
}

func TestReleaseNotesURLNormalizesTheTag(t *testing.T) {
	want := "https://github.com/pyahu/cli/releases/tag/v0.6.1"
	for _, latest := range []string{"0.6.1", "v0.6.1"} {
		if got := ReleaseNotesURL(latest); got != want {
			t.Fatalf("ReleaseNotesURL(%q) = %q", latest, got)
		}
	}
}
