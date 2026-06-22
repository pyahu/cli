package kube

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/pyahu/cli/pkg/schema"
)

func TestPostgresInitScriptCreatesOwnerRole(t *testing.T) {
	script := postgresInitScript([]schema.DatabaseConfig{{Name: "orders", Owner: "orders"}}, "pyahu", "", "")

	if !strings.Contains(script, "CREATE ROLE \"orders\"") {
		t.Fatalf("script does not create owner role:\n%s", script)
	}
	if !strings.Contains(script, "CREATE DATABASE \"orders\" OWNER \"orders\"") {
		t.Fatalf("script does not create database with owner:\n%s", script)
	}
}

func TestPostgresInitScriptUsesCustomDefaultOwner(t *testing.T) {
	script := postgresInitScript([]schema.DatabaseConfig{{Name: "orders"}}, "oms", "", "")

	if strings.Contains(script, "CREATE ROLE \"oms\"") {
		t.Fatalf("script should not create the default owner role:\n%s", script)
	}
	if !strings.Contains(script, "CREATE DATABASE \"orders\" OWNER \"oms\"") {
		t.Fatalf("script does not use custom default owner:\n%s", script)
	}
}

func TestPostgresInitScriptCreatesReplicationRole(t *testing.T) {
	script := postgresInitScript([]schema.DatabaseConfig{{Name: "orders"}}, "oms", "oms_replicator", "pa'ss")

	if !strings.Contains(script, `CREATE ROLE "oms_replicator" WITH REPLICATION LOGIN PASSWORD 'pa''ss'`) {
		t.Fatalf("script does not create replication role:\n%s", script)
	}
	if !strings.Contains(script, `ALTER ROLE "oms_replicator" WITH REPLICATION LOGIN PASSWORD 'pa''ss'`) {
		t.Fatalf("script does not rotate replication password:\n%s", script)
	}
}

func TestPostgresInitScriptRunsSeedInTargetDatabase(t *testing.T) {
	dir := t.TempDir()
	seedPath := filepath.Join(dir, "seed.sql")
	if err := os.WriteFile(seedPath, []byte("create table probe(id integer);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	script, err := postgresInitScriptWithSeeds([]schema.DatabaseConfig{{Name: "orders", Seed: "seed.sql"}}, "pyahu", "", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, `--dbname "orders"`) {
		t.Fatalf("script does not run seed against target database:\n%s", script)
	}
	if !strings.Contains(script, "create table probe") {
		t.Fatalf("script does not contain seed SQL:\n%s", script)
	}
}

func TestPostgresInitScriptFailsWhenSeedIsMissing(t *testing.T) {
	_, err := postgresInitScriptWithSeeds([]schema.DatabaseConfig{{Name: "orders", Seed: "missing.sql"}}, "pyahu", "", "", t.TempDir())
	if err == nil {
		t.Fatal("expected missing seed error")
	}
}

func TestPostgresReadReplicaResources(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.Postgres.ReadReplicas = 2
	stack.SetDefaults()

	primarySelector := postgresSelector(stack, "primary")
	primaryService := postgresService(stack.Cluster.Namespace, map[string]string{}, primarySelector)
	readService := postgresReadService(stack.Cluster.Namespace, map[string]string{}, postgresSelector(stack, "replica"))
	readSts := postgresReadStatefulSet(stack, map[string]string{}, postgresSelector(stack, "replica"))

	if primaryService.Spec.Selector["pyahu.io/postgres-role"] != "primary" {
		t.Fatalf("primary service selector = %#v", primaryService.Spec.Selector)
	}
	if readService.Spec.Selector["pyahu.io/postgres-role"] != "replica" {
		t.Fatalf("read service selector = %#v", readService.Spec.Selector)
	}
	if readService.Spec.Ports[0].NodePort != nodePortPostgresRead {
		t.Fatalf("read nodePort = %d", readService.Spec.Ports[0].NodePort)
	}
	if got := *readSts.Spec.Replicas; got != 2 {
		t.Fatalf("read replicas = %d", got)
	}
	if readSts.Spec.Template.Spec.InitContainers[0].Name != "basebackup" {
		t.Fatalf("missing basebackup init container")
	}
}

func TestPostgresStatefulSetEnablesLogicalReplication(t *testing.T) {
	stack := testPlatformStack()
	sts := postgresStatefulSet(stack, map[string]string{}, postgresSelector(stack, "primary"))
	args := strings.Join(sts.Spec.Template.Spec.Containers[0].Args, " ")

	for _, want := range []string{"wal_level=logical", "max_replication_slots=10", "max_wal_senders=10"} {
		if !strings.Contains(args, want) {
			t.Fatalf("postgres args do not contain %q: %s", want, args)
		}
	}
}

func TestKafkaStatefulSetAdvertisesHostAndClusterListeners(t *testing.T) {
	stack := testPlatformStack()
	sts := kafkaStatefulSet(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "kafka"})
	env := containerEnv(sts.Spec.Template.Spec.Containers[0].Env)

	got := env["KAFKA_ADVERTISED_LISTENERS"]
	if !strings.Contains(got, "PLAINTEXT://kafka.pyahu-local-dev.svc.cluster.local:9092") {
		t.Fatalf("missing cluster listener: %s", got)
	}
	if !strings.Contains(got, "EXTERNAL://localhost:9092") {
		t.Fatalf("missing host listener: %s", got)
	}
}

