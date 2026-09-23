package cli

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// openInBrowser tries to open a URL in the person's browser, and says whether it managed to.
//
// Best-effort on purpose: the caller ALWAYS prints the URL as well. A device flow exists precisely
// because the machine running the CLI may have no browser at all (an SSH session, a container, a CI
// runner), so failing to open one is an ordinary outcome here, not an error worth reporting.
//
// It is also skipped when the environment says a browser would be wrong: DISPLAY-less Linux, an
// explicit opt-out, or anything that looks like CI. Spawning a browser under automation is at best
// noise and at worst a hang.
func openInBrowser(target string) bool {
	if target == "" || !browserWanted() {
		return false
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	// Detached from this process's streams: a browser launcher that writes to stderr would otherwise
	// scribble over the code the person still needs to read.
	cmd.Stdout, cmd.Stderr, cmd.Stdin = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return false
	}
	// Reaped in the background so the CLI neither waits for the browser to close nor leaves a zombie.
	go func() { _ = cmd.Wait() }()
	return true
}

func browserWanted() bool {
	// Honour the de facto opt-outs before anything else.
	if v := strings.TrimSpace(os.Getenv("PYAHU_NO_BROWSER")); v != "" && v != "0" && v != "false" {
		return false
	}
	if v := strings.TrimSpace(os.Getenv("CI")); v != "" && v != "0" && v != "false" {
		return false
	}
	// On a Unix desktop, no display means no browser. macOS and Windows have no such variable and are
	// assumed to have one.
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		}
	}
	return true
}
