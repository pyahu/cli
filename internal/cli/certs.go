package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/certs"
	"github.com/pyahu/cli/internal/config"
	"github.com/pyahu/cli/pkg/schema"
)

type certCommandContext struct {
	stackDir        string
	domains         []string
	secretName      string
	caConfigMapName string
	stackLoaded     bool
}

func (a *app) newCertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "certs",
		Short: "Manage local TLS certificates",
	}
	cmd.AddCommand(a.newCertsStatusCmd())
	cmd.AddCommand(a.newCertsTrustCmd())
	cmd.AddCommand(a.newCertsRotateCmd())
	return cmd
}

func (a *app) newCertsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local TLS certificate status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := a.certContext()
			if err != nil {
				return usageError(err.Error())
			}
			status, err := certs.Inspect(ctx.stackDir, ctx.domains)
			if err != nil {
				return serviceError(err.Error())
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{
					"stackLoaded":     ctx.stackLoaded,
					"secretName":      ctx.secretName,
					"caConfigMapName": ctx.caConfigMapName,
					"status":          status,
				})
			}
			a.printCertStatus(ctx, status)
			return nil
		},
	}
}

func (a *app) newCertsTrustCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trust",
		Short: "Install the local Pyahu CA into the host trust store",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := a.certContext()
			if err != nil {
				return usageError(err.Error())
			}
			bundle, err := certs.Ensure(ctx.stackDir, ctx.domains)
			if err != nil {
				return serviceError(err.Error())
			}
			trusted, err := certs.CATrusted(bundle.CACertificatePEM)
			if err == nil && trusted {
				if a.opts.output == "json" {
					return writeJSON(a.opts.out, map[string]any{"trusted": true, "ca": bundle.Paths.CACert})
				}
				a.info("local CA already trusted: %s", displayPath(bundle.Paths.CACert))
				return nil
			}
			if err := certs.TrustHost(cmd.Context(), bundle.Paths.CACert, certs.TrustOptions{
				NoInput: a.opts.noInput,
				Verbose: a.opts.verbose,
				Out:     a.opts.out,
				Err:     a.opts.err,
			}); err != nil {
				return dependencyError(err.Error())
			}
			trusted, _ = certs.CATrusted(bundle.CACertificatePEM)
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{"trusted": trusted, "ca": bundle.Paths.CACert})
			}
			if trusted {
				a.info("trusted local CA: %s", displayPath(bundle.Paths.CACert))
			} else {
				a.info("installed local CA, but this process could not verify host trust yet")
				a.info("CA: %s", displayPath(bundle.Paths.CACert))
			}
			return nil
		},
	}
}

func (a *app) newCertsRotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate",
		Short: "Regenerate the local CA and wildcard certificate",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := a.certContext()
			if err != nil {
				return usageError(err.Error())
			}
			bundle, err := certs.Rotate(ctx.stackDir, ctx.domains)
			if err != nil {
				return serviceError(err.Error())
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{
					"rotated":     true,
					"ca":          bundle.Paths.CACert,
					"certificate": bundle.Paths.Cert,
					"domains":     bundle.Domains,
				})
			}
			a.info("rotated local CA and wildcard certificate")
			a.info("CA:          %s", displayPath(bundle.Paths.CACert))
			a.info("certificate: %s", displayPath(bundle.Paths.Cert))
			a.info("next: pyahu certs trust")
			if ctx.stackLoaded {
				a.info("next: pyahu up")
			}
			return nil
		},
	}
}

func (a *app) certContext() (certCommandContext, error) {
	loaded, err := a.loadStack()
	if err == nil {
		return certContextFromStack(loaded), nil
	}
	if a.opts.file != "" || !strings.Contains(err.Error(), "no Pyahu stack file found") {
		return certCommandContext{}, err
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return certCommandContext{}, cwdErr
	}
	return certCommandContext{
		stackDir:        cwd,
		domains:         schema.DefaultLocalTLSDomains(),
		secretName:      schema.DefaultLocalTLSSecretName,
		caConfigMapName: schema.DefaultLocalTLSCAConfigMap,
		stackLoaded:     false,
	}, nil
}

func certContextFromStack(loaded *config.LoadedStack) certCommandContext {
	return certCommandContext{
		stackDir:        loaded.Dir,
		domains:         loaded.Data.LocalTLSDomains(),
		secretName:      loaded.Data.LocalTLSSecretName(),
		caConfigMapName: loaded.Data.LocalTLSCAConfigMapName(),
		stackLoaded:     true,
	}
}

func (a *app) printCertStatus(ctx certCommandContext, status certs.Status) {
	a.info("local CA:      %s", displayPath(status.Paths.CACert))
	a.info("CA status:     %s", a.colorCertStatus(status.CA))
	a.info("host trust:    %s", a.colorTrust(status))
	a.info("certificate:   %s", displayPath(status.Paths.Cert))
	a.info("cert status:   %s", a.colorCertStatus(status.Certificate))
	a.info("domains:       %s", strings.Join(status.Domains, ", "))
	if ctx.stackLoaded {
		a.info("k8s secret:    %s", ctx.secretName)
		a.info("k8s CA config: %s", ctx.caConfigMapName)
	}
	if !status.CA.Exists || !status.Certificate.Exists {
		a.info("next: pyahu up")
	}
	if status.CA.Exists && status.CA.Valid && !status.HostTrusted {
		a.info("next: pyahu certs trust")
	}
}

func (a *app) colorCertStatus(status certs.CertStatus) string {
	text := formatCertStatus(status)
	s := a.styler()
	if !status.Exists || !status.Valid {
		return s.red(text)
	}
	return s.green(text)
}

func (a *app) colorTrust(status certs.Status) string {
	text := trustStatus(status)
	s := a.styler()
	switch text {
	case "trusted":
		return s.green(text)
	case "not trusted":
		return s.yellow(text)
	default:
		return s.dim(text)
	}
}

func formatCertStatus(status certs.CertStatus) string {
	if !status.Exists {
		return "missing"
	}
	if !status.Valid {
		if status.Message != "" {
			return "invalid (" + status.Message + ")"
		}
		return "invalid"
	}
	return fmt.Sprintf("valid until %s", status.ExpiresAt.Format("2006-01-02"))
}

func trustStatus(status certs.Status) string {
	if !status.CA.Exists || !status.CA.Valid {
		return "unavailable"
	}
	if status.HostTrusted {
		return "trusted"
	}
	return "not trusted"
}
