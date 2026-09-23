package cli

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/cloud"
)

func (a *app) newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to Pyahu Cloud",
		Long: "Signs in to Pyahu Cloud and stores the session in ~/.pyahu/credentials.json.\n\n" +
			"Sign-in happens in your browser, on a page served by the identity provider: this CLI " +
			"prints a code, you approve it there, and no password is ever typed into a terminal.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := cloud.LoadConfig()
			client := &http.Client{Timeout: 30 * time.Second}

			device, err := cloud.StartDeviceLogin(cmd.Context(), cfg, client)
			if err != nil {
				return err
			}

			target := device.VerificationURIComplete
			if target == "" {
				target = device.VerificationURI
			}
			// The URL is printed whether or not the browser opens: the device flow exists for machines
			// that have no browser, and the person may well be approving this on their phone.
			if openInBrowser(target) {
				_, _ = fmt.Fprintf(a.opts.out, "Opening %s\n", target)
				_, _ = fmt.Fprintf(a.opts.out, "If it did not open, paste that link yourself.\n")
			} else {
				_, _ = fmt.Fprintf(a.opts.out, "Open %s\n", target)
			}
			_, _ = fmt.Fprintf(a.opts.out, "and confirm the code: %s\n\n", device.UserCode)
			if !a.opts.quiet {
				_, _ = fmt.Fprintln(a.opts.out, "Waiting for you to approve...")
			}

			creds, err := device.Wait(cmd.Context(), cfg, client)
			if err != nil {
				return err
			}
			if err := cloud.SaveCredentials(creds); err != nil {
				return err
			}

			path, _ := cloud.CredentialsPath()
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]string{"status": "signed in", "credentials": path})
			}
			_, _ = fmt.Fprintf(a.opts.out, "\nSigned in. Session saved to %s\n", path)
			_, _ = fmt.Fprintln(a.opts.out, "Next: pyahu kube config")
			return nil
		},
	}
	return cmd
}

func (a *app) newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the Pyahu Cloud session from this machine",
		Long: "Deletes ~/.pyahu/credentials.json.\n\n" +
			"This signs this machine out. It does not revoke anything centrally, and any kubectl " +
			"credential already issued keeps working until it expires, which is at most ten minutes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cloud.ForgetCredentials(); err != nil {
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]string{"status": "signed out"})
			}
			_, _ = fmt.Fprintln(a.opts.out, "Signed out.")
			return nil
		},
	}
}

// notSignedIn turns the sentinel into the one instruction that fixes it, wherever it surfaces.
func notSignedIn(err error) error {
	if errors.Is(err, cloud.ErrNotSignedIn) {
		return usageError("not signed in; run `pyahu login` first")
	}
	return err
}
