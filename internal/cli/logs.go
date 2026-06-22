package cli

import (
	"io"

	"github.com/spf13/cobra"
)

func (a *app) newLogsCmd() *cobra.Command {
	var follow bool
	var tail int64
	cmd := &cobra.Command{
		Use:   "logs <service>",
		Short: "Stream logs from a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			service := args[0]
			if !validService(service) {
				return usageError("service must be one of: postgres, zitadel, rabbitmq, kafka, kafka-connect, kafka-ui")
			}
			rt := a.deps.newRuntime(a.opts)
			kubeconfig, err := rt.Kubeconfig(cmd.Context(), loaded.Data.Cluster.Name)
			if err != nil {
				return clusterError(err.Error())
			}
			client, err := a.deps.newKube(kubeconfig)
			if err != nil {
				return clusterError(err.Error())
			}
			stream, err := client.Logs(cmd.Context(), loaded.Data.Cluster.Namespace, service, follow, tail)
			if err != nil {
				return err
			}
			defer stream.Close()
			_, err = io.Copy(a.opts.out, stream)
			return err
		},
	}
	cmd.Flags().BoolVar(&follow, "follow", false, "follow logs")
	cmd.Flags().Int64Var(&tail, "tail", 100, "number of lines to show")
	return cmd
}