func TestKafkaTopicJobNameFitsKubernetesLimit(t *testing.T) {
	name := kafkaTopicJobName("company.product.billing.very.long.topic.name.with.many.sections.and.underscores")

	if len(name) > 63 {
		t.Fatalf("job name length = %d: %s", len(name), name)
	}
	if !strings.HasPrefix(name, "kafka-topic-") {
		t.Fatalf("job name = %q", name)
	}
}

func TestKafkaTopicJobUsesKafkaScriptsPath(t *testing.T) {
	stack := testPlatformStack()
	job := kafkaTopicJob(stack, schema.TopicConfig{Name: "app.events", Partitions: 1, Replicas: 1})
	args := strings.Join(job.Spec.Template.Spec.Containers[0].Args, " ")

	if !strings.Contains(args, "/opt/kafka/bin/kafka-topics.sh") {
		t.Fatalf("topic job does not use kafka scripts path: %s", args)
	}
}

func TestKafkaConnectConnectorJobNameFitsKubernetesLimit(t *testing.T) {
	stack := testPlatformStack()
	connector := schema.DebeziumConnector{
		Name:         "orders-cdc-very-long-connector-name-for-a-large-monorepo-context",
		Database:     "app",
		TopicPrefix:  "orders-cdc",
		Slot:         "orders_cdc_slot",
		Publication:  "orders_cdc_publication",
		SnapshotMode: "initial",
	}
	payload, err := kafkaConnectConnectorPayload(stack, connector)
	if err != nil {
		t.Fatal(err)
	}
	name := kafkaConnectConnectorResourceName(connector, payload)

	if len(name) > 63 {
		t.Fatalf("job name length = %d: %s", len(name), name)
	}
	if !strings.HasPrefix(name, "kafka-connect-") {
		t.Fatalf("job name = %q", name)
	}
}

func TestKafkaConnectServiceUsesStableNodePort(t *testing.T) {
	svc := kafkaConnectService("pyahu-local-dev", map[string]string{}, map[string]string{})

	if got := svc.Spec.Ports[0].NodePort; got != nodePortKafkaConnect {
		t.Fatalf("kafka connect nodePort = %d", got)
	}
}

func TestKafkaConnectDeploymentUsesDebeziumImageAndKafkaBootstrap(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.KafkaConnect = &schema.KafkaConnectService{Enabled: schema.Bool(true)}
	stack.SetDefaults()

	deployment := kafkaConnectDeployment(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "kafka-connect"})
	container := deployment.Spec.Template.Spec.Containers[0]
	env := containerEnv(container.Env)

	if container.Image != "quay.io/debezium/connect:3.5.2.Final" {
		t.Fatalf("image = %q", container.Image)
	}
	if env["BOOTSTRAP_SERVERS"] != "kafka.pyahu-local-dev.svc.cluster.local:9092" {
		t.Fatalf("BOOTSTRAP_SERVERS = %q", env["BOOTSTRAP_SERVERS"])
	}
	if env["CONFIG_STORAGE_TOPIC"] != "pyahu-local.connect.configs" {
		t.Fatalf("CONFIG_STORAGE_TOPIC = %q", env["CONFIG_STORAGE_TOPIC"])
	}
}

func TestKafkaUIServiceIsClusterIPForIngress(t *testing.T) {
	svc := kafkaUIService("pyahu-local-dev", map[string]string{}, map[string]string{})

	// Kafka UI is exposed through Traefik, so it uses a ClusterIP service (no NodePort).
	if svc.Spec.Type != "" && svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("kafka ui service type = %q, want ClusterIP", svc.Spec.Type)
	}
	if got := svc.Spec.Ports[0].NodePort; got != 0 {
		t.Fatalf("kafka ui nodePort = %d, want 0", got)
	}
	if got := svc.Spec.Ports[0].Port; got != 8080 {
		t.Fatalf("kafka ui port = %d, want 8080", got)
	}
}

