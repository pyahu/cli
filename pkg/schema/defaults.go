package schema

import "strings"

func (s *Stack) SetDefaults() {
	if s.APIVersion == "" {
		s.APIVersion = APIVersion
	}
	if s.Kind == "" {
		s.Kind = Kind
	}
	if s.Cluster.Runtime == "" {
		s.Cluster.Runtime = DefaultRuntime
	}
	if s.Cluster.Name == "" {
		s.Cluster.Name = s.Metadata.Name
	}
	if s.Cluster.Namespace == "" && s.Metadata.Name != "" {
		s.Cluster.Namespace = s.Metadata.Name + "-dev"
	}
	if s.Cluster.K3SVersion == "" {
		s.Cluster.K3SVersion = DefaultK3SImage
	}
	if s.Cluster.Servers == 0 {
		s.Cluster.Servers = 1
	}
	if s.LocalTLS.SecretName == "" {
		s.LocalTLS.SecretName = DefaultLocalTLSSecretName
	}
	if s.LocalTLS.CAConfigMapName == "" {
		s.LocalTLS.CAConfigMapName = DefaultLocalTLSCAConfigMap
	}
	s.LocalTLS.Domains = normalizeLocalTLSDomains(s.LocalTLS.Domains)

	if s.Services.Postgres != nil {
		s.Services.Postgres.Ports.Primary = defaultPort(s.Services.Postgres.Ports.Primary, s.Cluster.LegacyPorts.Postgres, DefaultPostgresPort)
		if s.Services.Postgres.Version == "" {
			s.Services.Postgres.Version = DefaultPostgresVersion
		}
		if s.Services.Postgres.Auth.Username == "" {
			s.Services.Postgres.Auth.Username = DefaultPostgresUser
		}
		if s.Services.Postgres.Auth.Password == "" {
			s.Services.Postgres.Auth.Password = DefaultPostgresPassword
		}
		if s.Services.Postgres.Replication.Username == "" {
			s.Services.Postgres.Replication.Username = DefaultPostgresReplUser
		}
		if s.Services.Postgres.Replication.Password == "" {
			s.Services.Postgres.Replication.Password = DefaultPostgresReplPass
		}
		if s.Services.Postgres.Instances == 0 {
			s.Services.Postgres.Instances = 1
		}
		if s.Services.Postgres.ReadReplicas > 0 {
			s.Services.Postgres.Ports.Read = defaultPort(s.Services.Postgres.Ports.Read, s.Cluster.LegacyPorts.PostgresRead, DefaultPostgresReadPort)
		}
		if s.Services.Postgres.Storage == "" {
			s.Services.Postgres.Storage = "2Gi"
		}
		if len(s.Services.Postgres.Databases) == 0 {
			s.Services.Postgres.Databases = []DatabaseConfig{{Name: "app"}}
		}
		for i := range s.Services.Postgres.Databases {
			if s.Services.Postgres.Databases[i].Owner == "" {
				s.Services.Postgres.Databases[i].Owner = s.PostgresUser()
			}
		}
	}
	if s.Services.Zitadel != nil {
		s.Services.Zitadel.Ports.HTTP = defaultPort(s.Services.Zitadel.Ports.HTTP, s.Cluster.LegacyPorts.HTTP, DefaultZitadelHTTPPort)
		s.Services.Zitadel.Ports.HTTPS = defaultPort(s.Services.Zitadel.Ports.HTTPS, s.Cluster.LegacyPorts.HTTPS, DefaultZitadelHTTPSPort)
		if s.Services.Zitadel.Version == "" {
			s.Services.Zitadel.Version = DefaultZitadelVersion
		}
		if s.Services.Zitadel.DatabaseRef == "" {
			s.Services.Zitadel.DatabaseRef = "postgres"
		}
		if s.Services.Zitadel.ExternalURL == "" {
			// HTTP services share the Traefik entrypoints on host 80/443, so the
			// external URL is portless. TCP services keep their own host ports.
			if s.LocalTLSEnabled() {
				s.Services.Zitadel.ExternalURL = "https://zitadel.localhost"
			} else {
				s.Services.Zitadel.ExternalURL = "http://zitadel.localhost"
			}
		}
		if s.Services.Zitadel.Admin.Username == "" {
			s.Services.Zitadel.Admin.Username = DefaultZitadelAdminUser
		}
		if s.Services.Zitadel.Admin.Password == "" {
			s.Services.Zitadel.Admin.Password = DefaultZitadelPassword
		}
		if s.Services.Zitadel.MasterKey == "" {
			s.Services.Zitadel.MasterKey = DefaultZitadelMasterKey
		}
		if s.Services.Postgres != nil && !hasDatabase(s.Services.Postgres.Databases, "zitadel") {
			s.Services.Postgres.Databases = append(s.Services.Postgres.Databases, DatabaseConfig{Name: "zitadel", Owner: "zitadel"})
		}
	}
	if s.Services.RabbitMQ != nil {
		s.Services.RabbitMQ.Ports.AMQP = defaultPort(s.Services.RabbitMQ.Ports.AMQP, s.Cluster.LegacyPorts.RabbitMQ, DefaultRabbitMQPort)
		s.Services.RabbitMQ.Ports.Management = defaultPort(s.Services.RabbitMQ.Ports.Management, s.Cluster.LegacyPorts.RabbitMQManagement, DefaultRabbitMQMgmtPort)
		if s.Services.RabbitMQ.Version == "" {
			s.Services.RabbitMQ.Version = DefaultRabbitMQVersion
		}
		if s.Services.RabbitMQ.Auth.Username == "" {
			s.Services.RabbitMQ.Auth.Username = DefaultRabbitMQUser
		}
		if s.Services.RabbitMQ.Auth.Password == "" {
			s.Services.RabbitMQ.Auth.Password = DefaultRabbitMQPassword
		}
		if s.Services.RabbitMQ.Replicas == 0 {
			s.Services.RabbitMQ.Replicas = 1
		}
		if s.Services.RabbitMQ.Storage == "" {
			s.Services.RabbitMQ.Storage = "2Gi"
		}
		if s.Services.RabbitMQ.Management == nil {
			s.Services.RabbitMQ.Management = Bool(true)
		}
		if len(s.Services.RabbitMQ.VHosts) == 0 {
			s.Services.RabbitMQ.VHosts = []RabbitMQVHost{{Name: "/"}}
		}
		defaultVHost := firstRabbitMQVHost(s.Services.RabbitMQ.VHosts)
		if len(s.Services.RabbitMQ.Users) == 0 {
			s.Services.RabbitMQ.Users = []RabbitMQUser{{
				Name:     s.RabbitMQUser(),
				Password: s.RabbitMQPassword(),
				Tags:     "administrator",
				Permissions: []RabbitMQPermission{{
					VHost:     defaultVHost,
					Configure: ".*",
					Write:     ".*",
					Read:      ".*",
				}},
			}}
		} else if !hasRabbitMQUser(s.Services.RabbitMQ.Users, s.RabbitMQUser()) {
			s.Services.RabbitMQ.Users = append(s.Services.RabbitMQ.Users, RabbitMQUser{
				Name:     s.RabbitMQUser(),
				Password: s.RabbitMQPassword(),
				Tags:     "administrator",
				Permissions: []RabbitMQPermission{{
					VHost:     defaultVHost,
					Configure: ".*",
					Write:     ".*",
					Read:      ".*",
				}},
			})
		}
		for i := range s.Services.RabbitMQ.Users {
			if s.Services.RabbitMQ.Users[i].Password == "" {
				s.Services.RabbitMQ.Users[i].Password = s.RabbitMQPassword()
			}
			if s.Services.RabbitMQ.Users[i].Tags == "" {
				s.Services.RabbitMQ.Users[i].Tags = "administrator"
			}
			if len(s.Services.RabbitMQ.Users[i].Permissions) == 0 {
				s.Services.RabbitMQ.Users[i].Permissions = []RabbitMQPermission{{VHost: defaultVHost, Configure: ".*", Write: ".*", Read: ".*"}}
			}
			for j := range s.Services.RabbitMQ.Users[i].Permissions {
				permission := &s.Services.RabbitMQ.Users[i].Permissions[j]
				if permission.VHost == "" {
					permission.VHost = defaultVHost
				}
				if permission.Configure == "" {
					permission.Configure = ".*"
				}
				if permission.Write == "" {
					permission.Write = ".*"
				}
				if permission.Read == "" {
					permission.Read = ".*"
				}
			}
		}
	}
	if s.Services.Redis != nil {
		s.Services.Redis.Ports.Client = defaultPort(s.Services.Redis.Ports.Client, 0, DefaultRedisPort)
		if s.Services.Redis.Image == "" {
			s.Services.Redis.Image = DefaultRedisImage
		}
		if s.Services.Redis.Version == "" {
			s.Services.Redis.Version = DefaultRedisVersion
		}
		if s.Services.Redis.AppendOnly == nil {
			s.Services.Redis.AppendOnly = Bool(true)
		}
		if s.Services.Redis.Storage == "" {
			s.Services.Redis.Storage = "1Gi"
		}
	}
	if s.Services.Kafka != nil {
		s.Services.Kafka.Ports.Bootstrap = defaultPort(s.Services.Kafka.Ports.Bootstrap, s.Cluster.LegacyPorts.Kafka, DefaultKafkaPort)
		if s.Services.Kafka.Version == "" {
			s.Services.Kafka.Version = DefaultKafkaVersion
		}
		if s.Services.Kafka.Replicas == 0 {
			s.Services.Kafka.Replicas = 1
		}
		if s.Services.Kafka.Storage == "" {
			s.Services.Kafka.Storage = "4Gi"
		}
		for i := range s.Services.Kafka.Topics {
			if s.Services.Kafka.Topics[i].Partitions == 0 {
				s.Services.Kafka.Topics[i].Partitions = 1
			}
			if s.Services.Kafka.Topics[i].Replicas == 0 {
				s.Services.Kafka.Topics[i].Replicas = 1
			}
		}
	}
	if s.Services.KafkaConnect != nil {
		s.Services.KafkaConnect.Ports.REST = defaultPort(s.Services.KafkaConnect.Ports.REST, s.Cluster.LegacyPorts.KafkaConnect, DefaultKafkaConnectPort)
		if s.Services.KafkaConnect.Image == "" {
			s.Services.KafkaConnect.Image = DefaultKafkaConnectImage
		}
		if s.Services.KafkaConnect.Version == "" {
			s.Services.KafkaConnect.Version = DefaultKafkaConnectVersion
		}
		if s.Services.KafkaConnect.Replicas == 0 {
			s.Services.KafkaConnect.Replicas = 1
		}
		for i := range s.Services.KafkaConnect.Connectors {
			connector := &s.Services.KafkaConnect.Connectors[i]
			if connector.Kind == "" {
				if strings.TrimSpace(connector.Config["connector.class"]) != "" {
					connector.Kind = "custom"
				} else {
					connector.Kind = "debezium.postgres"
				}
			}
			if connector.Type == "" {
				connector.Type = "source"
			}
			if connector.Kind == "debezium.postgres" {
				if connector.Database == "" && s.Services.Postgres != nil && len(s.Services.Postgres.Databases) > 0 {
					connector.Database = s.Services.Postgres.Databases[0].Name
				}
				if connector.TopicPrefix == "" {
					connector.TopicPrefix = connector.Name
				}
				if connector.Slot == "" {
					connector.Slot = strings.ReplaceAll(connector.Name, "-", "_") + "_slot"
				}
				if connector.Publication == "" {
					connector.Publication = strings.ReplaceAll(connector.Name, "-", "_") + "_publication"
				}
				if connector.SnapshotMode == "" {
					connector.SnapshotMode = "initial"
				}
			}
		}
	}
	if s.Services.KafkaUI != nil {
		s.Services.KafkaUI.Ports.HTTP = defaultPort(s.Services.KafkaUI.Ports.HTTP, s.Cluster.LegacyPorts.KafkaUI, DefaultKafkaUIPort)
		if s.Services.KafkaUI.Image == "" {
			s.Services.KafkaUI.Image = DefaultKafkaUIImage
		}
		if s.Services.KafkaUI.Version == "" {
			s.Services.KafkaUI.Version = DefaultKafkaUIVersion
		}
		if s.Services.KafkaUI.Replicas == 0 {
			s.Services.KafkaUI.Replicas = 1
		}
	}
}
