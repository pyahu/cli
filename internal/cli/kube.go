package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/cloud"
)

// newKubeCmd groups everything that connects `kubectl` to a Pyahu Cloud environment.
//
// Separate from the existing `kubeconfig` command, which prints the path of the LOCAL k3d cluster this
// CLI creates. The two are genuinely different things and merging them would make `pyahu kubeconfig`
// mean "local" in one invocation and "your production cluster" in another.
func (a *app) newKubeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kube",
		Short: "Reach a Pyahu Cloud environment with kubectl",
	}
	cmd.AddCommand(a.newKubeConfigCmd())
	cmd.AddCommand(a.newKubeCredentialsCmd())
	cmd.AddCommand(a.newKubeListCmd())
	cmd.AddCommand(a.newKubeDoctorCmd())
	return cmd
}

func (a *app) newKubeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the environments you can reach with kubectl",
		RunE: func(cmd *cobra.Command, args []string) error {
			environments, err := a.cloudEnvironments(cmd)
			if err != nil {
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{"environments": environments})
			}
			if len(environments) == 0 {
				_, _ = fmt.Fprintln(a.opts.out, "No environments offer kubectl access to you yet.")
				_, _ = fmt.Fprintln(a.opts.out,
					"An organization admin turns it on per environment in the console.")
				return nil
			}
			for _, env := range environments {
				_, _ = fmt.Fprintf(a.opts.out, "%-24s %-20s %s\n", env.Name, env.OrganizationName, env.Level)
			}
			return nil
		},
	}
}

func (a *app) newKubeConfigCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Add an environment to your kubeconfig",
		Long: "Adds a kubectl context for a Pyahu Cloud environment.\n\n" +
			"The entry carries no credential: it records where the cluster is and that this CLI should " +
			"be asked for a token when kubectl needs one. Your kubeconfig is backed up before it is " +
			"changed, and every other context in it is left alone.",
		RunE: func(cmd *cobra.Command, args []string) error {
			environments, err := a.cloudEnvironments(cmd)
			if err != nil {
				return err
			}
			env, err := pickEnvironment(environments, name)
			if err != nil {
				return err
			}

			path := cloud.KubeconfigPath()
			backup, err := cloud.WriteContext(env, path)
			if err != nil {
				return err
			}

			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]string{
					"context": env.ContextName, "kubeconfig": path, "backup": backup,
				})
			}
			if backup != "" {
				_, _ = fmt.Fprintf(a.opts.out, "Backed up %s to %s\n", path, backup)
			}
			_, _ = fmt.Fprintf(a.opts.out, "Context %q is now current in %s\n", env.ContextName, path)
			_, _ = fmt.Fprintf(a.opts.out, "Namespace: %s   Access: %s\n\n", env.Namespace, env.Level)
			_, _ = fmt.Fprintln(a.opts.out, "Try: kubectl get pods")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "env", "", "environment name (optional when you can reach exactly one)")
	return cmd
}

// newKubeCredentialsCmd is the exec plugin kubectl calls. It is not meant to be run by hand.
func (a *app) newKubeCredentialsCmd() *cobra.Command {
	var organizationID, tenantID string
	cmd := &cobra.Command{
		Use:    "credentials",
		Short:  "Print a short-lived credential for kubectl (called by kubectl, not by you)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if organizationID == "" || tenantID == "" {
				return usageError("--org and --tenant are required; run `pyahu kube config` to set this up")
			}
			cfg := cloud.LoadConfig()
			token, err := cloud.AccessToken(cmd.Context(), cfg, cloud.NewClient(cfg).HTTP)
			if err != nil {
				return notSignedIn(err)
			}
			credential, err := cloud.NewClient(cfg).Credential(cmd.Context(), token, organizationID, tenantID)
			if err != nil {
				return notSignedIn(err)
			}
			// Always JSON on stdout, whatever --output says: kubectl parses this and nothing else reads
			// it. Honouring a human format here would break every kubectl call.
			encoder := json.NewEncoder(a.opts.out)
			return encoder.Encode(cloud.NewExecCredential(credential))
		},
	}
	cmd.Flags().StringVar(&organizationID, "org", "", "organization id")
	cmd.Flags().StringVar(&tenantID, "tenant", "", "environment id")
	return cmd
}

