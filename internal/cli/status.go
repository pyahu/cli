package cli

import "github.com/spf13/cobra"

func (a *app) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cluster and service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			stack := loaded.Data
			rt := a.deps.newRuntime(a.opts)
			exists, err := rt.Exists(cmd.Context(), stack.Cluster.Name)
			if err != nil {
				return err
			}
			if !exists {
				if a.opts.output == "json" {
					return writeJSON(a.opts.out, map[string]any{"cluster": stack.Cluster.Name, "running": false})
				}
				a.info("cluster %s is not running", stack.Cluster.Name)
				return nil
			}
			kubeconfig, err := rt.Kubeconfig(cmd.Context(), stack.Cluster.Name)
			if err != nil {
				return clusterError(err.Error())
			}
			client, err := a.deps.newKube(kubeconfig)
			if err != nil {
				return clusterError(err.Error())
			}
			statuses, err := client.Status(cmd.Context(), stack)
			if err != nil {
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{"cluster": stack.Cluster.Name, "namespace": stack.Cluster.Namespace, "running": true, "services": statuses})
			}
			a.info("cluster: %s", stack.Cluster.Name)
			a.info("namespace: %s", stack.Cluster.Namespace)
			for _, status := range statuses {
				state := "disabled"
				if status.Enabled && status.Ready {
					state = "ready"
				} else if status.Enabled {
					state = "waiting"
				}
				a.info("%-10s %s %s", status.Name, a.field(state, 8, a.stateColorFn(state)), status.Message)
				for _, pod := range status.Pods {
					podState := "waiting"
					if pod.Ready {
						podState = "ready"
					}
					if pod.Reason != "" {
						podState += " (" + pod.Reason + ")"
					}
					a.info("  %-40s %s", pod.Name, a.colorPodState(podState, pod.Ready))
				}
			}
			return nil
		},
	}
}
