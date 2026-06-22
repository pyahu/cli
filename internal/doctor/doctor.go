package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pyahu/cli/internal/runtime/k3d"
	"github.com/pyahu/cli/pkg/schema"
)

type Check struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
}

func Run(ctx context.Context, stack *schema.Stack, clusterExists bool) []Check {
	checks := []Check{
		checkCommand("k3d", "k3d is installed"),
		checkContainerRuntime(ctx),
		checkLocalClusters(ctx, stack.Cluster.Name),
	}
	if !clusterExists {
		checks = append(checks, checkPorts(stack)...)
	}
	checks = append(checks, Check{
		Name:    "host",
		OK:      true,
		Message: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	})
	return checks
}

func Healthy(checks []Check) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func Warning(check Check) bool {
	return check.OK && check.Severity == "warning"
}

func checkCommand(command string, success string) Check {
	if _, err := exec.LookPath(command); err != nil {
		return Check{Name: command, OK: false, Message: command + " not found in PATH"}
	}
	return Check{Name: command, OK: true, Message: success}
}

func checkContainerRuntime(ctx context.Context) Check {
	if _, err := exec.LookPath("docker"); err == nil {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := exec.CommandContext(cctx, "docker", "info").Run(); err == nil {
			return Check{Name: "container-runtime", OK: true, Message: "docker is available"}
		}
	}
	if _, err := exec.LookPath("podman"); err == nil {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := exec.CommandContext(cctx, "podman", "info").Run(); err == nil {
			return Check{Name: "container-runtime", OK: true, Message: "podman is available"}
		}
	}
	return Check{Name: "container-runtime", OK: false, Message: "Docker or Podman is required and must be running"}
}

func checkPorts(stack *schema.Stack) []Check {
	ports := portChecks(stack)
	checks := make([]Check, 0, len(ports))
	for _, item := range ports {
		if !item.enabled {
			continue
		}
		checks = append(checks, checkPort(item.name, item.port, item.field))
	}
	return checks
}

type portCheck struct {
	name    string
	enabled bool
	port    int
	field   string
}

func portChecks(stack *schema.Stack) []portCheck {
	return []portCheck{
		{name: "http", enabled: stack.HTTPIngressEnabled(), port: schema.DefaultHTTPPort, field: "traefik web entrypoint"},
		{name: "https", enabled: stack.HTTPIngressEnabled() && stack.LocalTLSEnabled(), port: schema.DefaultHTTPSPort, field: "traefik websecure entrypoint"},
		{name: "postgres", enabled: stack.PostgresEnabled(), port: stack.PostgresPort(), field: "services.postgres.ports.primary"},
		{name: "postgres-read", enabled: stack.PostgresEnabled() && stack.PostgresReadReplicas() > 0, port: stack.PostgresReadPort(), field: "services.postgres.ports.read"},
		{name: "rabbitmq", enabled: stack.RabbitMQEnabled(), port: stack.RabbitMQPort(), field: "services.rabbitmq.ports.amqp"},
		{name: "kafka", enabled: stack.KafkaEnabled(), port: stack.KafkaPort(), field: "services.kafka.ports.bootstrap"},
		{name: "kafka-connect", enabled: stack.KafkaConnectEnabled(), port: stack.KafkaConnectPort(), field: "services.kafkaConnect.ports.rest"},
	}
}

func checkPort(name string, port int, field string) Check {
	available := Check{Name: "port:" + name, OK: true, Message: fmt.Sprintf("127.0.0.1:%d is available", port)}
	busy := Check{Name: "port:" + name, OK: false, Message: fmt.Sprintf("127.0.0.1:%d is busy (%s)", port, field)}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		_ = listener.Close()
		return available
	}
	// Privileged ports (<1024) cannot be bound by an unprivileged process, but
	// Docker publishes them for the k3d loadbalancer. A bind permission error
	// does not mean the port is taken, so confirm with a connection probe.
	if errors.Is(err, os.ErrPermission) {
		if portInUse(port) {
			return busy
		}
		return available
	}
	return busy
}

func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

type localCluster struct {
	Runtime string
	Name    string
}

func checkLocalClusters(ctx context.Context, pyahuCluster string) Check {
	clusters, warnings := discoverLocalClusters(ctx)
	others := make([]localCluster, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.Runtime == "k3d" && cluster.Name == pyahuCluster {
			continue
		}
		others = append(others, cluster)
	}
	if len(others) > 0 {
		return Check{
			Name:     "local-clusters",
			OK:       true,
			Severity: "warning",
			Message:  fmt.Sprintf("found other local Kubernetes clusters: %s; Pyahu works best with one local cluster to avoid port and context conflicts", formatLocalClusters(others)),
		}
	}
	if len(warnings) > 0 {
		return Check{
			Name:     "local-clusters",
			OK:       true,
			Severity: "warning",
			Message:  "could not inspect all local clusters: " + strings.Join(warnings, "; "),
		}
	}
	return Check{Name: "local-clusters", OK: true, Message: "no other local Kubernetes clusters detected"}
}

func discoverLocalClusters(ctx context.Context) ([]localCluster, []string) {
	clusters := []localCluster{}
	warnings := []string{}

	k3dClusters, err := discoverK3DClusters(ctx)
	if err != nil {
		warnings = append(warnings, err.Error())
	} else {
		clusters = append(clusters, k3dClusters...)
	}

	kindClusters, err := discoverKindClusters(ctx)
	if err != nil {
		warnings = append(warnings, err.Error())
	} else {
		clusters = append(clusters, kindClusters...)
	}

	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Runtime == clusters[j].Runtime {
			return clusters[i].Name < clusters[j].Name
		}
		return clusters[i].Runtime < clusters[j].Runtime
	})
	return clusters, warnings
}

func discoverK3DClusters(ctx context.Context) ([]localCluster, error) {
	if _, err := exec.LookPath("k3d"); err != nil {
		return nil, nil
	}
	out, err := runLocalClusterCommand(ctx, "k3d", "cluster", "list", "--no-headers")
	if err != nil {
		return nil, fmt.Errorf("k3d: %w", err)
	}
	return parseClusterLines("k3d", out), nil
}

func discoverKindClusters(ctx context.Context) ([]localCluster, error) {
	if _, err := exec.LookPath("kind"); err != nil {
		return nil, nil
	}
	out, err := runLocalClusterCommand(ctx, "kind", "get", "clusters")
	if err != nil {
		return nil, fmt.Errorf("kind: %w", err)
	}
	return parseClusterLines("kind", out), nil
}

func runLocalClusterCommand(ctx context.Context, name string, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, name, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func parseClusterLines(runtime string, output string) []localCluster {
	clusters := []localCluster{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if strings.EqualFold(line, "No kind clusters found.") {
			continue
		}
		clusters = append(clusters, localCluster{Runtime: runtime, Name: fields[0]})
	}
	return clusters
}

func formatLocalClusters(clusters []localCluster) string {
	values := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		values = append(values, cluster.Runtime+"/"+cluster.Name)
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func ClusterExists(ctx context.Context, stack *schema.Stack) bool {
	runtime := k3d.Runtime{}
	exists, err := runtime.Exists(ctx, stack.Cluster.Name)
	return err == nil && exists
}