func TestHTTPIngressTargetsHostAndService(t *testing.T) {
	ingress := httpIngress("kafka-ui", "pyahu-local-dev", map[string]string{}, "kafka-ui.localhost", "kafka-ui", 8080, true, "pyahu-local-tls")

	rule := ingress.Spec.Rules[0]
	if rule.Host != "kafka-ui.localhost" {
		t.Fatalf("host = %q", rule.Host)
	}
	backend := rule.HTTP.Paths[0].Backend.Service
	if backend.Name != "kafka-ui" || backend.Port.Number != 8080 {
		t.Fatalf("backend = %+v", backend)
	}
	if len(ingress.Spec.TLS) != 1 || ingress.Spec.TLS[0].SecretName != "pyahu-local-tls" {
		t.Fatalf("tls = %+v", ingress.Spec.TLS)
	}
	if ingress.Spec.TLS[0].Hosts[0] != "kafka-ui.localhost" {
		t.Fatalf("tls host = %v", ingress.Spec.TLS[0].Hosts)
	}
}

func TestKafkaUIDeploymentUsesKafbatImageAndKafkaBootstrap(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.KafkaConnect = &schema.KafkaConnectService{Enabled: schema.Bool(true)}
	stack.Services.KafkaUI = &schema.KafkaUIService{Enabled: schema.Bool(true)}
	stack.SetDefaults()

	deployment := kafkaUIDeployment(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "kafka-ui"})
	container := deployment.Spec.Template.Spec.Containers[0]
	env := containerEnv(container.Env)

	if container.Image != "ghcr.io/kafbat/kafka-ui:v1.5.0" {
		t.Fatalf("image = %q", container.Image)
	}
	if env["KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS"] != "kafka.pyahu-local-dev.svc.cluster.local:9092" {
		t.Fatalf("KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS = %q", env["KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS"])
	}
	if env["KAFKA_CLUSTERS_0_KAFKACONNECT_0_ADDRESS"] != "http://kafka-connect.pyahu-local-dev.svc.cluster.local:8083" {
		t.Fatalf("KAFKA_CLUSTERS_0_KAFKACONNECT_0_ADDRESS = %q", env["KAFKA_CLUSTERS_0_KAFKACONNECT_0_ADDRESS"])
	}
	if got := *deployment.Spec.Replicas; got != 1 {
		t.Fatalf("replicas = %d", got)
	}
}

func TestKafkaConnectConnectorPayloadUsesPostgresSuperuser(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.Postgres.Auth = schema.AuthConfig{Username: "oms", Password: "oms_local"}
	stack.Services.KafkaConnect = &schema.KafkaConnectService{
		Enabled: schema.Bool(true),
		Connectors: []schema.DebeziumConnector{{
			Name: "orders-cdc",
			Tables: schema.TableFilter{
				Include: []string{"public.orders"},
			},
		}},
	}
	stack.SetDefaults()

	payload, err := kafkaConnectConnectorPayload(stack, stack.Services.KafkaConnect.Connectors[0])
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(payload), &config); err != nil {
		t.Fatal(err)
	}

	assertConfig := map[string]string{
		"connector.class":             "io.debezium.connector.postgresql.PostgresConnector",
		"database.user":               "oms",
		"database.password":           "oms_local",
		"database.dbname":             "app",
		"topic.prefix":                "orders-cdc",
		"slot.name":                   "orders_cdc_slot",
		"publication.name":            "orders_cdc_publication",
		"publication.autocreate.mode": "filtered",
		"table.include.list":          "public.orders",
	}
	for key, want := range assertConfig {
		if got := config[key]; got != want {
			t.Fatalf("%s = %q, want %q in %s", key, got, want, payload)
		}
	}
}

func TestKafkaConnectConnectorPayloadUsesAllTablesPublicationWithoutFilters(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.KafkaConnect = &schema.KafkaConnectService{
		Enabled: schema.Bool(true),
		Connectors: []schema.DebeziumConnector{{
			Name: "app-cdc",
		}},
	}
	stack.SetDefaults()

	payload, err := kafkaConnectConnectorPayload(stack, stack.Services.KafkaConnect.Connectors[0])
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(payload), &config); err != nil {
		t.Fatal(err)
	}
	if got := config["publication.autocreate.mode"]; got != "all_tables" {
		t.Fatalf("publication.autocreate.mode = %q, want all_tables in %s", got, payload)
	}
}

