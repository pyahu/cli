package schema

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

func (s *Stack) Validate() error {
	var errs []string
	if s.APIVersion != APIVersion {
		errs = append(errs, fmt.Sprintf("apiVersion must be %q", APIVersion))
	}
	if s.Kind != Kind {
		errs = append(errs, fmt.Sprintf("kind must be %q", Kind))
	}
	if s.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	} else if !dnsLabelRE.MatchString(s.Metadata.Name) || len(s.Metadata.Name) > 48 {
		errs = append(errs, "metadata.name must be a DNS label up to 48 characters")
	}
	if s.Cluster.Runtime != DefaultRuntime {
		errs = append(errs, "cluster.runtime currently only supports k3d")
	}
	if s.Cluster.Name == "" {
		errs = append(errs, "cluster.name is required")
	} else if !dnsLabelRE.MatchString(s.Cluster.Name) {
		errs = append(errs, "cluster.name must be a DNS label")
	}
	if s.Cluster.Namespace == "" {
		errs = append(errs, "cluster.namespace is required")
	} else if !dnsLabelRE.MatchString(s.Cluster.Namespace) {
		errs = append(errs, "cluster.namespace must be a DNS label")
	}
	if s.Cluster.Servers < 1 {
		errs = append(errs, "cluster.servers must be at least 1")
	}
	if s.Cluster.Agents < 0 {
		errs = append(errs, "cluster.agents cannot be negative")
	}
	if s.LocalTLSEnabled() {
		if !dnsLabelRE.MatchString(s.LocalTLSSecretName()) {
			errs = append(errs, "localTLS.secretName must be a DNS label")
		}
		if !dnsLabelRE.MatchString(s.LocalTLSCAConfigMapName()) {
			errs = append(errs, "localTLS.caConfigMapName must be a DNS label")
		}
		for i, domain := range s.LocalTLSDomains() {
			if !validLocalTLSDomain(domain) {
				errs = append(errs, fmt.Sprintf("localTLS.domains[%d] must be localhost, *.localhost, or a .localhost name", i))
			}
		}
	}
	if portErrs := validatePorts(s.enabledHostPorts()); len(portErrs) > 0 {
		errs = append(errs, portErrs...)
	}
	if len(s.EnabledServices()) == 0 {
		errs = append(errs, "at least one service must be enabled")
	}
	if s.ZitadelEnabled() && !s.PostgresEnabled() {
		errs = append(errs, "services.zitadel requires services.postgres")
	}
	if s.KafkaConnectEnabled() && !s.KafkaEnabled() {
		errs = append(errs, "services.kafkaConnect requires services.kafka")
	}
	if s.KafkaUIEnabled() && !s.KafkaEnabled() {
		errs = append(errs, "services.kafkaUI requires services.kafka")
	}
	if s.PostgresEnabled() {
		if !dbNameRE.MatchString(s.PostgresUser()) {
			errs = append(errs, "services.postgres.auth.username must be a PostgreSQL identifier")
		}
		if strings.TrimSpace(s.PostgresPassword()) == "" {
			errs = append(errs, "services.postgres.auth.password is required")
		}
		if s.Services.Postgres.Instances < 1 {
			errs = append(errs, "services.postgres.instances must be at least 1")
		}
		if s.Services.Postgres.Instances != 1 {
			errs = append(errs, "services.postgres.instances currently must be 1; use services.postgres.readReplicas for read scaling")
		}
		if s.Services.Postgres.ReadReplicas < 0 {
			errs = append(errs, "services.postgres.readReplicas cannot be negative")
		}
		if s.Services.Postgres.ReadReplicas > 0 {
			if !dbNameRE.MatchString(s.PostgresReplicationUser()) {
				errs = append(errs, "services.postgres.replication.username must be a PostgreSQL identifier")
			}
			if strings.TrimSpace(s.PostgresReplicationPassword()) == "" {
				errs = append(errs, "services.postgres.replication.password is required when readReplicas is enabled")
			}
		}
		for i, db := range s.Services.Postgres.Databases {
			if !dbNameRE.MatchString(db.Name) {
				errs = append(errs, fmt.Sprintf("services.postgres.databases[%d].name must be a PostgreSQL identifier", i))
			}
			if db.Owner != "" && !dbNameRE.MatchString(db.Owner) {
				errs = append(errs, fmt.Sprintf("services.postgres.databases[%d].owner must be a PostgreSQL identifier", i))
			}
			if db.Seed != "" && filepath.IsAbs(db.Seed) {
				errs = append(errs, fmt.Sprintf("services.postgres.databases[%d].seed must be relative to the stack file", i))
			}
		}
	}
	if s.RedisEnabled() {
		if strings.TrimSpace(s.Services.Redis.Image) == "" {
			errs = append(errs, "services.redis.image is required")
		}
		if strings.TrimSpace(s.Services.Redis.Version) == "" {
			errs = append(errs, "services.redis.version is required")
		}
		if _, err := resource.ParseQuantity(s.Services.Redis.Storage); err != nil {
			errs = append(errs, "services.redis.storage must be a Kubernetes quantity")
		}
	}
	if s.KafkaEnabled() {
		if s.Services.Kafka.Replicas < 1 {
			errs = append(errs, "services.kafka.replicas must be at least 1")
		}
		if s.Services.Kafka.Replicas != 1 {
			errs = append(errs, "services.kafka.replicas currently must be 1; multi-broker KRaft is not supported yet")
		}
		for i, topic := range s.Services.Kafka.Topics {
			if !topicNameRE.MatchString(topic.Name) {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].name contains unsupported characters", i))
			}
			if topic.Partitions < 1 {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].partitions must be at least 1", i))
			}
			if topic.Replicas < 1 {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].replicas must be at least 1", i))
			} else if topic.Replicas > s.Services.Kafka.Replicas {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].replicas cannot exceed services.kafka.replicas", i))
			}
		}
	}
	if s.KafkaConnectEnabled() {
		if strings.TrimSpace(s.Services.KafkaConnect.Image) == "" {
			errs = append(errs, "services.kafkaConnect.image is required")
		}
		if strings.TrimSpace(s.Services.KafkaConnect.Version) == "" {
			errs = append(errs, "services.kafkaConnect.version is required")
		}
		if s.Services.KafkaConnect.Replicas < 1 {
			errs = append(errs, "services.kafkaConnect.replicas must be at least 1")
		}
		for i, plugin := range s.Services.KafkaConnect.Plugins {
			if !dnsLabelRE.MatchString(plugin.Name) {
				errs = append(errs, fmt.Sprintf("services.kafkaConnect.plugins[%d].name must be a DNS label", i))
			}
			hasURL := strings.TrimSpace(plugin.URL) != ""
			hasFile := strings.TrimSpace(plugin.File) != ""
			switch {
			case hasURL == hasFile:
				errs = append(errs, fmt.Sprintf("services.kafkaConnect.plugins[%d] must define exactly one of url or file", i))
			case hasURL:
				// An unverified download would silently change what the worker runs.
				if !sha256RE.MatchString(plugin.SHA256) {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.plugins[%d].sha256 is required with url and must be 64 lowercase hex characters", i))
				}
			case hasFile:
				if plugin.SHA256 != "" && !sha256RE.MatchString(plugin.SHA256) {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.plugins[%d].sha256 must be 64 lowercase hex characters", i))
				}
				if filepath.IsAbs(plugin.File) {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.plugins[%d].file must be relative to the stack file", i))
				}
			}
			if hasURL || hasFile {
				if _, err := PluginArtifactFormat(plugin); err != nil {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.plugins[%d]: %v", i, err))
				}
			}
		}
		names := map[string]bool{}
		for i, connector := range s.Services.KafkaConnect.Connectors {
			if !dnsLabelRE.MatchString(connector.Name) {
				errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].name must be a DNS label", i))
			} else if names[connector.Name] {
				errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].name duplicates another connector", i))
			} else {
				names[connector.Name] = true
			}
			if connector.Type != "source" && connector.Type != "sink" {
				errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].type must be source or sink", i))
			}
			switch connector.Kind {
			case "debezium.postgres":
				if connector.Type != "source" {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].kind debezium.postgres must use type source", i))
				}
				if !s.PostgresEnabled() {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d] requires services.postgres", i))
				}
				if !dbNameRE.MatchString(connector.Database) {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].database must be a PostgreSQL identifier", i))
				}
				if strings.TrimSpace(connector.TopicPrefix) == "" {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].topicPrefix is required", i))
				}
				if !dbNameRE.MatchString(connector.Slot) {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].slot must be a PostgreSQL identifier", i))
				}
				if !dbNameRE.MatchString(connector.Publication) {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].publication must be a PostgreSQL identifier", i))
				}
				if len(connector.Tables.Include) > 0 && len(connector.Tables.Exclude) > 0 {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].tables cannot define both include and exclude", i))
				}
			case "custom":
				if strings.TrimSpace(connector.Config["connector.class"]) == "" {
					errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].config.connector.class is required for custom connectors", i))
				}
			default:
				errs = append(errs, fmt.Sprintf("services.kafkaConnect.connectors[%d].kind must be debezium.postgres or custom", i))
			}
		}
	}
	if s.KafkaUIEnabled() {
		if strings.TrimSpace(s.Services.KafkaUI.Image) == "" {
			errs = append(errs, "services.kafkaUI.image is required")
		}
		if strings.TrimSpace(s.Services.KafkaUI.Version) == "" {
			errs = append(errs, "services.kafkaUI.version is required")
		}
		if s.Services.KafkaUI.Replicas < 1 {
			errs = append(errs, "services.kafkaUI.replicas must be at least 1")
		}
	}
	if s.ZitadelEnabled() {
		if strings.TrimSpace(s.ZitadelAdminPassword()) == "" {
			errs = append(errs, "services.zitadel.admin.password is required")
		}
		if len(s.ZitadelMasterKey()) != 32 {
			errs = append(errs, "services.zitadel.masterKey must be exactly 32 characters")
		}
	}
	if s.RabbitMQEnabled() {
		if s.Services.RabbitMQ.Replicas < 1 {
			errs = append(errs, "services.rabbitmq.replicas must be at least 1")
		}
		if s.Services.RabbitMQ.Replicas != 1 {
			errs = append(errs, "services.rabbitmq.replicas currently must be 1; clustering is not supported yet")
		}
		if strings.TrimSpace(s.RabbitMQUser()) == "" {
			errs = append(errs, "services.rabbitmq.auth.username is required")
		}
		if strings.TrimSpace(s.RabbitMQPassword()) == "" {
			errs = append(errs, "services.rabbitmq.auth.password is required")
		}
		vhosts := map[string]bool{}
		for i, vhost := range s.Services.RabbitMQ.VHosts {
			if strings.TrimSpace(vhost.Name) == "" {
				errs = append(errs, fmt.Sprintf("services.rabbitmq.vhosts[%d].name is required", i))
				continue
			}
			if vhosts[vhost.Name] {
				errs = append(errs, fmt.Sprintf("services.rabbitmq.vhosts[%d].name duplicates another vhost", i))
			}
			vhosts[vhost.Name] = true
		}
		users := map[string]bool{}
		for i, user := range s.Services.RabbitMQ.Users {
			if strings.TrimSpace(user.Name) == "" {
				errs = append(errs, fmt.Sprintf("services.rabbitmq.users[%d].name is required", i))
			} else if users[user.Name] {
				errs = append(errs, fmt.Sprintf("services.rabbitmq.users[%d].name duplicates another user", i))
			} else {
				users[user.Name] = true
			}
			if strings.TrimSpace(user.Password) == "" {
				errs = append(errs, fmt.Sprintf("services.rabbitmq.users[%d].password is required", i))
			}
			for j, permission := range user.Permissions {
				if !vhosts[permission.VHost] {
					errs = append(errs, fmt.Sprintf("services.rabbitmq.users[%d].permissions[%d].vhost must reference services.rabbitmq.vhosts", i, j))
				}
			}
		}
	}
	for name, cm := range s.ConfigMaps {
		if !dnsLabelRE.MatchString(name) {
			errs = append(errs, fmt.Sprintf("configMaps.%s must use a DNS-label key", name))
		}
		for key := range cm.Data {
			if strings.TrimSpace(key) == "" {
				errs = append(errs, fmt.Sprintf("configMaps.%s.data contains an empty key", name))
			}
		}
	}
	for name, secret := range s.Secrets {
		if !dnsLabelRE.MatchString(name) {
			errs = append(errs, fmt.Sprintf("secrets.%s must use a DNS-label key", name))
		}
		for key, value := range secret.StringData {
			sources := 0
			if value.FromEnv != "" {
				sources++
			}
			if value.FromFile != "" {
				sources++
			}
			if sources != 1 {
				errs = append(errs, fmt.Sprintf("secrets.%s.stringData.%s must define exactly one source", name, key))
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
