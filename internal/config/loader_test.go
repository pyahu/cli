package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pyahu/cli/pkg/schema"
)

func TestLoadPlatformPreset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	content, err := Preset("platform")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	stack := loaded.Data
	if stack.Cluster.Name != "pyahu-local" {
		t.Fatalf("cluster name = %q", stack.Cluster.Name)
	}
	if stack.Cluster.Namespace != "pyahu-local-dev" {
		t.Fatalf("namespace = %q", stack.Cluster.Namespace)
	}
	services := strings.Join(stack.EnabledServices(), ",")
	if services != "postgres,zitadel,rabbitmq,redis,kafka,kafka-connect,kafka-ui" {
		t.Fatalf("enabled services = %q", services)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  mongodb:
    enabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "field mongodb not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRedisDefaultsAndConnectionEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  redis:
    enabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	stack := loaded.Data
	redis := stack.Services.Redis

	if redis.Image != "valkey/valkey" || redis.Version != "8.1-alpine" {
		t.Fatalf("redis image = %s:%s", redis.Image, redis.Version)
	}
	if redis.Ports.Client != 6379 {
		t.Fatalf("redis client port = %d", redis.Ports.Client)
	}
	if !stack.RedisAppendOnly() {
		t.Fatal("appendOnly should default to true")
	}
	if redis.Storage != "1Gi" {
		t.Fatalf("redis storage = %q", redis.Storage)
	}
	if got := stack.RedisInternalHost(); got != "redis.demo-dev.svc.cluster.local" {
		t.Fatalf("redis internal host = %q", got)
	}
	if services := strings.Join(stack.EnabledServices(), ","); services != "redis" {
		t.Fatalf("enabled services = %q", services)
	}

	env := stack.ConnectionEnv()
	assertEnv(t, env, "REDIS_HOST", "localhost")
	assertEnv(t, env, "REDIS_PORT", "6379")
	assertEnv(t, env, "REDIS_PASSWORD", "")
	assertEnv(t, env, "REDIS_URL", "redis://localhost:6379")
}

func TestRedisPasswordIsCarriedIntoTheConnectionURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  redis:
    enabled: true
    ports:
      client: 6380
    auth:
      password: redis_local
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	env := loaded.Data.ConnectionEnv()
	assertEnv(t, env, "REDIS_PASSWORD", "redis_local")
	assertEnv(t, env, "REDIS_URL", "redis://:redis_local@localhost:6380")
}

func TestLoadRejectsUnparseableRedisStorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  redis:
    enabled: true
    storage: plenty
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected storage validation error")
	}
	if !strings.Contains(err.Error(), "services.redis.storage must be a Kubernetes quantity") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedisPortMustNotCollideWithAnotherService(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  redis:
    enabled: true
    ports:
      client: 5432
  postgres:
    enabled: true
    ports:
      primary: 5432
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected duplicated host port error")
	}
	if !strings.Contains(err.Error(), "services.redis.ports.client") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlatformPresetConnectionEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	content, err := Preset("platform")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	env := loaded.Data.ConnectionEnv()
	assertEnv(t, env, "POSTGRES_URL", "postgresql://pyahu:pyahu_local@localhost:5432/app?sslmode=disable")
	assertEnv(t, env, "KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
	assertEnv(t, env, "KAFKA_CONNECT_URL", "http://localhost:8083")
	assertEnv(t, env, "KAFKA_UI_URL", "https://kafka-ui.localhost")
	assertEnv(t, env, "RABBITMQ_URL", "amqp://pyahu:pyahu_local@localhost:5672")
	assertEnv(t, env, "RABBITMQ_MANAGEMENT_URL", "https://rabbitmq.localhost")
	assertEnv(t, env, "ZITADEL_ISSUER", "https://zitadel.localhost")
}

