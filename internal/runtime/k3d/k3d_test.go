package k3d

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pyahu/cli/pkg/schema"
)

func TestRenderConfigIncludesPostgresReadPortWhenReadReplicasEnabled(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Postgres: &schema.PostgresService{
				Enabled:      schema.Bool(true),
				ReadReplicas: 1,
			},
		},
	}
	stack.SetDefaults()

	data, err := RenderConfig(stack)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	if !strings.Contains(config, "127.0.0.1:5432:30543") {
		t.Fatalf("missing postgres primary port:\n%s", config)
	}
	if !strings.Contains(config, "127.0.0.1:5433:30544") {
		t.Fatalf("missing postgres read port:\n%s", config)
	}
}

func TestRenderConfigIncludesKafkaConnectPortWhenEnabled(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Kafka:        &schema.KafkaService{Enabled: schema.Bool(true)},
			KafkaConnect: &schema.KafkaConnectService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()

	data, err := RenderConfig(stack)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	if !strings.Contains(config, "127.0.0.1:8083:30083") {
		t.Fatalf("missing kafka connect port:\n%s", config)
	}
}

func TestRenderConfigRoutesKafkaUIThroughTraefikEntrypoints(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Kafka:   &schema.KafkaService{Enabled: schema.Bool(true)},
			KafkaUI: &schema.KafkaUIService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()

	data, err := RenderConfig(stack)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	if !strings.Contains(config, "127.0.0.1:80:80") || !strings.Contains(config, "127.0.0.1:443:443") {
		t.Fatalf("missing traefik entrypoints for kafka-ui:\n%s", config)
	}
	if strings.Contains(config, "8084") {
		t.Fatalf("kafka-ui should not map its own host port:\n%s", config)
	}
}

func TestRenderConfigOmitsZitadelHTTPSPortWhenLocalTLSDisabled(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		LocalTLS: schema.LocalTLSConfig{Enabled: schema.Bool(false)},
		Services: schema.Services{
			Postgres: &schema.PostgresService{Enabled: schema.Bool(true)},
			Zitadel:  &schema.ZitadelService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()

	data, err := RenderConfig(stack)
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	if !strings.Contains(config, "127.0.0.1:80:80") {
		t.Fatalf("missing traefik web entrypoint:\n%s", config)
	}
	if strings.Contains(config, "127.0.0.1:443:443") {
		t.Fatalf("unexpected traefik websecure entrypoint:\n%s", config)
	}
}

func TestConfigurationDriftDetectsNewHostPort(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "k3d.yaml")
	existing := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 8080:80
`)
	if err := os.WriteFile(existingPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 8080:80
  - port: 8443:443
`)

	drift, err := configurationDrift(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0] != "missing port 8443:443" {
		t.Fatalf("configuration drift = %#v", drift)
	}
}

func TestConfigurationDriftAllowsExistingExtraPorts(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "k3d.yaml")
	existing := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 8080:80
  - port: 8443:443
`)
	if err := os.WriteFile(existingPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 8080:80
`)

	drift, err := configurationDrift(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 0 {
		t.Fatalf("configuration drift = %#v", drift)
	}
}

func TestConfigurationDriftRejectsUnboundExistingMapping(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "k3d.yaml")
	existing := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 5432:30543
`)
	if err := os.WriteFile(existingPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 127.0.0.1:5432:30543
`)

	drift, err := configurationDrift(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0] != "missing port 127.0.0.1:5432:30543" {
		t.Fatalf("configuration drift = %#v", drift)
	}
}

func TestConfigurationDriftDetectsImmutableClusterChanges(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "k3d.yaml")
	existing := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
metadata:
  name: demo
servers: 1
agents: 0
image: rancher/k3s:v1.35.8-k3s1
network: demo-net
volumes:
  - volume: /old/storage:/var/lib/rancher/k3s/storage
    nodeFilters: ["server:*"]
`)
	if err := os.WriteFile(existingPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
metadata:
  name: demo
servers: 2
agents: 1
image: rancher/k3s:v1.36.4-k3s1
network: replacement-net
volumes:
  - volume: /new/storage:/var/lib/rancher/k3s/storage
    nodeFilters: ["server:*"]
`)

	drift, err := configurationDrift(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"servers 1 -> 2",
		"agents 0 -> 1",
		`image "rancher/k3s:v1.35.8-k3s1" -> "rancher/k3s:v1.36.4-k3s1"`,
		`network "demo-net" -> "replacement-net"`,
		"persistent volume mapping changed",
	}
	if strings.Join(drift, "|") != strings.Join(want, "|") {
		t.Fatalf("configuration drift = %#v, want %#v", drift, want)
	}
}

func TestConfigurationDriftDetectsChangedPortNodeFilter(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "k3d.yaml")
	existing := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 127.0.0.1:5432:30543
    nodeFilters: ["loadbalancer"]
`)
	if err := os.WriteFile(existingPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}
	desired := []byte(`apiVersion: k3d.io/v1alpha5
kind: Simple
ports:
  - port: 127.0.0.1:5432:30543
    nodeFilters: ["server:0"]
`)

	drift, err := configurationDrift(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0] != "missing port 127.0.0.1:5432:30543" {
		t.Fatalf("configuration drift = %#v", drift)
	}
}

func TestRenderConfigIncludesRedisPortWhenEnabled(t *testing.T) {
	stack := &schema.Stack{
		APIVersion: schema.APIVersion,
		Kind:       schema.Kind,
		Metadata:   schema.Metadata{Name: "demo"},
		Cluster:    schema.ClusterConfig{Runtime: "k3d", Name: "demo", Namespace: "demo-dev", Servers: 1},
		Services: schema.Services{
			Redis: &schema.RedisService{Enabled: schema.Bool(true), Ports: schema.RedisPorts{Client: 6380}},
		},
	}
	stack.SetDefaults()

	data, err := RenderConfig(stack)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "port: 127.0.0.1:6380:30379") {
		t.Fatalf("redis port mapping missing:\n%s", data)
	}
}
