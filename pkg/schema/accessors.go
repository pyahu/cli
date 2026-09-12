package schema

import "fmt"

func (s *Stack) EnabledServices() []string {
	services := make([]string, 0, 6)
	if s.PostgresEnabled() {
		services = append(services, "postgres")
	}
	if s.ZitadelEnabled() {
		services = append(services, "zitadel")
	}
	if s.RabbitMQEnabled() {
		services = append(services, "rabbitmq")
	}
	if s.RedisEnabled() {
		services = append(services, "redis")
	}
	if s.KafkaEnabled() {
		services = append(services, "kafka")
	}
	if s.KafkaConnectEnabled() {
		services = append(services, "kafka-connect")
	}
	if s.KafkaUIEnabled() {
		services = append(services, "kafka-ui")
	}
	return services
}

func (s *Stack) PostgresEnabled() bool {
	return s.Services.Postgres != nil && enabled(s.Services.Postgres.Enabled)
}

func (s *Stack) ZitadelEnabled() bool {
	return s.Services.Zitadel != nil && enabled(s.Services.Zitadel.Enabled)
}

func (s *Stack) RabbitMQEnabled() bool {
	return s.Services.RabbitMQ != nil && enabled(s.Services.RabbitMQ.Enabled)
}

func (s *Stack) RedisEnabled() bool {
	return s.Services.Redis != nil && enabled(s.Services.Redis.Enabled)
}

func (s *Stack) KafkaEnabled() bool {
	return s.Services.Kafka != nil && enabled(s.Services.Kafka.Enabled)
}

func (s *Stack) KafkaConnectEnabled() bool {
	return s.Services.KafkaConnect != nil && enabled(s.Services.KafkaConnect.Enabled)
}

func (s *Stack) KafkaUIEnabled() bool {
	return s.Services.KafkaUI != nil && enabled(s.Services.KafkaUI.Enabled)
}

// RabbitMQManagementEnabled reports whether the management UI is served (default on).
func (s *Stack) RabbitMQManagementEnabled() bool {
	return s.RabbitMQEnabled() && (s.Services.RabbitMQ.Management == nil || *s.Services.RabbitMQ.Management)
}

// HTTPIngressEnabled reports whether any service is exposed through Traefik on the
// shared host entrypoints (80/443). TCP services are exposed via NodePort instead.
func (s *Stack) HTTPIngressEnabled() bool {
	return s.ZitadelEnabled() || s.KafkaUIEnabled() || s.RabbitMQManagementEnabled()
}

func (s *Stack) localHTTPScheme() string {
	if s.LocalTLSEnabled() {
		return "https"
	}
	return "http"
}

func (s *Stack) KafkaUIExternalURL() string {
	return s.localHTTPScheme() + "://kafka-ui.localhost"
}

func (s *Stack) RabbitMQManagementExternalURL() string {
	return s.localHTTPScheme() + "://rabbitmq.localhost"
}

func (s *Stack) LocalTLSEnabled() bool {
	return enabled(s.LocalTLS.Enabled)
}

func (s *Stack) LocalTLSRequired() bool {
	return s.LocalTLSEnabled() && s.HTTPIngressEnabled()
}

func (s *Stack) LocalTLSDomains() []string {
	return normalizeLocalTLSDomains(s.LocalTLS.Domains)
}

func (s *Stack) LocalTLSSecretName() string {
	if s.LocalTLS.SecretName == "" {
		return DefaultLocalTLSSecretName
	}
	return s.LocalTLS.SecretName
}

func (s *Stack) LocalTLSCAConfigMapName() string {
	if s.LocalTLS.CAConfigMapName == "" {
		return DefaultLocalTLSCAConfigMap
	}
	return s.LocalTLS.CAConfigMapName
}

func DefaultLocalTLSDomains() []string {
	// *.localhost covers every single-label subdomain (zitadel.localhost,
	// kafka-ui.localhost, ...). localhost is kept because the wildcard does not
	// match the bare host.
	return []string{"localhost", "*.localhost"}
}