func TestKafkaConnectConnectorPayloadUsesCustomSinkConfig(t *testing.T) {
	stack := testPlatformStack()
	connector := schema.KafkaConnectConnector{
		Name: "orders-sink",
		Type: "sink",
		Kind: "custom",
		Config: map[string]string{
			"connector.class": "io.confluent.connect.jdbc.JdbcSinkConnector",
			"tasks.max":       "1",
			"topics":          "app-cdc.public.orders",
		},
	}

	payload, err := kafkaConnectConnectorPayload(stack, connector)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]string
	if err := json.Unmarshal([]byte(payload), &config); err != nil {
		t.Fatal(err)
	}

	if got := config["connector.class"]; got != "io.confluent.connect.jdbc.JdbcSinkConnector" {
		t.Fatalf("connector.class = %q", got)
	}
	if got := config["topics"]; got != "app-cdc.public.orders" {
		t.Fatalf("topics = %q", got)
	}
	if _, ok := config["database.hostname"]; ok {
		t.Fatalf("custom sink payload includes Debezium fields: %s", payload)
	}
}

func TestKafkaConnectConnectorJobWaitsForHealthyTask(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.KafkaConnect = &schema.KafkaConnectService{Enabled: schema.Bool(true)}
	stack.SetDefaults()
	connector := schema.DebeziumConnector{Name: "app-cdc"}
	payload, err := kafkaConnectConnectorPayload(stack, connector)
	if err != nil {
		t.Fatal(err)
	}
	job := kafkaConnectConnectorJob(stack, connector, "app-cdc", payload)
	args := strings.Join(job.Spec.Template.Spec.Containers[0].Args, "\n")

	for _, want := range []string{"/connectors/app-cdc/status", `"state":"FAILED"`, `"state":"RUNNING"`} {
		if !strings.Contains(args, want) {
			t.Fatalf("connector job script does not contain %q:\n%s", want, args)
		}
	}
}

func TestRabbitMQServiceUsesStableNodePorts(t *testing.T) {
	svc := rabbitMQService("pyahu-local-dev", map[string]string{}, map[string]string{})

	ports := map[string]int32{}
	for _, port := range svc.Spec.Ports {
		ports[port.Name] = port.NodePort
	}
	if ports["amqp"] != nodePortRabbitMQ {
		t.Fatalf("amqp nodePort = %d", ports["amqp"])
	}
	if ports["management"] != nodePortRabbitMQManagement {
		t.Fatalf("management nodePort = %d", ports["management"])
	}
}

func TestRabbitMQDefinitionsUsePasswordHashesAndConfiguredTopology(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.RabbitMQ.Auth = schema.AuthConfig{Username: "oms", Password: "oms_local"}
	stack.Services.RabbitMQ.VHosts = []schema.RabbitMQVHost{{Name: "orders"}}
	stack.Services.RabbitMQ.Users = []schema.RabbitMQUser{{
		Name:     "worker",
		Password: "worker_local",
		Tags:     "management",
		Permissions: []schema.RabbitMQPermission{{
			VHost:     "orders",
			Configure: "^worker\\.",
			Write:     ".*",
			Read:      ".*",
		}},
	}}
	stack.SetDefaults()

	data, err := rabbitMQDefinitions(stack)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(data, "worker_local") || strings.Contains(data, "oms_local") {
		t.Fatalf("definitions should not contain plaintext passwords:\n%s", data)
	}

	var definitions rabbitMQDefinitionFile
	if err := json.Unmarshal([]byte(data), &definitions); err != nil {
		t.Fatal(err)
	}
	if len(definitions.Users) != 2 {
		t.Fatalf("users = %d, want 2: %s", len(definitions.Users), data)
	}
	if definitions.Users[0].PasswordHash == "" {
		t.Fatalf("missing password hash: %s", data)
	}
	if definitions.Users[0].HashingAlgorithm != "rabbit_password_hashing_sha256" {
		t.Fatalf("hashing algorithm = %q", definitions.Users[0].HashingAlgorithm)
	}
	if definitions.VHosts[0].Name != "orders" {
		t.Fatalf("vhost = %q", definitions.VHosts[0].Name)
	}
}

func TestRabbitMQStatefulSetAnnotatesDefinitionsChecksum(t *testing.T) {
	stack := testPlatformStack()
	first := rabbitMQStatefulSet(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "rabbitmq"}, `{"users":[]}`)
	second := rabbitMQStatefulSet(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "rabbitmq"}, `{"users":[{"name":"pyahu"}]}`)

	firstChecksum := first.Spec.Template.Annotations["pyahu.io/rabbitmq-definitions-sha256"]
	secondChecksum := second.Spec.Template.Annotations["pyahu.io/rabbitmq-definitions-sha256"]
	if firstChecksum == "" {
		t.Fatal("missing definitions checksum annotation")
	}
	if firstChecksum == secondChecksum {
		t.Fatal("definitions checksum did not change")
	}
}

