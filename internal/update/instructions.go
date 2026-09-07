package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Instructions returns the upgrade commands to show, most relevant first.
//
// How the binary was installed is not recorded anywhere, so it is inferred from
// its own path: a mise shim lives under the mise install tree, a `go install`
// binary under GOPATH/bin. Printing every possible command instead would make
// the reader pick, and the point of the message is to not make them think.
func Instructions(executable string, latest string) []string {
	switch {
	case installedByMise(executable):
		return []string{
			fmt.Sprintf("mise use github:pyahu/cli@%s", normalize(latest)),
		}
	case installedByGo(executable):
		return []string{
			"go install github.com/pyahu/cli/cmd/pyahu@latest",
		}
	default:
		return []string{
			"curl -fsSL https://cli.pyahu.io/install.sh | sh",
		}
	}
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
