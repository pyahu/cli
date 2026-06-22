package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/catalog"
)

func (a *app) newServicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "services",
		Aliases: []string{"svc", "ls"},
		Short:   "List local Pyahu services and endpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			snapshot, err := a.loadServiceSnapshot(cmd.Context())
			if err != nil {
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{
					"cluster":   snapshot.Loaded.Data.Cluster.Name,
					"namespace": snapshot.Loaded.Data.Cluster.Namespace,
					"running":   snapshot.ClusterRunning,
					"services":  snapshot.Services,
				})
			}
			a.renderServices(snapshot)
			return nil
		},
	}
}

func (a *app) renderServices(snapshot serviceSnapshot) {
	s := a.styler()
	stack := snapshot.Loaded.Data
	a.info("cluster:   %s", stack.Cluster.Name)
	a.info("namespace: %s", stack.Cluster.Namespace)
	a.info("state:     %s", a.colorState(clusterState(snapshot.ClusterRunning)))
	a.info("")

	type serviceRow struct {
		name      string
		status    string
		version   string
		endpoints string
	}
	rows := make([]serviceRow, 0, len(snapshot.Services))
	wName, wStatus, wVersion := len("SERVICE"), len("STATUS"), len("VERSION")
	for _, service := range snapshot.Services {
		if !service.Enabled {
			continue
		}
		row := serviceRow{
			name:      service.Name,
			status:    service.Status,
			version:   valueOrDash(service.Version),
			endpoints: compactEndpoints(service.Endpoints),
		}
		rows = append(rows, row)
		wName = max(wName, len(row.name))
		wStatus = max(wStatus, len(row.status))
		wVersion = max(wVersion, len(row.version))
	}

	header := fmt.Sprintf("%-*s  %-*s  %-*s  %s", wName, "SERVICE", wStatus, "STATUS", wVersion, "VERSION", "ENDPOINTS")
	a.info("%s", s.dim(s.bold(header)))
	for _, row := range rows {
		a.info("%s  %s  %s  %s",
			a.field(row.name, wName, nil),
			a.field(row.status, wStatus, a.stateColorFn(row.status)),
			a.field(row.version, wVersion, s.dim),
			row.endpoints,
		)
	}
}

func compactEndpoints(endpoints []catalog.Endpoint) string {
	values := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		switch {
		case (endpoint.Protocol == "http" || endpoint.Protocol == "https") && endpoint.URL != "":
			values = append(values, endpoint.URL)
		case endpoint.Port > 0:
			values = append(values, fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port))
		default:
			values = append(values, endpoint.Host)
		}
	}
	return strings.Join(values, ", ")
}