func TestZitadelIngressUsesExternalDomain(t *testing.T) {
	ingress := zitadelIngress("pyahu-local-dev", map[string]string{}, "zitadel.localhost", true, "pyahu-local-tls")

	if got := ingress.Spec.Rules[0].Host; got != "zitadel.localhost" {
		t.Fatalf("host = %q", got)
	}
	if len(ingress.Spec.TLS) != 1 {
		t.Fatalf("TLS entries = %d", len(ingress.Spec.TLS))
	}
	if got := ingress.Spec.TLS[0].SecretName; got != "pyahu-local-tls" {
		t.Fatalf("TLS secret = %q", got)
	}
	if got := ingress.Annotations["traefik.ingress.kubernetes.io/service.serversscheme"]; got != "h2c" {
		t.Fatalf("scheme annotation = %q", got)
	}
}

func TestZitadelDeploymentUsesCustomPostgresCredentials(t *testing.T) {
	stack := testPlatformStack()
	stack.Services.Postgres.Auth = schema.AuthConfig{Username: "oms", Password: "oms_local"}
	deployment := zitadelDeployment(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "zitadel"}, zitadelExternal{domain: "zitadel.localhost", port: "8080"})
	container := deployment.Spec.Template.Spec.Containers[0]
	env := containerEnv(container.Env)

	got := env["ZITADEL_DATABASE_POSTGRES_DSN"]
	want := "postgresql://oms:oms_local@postgres.pyahu-local-dev.svc.cluster.local:5432/zitadel?sslmode=disable"
	if got != want {
		t.Fatalf("dsn = %q, want %q", got, want)
	}
	if env["ZITADEL_PORT"] != "8080" {
		t.Fatalf("ZITADEL_PORT = %q", env["ZITADEL_PORT"])
	}
	if env["ZITADEL_TLS_ENABLED"] != "false" {
		t.Fatalf("ZITADEL_TLS_ENABLED = %q", env["ZITADEL_TLS_ENABLED"])
	}
	if env["ZITADEL_EXTERNALSECURE"] != "false" {
		t.Fatalf("ZITADEL_EXTERNALSECURE = %q", env["ZITADEL_EXTERNALSECURE"])
	}
	if env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED"] != "false" {
		t.Fatalf("ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED = %q", env["ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED"])
	}
	if strings.Contains(strings.Join(container.Args, " "), "tlsMode") {
		t.Fatalf("unexpected tlsMode arg: %#v", container.Args)
	}
}

func TestZitadelDeploymentMarksHTTPSExternalURLSecure(t *testing.T) {
	stack := testPlatformStack()
	deployment := zitadelDeployment(stack, map[string]string{}, map[string]string{"app.kubernetes.io/name": "zitadel"}, zitadelExternal{domain: "zitadel.localhost", port: "8443", secure: true})
	container := deployment.Spec.Template.Spec.Containers[0]
	env := containerEnv(container.Env)

	if env["ZITADEL_EXTERNALSECURE"] != "true" {
		t.Fatalf("ZITADEL_EXTERNALSECURE = %q", env["ZITADEL_EXTERNALSECURE"])
	}
	if env["ZITADEL_EXTERNALPORT"] != "8443" {
		t.Fatalf("ZITADEL_EXTERNALPORT = %q", env["ZITADEL_EXTERNALPORT"])
	}
	// Login V2 is a separate app Pyahu does not deploy; the core must serve the v1 login.
	if env["ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED"] != "false" {
		t.Fatalf("ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED = %q, want false", env["ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED"])
	}
}

func testPlatformStack() *schema.Stack {
	stack := &schema.Stack{
		APIVersion: schema.APIVersion,
		Kind:       schema.Kind,
		Metadata:   schema.Metadata{Name: "pyahu-local"},
		Services: schema.Services{
			Postgres: &schema.PostgresService{Enabled: schema.Bool(true)},
			Zitadel:  &schema.ZitadelService{Enabled: schema.Bool(true)},
			RabbitMQ: &schema.RabbitMQService{Enabled: schema.Bool(true)},
			Kafka:    &schema.KafkaService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()
	return stack
}

func containerEnv(env []corev1.EnvVar) map[string]string {
	values := map[string]string{}
	for _, item := range env {
		values[item.Name] = item.Value
	}
	return values
}