// newKubeDoctorCmd answers "why does kubectl not work" in one place.
//
// It exists because the failure surfaces as a kubectl error about an exec plugin, which says nothing
// about which of the four possible causes it is: not signed in, no environment offers access, the
// context was never written, or the session expired.
func (a *app) newKubeDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check what is missing between you and kubectl",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := cloud.LoadConfig()
			report := map[string]string{"console": cfg.ConsoleURL, "issuer": cfg.Issuer}

			token, err := cloud.AccessToken(cmd.Context(), cfg, cloud.NewClient(cfg).HTTP)
			if err != nil {
				report["signed_in"] = "no — run `pyahu login`"
				return a.reportDoctor(report)
			}
			report["signed_in"] = "yes"

			environments, err := cloud.NewClient(cfg).Environments(cmd.Context(), token)
			if err != nil {
				report["environments"] = "unreachable: " + err.Error()
				return a.reportDoctor(report)
			}
			if len(environments) == 0 {
				report["environments"] = "none offer you kubectl access yet"
				return a.reportDoctor(report)
			}
			names := make([]string, 0, len(environments))
			for _, env := range environments {
				names = append(names, fmt.Sprintf("%s (%s)", env.Name, env.Level))
			}
			report["environments"] = strings.Join(names, ", ")
			report["kubeconfig"] = cloud.KubeconfigPath()
			return a.reportDoctor(report)
		},
	}
}

func (a *app) reportDoctor(report map[string]string) error {
	if a.opts.output == "json" {
		return writeJSON(a.opts.out, report)
	}
	for _, key := range []string{"console", "issuer", "signed_in", "environments", "kubeconfig"} {
		if value, ok := report[key]; ok {
			_, _ = fmt.Fprintf(a.opts.out, "%-14s %s\n", key, value)
		}
	}
	return nil
}

func (a *app) cloudEnvironments(cmd *cobra.Command) ([]cloud.Environment, error) {
	cfg := cloud.LoadConfig()
	client := cloud.NewClient(cfg)
	token, err := cloud.AccessToken(cmd.Context(), cfg, client.HTTP)
	if err != nil {
		return nil, notSignedIn(err)
	}
	environments, err := client.Environments(cmd.Context(), token)
	if err != nil {
		return nil, notSignedIn(err)
	}
	return environments, nil
}

// pickEnvironment resolves which environment was meant.
//
// A single reachable environment needs no name, because asking somebody to name the only option is
// ceremony. Anything else is named explicitly: guessing between two production environments is the
// kind of convenience that eventually points kubectl at the wrong one.
func pickEnvironment(environments []cloud.Environment, name string) (cloud.Environment, error) {
	if len(environments) == 0 {
		return cloud.Environment{}, usageError(
			"no environment offers you kubectl access yet; an organization admin turns it on in the console")
	}
	if name == "" {
		if len(environments) == 1 {
			return environments[0], nil
		}
		names := make([]string, 0, len(environments))
		for _, env := range environments {
			names = append(names, env.Name)
		}
		return cloud.Environment{}, usageError(
			"more than one environment is available; pick one with --env: " + strings.Join(names, ", "))
	}
	matches := make([]cloud.Environment, 0, 1)
	for _, env := range environments {
		if strings.EqualFold(env.Name, name) {
			matches = append(matches, env)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return cloud.Environment{}, usageError(fmt.Sprintf("no environment named %q is available to you", name))
	default:
		// Two organizations can each have an environment called "production". Naming one is the only
		// way to be sure, and picking for them would be picking somebody's production at random.
		return cloud.Environment{}, usageError(fmt.Sprintf(
			"%q exists in more than one organization; use `pyahu kube list` and pick by organization", name))
	}
}
