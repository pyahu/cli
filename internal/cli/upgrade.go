package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pyahu/cli/internal/update"
)

func (a *app) newCheckUpdateCmd() *cobra.Command {
	var exitCode bool
	cmd := &cobra.Command{
		Use:     "check-update",
		Aliases: []string{"check-updates"},
		Short:   "Check whether a newer Pyahu CLI release is published",
		Long: "Asks the releases endpoint directly, so the answer is current rather than the\n" +
			"cached one behind the passive notice.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This command is the report; the passive notice would just repeat it.
			a.noticeHandled = true
			latest, err := a.deps.latestRelease(cmd.Context())
			if err != nil {
				return dependencyError(err.Error())
			}
			current := a.opts.version
			outdated := update.Outdated(current, latest)

			if a.opts.output == "json" {
				if err := writeJSON(a.opts.out, map[string]any{
					"current":  current,
					"latest":   latest,
					"outdated": outdated,
				}); err != nil {
					return err
				}
			} else {
				a.renderCheckUpdate(current, latest, outdated)
			}
			if outdated && exitCode {
				return codedError{code: 1, msg: "a newer release is available"}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&exitCode, "exit-code", false, "exit 1 when a newer release is available (for scripts)")
	return cmd
}

func (a *app) renderCheckUpdate(current string, latest string, outdated bool) {
	s := a.styler()
	if !outdated {
		if !update.Outdated(latest, current) {
			a.info("%s %s", s.green(iconOK), s.ok(fmt.Sprintf("pyahu %s is the latest release", current)))
			return
		}
		// A local build of an unreleased commit is ahead, not behind.
		a.info("%s pyahu %s is ahead of the latest release (%s)", s.dim("·"), current, latest)
		return
	}
	a.info("%s %s", s.yellow(iconWarn), s.yellow(fmt.Sprintf("pyahu %s is out of date — %s is available", current, latest)))
	for _, command := range update.Instructions(executablePath(), latest) {
		a.info("  %s", s.bold(command))
	}
	a.info("  %s", s.dim(update.ReleaseNotesURL(latest)))
}

func (a *app) newUpgradeCmd() *cobra.Command {
	var yes bool
	var pinned string
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Replace this binary with a newer Pyahu CLI release",
		Long: "Downloads the release archive, verifies its SHA-256 against the published\n" +
			"checksums, and swaps this binary for the one inside it.\n\n" +
			"A binary installed by mise or `go install` is left alone: replacing it would be\n" +
			"undone by that tool. The command prints the right upgrade command instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Whatever happens here, the notice must not announce the version this
			// command may have just replaced.
			a.noticeHandled = true
			executable := executablePath()
			if executable == "" {
				return serviceError("could not resolve the path of the running binary")
			}
			if manager := update.DetectManager(executable); !manager.SelfManaged() {
				return guidedError(fmt.Sprintf(
					"this pyahu is managed by another tool; upgrade it with: %s",
					manager.UpgradeCommand(a.upgradeTargetHint(cmd.Context(), pinned))))
			}
			if pinned == "" && !update.Enabled(a.opts.version) {
				return guidedError("this is a locally built binary (version " + a.opts.version +
					"); rebuild from source, or pass --version to install a published release")
			}

			target := pinned
			if target == "" {
				latest, err := a.deps.latestRelease(cmd.Context())
				if err != nil {
					return dependencyError(err.Error())
				}
				if !update.Outdated(a.opts.version, latest) {
					a.info("%s pyahu %s is already the latest release", a.styler().green(iconOK), a.opts.version)
					return nil
				}
				target = latest
			}

			release, err := update.ReleaseFor(target, runtime.GOOS, runtime.GOARCH)
			if err != nil {
				return usageError(err.Error())
			}
			if err := a.confirmUpgrade(executable, release.Version, yes); err != nil {
				return err
			}

			var binary []byte
			if err := a.phase("Downloading pyahu "+release.Version, func() (string, error) {
				downloaded, err := a.deps.downloadRelease(cmd.Context(), release)
				if err != nil {
					return "", serviceError(err.Error())
				}
				binary = downloaded
				return "Checksum verified", nil
			}); err != nil {
				return err
			}
			if err := a.phase("Installing to "+displayPath(executable), func() (string, error) {
				if err := a.deps.replaceBinary(executable, binary); err != nil {
					return "", codedError{code: 5, msg: upgradeWriteHint(err), guided: true}
				}
				return "", nil
			}); err != nil {
				return err
			}

			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{
					"previous": a.opts.version,
					"current":  release.Version,
					"path":     executable,
				})
			}
			s := a.styler()
			a.info("%s %s", s.green(iconOK), s.ok(fmt.Sprintf("pyahu %s installed", release.Version)))
			a.info("  %s", s.dim(update.ReleaseNotesURL(release.Version)))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "replace the binary without prompting")
	cmd.Flags().StringVar(&pinned, "version", "", "install this version instead of the latest (also downgrades)")
	return cmd
}

// upgradeTargetHint resolves the version to name in the "use this other tool"
// message. It is only a hint, so a failed lookup falls back to the flag value.
func (a *app) upgradeTargetHint(ctx context.Context, pinned string) string {
	if pinned != "" {
		return pinned
	}
	if latest, err := a.deps.latestRelease(ctx); err == nil {
		return latest
	}
	return "latest"
}

func (a *app) confirmUpgrade(executable string, version string, yes bool) error {
	if yes {
		return nil
	}
	if a.opts.noInput {
		return usageError("upgrade requires --yes when --no-input is set")
	}
	if a.opts.output != "human" {
		return usageError("upgrade requires --yes for non-human output")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return usageError("upgrade requires --yes in non-interactive mode")
	}
	if _, err := fmt.Fprintf(a.opts.out, "This replaces %s with pyahu %s.\n", executable, version); err != nil {
		return serviceError(fmt.Sprintf("write confirmation prompt: %v", err))
	}
	if _, err := fmt.Fprint(a.opts.out, "Continue? [y/N]: "); err != nil {
		return serviceError(fmt.Sprintf("write confirmation prompt: %v", err))
	}
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return usageError(fmt.Sprintf("read confirmation: %v", err))
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	default:
		return usageError("upgrade cancelled")
	}
}

// upgradeWriteHint turns a permission failure into the two ways out, because
// "permission denied" alone leaves the user guessing.
func upgradeWriteHint(err error) string {
	if !errors.Is(err, fs.ErrPermission) {
		return err.Error()
	}
	return fmt.Sprintf("%v — rerun with sudo, or reinstall somewhere you own: %s", err,
		`curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --bin-dir "$HOME/.local/bin"`)
}
