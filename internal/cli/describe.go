package cli

import (
	"fmt"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/catalog"
)

func (a *app) newDescribeCmd() *cobra.Command {
	var showSecrets bool
	cmd := &cobra.Command{
		Use:   "describe <service>",
		Short: "Show detailed information for a local service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !validService(name) {
				return usageError("service must be one of: postgres, zitadel, rabbitmq, kafka, kafka-connect, kafka-ui")
			}
			snapshot, err := a.loadServiceSnapshot(cmd.Context())
			if err != nil {
				return err
			}
			service, ok := findSnapshotService(snapshot.Services, name)
			if !ok {
				return usageError("service must be one of: postgres, zitadel, rabbitmq, kafka, kafka-connect, kafka-ui")
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, service)
			}
			a.renderDescribe(service, showSecrets)
			return nil
		},
	}
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "show secret values in human output")
	return cmd
}

func (a *app) renderDescribe(service catalog.Service, showSecrets bool) {
	s := a.styler()
	a.info("service:   %s", s.bold(service.DisplayName))
	a.info("name:      %s", service.Name)
	a.info("status:    %s", a.colorState(service.Status))
	a.info("enabled:   %t", service.Enabled)
	a.info("ready:     %t", service.Ready)
	a.info("namespace: %s", service.Namespace)
	a.info("workload:  %s", valueOrDash(service.Workload))
	a.info("version:   %s", valueOrDash(service.Version))
	if service.Message != "" {
		a.info("message:   %s", service.Message)
	}

	a.info("")
	a.info("%s", s.bold("Endpoints"))
	tw := tabwriter.NewWriter(a.opts.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPROTOCOL\tHOST\tPORT\tURL\tINTERNAL")
	for _, endpoint := range service.Endpoints {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			endpoint.Name,
			endpoint.Protocol,
			valueOrDash(endpoint.Host),
			portOrDash(endpoint.Port),
			valueOrDash(displayEndpointURL(endpoint.URL, showSecrets)),
			valueOrDash(endpoint.Internal),
		)
	}
	_ = tw.Flush()

	if len(service.Env) > 0 {
		a.info("")
		a.info("%s", s.bold("Environment"))
		for _, key := range catalog.EnvKeys(service.Env) {
			a.info("%s %s", a.field(key, 28, s.cyan), displayEnvValue(key, service.Env[key], showSecrets))
		}
	}

	if len(service.Details) > 0 {
		a.info("")
		a.info("%s", s.bold("Details"))
		for _, key := range catalog.DetailKeys(service.Details) {
			if service.Details[key] == "" {
				continue
			}
			a.info("%s %s", a.field(key, 28, s.cyan), service.Details[key])
		}
	}

	a.info("")
	a.info("%s", s.bold("Pods"))
	if len(service.Pods) == 0 {
		a.info("none")
		return
	}
	pods := tabwriter.NewWriter(a.opts.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(pods, "NAME\tREADY\tPHASE\tREASON")
	for _, pod := range service.Pods {
		fmt.Fprintf(pods, "%s\t%t\t%s\t%s\n", pod.Name, pod.Ready, pod.Phase, valueOrDash(pod.Reason))
	}
	_ = pods.Flush()
}

func findSnapshotService(services []catalog.Service, name string) (catalog.Service, bool) {
	for _, service := range services {
		if service.Name == name {
			return service, true
		}
	}
	return catalog.Service{}, false
}

func displayEndpointURL(value string, showSecrets bool) string {
	if showSecrets {
		return value
	}
	return maskURLPassword(value)
}

func displayEnvValue(key string, value string, showSecrets bool) string {
	if showSecrets {
		return value
	}
	if isSecretEnvKey(key) {
		return "<hidden>"
	}
	return maskURLPassword(value)
}

func isSecretEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "PRIVATE_KEY") ||
		strings.Contains(upper, "MASTER_KEY")
}

func maskURLPassword(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User == nil {
		return value
	}
	username := parsed.User.Username()
	if _, ok := parsed.User.Password(); !ok {
		return value
	}
	parsed.User = url.UserPassword(username, "hidden")
	return parsed.String()
}

func portOrDash(port int) string {
	if port == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", port)
}
