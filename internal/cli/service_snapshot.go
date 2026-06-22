package cli

import (
	"context"

	"github.com/pyahu/cli/internal/catalog"
	"github.com/pyahu/cli/internal/config"
	"github.com/pyahu/cli/internal/kube"
)

type serviceSnapshot struct {
	Loaded         *config.LoadedStack
	ClusterRunning bool
	Services       []catalog.Service
}

func (a *app) loadServiceSnapshot(ctx context.Context) (serviceSnapshot, error) {
	loaded, err := a.loadStack()
	if err != nil {
		return serviceSnapshot{}, usageError(err.Error())
	}
	stack := loaded.Data
	rt := a.deps.newRuntime(a.opts)

	clusterRunning := false
	var statuses []kube.ServiceStatus
	if err := rt.CheckInstalled(); err == nil {
		exists, err := rt.Exists(ctx, stack.Cluster.Name)
		if err != nil {
			return serviceSnapshot{}, dependencyError(err.Error())
		}
		clusterRunning = exists
	}
	if clusterRunning {
		kubeconfig, err := rt.Kubeconfig(ctx, stack.Cluster.Name)
		if err != nil {
			return serviceSnapshot{}, clusterError(err.Error())
		}
		client, err := a.deps.newKube(kubeconfig)
		if err != nil {
			return serviceSnapshot{}, clusterError(err.Error())
		}
		statuses, err = client.Status(ctx, stack)
		if err != nil {
			return serviceSnapshot{}, serviceError(err.Error())
		}
	}

	return serviceSnapshot{
		Loaded:         loaded,
		ClusterRunning: clusterRunning,
		Services:       catalog.Build(stack, statuses, clusterRunning),
	}, nil
}
