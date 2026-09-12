package schema

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func (s *Stack) ConnectionEnv() map[string]string {
	env := map[string]string{}
	if s.PostgresEnabled() {
		db := s.Services.Postgres.Databases[0].Name
		env["POSTGRES_HOST"] = "localhost"
		env["POSTGRES_PORT"] = fmt.Sprintf("%d", s.PostgresPort())
		env["POSTGRES_DATABASE"] = db
		env["POSTGRES_USER"] = s.PostgresUser()
		env["POSTGRES_PASSWORD"] = s.PostgresPassword()
		env["POSTGRES_URL"] = s.postgresURL("localhost", s.PostgresPort(), db)
		if s.PostgresReadReplicas() > 0 {
			env["POSTGRES_READ_HOST"] = "localhost"
			env["POSTGRES_READ_PORT"] = fmt.Sprintf("%d", s.PostgresReadPort())
			env["POSTGRES_READ_URL"] = s.postgresURL("localhost", s.PostgresReadPort(), db)
		}
	}
	if s.ZitadelEnabled() {
		env["ZITADEL_ISSUER"] = s.Services.Zitadel.ExternalURL
		env["ZITADEL_CONSOLE_URL"] = strings.TrimRight(s.Services.Zitadel.ExternalURL, "/") + "/ui/console"
		env["ZITADEL_ADMIN_USER"] = s.Services.Zitadel.Admin.Username
		env["ZITADEL_ADMIN_PASSWORD"] = s.ZitadelAdminPassword()
	}
	if s.RabbitMQEnabled() {
		env["RABBITMQ_HOST"] = "localhost"
		env["RABBITMQ_PORT"] = fmt.Sprintf("%d", s.RabbitMQPort())
		env["RABBITMQ_MANAGEMENT_URL"] = s.RabbitMQManagementExternalURL()
		env["RABBITMQ_USER"] = s.RabbitMQUser()
		env["RABBITMQ_PASSWORD"] = s.RabbitMQPassword()
		env["RABBITMQ_URL"] = credentialURL("amqp", s.RabbitMQUser(), s.RabbitMQPassword(), "localhost", s.RabbitMQPort(), "", "")
	}
	if s.RedisEnabled() {
		env["REDIS_HOST"] = "localhost"
		env["REDIS_PORT"] = fmt.Sprintf("%d", s.RedisPort())
		env["REDIS_PASSWORD"] = s.RedisPassword()
		env["REDIS_URL"] = s.RedisURL()
	}
	if s.KafkaEnabled() {
		env["KAFKA_BOOTSTRAP_SERVERS"] = fmt.Sprintf("localhost:%d", s.KafkaPort())
	}
	if s.KafkaConnectEnabled() {
		env["KAFKA_CONNECT_URL"] = fmt.Sprintf("http://localhost:%d", s.KafkaConnectPort())
	}
	if s.KafkaUIEnabled() {
		env["KAFKA_UI_URL"] = s.KafkaUIExternalURL()
	}
	return env
}

func (s *Stack) postgresURL(host string, port int, database string) string {
	return credentialURL("postgresql", s.PostgresUser(), s.PostgresPassword(), host, port, database, "sslmode=disable")
}

func (s *Stack) PostgresInternalURL(database string) string {
	return s.postgresURL(fmt.Sprintf("postgres.%s.svc.cluster.local", s.Cluster.Namespace), 5432, database)
}

func credentialURL(scheme string, username string, password string, host string, port int, pathValue string, rawQuery string) string {
	value := &url.URL{
		Scheme:   scheme,
		User:     url.UserPassword(username, password),
		Host:     net.JoinHostPort(host, fmt.Sprintf("%d", port)),
		RawQuery: rawQuery,
	}
	if pathValue != "" {
		value.Path = "/" + pathValue
	}
	return value.String()
}
