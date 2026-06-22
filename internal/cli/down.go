package cli

import "github.com/spf13/cobra"

func (a *app) newDownCmd() *cobra.Command {
	var keepCluster bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Delete local Pyahu resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			a.reportConfig(loaded)
			stack := loaded.Data
			ctx := cmd.Context()
			rt := a.deps.newRuntime(a.opts)
			if keepCluster {
				kubeconfig, err := rt.Kubeconfig(ctx, stack.Cluster.Name)
				if err != nil {
					return clusterError(err.Error())
				}
				client, err := a.deps.newKube(kubeconfig)
				if err != nil {
					return clusterError(err.Error())
				}
				return a.phase("Removendo namespace "+stack.Cluster.Namespace, func() (string, error) {
					if err := client.DeleteNamespace(ctx, stack.Cluster.Namespace); err != nil {
						return "", serviceError(err.Error())
					}
					return "Namespace " + stack.Cluster.Namespace + " removido (cluster " + stack.Cluster.Name + " mantido)", nil
				})
			}
			return a.phase("Removendo cluster k3d "+stack.Cluster.Name, func() (string, error) {
				if err := rt.Delete(ctx, stack.Cluster.Name); err != nil {
					return "", clusterError(err.Error())
				}
				return "Cluster " + stack.Cluster.Name + " removido", nil
			})
		},
	}
	cmd.Flags().BoolVar(&keepCluster, "keep-cluster", false, "delete stack namespace but keep the k3d cluster")
	return cmd
}
