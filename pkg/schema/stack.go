package schema

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	APIVersion = "cli.pyahu.io/v1alpha1"
	Kind       = "Stack"

	DefaultRuntime             = "k3d"
	DefaultK3SImage            = "rancher/k3s:v1.31.3-k3s1"
	DefaultHTTPPort            = 80
	DefaultHTTPSPort           = 443
	DefaultPostgresVersion     = "18.4"
	DefaultPostgresPort        = 5432
	DefaultPostgresReadPort    = 5433
	DefaultKafkaVersion        = "4.3.0"
	DefaultKafkaPort           = 9092
	DefaultKafkaConnectImage   = "quay.io/debezium/connect"
	DefaultKafkaConnectVersion = "3.5.2.Final"
	DefaultKafkaConnectPort    = 8083
	DefaultKafkaUIImage        = "ghcr.io/kafbat/kafka-ui"
	DefaultKafkaUIVersion      = "v1.5.0"
	DefaultKafkaUIPort         = 8084
	DefaultRabbitMQVersion     = "4.3.2-management-alpine"
	DefaultRabbitMQPort        = 5672
	DefaultRabbitMQMgmtPort    = 15672
	DefaultZitadelVersion      = "v4.15.2"
	DefaultZitadelHTTPPort     = 8080
	DefaultZitadelHTTPSPort    = 8443
	DefaultPostgresUser        = "pyahu"
	DefaultPostgresPassword    = "pyahu_local"
	DefaultPostgresReplUser    = "pyahu_replicator"
	DefaultPostgresReplPass    = "pyahu_replicator_local"
	DefaultRabbitMQUser        = "pyahu"
	DefaultRabbitMQPassword    = "pyahu_local"
	DefaultZitadelAdminUser    = "admin@pyahu.local"
	DefaultZitadelPassword     = "Password1!"
	DefaultZitadelMasterKey    = "MasterkeyNeedsToHave32Characters"
	DefaultLocalStateDir       = ".pyahu/local"
	DefaultLocalTLSSecretName  = "pyahu-local-tls"
	DefaultLocalTLSCAConfigMap = "pyahu-local-ca"
)