func TestConnectionEnvUsesConfiguredLocalCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: caelix
services:
  postgres:
    enabled: true
    ports:
      primary: 15432
    auth:
      username: oms
      password: oms_local
    databases:
      - name: oms_core
  zitadel:
    enabled: true
    externalURL: http://zitadel.localhost:18080
    masterKey: 0123456789abcdef0123456789abcdef
    admin:
      username: admin@caelix.local
      password: Caelix1!
  rabbitmq:
    enabled: true
    ports:
      amqp: 15672
      management: 25672
    auth:
      username: oms
      password: oms_local
    vhosts:
      - name: orders
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	env := loaded.Data.ConnectionEnv()
	assertEnv(t, env, "POSTGRES_URL", "postgresql://oms:oms_local@localhost:15432/oms_core?sslmode=disable")
	assertEnv(t, env, "RABBITMQ_URL", "amqp://oms:oms_local@localhost:15672")
	assertEnv(t, env, "ZITADEL_ADMIN_USER", "admin@caelix.local")
	assertEnv(t, env, "ZITADEL_ADMIN_PASSWORD", "Caelix1!")
}

func TestConnectionEnvIncludesPostgresReadReplicaEndpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    ports:
      primary: 15432
      read: 15433
    auth:
      username: app
      password: app_local
    readReplicas: 2
    replication:
      username: app_replicator
      password: repl_local
    databases:
      - name: app
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	env := loaded.Data.ConnectionEnv()
	assertEnv(t, env, "POSTGRES_URL", "postgresql://app:app_local@localhost:15432/app?sslmode=disable")
	assertEnv(t, env, "POSTGRES_READ_URL", "postgresql://app:app_local@localhost:15433/app?sslmode=disable")
	assertEnv(t, env, "POSTGRES_READ_PORT", "15433")
}

func TestZitadelDefaultsToHTTPSWhenLocalTLSIsEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    databases:
      - name: app
  zitadel:
    enabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	assertEnv(t, loaded.Data.ConnectionEnv(), "ZITADEL_ISSUER", "https://zitadel.localhost")
	if !loaded.Data.LocalTLSEnabled() {
		t.Fatal("expected local TLS to be enabled by default")
	}
}

func TestZitadelDefaultsToHTTPWhenLocalTLSIsDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
localTLS:
  enabled: false
services:
  postgres:
    enabled: true
    databases:
      - name: app
  zitadel:
    enabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	assertEnv(t, loaded.Data.ConnectionEnv(), "ZITADEL_ISSUER", "http://zitadel.localhost")
	if loaded.Data.LocalTLSEnabled() {
		t.Fatal("expected local TLS to be disabled")
	}
}

func TestLoadRejectsLocalTLSDomainOutsideLocalhost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
localTLS:
  domains:
    - example.com
services:
  postgres:
    enabled: true
    databases:
      - name: app
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "localTLS.domains") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLegacyClusterPortsAreAppliedWhenServicePortsAreAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
cluster:
  ports:
    postgres: 15432
    kafka: 19092
services:
  postgres:
    enabled: true
    databases:
      - name: app
  kafka:
    enabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Data.PostgresPort() != 15432 {
		t.Fatalf("postgres port = %d", loaded.Data.PostgresPort())
	}
	if loaded.Data.KafkaPort() != 19092 {
		t.Fatalf("kafka port = %d", loaded.Data.KafkaPort())
	}
}

func TestLoadDebeziumConnectorDefaultsAndUsesPostgresSuperuser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    auth:
      username: app
      password: app_local
    databases:
      - name: appdb
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-cdc
        tables:
          include:
            - public.orders
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	connector := loaded.Data.Services.KafkaConnect.Connectors[0]
	if connector.Type != "source" {
		t.Fatalf("type = %q", connector.Type)
	}
	if connector.Kind != "debezium.postgres" {
		t.Fatalf("kind = %q", connector.Kind)
	}
	if connector.Database != "appdb" {
		t.Fatalf("database = %q", connector.Database)
	}
	if connector.TopicPrefix != "app-cdc" {
		t.Fatalf("topic prefix = %q", connector.TopicPrefix)
	}
	if connector.Slot != "app_cdc_slot" {
		t.Fatalf("slot = %q", connector.Slot)
	}

	config := loaded.Data.DebeziumPostgresConfig(connector)
	if config["database.user"] != "app" || config["database.password"] != "app_local" {
		t.Fatalf("connector does not use local postgres superuser: %#v", config)
	}
	if config["connector.class"] != "io.debezium.connector.postgresql.PostgresConnector" {
		t.Fatalf("connector.class = %q", config["connector.class"])
	}
	if config["publication.autocreate.mode"] != "filtered" {
		t.Fatalf("publication.autocreate.mode = %q", config["publication.autocreate.mode"])
	}
	if config["table.include.list"] != "public.orders" {
		t.Fatalf("table.include.list = %q", config["table.include.list"])
	}
}

func TestLoadCustomSinkConnectorConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: orders-sink
        type: sink
        config:
          connector.class: io.confluent.connect.jdbc.JdbcSinkConnector
          tasks.max: "1"
          topics: app-cdc.public.orders
          connection.url: jdbc:postgresql://warehouse.local/orders
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	connector := loaded.Data.Services.KafkaConnect.Connectors[0]
	if connector.Type != "sink" {
		t.Fatalf("type = %q", connector.Type)
	}
	if connector.Kind != "custom" {
		t.Fatalf("kind = %q", connector.Kind)
	}
	config := loaded.Data.KafkaConnectConnectorConfig(connector)
	if config["connector.class"] != "io.confluent.connect.jdbc.JdbcSinkConnector" {
		t.Fatalf("connector.class = %q", config["connector.class"])
	}
	if config["topics"] != "app-cdc.public.orders" {
		t.Fatalf("topics = %q", config["topics"])
	}
	if _, ok := config["database.hostname"]; ok {
		t.Fatalf("custom sink config includes Debezium fields: %#v", config)
	}
}

func TestCustomConnectorDefaultsToSourceType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-source
        config:
          connector.class: io.example.SourceConnector
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	connector := loaded.Data.Services.KafkaConnect.Connectors[0]
	if connector.Kind != "custom" || connector.Type != "source" {
		t.Fatalf("connector kind/type = %s/%s", connector.Kind, connector.Type)
	}
	if schema.ConnectorOptional(connector) {
		t.Fatal("optional should default to false")
	}
}

func TestLoadRejectsConnectorWithUnsupportedType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-sink
        type: transform
        config:
          connector.class: io.example.SinkConnector
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "type must be source or sink") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOptionalConnectorIsMarkedOptional(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: checkout-outbox
        kind: custom
        optional: true
        config:
          connector.class: com.redis.kafka.connect.RedisStreamSourceConnector
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !schema.ConnectorOptional(loaded.Data.Services.KafkaConnect.Connectors[0]) {
		t.Fatal("connector should be optional")
	}
}

func TestLoadRejectsDebeziumSinkConnector(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    databases:
      - name: app
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-cdc
        type: sink
        kind: debezium.postgres
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "kind debezium.postgres must use type source") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsKafkaUIWithoutKafka(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafkaUI:
    enabled: true
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "services.kafkaUI requires services.kafka") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostgresReadPortIsIgnoredWhenReadReplicasDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    ports:
      primary: 5433
      read: 15433
    databases:
      - name: app
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	env := loaded.Data.ConnectionEnv()
	if _, ok := env["POSTGRES_READ_URL"]; ok {
		t.Fatalf("unexpected POSTGRES_READ_URL: %#v", env)
	}
	assertEnv(t, env, "POSTGRES_URL", "postgresql://pyahu:pyahu_local@localhost:5433/app?sslmode=disable")
}

func TestDisabledServicePortsAreIgnoredDuringValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    ports:
      primary: 15432
    databases:
      - name: app
  kafka:
    enabled: false
    ports:
      bootstrap: 15432
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("disabled kafka port should not be validated: %v", err)
	}
}

