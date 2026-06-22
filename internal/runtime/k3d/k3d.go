package k3d

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pyahu/cli/pkg/schema"
	"gopkg.in/yaml.v3"
)

const (
	nodePortPostgres     = 30543
	nodePortPostgresRead = 30544
	nodePortKafka        = 30092
	nodePortKafkaConnect = 30083
	nodePortRabbitMQ     = 30672
)

type Runtime struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
}

func (r Runtime) CheckInstalled() error {
	if _, err := exec.LookPath("k3d"); err != nil {
		return fmt.Errorf("k3d not found in PATH")
	}
	return nil
}

func (r Runtime) Exists(ctx context.Context, name string) (bool, error) {
	out, err := exec.CommandContext(ctx, "k3d", "cluster", "list", "--no-headers").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("list k3d clusters: %w: %s", err, strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true, nil
		}
	}
	return false, nil
}

func (r Runtime) Create(ctx context.Context, stack *schema.Stack, stackDir string) (bool, error) {
	if err := r.CheckInstalled(); err != nil {
		return false, err
	}
	exists, err := r.Exists(ctx, stack.Cluster.Name)
	if err != nil {
		return false, err
	}
	configPath := ConfigPath(stackDir)
	configData, err := RenderConfig(stack)
	if err != nil {
		return false, err
	}
	if exists {
		missing, err := missingDesiredPorts(configPath, configData)
		if err != nil {
			return false, err
		}
		if len(missing) > 0 {
			return false, fmt.Errorf("k3d cluster %s exists without required host port mappings %s; run `pyahu down` and then `pyahu up` to recreate the cluster", stack.Cluster.Name, strings.Join(missing, ", "))
		}
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return false, err
	}
	if err := os.MkdirAll(StorageDir(stack.Cluster.Name), 0o755); err != nil {
		return false, fmt.Errorf("create k3d storage directory: %w", err)
	}
	if err := os.WriteFile(configPath, configData, 0o644); err != nil {
		return false, fmt.Errorf("write k3d config: %w", err)
	}

	cmd := exec.CommandContext(ctx, "k3d", "cluster", "create", "--config", configPath)
	cmd.Stdout = r.commandStdout()
	cmd.Stderr = r.commandStderr()
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("create k3d cluster %s: %w", stack.Cluster.Name, err)
	}
	return true, nil
}

func (r Runtime) Delete(ctx context.Context, name string) error {
	if err := r.CheckInstalled(); err != nil {
		return err
	}
	exists, err := r.Exists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	cmd := exec.CommandContext(ctx, "k3d", "cluster", "delete", name)
	cmd.Stdout = r.commandStdout()
	cmd.Stderr = r.commandStderr()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("delete k3d cluster %s: %w", name, err)
	}
	return nil
}

func (r Runtime) Kubeconfig(ctx context.Context, name string) (string, error) {
	out, err := exec.CommandContext(ctx, "k3d", "kubeconfig", "write", name).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("write kubeconfig for k3d cluster %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func missingDesiredPorts(existingConfigPath string, desiredData []byte) ([]string, error) {
	existingData, err := os.ReadFile(existingConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("k3d cluster exists but %s is missing; run `pyahu down` and then `pyahu up` to recreate it with the current stack", existingConfigPath)
		}
		return nil, fmt.Errorf("read existing k3d config: %w", err)
	}
	var existing simpleConfig
	if err := yaml.Unmarshal(existingData, &existing); err != nil {
		return nil, fmt.Errorf("parse existing k3d config %s: %w", existingConfigPath, err)
	}
	var desired simpleConfig
	if err := yaml.Unmarshal(desiredData, &desired); err != nil {
		return nil, fmt.Errorf("parse desired k3d config: %w", err)
	}
	existingPorts := map[string]bool{}
	for _, port := range existing.Ports {
		existingPorts[port.Port] = true
	}
	missing := []string{}
	for _, port := range desired.Ports {
		if !existingPorts[port.Port] {
			missing = append(missing, port.Port)
		}
	}
	return missing, nil
}

func ConfigPath(stackDir string) string {
	return filepath.Join(stackDir, schema.DefaultLocalStateDir, "k3d.yaml")
}

func StorageDir(clusterName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".pyahu", "clusters", clusterName, "storage")
	}
	return filepath.Join(home, ".pyahu", "clusters", clusterName, "storage")
}

