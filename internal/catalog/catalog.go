package catalog

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/pyahu/cli/internal/kube"
	"github.com/pyahu/cli/pkg/schema"
)

type Service struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Enabled     bool              `json:"enabled"`
	Ready       bool              `json:"ready"`
	Status      string            `json:"status"`
	Message     string            `json:"message,omitempty"`
	Version     string            `json:"version,omitempty"`
	Workload    string            `json:"workload,omitempty"`
	Namespace   string            `json:"namespace"`
	Endpoints   []Endpoint        `json:"endpoints,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	Pods        []kube.PodStatus  `json:"pods,omitempty"`
}

type Endpoint struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"`
	URL      string `json:"url,omitempty"`
	Internal string `json:"internal,omitempty"`
}

func Build(stack *schema.Stack, statuses []kube.ServiceStatus, clusterRunning bool) []Service {
	statusByName := map[string]kube.ServiceStatus{}
	for _, status := range statuses {
		statusByName[status.Name] = status
	}

	services := []Service{
		postgres(stack, statusByName["postgres"], clusterRunning),
		zitadel(stack, statusByName["zitadel"], clusterRunning),
		rabbitmq(stack, statusByName["rabbitmq"], clusterRunning),
		kafka(stack, statusByName["kafka"], clusterRunning),
		kafkaConnect(stack, statusByName["kafka-connect"], clusterRunning),
		kafkaUI(stack, statusByName["kafka-ui"], clusterRunning),
	}
	return services
}

func EnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func DetailKeys(details map[string]string) []string {
	keys := make([]string, 0, len(details))
	for key := range details {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func statusFor(enabled bool, status kube.ServiceStatus, clusterRunning bool) (bool, string, string, []kube.PodStatus) {
	if !enabled {
		return false, "disabled", "disabled in stack file", nil
	}
	if !clusterRunning {
		return false, "stopped", "cluster is not running", nil
	}
	if status.Ready {
		return true, "ready", status.Message, status.Pods
	}
	if status.Message != "" {
		return false, "waiting", status.Message, status.Pods
	}
	return false, "waiting", "waiting for pods", status.Pods
}

func postgres(stack *schema.Stack, status kube.ServiceStatus, clusterRunning bool) Service {
	ready, state, message, pods := statusFor(stack.PostgresEnabled(), status, clusterRunning)
	env := stack.ConnectionEnv()
	details := map[string]string{}
	version := ""
	if stack.Services.Postgres != nil {
		version = stack.Services.Postgres.Version
		details["authUser"] = stack.PostgresUser()
		details["instances"] = fmt.Sprintf("%d", stack.Services.Postgres.Instances)
		details["readReplicas"] = fmt.Sprintf("%d", stack.PostgresReadReplicas())
		details["storage"] = stack.Services.Postgres.Storage
		details["databases"] = postgresDatabases(stack.Services.Postgres.Databases)
	}
	endpoints := []Endpoint{{
		Name:     "primary",
		Protocol: "tcp",
		Host:     "localhost",
		Port:     stack.PostgresPort(),
		URL:      env["POSTGRES_URL"],
		Internal: fmt.Sprintf("postgres.%s.svc.cluster.local:5432", stack.Cluster.Namespace),
	}}
	if stack.PostgresReadReplicas() > 0 {
		endpoints = append(endpoints, Endpoint{
			Name:     "read",
			Protocol: "tcp",
			Host:     "localhost",
			Port:     stack.PostgresReadPort(),
			URL:      env["POSTGRES_READ_URL"],
			Internal: fmt.Sprintf("postgres-read.%s.svc.cluster.local:5432", stack.Cluster.Namespace),
		})
	}
	return Service{
		Name:        "postgres",
		DisplayName: "PostgreSQL",
		Enabled:     stack.PostgresEnabled(),
		Ready:       ready,
		Status:      state,
		Message:     message,
		Version:     version,
		Workload:    "StatefulSet/postgres",
		Namespace:   stack.Cluster.Namespace,
		Endpoints:   endpoints,
		Env: selectEnv(env,
			"POSTGRES_HOST",
			"POSTGRES_PORT",
			"POSTGRES_DATABASE",
			"POSTGRES_USER",
			"POSTGRES_PASSWORD",
			"POSTGRES_URL",
			"POSTGRES_READ_HOST",
			"POSTGRES_READ_PORT",
			"POSTGRES_READ_URL",
		),
		Details: details,
		Pods:    pods,
	}
}

func zitadel(stack *schema.Stack, status kube.ServiceStatus, clusterRunning bool) Service {
	ready, state, message, pods := statusFor(stack.ZitadelEnabled(), status, clusterRunning)
	env := stack.ConnectionEnv()
	version := ""
	details := map[string]string{}
	endpoint := parsedEndpoint{Scheme: "http", Host: "zitadel.localhost", Port: stack.ZitadelHTTPPort()}
	if stack.Services.Zitadel != nil {
		version = stack.Services.Zitadel.Version
		details["databaseRef"] = stack.Services.Zitadel.DatabaseRef
		details["adminUser"] = stack.ZitadelAdminUser()
		endpoint = parseEndpoint(stack.Services.Zitadel.ExternalURL, endpoint)
	}
	return Service{
		Name:        "zitadel",
		DisplayName: "ZITADEL",
		Enabled:     stack.ZitadelEnabled(),
		Ready:       ready,
		Status:      state,
		Message:     message,
		Version:     version,
		Workload:    "Deployment/zitadel",
		Namespace:   stack.Cluster.Namespace,
		Endpoints: []Endpoint{{
			Name:     endpoint.Scheme,
			Protocol: endpoint.Scheme,
			Host:     endpoint.Host,
			Port:     endpoint.Port,
			URL:      env["ZITADEL_ISSUER"],
			Internal: fmt.Sprintf("zitadel.%s.svc.cluster.local:8080", stack.Cluster.Namespace),
		}, {
			Name:     "console",
			Protocol: endpoint.Scheme,
			Host:     endpoint.Host,
			Port:     endpoint.Port,
			URL:      env["ZITADEL_CONSOLE_URL"],
			Internal: fmt.Sprintf("zitadel.%s.svc.cluster.local:8080/ui/console", stack.Cluster.Namespace),
		}},
		Env: selectEnv(env,
			"ZITADEL_ISSUER",
			"ZITADEL_CONSOLE_URL",
			"ZITADEL_ADMIN_USER",
			"ZITADEL_ADMIN_PASSWORD",
		),
		Details: details,
		Pods:    pods,
	}
}

type parsedEndpoint struct {
	Scheme string
	Host   string
	Port   int
}

func parseEndpoint(rawURL string, fallback parsedEndpoint) parsedEndpoint {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fallback
	}
	endpoint := fallback
	if parsed.Scheme != "" {
		endpoint.Scheme = parsed.Scheme
	}
	if parsed.Hostname() != "" {
		endpoint.Host = parsed.Hostname()
	}
	if parsed.Port() != "" {
		port, err := strconv.Atoi(parsed.Port())
		if err == nil {
			endpoint.Port = port
		}
	} else if endpoint.Scheme == "https" {
		endpoint.Port = 443
	} else if endpoint.Scheme == "http" {
		endpoint.Port = 80
	}
	return endpoint
}

func rabbitmq(stack *schema.Stack, status kube.ServiceStatus, clusterRunning bool) Service {
	ready, state, message, pods := statusFor(stack.RabbitMQEnabled(), status, clusterRunning)
	env := stack.ConnectionEnv()
	version := ""
	details := map[string]string{}
	if stack.Services.RabbitMQ != nil {
		version = stack.Services.RabbitMQ.Version
		details["replicas"] = fmt.Sprintf("%d", stack.Services.RabbitMQ.Replicas)
		details["storage"] = stack.Services.RabbitMQ.Storage
		details["management"] = fmt.Sprintf("%t", stack.Services.RabbitMQ.Management == nil || *stack.Services.RabbitMQ.Management)
		details["authUser"] = stack.RabbitMQUser()
		details["vhosts"] = rabbitMQVHosts(stack.Services.RabbitMQ.VHosts)
		details["users"] = rabbitMQUsers(stack.Services.RabbitMQ.Users)
	}
	return Service{
		Name:        "rabbitmq",
		DisplayName: "RabbitMQ",
		Enabled:     stack.RabbitMQEnabled(),
		Ready:       ready,
		Status:      state,
		Message:     message,
		Version:     version,
		Workload:    "StatefulSet/rabbitmq",
		Namespace:   stack.Cluster.Namespace,
		Endpoints: []Endpoint{{
			Name:     "amqp",
			Protocol: "amqp",
			Host:     "localhost",
			Port:     stack.RabbitMQPort(),
			URL:      env["RABBITMQ_URL"],
			Internal: fmt.Sprintf("rabbitmq.%s.svc.cluster.local:5672", stack.Cluster.Namespace),
		}, {
			Name:     "management",
			Protocol: schemeName(env["RABBITMQ_MANAGEMENT_URL"]),
			Host:     "rabbitmq.localhost",
			URL:      env["RABBITMQ_MANAGEMENT_URL"],
			Internal: fmt.Sprintf("rabbitmq.%s.svc.cluster.local:15672", stack.Cluster.Namespace),
		}},
		Env: selectEnv(env,
			"RABBITMQ_HOST",
			"RABBITMQ_PORT",
			"RABBITMQ_MANAGEMENT_URL",
			"RABBITMQ_USER",
			"RABBITMQ_PASSWORD",
			"RABBITMQ_URL",
		),
		Details: details,
		Pods:    pods,
	}
}

func kafka(stack *schema.Stack, status kube.ServiceStatus, clusterRunning bool) Service {
	ready, state, message, pods := statusFor(stack.KafkaEnabled(), status, clusterRunning)
	env := stack.ConnectionEnv()
	version := ""
	details := map[string]string{}
	if stack.Services.Kafka != nil {
		version = stack.Services.Kafka.Version
		details["replicas"] = fmt.Sprintf("%d", stack.Services.Kafka.Replicas)
		details["storage"] = stack.Services.Kafka.Storage
		details["topics"] = kafkaTopics(stack.Services.Kafka.Topics)
	}
	return Service{
		Name:        "kafka",
		DisplayName: "Kafka",
		Enabled:     stack.KafkaEnabled(),
		Ready:       ready,
		Status:      state,
		Message:     message,
		Version:     version,
		Workload:    "StatefulSet/kafka",
		Namespace:   stack.Cluster.Namespace,
		Endpoints: []Endpoint{{
			Name:     "bootstrap",
			Protocol: "tcp",
			Host:     "localhost",
			Port:     stack.KafkaPort(),
			URL:      env["KAFKA_BOOTSTRAP_SERVERS"],
			Internal: fmt.Sprintf("kafka.%s.svc.cluster.local:9092", stack.Cluster.Namespace),
		}},
		Env:     selectEnv(env, "KAFKA_BOOTSTRAP_SERVERS"),
		Details: details,
		Pods:    pods,
	}
}

func kafkaConnect(stack *schema.Stack, status kube.ServiceStatus, clusterRunning bool) Service {
	ready, state, message, pods := statusFor(stack.KafkaConnectEnabled(), status, clusterRunning)
	env := stack.ConnectionEnv()
	version := ""
	details := map[string]string{}
	if stack.Services.KafkaConnect != nil {
		version = stack.Services.KafkaConnect.Version
		details["image"] = stack.KafkaConnectImage()
		details["replicas"] = fmt.Sprintf("%d", stack.Services.KafkaConnect.Replicas)
		details["bootstrapServers"] = stack.KafkaInternalBootstrapServers()
		details["configTopic"] = stack.KafkaConnectConfigTopic()
		details["offsetTopic"] = stack.KafkaConnectOffsetTopic()
		details["statusTopic"] = stack.KafkaConnectStatusTopic()
		details["connectors"] = kafkaConnectConnectors(stack.Services.KafkaConnect.Connectors)
	}
	return Service{
		Name:        "kafka-connect",
		DisplayName: "Kafka Connect",
		Enabled:     stack.KafkaConnectEnabled(),
		Ready:       ready,
		Status:      state,
		Message:     message,
		Version:     version,
		Workload:    "Deployment/kafka-connect",
		Namespace:   stack.Cluster.Namespace,
		Endpoints: []Endpoint{{
			Name:     "rest",
			Protocol: "http",
			Host:     "localhost",
			Port:     stack.KafkaConnectPort(),
			URL:      env["KAFKA_CONNECT_URL"],
			Internal: fmt.Sprintf("kafka-connect.%s.svc.cluster.local:8083", stack.Cluster.Namespace),
		}},
		Env:     selectEnv(env, "KAFKA_CONNECT_URL"),
		Details: details,
		Pods:    pods,
	}
}

func kafkaUI(stack *schema.Stack, status kube.ServiceStatus, clusterRunning bool) Service {
	ready, state, message, pods := statusFor(stack.KafkaUIEnabled(), status, clusterRunning)
	env := stack.ConnectionEnv()
	version := ""
	details := map[string]string{}
	if stack.Services.KafkaUI != nil {
		version = stack.Services.KafkaUI.Version
		details["image"] = stack.KafkaUIImage()
		details["replicas"] = fmt.Sprintf("%d", stack.Services.KafkaUI.Replicas)
		details["bootstrapServers"] = stack.KafkaInternalBootstrapServers()
		if stack.KafkaConnectEnabled() {
			details["kafkaConnect"] = stack.KafkaConnectInternalURL()
		}
	}
	return Service{
		Name:        "kafka-ui",
		DisplayName: "Kafka UI",
		Enabled:     stack.KafkaUIEnabled(),
		Ready:       ready,
		Status:      state,
		Message:     message,
		Version:     version,
		Workload:    "Deployment/kafka-ui",
		Namespace:   stack.Cluster.Namespace,
		Endpoints: []Endpoint{{
			Name:     schemeName(env["KAFKA_UI_URL"]),
			Protocol: schemeName(env["KAFKA_UI_URL"]),
			Host:     "kafka-ui.localhost",
			URL:      env["KAFKA_UI_URL"],
			Internal: fmt.Sprintf("kafka-ui.%s.svc.cluster.local:8080", stack.Cluster.Namespace),
		}},
		Env:     selectEnv(env, "KAFKA_UI_URL"),
		Details: details,
		Pods:    pods,
	}
}

func selectEnv(env map[string]string, keys ...string) map[string]string {
	selected := map[string]string{}
	for _, key := range keys {
		if value, ok := env[key]; ok {
			selected[key] = value
		}
	}
	return selected
}

func postgresDatabases(databases []schema.DatabaseConfig) string {
	if len(databases) == 0 {
		return ""
	}
	names := make([]string, 0, len(databases))
	for _, database := range databases {
		if database.Owner != "" {
			names = append(names, fmt.Sprintf("%s(owner=%s)", database.Name, database.Owner))
			continue
		}
		names = append(names, database.Name)
	}
	return join(names)
}

func rabbitMQVHosts(vhosts []schema.RabbitMQVHost) string {
	if len(vhosts) == 0 {
		return ""
	}
	names := make([]string, 0, len(vhosts))
	for _, vhost := range vhosts {
		names = append(names, vhost.Name)
	}
	return join(names)
}

func rabbitMQUsers(users []schema.RabbitMQUser) string {
	if len(users) == 0 {
		return ""
	}
	names := make([]string, 0, len(users))
	for _, user := range users {
		permissions := make([]string, 0, len(user.Permissions))
		for _, permission := range user.Permissions {
			permissions = append(permissions, permission.VHost)
		}
		if len(permissions) == 0 {
			names = append(names, user.Name)
			continue
		}
		names = append(names, fmt.Sprintf("%s(vhosts=%s)", user.Name, strings.Join(permissions, ",")))
	}
	return join(names)
}

func kafkaTopics(topics []schema.TopicConfig) string {
	if len(topics) == 0 {
		return ""
	}
	names := make([]string, 0, len(topics))
	for _, topic := range topics {
		names = append(names, fmt.Sprintf("%s(partitions=%d,replicas=%d)", topic.Name, topic.Partitions, topic.Replicas))
	}
	return join(names)
}

func kafkaConnectConnectors(connectors []schema.KafkaConnectConnector) string {
	if len(connectors) == 0 {
		return ""
	}
	names := make([]string, 0, len(connectors))
	for _, connector := range connectors {
		if connector.Kind == "debezium.postgres" {
			names = append(names, fmt.Sprintf("%s(%s:%s:%s)", connector.Name, connector.Type, connector.Kind, connector.Database))
			continue
		}
		names = append(names, fmt.Sprintf("%s(%s:%s)", connector.Name, connector.Type, connector.Kind))
	}
	return join(names)
}

func join(values []string) string {
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func schemeName(rawURL string) string {
	if strings.HasPrefix(rawURL, "https") {
		return "https"
	}
	return "http"
}
