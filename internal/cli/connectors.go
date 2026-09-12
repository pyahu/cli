package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/connect"
	"github.com/pyahu/cli/pkg/schema"
)

func (a *app) newConnectorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connectors",
		Short: "Register and inspect Kafka Connect connectors",
	}
	cmd.AddCommand(a.newConnectorsApplyCmd())
	cmd.AddCommand(a.newConnectorsStatusCmd())
	return cmd
}

func (a *app) newConnectorsApplyCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Register the declared connectors and wait for their tasks to run",
		Long: "Re-applies the connector Secret and Job and waits for the tasks to reach RUNNING.\n" +
			"Run it after the application that owns a connector's source has booted once:\n" +
			"an outbox connector reads a table, publication, or stream that does not exist\n" +
			"on a fresh cluster.",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, client, err := a.connectorsClient(cmd.Context())
			if err != nil {
				return err
			}
			stack := loaded.Data
			connectors, err := selectConnectors(stack, name)
			if err != nil {
				return err
			}

			label := fmt.Sprintf("Registering %d connector(s)", len(connectors))
			if name != "" {
				label = "Registering connector " + name
			}
			if err := a.phase(label, func() (string, error) {
				if err := client.ApplyConnectors(cmd.Context(), stack, name); err != nil {
					return "", serviceError(err.Error())
				}
				return "", nil
			}); err != nil {
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{"connectors": connectorNames(connectors), "applied": true})
			}
			a.info("%d connector(s) registered; run `pyahu connectors status` to inspect their tasks", len(connectors))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "apply a single connector by name")
	return cmd
}

func (a *app) newConnectorsStatusCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the state of each connector and each of its tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			stack := loaded.Data
			if !stack.KafkaConnectEnabled() {
				return usageError("services.kafkaConnect is not enabled")
			}
			connectors, err := connect.New(fmt.Sprintf("http://localhost:%d", stack.KafkaConnectPort())).Connectors(cmd.Context())
			if err != nil {
				return serviceError(err.Error())
			}

			switch format {
			case "json":
				if err := writeJSON(a.opts.out, map[string]any{
					"connectors": connectors,
					"healthy":    connect.Healthy(connectors),
				}); err != nil {
					return err
				}
			case "human":
				a.renderConnectorsStatus(connectors)
			default:
				return usageError("connectors status --format must be human or json")
			}
			if !connect.Healthy(connectors) {
				return serviceError("one or more connector tasks are not RUNNING")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "human", "format: human or json")
	return cmd
}

// renderConnectorsStatus prints one row per connector and one per task. The task
// rows are the point: a connector stays RUNNING while its only task is FAILED,
// and that silent stop is what connector-level checks miss.
func (a *app) renderConnectorsStatus(connectors []connect.Connector) {
	if a.opts.output == "json" || a.opts.quiet {
		return
	}
	if len(connectors) == 0 {
		a.info("no connectors registered")
		return
	}
	s := a.styler()
	tw := tabwriter.NewWriter(a.opts.out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, s.dim(s.bold("CONNECTOR\tTASK\tSTATE\tDETAIL")))
	for _, connector := range connectors {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", connector.Name, "-", a.colorConnectorState(connector.State), valueOrDash(connector.Type))
		if len(connector.Tasks) == 0 {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", "", "-", a.colorConnectorState("NO TASKS"), "the connector has no tasks")
			continue
		}
		for _, task := range connector.Tasks {
			_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", "", task.ID, a.colorConnectorState(task.State), valueOrDash(task.Trace))
		}
	}
	_ = tw.Flush()
}

func (a *app) colorConnectorState(state string) string {
	s := a.styler()
	switch state {
	case "RUNNING":
		return s.green(state)
	case "PAUSED", "UNASSIGNED":
		return s.yellow(state)
	default:
		return s.red(state)
	}
}

func (a *app) connectorsClient(ctx context.Context) (*loadedStack, localKube, error) {
	loaded, client, err := a.backupRestoreClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !loaded.Data.KafkaConnectEnabled() {
		return nil, nil, usageError("services.kafkaConnect is not enabled")
	}
	return loaded, client, nil
}

func selectConnectors(stack *schema.Stack, name string) ([]schema.KafkaConnectConnector, error) {
	all := stack.Services.KafkaConnect.Connectors
	if name == "" {
		if len(all) == 0 {
			return nil, usageError("services.kafkaConnect.connectors is empty")
		}
		return all, nil
	}
	for _, connector := range all {
		if connector.Name == name {
			return []schema.KafkaConnectConnector{connector}, nil
		}
	}
	return nil, usageError(fmt.Sprintf("connector %q is not declared under services.kafkaConnect.connectors (declared: %s)", name, strings.Join(connectorNames(all), ", ")))
}

func connectorNames(connectors []schema.KafkaConnectConnector) []string {
	names := make([]string, 0, len(connectors))
	for _, connector := range connectors {
		names = append(names, connector.Name)
	}
	return names
}