func RenderConfig(stack *schema.Stack) ([]byte, error) {
	cfg := simpleConfig{
		APIVersion: "k3d.io/v1alpha5",
		Kind:       "Simple",
		Metadata: metadata{
			Name: stack.Cluster.Name,
		},
		Servers: stack.Cluster.Servers,
		Agents:  stack.Cluster.Agents,
		Image:   stack.Cluster.K3SVersion,
		Network: stack.Cluster.Name + "-net",
		Options: options{
			K3D: k3dOptions{
				Wait:                true,
				Timeout:             "120s",
				DisableLoadbalancer: false,
				DisableImageVolume:  false,
				DisableRollback:     false,
			},
			Kubeconfig: kubeconfigOptions{
				UpdateDefaultKubeconfig: true,
				SwitchCurrentContext:    true,
			},
		},
	}

	// HTTP/HTTPS services share the Traefik entrypoints on host 80/443 (the k3d
	// loadbalancer). Each gets an Ingress with its own *.localhost hostname.
	if stack.HTTPIngressEnabled() {
		cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:80", schema.DefaultHTTPPort), NodeFilters: []string{"loadbalancer"}})
		if stack.LocalTLSEnabled() {
			cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:443", schema.DefaultHTTPSPort), NodeFilters: []string{"loadbalancer"}})
		}
	}
	if stack.PostgresEnabled() {
		cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:%d", stack.PostgresPort(), nodePortPostgres), NodeFilters: []string{"server:0"}})
		if stack.PostgresReadReplicas() > 0 {
			cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:%d", stack.PostgresReadPort(), nodePortPostgresRead), NodeFilters: []string{"server:0"}})
		}
	}
	if stack.KafkaEnabled() {
		cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:%d", stack.KafkaPort(), nodePortKafka), NodeFilters: []string{"server:0"}})
	}
	if stack.KafkaConnectEnabled() {
		cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:%d", stack.KafkaConnectPort(), nodePortKafkaConnect), NodeFilters: []string{"server:0"}})
	}
	if stack.RabbitMQEnabled() {
		cfg.Ports = append(cfg.Ports, portMapping{Port: fmt.Sprintf("%d:%d", stack.RabbitMQPort(), nodePortRabbitMQ), NodeFilters: []string{"server:0"}})
	}

	cfg.Volumes = append(cfg.Volumes, volumeMapping{
		Volume:      StorageDir(stack.Cluster.Name) + ":/var/lib/rancher/k3s/storage",
		NodeFilters: []string{"server:*"},
	})

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (r Runtime) commandStdout() io.Writer {
	if r.Verbose && r.Out != nil {
		return r.Out
	}
	return io.Discard
}

func (r Runtime) commandStderr() io.Writer {
	if r.Verbose && r.Err != nil {
		return r.Err
	}
	return io.Discard
}

type simpleConfig struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   metadata        `yaml:"metadata"`
	Servers    int             `yaml:"servers"`
	Agents     int             `yaml:"agents"`
	Image      string          `yaml:"image"`
	Ports      []portMapping   `yaml:"ports,omitempty"`
	Volumes    []volumeMapping `yaml:"volumes,omitempty"`
	Network    string          `yaml:"network,omitempty"`
	Options    options         `yaml:"options"`
}

type metadata struct {
	Name string `yaml:"name"`
}

type portMapping struct {
	Port        string   `yaml:"port"`
	NodeFilters []string `yaml:"nodeFilters"`
}

type volumeMapping struct {
	Volume      string   `yaml:"volume"`
	NodeFilters []string `yaml:"nodeFilters"`
}

type options struct {
	K3D        k3dOptions        `yaml:"k3d"`
	Kubeconfig kubeconfigOptions `yaml:"kubeconfig"`
}

type k3dOptions struct {
	Wait                bool   `yaml:"wait"`
	Timeout             string `yaml:"timeout"`
	DisableLoadbalancer bool   `yaml:"disableLoadbalancer"`
	DisableImageVolume  bool   `yaml:"disableImageVolume"`
	DisableRollback     bool   `yaml:"disableRollback"`
}

type kubeconfigOptions struct {
	UpdateDefaultKubeconfig bool `yaml:"updateDefaultKubeconfig"`
	SwitchCurrentContext    bool `yaml:"switchCurrentContext"`
}