func TestEnabledServicePortsMustBeUnique(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    ports:
      primary: 15432
    databases:
      - name: app
  kafka:
    enabled: true
    ports:
      bootstrap: 15432
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected duplicate port validation error")
	}
	if !strings.Contains(err.Error(), "duplicates") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsPostgresMultiplePrimaryInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    instances: 2
    databases:
      - name: app
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "readReplicas") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsUnsupportedServiceReplicas(t *testing.T) {
	tests := []struct {
		name     string
		services string
		want     string
	}{
		{
			name: "kafka brokers",
			services: `  kafka:
    enabled: true
    replicas: 2
`,
			want: "services.kafka.replicas currently must be 1",
		},
		{
			name: "rabbitmq nodes",
			services: `  rabbitmq:
    enabled: true
    replicas: 2
`,
			want: "services.rabbitmq.replicas currently must be 1",
		},
		{
			name: "topic replication",
			services: `  kafka:
    enabled: true
    replicas: 1
    topics:
      - name: orders
        replicas: 2
`,
			want: "services.kafka.topics[0].replicas cannot exceed services.kafka.replicas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "pyahu.yaml")
			data := []byte("apiVersion: cli.pyahu.io/v1alpha1\nkind: Stack\nmetadata:\n  name: demo\nservices:\n" + tt.services)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(path)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error does not contain %q: %v", tt.want, err)
			}
		})
	}
}

func TestLoadFromFlagUsesDefaultDiscovery(t *testing.T) {
	dir := t.TempDir()
	isolateUserConfigDir(t)
	content, err := Preset("minimal")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, DefaultFileName)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "apps", "api")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(subdir)

	loaded, err := LoadFromFlag("")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Path != path {
		t.Fatalf("loaded path = %q, want %q", loaded.Path, path)
	}
}

func TestLoadFromFlagMergesGlobalConfigWithProjectConfig(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "project")
	isolateUserConfigDir(t)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	globalPath, err := GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	global := []byte(`services:
  postgres:
    enabled: true
    version: "16"
    ports:
      primary: 15432
    storage: 1Gi
    databases:
      - name: global_db
  zitadel:
    enabled: true
    ports:
      http: 18080
    masterKey: 0123456789abcdef0123456789abcdef
  kafka:
    enabled: true
    version: "4.1.0"
    ports:
      bootstrap: 19092
    topics:
      - name: global.events
configMaps:
  app:
    data:
      SHARED: global
      GLOBAL_ONLY: "true"
`)
	if err := os.WriteFile(globalPath, global, 0o644); err != nil {
		t.Fatal(err)
	}

	projectPath := filepath.Join(projectDir, DefaultFileName)
	project := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: project
cluster:
  name: project-cluster
services:
  postgres:
    ports:
      primary: 25432
    storage: 5Gi
    databases:
      - name: app
  kafka:
    enabled: false
configMaps:
  app:
    data:
      SHARED: local
      LOCAL_ONLY: "true"
