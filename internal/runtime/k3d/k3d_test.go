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
	if !strings.Contains(config, "5432:30543") {
		t.Fatalf("missing postgres primary port:\n%s", config)
	}
	if !strings.Contains(config, "5433:30544") {
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
	if !strings.Contains(config, "8083:30083") {
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
	if !strings.Contains(config, "80:80") || !strings.Contains(config, "443:443") {
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
	if !strings.Contains(config, "80:80") {
		t.Fatalf("missing traefik web entrypoint:\n%s", config)
	}
	if strings.Contains(config, "443:443") {
		t.Fatalf("unexpected traefik websecure entrypoint:\n%s", config)
	}
}

func TestMissingDesiredPortsDetectsNewHostPort(t *testing.T) {
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

	missing, err := missingDesiredPorts(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "8443:443" {
		t.Fatalf("missing ports = %#v", missing)
	}
}

func TestMissingDesiredPortsAllowsExistingExtraPorts(t *testing.T) {
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

	missing, err := missingDesiredPorts(existingPath, desired)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing ports = %#v", missing)
	}
}
