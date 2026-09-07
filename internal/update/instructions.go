package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Manager is how the running binary got onto the machine. Nothing records it, so
// it is inferred from the binary's own path.
type Manager int

const (
	// ManagerStandalone is a plain binary the CLI may replace itself.
	ManagerStandalone Manager = iota
	// ManagerMise is a binary owned by mise; replacing it would be undone by the
	// next `mise install`, so the version has to change in mise instead.
	ManagerMise
	// ManagerGo is a binary produced by `go install`.
	ManagerGo
)

// DetectManager reports who owns the binary at executable.
func DetectManager(executable string) Manager {
	switch {
	case installedByMise(executable):
		return ManagerMise
	case installedByGo(executable):
		return ManagerGo
	default:
		return ManagerStandalone
	}
}

// SelfManaged reports whether `pyahu upgrade` may replace this binary itself.
func (m Manager) SelfManaged() bool {
	return m == ManagerStandalone
}

// UpgradeCommand is the command that upgrades a binary this CLI does not own.
func (m Manager) UpgradeCommand(latest string) string {
	switch m {
	case ManagerMise:
		return fmt.Sprintf("mise use github:pyahu/cli@%s", normalize(latest))
	case ManagerGo:
		return "go install github.com/pyahu/cli/cmd/pyahu@latest"
	default:
		return "pyahu upgrade"
	}
}

// Instructions returns the upgrade commands to show, most relevant first.
//
// Printing every possible command instead would make the reader pick, and the
// point of the message is to not make them think.
func Instructions(executable string, latest string) []string {
	manager := DetectManager(executable)
	if manager.SelfManaged() {
		return []string{"pyahu upgrade", "curl -fsSL https://cli.pyahu.io/install.sh | sh"}
	}
	return []string{manager.UpgradeCommand(latest)}
}

// ReleaseNotesURL points at the release the message is nagging about.
func ReleaseNotesURL(latest string) string {
	return "https://github.com/pyahu/cli/releases/tag/v" + normalize(latest)
}

func installedByMise(executable string) bool {
	normalized := filepath.ToSlash(executable)
	return strings.Contains(normalized, "/mise/installs/") || strings.Contains(normalized, "/mise/shims/")
}

func installedByGo(executable string) bool {
	normalized := filepath.ToSlash(executable)
	for _, dir := range goBinDirs() {
		if dir != "" && strings.HasPrefix(normalized, filepath.ToSlash(dir)+"/") {
			return true
		}
	}
	return false
}

func goBinDirs() []string {
	dirs := []string{os.Getenv("GOBIN")}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		for _, entry := range filepath.SplitList(gopath) {
			dirs = append(dirs, filepath.Join(entry, "bin"))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "go", "bin"))
	}
	return dirs
}