`)
	if err := os.WriteFile(projectPath, project, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(projectDir)

	loaded, err := LoadFromFlag("")
	if err != nil {
		t.Fatal(err)
	}
	stack := loaded.Data
	if loaded.GlobalPath != globalPath {
		t.Fatalf("global path = %q, want %q", loaded.GlobalPath, globalPath)
	}
	if loaded.ProjectPath != projectPath {
		t.Fatalf("project path = %q, want %q", loaded.ProjectPath, projectPath)
	}
	if stack.Metadata.Name != "project" {
		t.Fatalf("metadata.name = %q", stack.Metadata.Name)
	}
	if stack.Cluster.Name != "project-cluster" {
		t.Fatalf("cluster.name = %q", stack.Cluster.Name)
	}
	if stack.PostgresPort() != 25432 {
		t.Fatalf("postgres port = %d", stack.PostgresPort())
	}
	if stack.ZitadelHTTPPort() != 18080 {
		t.Fatalf("zitadel http port = %d", stack.ZitadelHTTPPort())
	}
	if stack.Services.Postgres.Version != "16" {
		t.Fatalf("postgres version = %q", stack.Services.Postgres.Version)
	}
	if stack.Services.Postgres.Storage != "5Gi" {
		t.Fatalf("postgres storage = %q", stack.Services.Postgres.Storage)
	}
	if got := stack.Services.Postgres.Databases[0].Name; got != "app" {
		t.Fatalf("postgres database = %q", got)
	}
	if stack.KafkaEnabled() {
		t.Fatal("expected local kafka.enabled=false to override global")
	}
	if got := stack.ConfigMaps["app"].Data["SHARED"]; got != "local" {
		t.Fatalf("configMaps.app.data.SHARED = %q", got)
	}
	if got := stack.ConfigMaps["app"].Data["GLOBAL_ONLY"]; got != "true" {
		t.Fatalf("configMaps.app.data.GLOBAL_ONLY = %q", got)
	}
	if got := stack.ConfigMaps["app"].Data["LOCAL_ONLY"]; got != "true" {
		t.Fatalf("configMaps.app.data.LOCAL_ONLY = %q", got)
	}
}

func isolateUserConfigDir(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	t.Setenv("APPDATA", filepath.Join(root, "AppData", "Roaming"))
	t.Setenv("USERPROFILE", root)
}

func assertEnv(t *testing.T, env map[string]string, key string, expected string) {
	t.Helper()
	if env[key] != expected {
		t.Fatalf("%s = %q, want %q", key, env[key], expected)
	}
}

func loadPluginStack(t *testing.T, pluginsYAML string) (*LoadedStack, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    plugins:
` + pluginsYAML)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

func TestLoadKafkaConnectPluginsAllowRepeatedNames(t *testing.T) {
	loaded, err := loadPluginStack(t, `      - name: redis-kafka-connect
        url: https://example.test/redis-kafka-connect-1.1.0.zip
        sha256: 7e4249ca356f702220cf09e9e150c8336e24824d7a72fce05d7999bf2a7e03df
      - name: redis-kafka-connect
        file: connect-plugins/router.jar
`)
	if err != nil {
		t.Fatal(err)
	}
	stack := loaded.Data
	if got := len(stack.KafkaConnectPlugins()); got != 2 {
		t.Fatalf("plugins = %d", got)
	}
	// Same name means one directory, which is what puts the transform on the
	// connector's classloader.
	if dirs := stack.KafkaConnectPluginDirs(); len(dirs) != 1 || dirs[0] != "redis-kafka-connect" {
		t.Fatalf("plugin dirs = %#v", dirs)
	}
}

func TestLoadRejectsPluginURLWithoutSHA256(t *testing.T) {
	_, err := loadPluginStack(t, `      - name: redis-kafka-connect
        url: https://example.test/redis-kafka-connect-1.1.0.zip
`)
	if err == nil || !strings.Contains(err.Error(), "sha256 is required with url") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsPluginWithBothSources(t *testing.T) {
	_, err := loadPluginStack(t, `      - name: router
        url: https://example.test/router.jar
        sha256: 7e4249ca356f702220cf09e9e150c8336e24824d7a72fce05d7999bf2a7e03df
        file: connect-plugins/router.jar
`)
	if err == nil || !strings.Contains(err.Error(), "exactly one of url or file") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsPluginWithNeitherSource(t *testing.T) {
	_, err := loadPluginStack(t, `      - name: router
`)
	if err == nil || !strings.Contains(err.Error(), "exactly one of url or file") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsPluginWithUnsupportedArtifact(t *testing.T) {
	_, err := loadPluginStack(t, `      - name: router
        file: connect-plugins/router.tgz.bak
`)
	if err == nil || !strings.Contains(err.Error(), "must be a .zip, .tar.gz, .tgz, or .jar") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsAbsolutePluginFile(t *testing.T) {
	_, err := loadPluginStack(t, `      - name: router
        file: /opt/router.jar
`)
	if err == nil || !strings.Contains(err.Error(), "must be relative to the stack file") {
		t.Fatalf("error = %v", err)
	}
}