func (s *Stack) ZitadelHTTPPort() int {
	if s.Services.Zitadel == nil || s.Services.Zitadel.Ports.HTTP == 0 {
		return DefaultZitadelHTTPPort
	}
	return s.Services.Zitadel.Ports.HTTP
}

func (s *Stack) ZitadelHTTPSPort() int {
	if s.Services.Zitadel == nil || s.Services.Zitadel.Ports.HTTPS == 0 {
		return DefaultZitadelHTTPSPort
	}
	return s.Services.Zitadel.Ports.HTTPS
}

func (s *Stack) PostgresPort() int {
	if s.Services.Postgres == nil || s.Services.Postgres.Ports.Primary == 0 {
		return DefaultPostgresPort
	}
	return s.Services.Postgres.Ports.Primary
}

func (s *Stack) PostgresReadPort() int {
	if s.Services.Postgres == nil || s.Services.Postgres.Ports.Read == 0 {
		return DefaultPostgresReadPort
	}
	return s.Services.Postgres.Ports.Read
}

func (s *Stack) RabbitMQPort() int {
	if s.Services.RabbitMQ == nil || s.Services.RabbitMQ.Ports.AMQP == 0 {
		return DefaultRabbitMQPort
	}
	return s.Services.RabbitMQ.Ports.AMQP
}

func (s *Stack) RabbitMQManagementPort() int {
	if s.Services.RabbitMQ == nil || s.Services.RabbitMQ.Ports.Management == 0 {
		return DefaultRabbitMQMgmtPort
	}
	return s.Services.RabbitMQ.Ports.Management
}

func (s *Stack) RedisPort() int {
	if s.Services.Redis == nil || s.Services.Redis.Ports.Client == 0 {
		return DefaultRedisPort
	}
	return s.Services.Redis.Ports.Client
}

func (s *Stack) RedisPassword() string {
	if s.Services.Redis == nil {
		return ""
	}
	return s.Services.Redis.Auth.Password
}

// RedisAppendOnly reports whether the server runs with AOF persistence (default on).
func (s *Stack) RedisAppendOnly() bool {
	return s.Services.Redis == nil || s.Services.Redis.AppendOnly == nil || *s.Services.Redis.AppendOnly
}

func (s *Stack) RedisImage() string {
	image := DefaultRedisImage
	version := DefaultRedisVersion
	if s.Services.Redis != nil {
		if s.Services.Redis.Image != "" {
			image = s.Services.Redis.Image
		}
		if s.Services.Redis.Version != "" {
			version = s.Services.Redis.Version
		}
	}
	return image + ":" + version
}

// RedisInternalHost is the in-cluster DNS name other workloads (Kafka Connect,
// for example) use to reach Redis; the host port is only for the developer machine.
func (s *Stack) RedisInternalHost() string {
	return fmt.Sprintf("redis.%s.svc.cluster.local", s.Cluster.Namespace)
}

func (s *Stack) RedisURL() string {
	if password := s.RedisPassword(); password != "" {
		return credentialURL("redis", "", password, "localhost", s.RedisPort(), "", "")
	}
	return fmt.Sprintf("redis://localhost:%d", s.RedisPort())
}

func (s *Stack) KafkaPort() int {
	if s.Services.Kafka == nil || s.Services.Kafka.Ports.Bootstrap == 0 {
		return DefaultKafkaPort
	}
	return s.Services.Kafka.Ports.Bootstrap
}

func (s *Stack) KafkaConnectPort() int {
	if s.Services.KafkaConnect == nil || s.Services.KafkaConnect.Ports.REST == 0 {
		return DefaultKafkaConnectPort
	}
	return s.Services.KafkaConnect.Ports.REST
}

