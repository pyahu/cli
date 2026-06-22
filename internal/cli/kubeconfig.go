package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *app) newKubeconfigCmd() *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Print the local cluster kubeconfig path",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			rt := a.deps.newRuntime(a.opts)
			path, err := rt.Kubeconfig(cmd.Context(), loaded.Data.Cluster.Name)
			if err != nil {
				return clusterError(err.Error())
			}
			if raw {
				data, err := a.deps.readFile(path)
				if err != nil {
					return err
				}
				_, err = a.opts.out.Write(data)
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]string{"path": path})
			}
			fmt.Fprintln(a.opts.out, path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "write kubeconfig contents to stdout")
	return cmd
}