var (
	dnsLabelRE  = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	dbNameRE    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	topicNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

type Stack struct {
	APIVersion string                         `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                         `json:"kind" yaml:"kind"`
	Metadata   Metadata                       `json:"metadata" yaml:"metadata"`
	Cluster    ClusterConfig                  `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	LocalTLS   LocalTLSConfig                 `json:"localTLS,omitempty" yaml:"localTLS,omitempty"`
	Services   Services                       `json:"services" yaml:"services"`
	ConfigMaps map[string]ConfigMapDefinition `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
	Secrets    map[string]SecretDefinition    `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

type Metadata struct {
	Name string `json:"name" yaml:"name"`
}

type ClusterConfig struct {
	Runtime     string     `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Name        string     `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace   string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	K3SVersion  string     `json:"k3sVersion,omitempty" yaml:"k3sVersion,omitempty"`
	Servers     int        `json:"servers,omitempty" yaml:"servers,omitempty"`
	Agents      int        `json:"agents,omitempty" yaml:"agents,omitempty"`
	LegacyPorts PortConfig `json:"ports,omitempty" yaml:"ports,omitempty"`
}

type LocalTLSConfig struct {
	Enabled         *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Domains         []string `json:"domains,omitempty" yaml:"domains,omitempty"`
	SecretName      string   `json:"secretName,omitempty" yaml:"secretName,omitempty"`
	CAConfigMapName string   `json:"caConfigMapName,omitempty" yaml:"caConfigMapName,omitempty"`
}

// PortConfig keeps compatibility with early prerelease stack files that used
// cluster.ports. New stack files should configure ports under each service.
type PortConfig struct {
	HTTP               int `json:"http,omitempty" yaml:"http,omitempty"`
	HTTPS              int `json:"https,omitempty" yaml:"https,omitempty"`
	Postgres           int `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	PostgresRead       int `json:"postgresRead,omitempty" yaml:"postgresRead,omitempty"`
	RabbitMQ           int `json:"rabbitmq,omitempty" yaml:"rabbitmq,omitempty"`
	RabbitMQManagement int `json:"rabbitmqManagement,omitempty" yaml:"rabbitmqManagement,omitempty"`
	Kafka              int `json:"kafka,omitempty" yaml:"kafka,omitempty"`
	KafkaConnect       int `json:"kafkaConnect,omitempty" yaml:"kafkaConnect,omitempty"`
	KafkaUI            int `json:"kafkaUI,omitempty" yaml:"kafkaUI,omitempty"`
}

type Services struct {
	Postgres     *PostgresService     `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Zitadel      *ZitadelService      `json:"zitadel,omitempty" yaml:"zitadel,omitempty"`
	RabbitMQ     *RabbitMQService     `json:"rabbitmq,omitempty" yaml:"rabbitmq,omitempty"`
	Kafka        *KafkaService        `json:"kafka,omitempty" yaml:"kafka,omitempty"`
	KafkaConnect *KafkaConnectService `json:"kafkaConnect,omitempty" yaml:"kafkaConnect,omitempty"`
	KafkaUI      *KafkaUIService      `json:"kafkaUI,omitempty" yaml:"kafkaUI,omitempty"`
}

type PostgresService struct {
	Enabled      *bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version      string           `json:"version,omitempty" yaml:"version,omitempty"`
	Ports        PostgresPorts    `json:"ports,omitempty" yaml:"ports,omitempty"`
	Auth         AuthConfig       `json:"auth,omitempty" yaml:"auth,omitempty"`
	Instances    int              `json:"instances,omitempty" yaml:"instances,omitempty"`
	ReadReplicas int              `json:"readReplicas,omitempty" yaml:"readReplicas,omitempty"`
	Replication  AuthConfig       `json:"replication,omitempty" yaml:"replication,omitempty"`
	Storage      string           `json:"storage,omitempty" yaml:"storage,omitempty"`
	Databases    []DatabaseConfig `json:"databases,omitempty" yaml:"databases,omitempty"`
}

type PostgresPorts struct {
	Primary int `json:"primary,omitempty" yaml:"primary,omitempty"`
	Read    int `json:"read,omitempty" yaml:"read,omitempty"`
}

type AuthConfig struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

type DatabaseConfig struct {
	Name  string `json:"name" yaml:"name"`
	Owner string `json:"owner,omitempty" yaml:"owner,omitempty"`
	Seed  string `json:"seed,omitempty" yaml:"seed,omitempty"`
}

type ZitadelService struct {
	Enabled     *bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version     string       `json:"version,omitempty" yaml:"version,omitempty"`
	Ports       ZitadelPorts `json:"ports,omitempty" yaml:"ports,omitempty"`
	ExternalURL string       `json:"externalURL,omitempty" yaml:"externalURL,omitempty"`
	DatabaseRef string       `json:"databaseRef,omitempty" yaml:"databaseRef,omitempty"`
	MasterKey   string       `json:"masterKey,omitempty" yaml:"masterKey,omitempty"`
	Admin       ZitadelAdmin `json:"admin,omitempty" yaml:"admin,omitempty"`
}

type ZitadelPorts struct {
	HTTP  int `json:"http,omitempty" yaml:"http,omitempty"`
	HTTPS int `json:"https,omitempty" yaml:"https,omitempty"`
}

type ZitadelAdmin struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

type RabbitMQService struct {
	Enabled    *bool           `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version    string          `json:"version,omitempty" yaml:"version,omitempty"`
	Ports      RabbitMQPorts   `json:"ports,omitempty" yaml:"ports,omitempty"`
	Auth       AuthConfig      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Replicas   int             `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Storage    string          `json:"storage,omitempty" yaml:"storage,omitempty"`
	Management *bool           `json:"management,omitempty" yaml:"management,omitempty"`
	VHosts     []RabbitMQVHost `json:"vhosts,omitempty" yaml:"vhosts,omitempty"`
	Users      []RabbitMQUser  `json:"users,omitempty" yaml:"users,omitempty"`
}

type RabbitMQPorts struct {
	AMQP       int `json:"amqp,omitempty" yaml:"amqp,omitempty"`
	Management int `json:"management,omitempty" yaml:"management,omitempty"`
}

type RabbitMQVHost struct {
	Name string `json:"name" yaml:"name"`
}

type RabbitMQUser struct {
	Name        string               `json:"name" yaml:"name"`
	Password    string               `json:"password,omitempty" yaml:"password,omitempty"`
	Tags        string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Permissions []RabbitMQPermission `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}

type RabbitMQPermission struct {
	VHost     string `json:"vhost,omitempty" yaml:"vhost,omitempty"`
	Configure string `json:"configure,omitempty" yaml:"configure,omitempty"`
	Write     string `json:"write,omitempty" yaml:"write,omitempty"`
	Read      string `json:"read,omitempty" yaml:"read,omitempty"`
}

type KafkaService struct {
	Enabled  *bool         `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version  string        `json:"version,omitempty" yaml:"version,omitempty"`
	Ports    KafkaPorts    `json:"ports,omitempty" yaml:"ports,omitempty"`
	Replicas int           `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Storage  string        `json:"storage,omitempty" yaml:"storage,omitempty"`
	Topics   []TopicConfig `json:"topics,omitempty" yaml:"topics,omitempty"`
}

type KafkaPorts struct {
	Bootstrap int `json:"bootstrap,omitempty" yaml:"bootstrap,omitempty"`
}

type KafkaConnectService struct {
	Enabled    *bool                   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Image      string                  `json:"image,omitempty" yaml:"image,omitempty"`
	Version    string                  `json:"version,omitempty" yaml:"version,omitempty"`
	Ports      KafkaConnectPorts       `json:"ports,omitempty" yaml:"ports,omitempty"`
	Replicas   int                     `json:"replicas,omitempty" yaml:"replicas,omitempty"`
	Connectors []KafkaConnectConnector `json:"connectors,omitempty" yaml:"connectors,omitempty"`
}

type KafkaConnectPorts struct {
	REST int `json:"rest,omitempty" yaml:"rest,omitempty"`
}

type KafkaUIService struct {
	Enabled  *bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Image    string    `json:"image,omitempty" yaml:"image,omitempty"`
	Version  string    `json:"version,omitempty" yaml:"version,omitempty"`
	Ports    HTTPPorts `json:"ports,omitempty" yaml:"ports,omitempty"`
	Replicas int       `json:"replicas,omitempty" yaml:"replicas,omitempty"`
}

type HTTPPorts struct {
	HTTP int `json:"http,omitempty" yaml:"http,omitempty"`
}

type KafkaConnectConnector struct {
	Name         string            `json:"name" yaml:"name"`
	Type         string            `json:"type,omitempty" yaml:"type,omitempty"`
	Kind         string            `json:"kind,omitempty" yaml:"kind,omitempty"`
	Database     string            `json:"database,omitempty" yaml:"database,omitempty"`
	TopicPrefix  string            `json:"topicPrefix,omitempty" yaml:"topicPrefix,omitempty"`
	Slot         string            `json:"slot,omitempty" yaml:"slot,omitempty"`
	Publication  string            `json:"publication,omitempty" yaml:"publication,omitempty"`
	Tables       TableFilter       `json:"tables,omitempty" yaml:"tables,omitempty"`
	SnapshotMode string            `json:"snapshotMode,omitempty" yaml:"snapshotMode,omitempty"`
	Config       map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}

type DebeziumConnector = KafkaConnectConnector

type TableFilter struct {
	Include []string `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type TopicConfig struct {
	Name       string `json:"name" yaml:"name"`
	Partitions int    `json:"partitions,omitempty" yaml:"partitions,omitempty"`
	Replicas   int    `json:"replicas,omitempty" yaml:"replicas,omitempty"`
}

type ConfigMapDefinition struct {
	Data  map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
	Files map[string]string `json:"files,omitempty" yaml:"files,omitempty"`
}

type SecretDefinition struct {
	StringData map[string]SecretValue `json:"stringData,omitempty" yaml:"stringData,omitempty"`
}

type SecretValue struct {
	FromEnv  string `json:"fromEnv,omitempty" yaml:"fromEnv,omitempty"`
	FromFile string `json:"fromFile,omitempty" yaml:"fromFile,omitempty"`
}

func Bool(v bool) *bool {
	return &v
}

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
			if connector.Kind == "debezium.postgres" {
				if connector.Type == "" {
					connector.Type = "source"
				}
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
	if s.KafkaEnabled() {
		for i, topic := range s.Services.Kafka.Topics {
			if !topicNameRE.MatchString(topic.Name) {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].name contains unsupported characters", i))
			}
			if topic.Partitions < 1 {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].partitions must be at least 1", i))
			}
			if topic.Replicas < 1 {
				errs = append(errs, fmt.Sprintf("services.kafka.topics[%d].replicas must be at least 1", i))
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
	return s.LocalTLSEnabled() && s.ZitadelEnabled()
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

func (s *Stack) ConnectionEnv() map[string]string {
	env := map[string]string{}
	if s.PostgresEnabled() {
		db := s.Services.Postgres.Databases[0].Name
		env["POSTGRES_HOST"] = "localhost"
		env["POSTGRES_PORT"] = fmt.Sprintf("%d", s.PostgresPort())
		env["POSTGRES_DATABASE"] = db
		env["POSTGRES_USER"] = s.PostgresUser()
		env["POSTGRES_PASSWORD"] = s.PostgresPassword()
		env["POSTGRES_URL"] = fmt.Sprintf("postgresql://%s:%s@localhost:%d/%s?sslmode=disable", s.PostgresUser(), s.PostgresPassword(), s.PostgresPort(), db)
		if s.PostgresReadReplicas() > 0 {
			env["POSTGRES_READ_HOST"] = "localhost"
			env["POSTGRES_READ_PORT"] = fmt.Sprintf("%d", s.PostgresReadPort())
			env["POSTGRES_READ_URL"] = fmt.Sprintf("postgresql://%s:%s@localhost:%d/%s?sslmode=disable", s.PostgresUser(), s.PostgresPassword(), s.PostgresReadPort(), db)
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
		env["RABBITMQ_URL"] = fmt.Sprintf("amqp://%s:%s@localhost:%d", s.RabbitMQUser(), s.RabbitMQPassword(), s.RabbitMQPort())
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

func (s *Stack) KafkaConnectConfigTopic() string {
	return s.Metadata.Name + ".connect.configs"
}

func (s *Stack) KafkaConnectOffsetTopic() string {
	return s.Metadata.Name + ".connect.offsets"
}

func (s *Stack) KafkaConnectStatusTopic() string {
	return s.Metadata.Name + ".connect.status"
}

func (s *Stack) KafkaConnectInternalTopics() []TopicConfig {
	return []TopicConfig{
		{Name: s.KafkaConnectConfigTopic(), Partitions: 1, Replicas: 1},
		{Name: s.KafkaConnectOffsetTopic(), Partitions: 1, Replicas: 1},
		{Name: s.KafkaConnectStatusTopic(), Partitions: 1, Replicas: 1},
	}
}

func (s *Stack) KafkaConnectConnectorConfig(connector KafkaConnectConnector) map[string]string {
	kind := connector.Kind
	if kind == "" {
		if strings.TrimSpace(connector.Config["connector.class"]) != "" {
			kind = "custom"
		} else {
			kind = "debezium.postgres"
		}
	}
	if kind == "debezium.postgres" {
		return s.DebeziumPostgresConfig(connector)
	}
	config := map[string]string{}
	for key, value := range connector.Config {
		config[key] = value
	}
	return config
}

func (s *Stack) DebeziumPostgresConfig(connector KafkaConnectConnector) map[string]string {
	config := map[string]string{
		"connector.class":   "io.debezium.connector.postgresql.PostgresConnector",
		"tasks.max":         "1",
		"database.hostname": fmt.Sprintf("postgres.%s.svc.cluster.local", s.Cluster.Namespace),
		"database.port":     "5432",
		"database.user":     s.PostgresUser(),
		"database.password": s.PostgresPassword(),
		"database.dbname":   connector.Database,
		"topic.prefix":      connector.TopicPrefix,
		"plugin.name":       "pgoutput",
		"slot.name":         connector.Slot,
		"publication.name":  connector.Publication,
		"snapshot.mode":     connector.SnapshotMode,
	}
	if len(connector.Tables.Include) > 0 {
		config["publication.autocreate.mode"] = "filtered"
		config["table.include.list"] = strings.Join(connector.Tables.Include, ",")
	}
	if len(connector.Tables.Exclude) > 0 {
		config["publication.autocreate.mode"] = "filtered"
		config["table.exclude.list"] = strings.Join(connector.Tables.Exclude, ",")
	}
	if config["publication.autocreate.mode"] == "" {
		config["publication.autocreate.mode"] = "all_tables"
	}
	for key, value := range connector.Config {
		config[key] = value
	}
	return config
}

func SortedEnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func enabled(v *bool) bool {
	return v == nil || *v
}

func hasDatabase(databases []DatabaseConfig, name string) bool {
	for _, db := range databases {
		if db.Name == name {
			return true
		}
	}
	return false
}

func firstRabbitMQVHost(vhosts []RabbitMQVHost) string {
	for _, vhost := range vhosts {
		if strings.TrimSpace(vhost.Name) != "" {
			return vhost.Name
		}
	}
	return "/"
}

func hasRabbitMQUser(users []RabbitMQUser, name string) bool {
	for _, user := range users {
		if user.Name == name {
			return true
		}
	}
	return false
}

func defaultPort(configured int, legacy int, fallback int) int {
	if configured != 0 {
		return configured
	}
	if legacy != 0 {
		return legacy
	}
	return fallback
}

type hostPort struct {
	field string
	port  int
}

func (s *Stack) enabledHostPorts() []hostPort {
	values := []hostPort{}
	// HTTP services share the Traefik entrypoints; only one pair of host ports.
	if s.HTTPIngressEnabled() {
		values = append(values, hostPort{field: "traefik web entrypoint (80)", port: DefaultHTTPPort})
		if s.LocalTLSEnabled() {
			values = append(values, hostPort{field: "traefik websecure entrypoint (443)", port: DefaultHTTPSPort})
		}
	}
	if s.PostgresEnabled() {
		values = append(values, hostPort{field: "services.postgres.ports.primary", port: s.PostgresPort()})
		if s.PostgresReadReplicas() > 0 {
			values = append(values, hostPort{field: "services.postgres.ports.read", port: s.PostgresReadPort()})
		}
	}
	if s.RabbitMQEnabled() {
		values = append(values, hostPort{field: "services.rabbitmq.ports.amqp", port: s.RabbitMQPort()})
	}
	if s.KafkaEnabled() {
		values = append(values, hostPort{field: "services.kafka.ports.bootstrap", port: s.KafkaPort()})
	}
	if s.KafkaConnectEnabled() {
		values = append(values, hostPort{field: "services.kafkaConnect.ports.rest", port: s.KafkaConnectPort()})
	}
	return values
}

func validatePorts(values []hostPort) []string {
	seen := map[int]string{}
	var errs []string
	for _, value := range values {
		if value.port < 1 || value.port > 65535 {
			errs = append(errs, fmt.Sprintf("%s must be between 1 and 65535", value.field))
			continue
		}
		if previous, ok := seen[value.port]; ok {
			errs = append(errs, fmt.Sprintf("%s duplicates %s on port %d", value.field, previous, value.port))
			continue
		}
		seen[value.port] = value.field
	}
	return errs
}

func normalizeLocalTLSDomains(domains []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(domains)+2)
	add := func(domain string) {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if seen[domain] {
			return
		}
		seen[domain] = true
		normalized = append(normalized, domain)
	}
	for _, domain := range DefaultLocalTLSDomains() {
		add(domain)
	}
	for _, domain := range domains {
		add(domain)
	}
	return normalized
}

func validLocalTLSDomain(domain string) bool {
	if domain == "localhost" || domain == "*.localhost" {
		return true
	}
	if !strings.HasSuffix(domain, ".localhost") {
		return false
	}
	labels := strings.Split(domain, ".")
	for i, label := range labels {
		if label == "*" {
			if i != 0 {
				return false
			}
			continue
		}
		if !dnsLabelRE.MatchString(label) {
			return false
		}
	}
	return true
}
