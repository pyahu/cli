package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/certs"
	"github.com/pyahu/cli/internal/doctor"
)

func (a *app) newUpCmd() *cobra.Command {
	var skipWait bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Create or reconcile the local Pyahu cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			a.reportConfig(loaded)
			stack := loaded.Data
			ctx := cmd.Context()
			rt := a.deps.newRuntime(a.opts)

			var checks []doctor.Check
			if err := a.phase("Checando dependências locais", func() (string, error) {
				clusterExists := false
				if err := rt.CheckInstalled(); err == nil {
					exists, err := rt.Exists(ctx, stack.Cluster.Name)
					if err != nil {
						return "", dependencyError(err.Error())
					}
					clusterExists = exists
				}
				checks = a.deps.runDoctor(ctx, stack, clusterExists)
				return "", nil
			}); err != nil {
				return err
			}
			a.printWarnings(checks)
			if !doctor.Healthy(checks) {
				if a.opts.output == "json" {
					if err := writeJSON(a.opts.out, map[string]any{"event": "preflight.failed", "checks": checks}); err != nil {
						return err
					}
					return dependencyError("preflight failed")
				}
				for _, check := range checks {
					if !check.OK {
						a.renderCheck(check)
					}
				}
				return dependencyError("preflight failed")
			}

			if err := a.phase("Provisionando cluster k3d "+stack.Cluster.Name, func() (string, error) {
				created, err := rt.Create(ctx, stack, loaded.Dir)
				if err != nil {
					return "", clusterError(err.Error())
				}
				if created {
					return "Cluster " + stack.Cluster.Name + " criado", nil
				}
				return "Cluster " + stack.Cluster.Name + " reutilizado", nil
			}); err != nil {
				return err
			}

			kubeconfig, err := rt.Kubeconfig(ctx, stack.Cluster.Name)
			if err != nil {
				return clusterError(err.Error())
			}
			client, err := a.deps.newKube(kubeconfig)
			if err != nil {
				return clusterError(err.Error())
			}

			if err := a.phase("Aguardando a API do Kubernetes", func() (string, error) {
				if err := client.WaitForAPI(ctx, 2*time.Minute); err != nil {
					return "", clusterError(err.Error())
				}
				return "", nil
			}); err != nil {
				return err
			}

			if err := a.phase("Configurando serviços: "+strings.Join(stack.EnabledServices(), ", "), func() (string, error) {
				if err := client.ApplyStack(ctx, stack, loaded.Dir); err != nil {
					return "", serviceError(err.Error())
				}
				return "", nil
			}); err != nil {
				return err
			}

			if !skipWait {
				if err := a.phase("Aguardando os serviços ficarem prontos", func() (string, error) {
					if err := client.WaitForStack(ctx, stack); err != nil {
						return "", readinessError(err.Error())
					}
					return "", nil
				}); err != nil {
					return err
				}
				if stack.ZitadelEnabled() {
					if err := a.phase("Exportando credencial de serviço do Zitadel", func() (string, error) {
						if err := client.CaptureZitadelPAT(ctx, stack); err != nil {
							return "", serviceError(err.Error())
						}
						return "", nil
					}); err != nil {
						return err
					}
				}
			}
			a.printLocalTLSHint(stack, loaded.Dir)
			return a.printSummary(stack, kubeconfig)
		},
	}
	cmd.Flags().BoolVar(&skipWait, "skip-wait", false, "apply resources without waiting for readiness")
	return cmd
}

func (a *app) printLocalTLSHint(stack localTLSStack, stackDir string) {
	if !stack.LocalTLSRequired() || a.opts.output != "human" || a.opts.quiet {
		return
	}
	status, err := certs.Inspect(stackDir, stack.LocalTLSDomains())
	if err != nil || !status.CA.Exists || !status.CA.Valid || status.HostTrusted {
		return
	}
	a.step("certs", "local CA is not trusted by this host; run `pyahu certs trust` before using https://*.localhost without warnings")
}

type localTLSStack interface {
	LocalTLSEnabled() bool
	LocalTLSRequired() bool
	LocalTLSDomains() []string
}
