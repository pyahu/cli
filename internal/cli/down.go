package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pyahu/cli/internal/runtime/k3d"
)

func (a *app) newDownCmd() *cobra.Command {
	var keepCluster bool
	var purgeData bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Delete local Pyahu resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			if keepCluster && purgeData {
				return usageError("down cannot combine --keep-cluster and --purge-data")
			}
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
				return a.phase("Removing namespace "+stack.Cluster.Namespace, func() (string, error) {
					if err := client.DeleteNamespace(ctx, stack.Cluster.Namespace); err != nil {
						return "", serviceError(err.Error())
					}
					return "Namespace " + stack.Cluster.Namespace + " removed (cluster " + stack.Cluster.Name + " retained)", nil
				})
			}
			storageDir := k3d.StorageDir(stack.Cluster.Name)
			if purgeData {
				if err := a.confirmDataPurge(stack.Cluster.Name, storageDir, yes); err != nil {
					return err
				}
			}
			if err := a.phase("Removing k3d cluster "+stack.Cluster.Name, func() (string, error) {
				if err := rt.Delete(ctx, stack.Cluster.Name); err != nil {
					return "", clusterError(err.Error())
				}
				return "Cluster " + stack.Cluster.Name + " removed", nil
			}); err != nil {
				return err
			}
			if !purgeData {
				a.info("data retained at %s", displayPath(storageDir))
				return nil
			}
			return a.phase("Removing local data at "+displayPath(storageDir), func() (string, error) {
				if filepath.Base(storageDir) != "storage" || filepath.Base(filepath.Dir(storageDir)) != stack.Cluster.Name {
					return "", serviceError("refusing to remove unexpected storage path " + storageDir)
				}
				if err := a.deps.removeAll(storageDir); err != nil {
					return "", serviceError(fmt.Sprintf("remove local data: %v", err))
				}
				return "Local data removed", nil
			})
		},
	}
	cmd.Flags().BoolVar(&keepCluster, "keep-cluster", false, "delete stack namespace but keep the k3d cluster")
	cmd.Flags().BoolVar(&purgeData, "purge-data", false, "permanently delete the stack's retained local storage")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm permanent local data deletion without prompting")
	return cmd
}

func (a *app) confirmDataPurge(clusterName string, storageDir string, yes bool) error {
	if yes {
		return nil
	}
	if a.opts.noInput {
		return guidedError("down --purge-data requires --yes when --no-input is set")
	}
	if a.opts.output != "human" {
		return guidedError("down --purge-data requires --yes for non-human output")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return guidedError("down --purge-data requires --yes in non-interactive mode")
	}
	if _, err := fmt.Fprintf(a.opts.out, "This will permanently delete local data at %s.\n", displayPath(storageDir)); err != nil {
		return usageError(fmt.Sprintf("write purge confirmation: %v", err))
	}
	if _, err := fmt.Fprintf(a.opts.out, "Type %s to continue: ", clusterName); err != nil {
		return usageError(fmt.Sprintf("write purge confirmation: %v", err))
	}
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return usageError(fmt.Sprintf("read confirmation: %v", err))
	}
	if strings.TrimSpace(answer) != clusterName {
		return guidedError("data purge cancelled")
	}
	return nil
}