func (s *Stack) KafkaUIPort() int {
	if s.Services.KafkaUI == nil || s.Services.KafkaUI.Ports.HTTP == 0 {
		return DefaultKafkaUIPort
	}
	return s.Services.KafkaUI.Ports.HTTP
}

func (s *Stack) ZitadelAdminUser() string {
	if s.Services.Zitadel == nil || s.Services.Zitadel.Admin.Username == "" {
		return DefaultZitadelAdminUser
	}
	return s.Services.Zitadel.Admin.Username
}

func (s *Stack) PostgresUser() string {
	if s.Services.Postgres == nil || s.Services.Postgres.Auth.Username == "" {
		return DefaultPostgresUser
	}
	return s.Services.Postgres.Auth.Username
}

func (s *Stack) PostgresPassword() string {
	if s.Services.Postgres == nil || s.Services.Postgres.Auth.Password == "" {
		return DefaultPostgresPassword
	}
	return s.Services.Postgres.Auth.Password
}

func (s *Stack) PostgresReadReplicas() int {
	if s.Services.Postgres == nil {
		return 0
	}
	return s.Services.Postgres.ReadReplicas
}

func (s *Stack) PostgresReplicationUser() string {
	if s.Services.Postgres == nil || s.Services.Postgres.Replication.Username == "" {
		return DefaultPostgresReplUser
	}
	return s.Services.Postgres.Replication.Username
}

func (s *Stack) PostgresReplicationPassword() string {
	if s.Services.Postgres == nil || s.Services.Postgres.Replication.Password == "" {
		return DefaultPostgresReplPass
	}
	return s.Services.Postgres.Replication.Password
}

func (s *Stack) RabbitMQUser() string {
	if s.Services.RabbitMQ == nil || s.Services.RabbitMQ.Auth.Username == "" {
		return DefaultRabbitMQUser
	}
	return s.Services.RabbitMQ.Auth.Username
}

func (s *Stack) RabbitMQPassword() string {
	if s.Services.RabbitMQ == nil || s.Services.RabbitMQ.Auth.Password == "" {
		return DefaultRabbitMQPassword
	}
	return s.Services.RabbitMQ.Auth.Password
}

func (s *Stack) ZitadelAdminPassword() string {
	if s.Services.Zitadel == nil || s.Services.Zitadel.Admin.Password == "" {
		return DefaultZitadelPassword
	}
	return s.Services.Zitadel.Admin.Password
}

func (s *Stack) ZitadelMasterKey() string {
	if s.Services.Zitadel == nil || s.Services.Zitadel.MasterKey == "" {
		return DefaultZitadelMasterKey
	}
	return s.Services.Zitadel.MasterKey
}

func (s *Stack) KafkaInternalBootstrapServers() string {
	return fmt.Sprintf("kafka.%s.svc.cluster.local:9092", s.Cluster.Namespace)
}

func (s *Stack) KafkaConnectImage() string {
	image := DefaultKafkaConnectImage
	version := DefaultKafkaConnectVersion
	if s.Services.KafkaConnect != nil {
		if s.Services.KafkaConnect.Image != "" {
			image = s.Services.KafkaConnect.Image
		}
		if s.Services.KafkaConnect.Version != "" {
			version = s.Services.KafkaConnect.Version
		}
	}
	return image + ":" + version
}

func (s *Stack) KafkaUIImage() string {
	image := DefaultKafkaUIImage
	version := DefaultKafkaUIVersion
	if s.Services.KafkaUI != nil {
		if s.Services.KafkaUI.Image != "" {
			image = s.Services.KafkaUI.Image
		}
		if s.Services.KafkaUI.Version != "" {
			version = s.Services.KafkaUI.Version
		}
	}
	return image + ":" + version
}

func (s *Stack) KafkaConnectInternalURL() string {
	return fmt.Sprintf("http://kafka-connect.%s.svc.cluster.local:8083", s.Cluster.Namespace)
}

// PluginArtifactFormat reports how the installer unpacks an artifact: "zip",
// "tar.gz", or "jar".
