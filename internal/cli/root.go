package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *app) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "pyahu",
		Short:         "Local Pyahu infrastructure for development",
		Long:          "Pyahu CLI provisions a local k3d cluster with PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect, and Kafka UI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", a.opts.version, a.opts.commit, a.opts.date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return a.validateGlobalOptions()
		},
	}

	root.PersistentFlags().StringVarP(&a.opts.file, "file", "f", "", "path to project stack file; global config still applies")
	root.PersistentFlags().StringVarP(&a.opts.output, "output", "o", "human", "output format: human or json")
	root.PersistentFlags().BoolVar(&a.opts.noColor, "no-color", false, "disable color output")
	root.PersistentFlags().BoolVarP(&a.opts.quiet, "quiet", "q", false, "suppress non-essential output")
	root.PersistentFlags().BoolVarP(&a.opts.verbose, "verbose", "v", false, "show diagnostic output")
	root.PersistentFlags().BoolVar(&a.opts.noInput, "no-input", false, "never prompt for input")

	root.AddCommand(a.newInitCmd())
	root.AddCommand(a.newCertsCmd())
	root.AddCommand(a.newDoctorCmd())
	root.AddCommand(a.newUpCmd())
	root.AddCommand(a.newDownCmd())
	root.AddCommand(a.newStatusCmd())
	root.AddCommand(a.newServicesCmd())
	root.AddCommand(a.newDescribeCmd())
	root.AddCommand(a.newLogsCmd())
	root.AddCommand(a.newEnvCmd())
	root.AddCommand(a.newBackupCmd())
	root.AddCommand(a.newRestoreCmd())
	root.AddCommand(a.newKubeconfigCmd())
	root.AddCommand(a.newCompletionCmd())
	return root
}

func (a *app) validateGlobalOptions() error {
	switch a.opts.output {
	case "human", "json":
		return nil
	default:
		return usageError("--output must be human or json")
	}
}
