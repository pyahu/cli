package schema

import "regexp"

const (
	APIVersion = "cli.pyahu.io/v1alpha1"
	Kind       = "Stack"

	DefaultRuntime             = "k3d"
	DefaultK3SImage            = "rancher/k3s:v1.36.4-k3s1"
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
	DefaultRedisImage          = "valkey/valkey"
	DefaultRedisVersion        = "8.1-alpine"
	DefaultRedisPort           = 6379
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
	sha256RE    = regexp.MustCompile(`^[a-f0-9]{64}$`)
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
	Redis        *RedisService        `json:"redis,omitempty" yaml:"redis,omitempty"`
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

type RedisService struct {
	Enabled *bool      `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Image   string     `json:"image,omitempty" yaml:"image,omitempty"`
	Version string     `json:"version,omitempty" yaml:"version,omitempty"`
	Ports   RedisPorts `json:"ports,omitempty" yaml:"ports,omitempty"`
	Auth    RedisAuth  `json:"auth,omitempty" yaml:"auth,omitempty"`
	// AppendOnly defaults to true: clients that use the WAITAOF durability
	// barrier fail every write when AOF is off.
	AppendOnly *bool  `json:"appendOnly,omitempty" yaml:"appendOnly,omitempty"`
	Storage    string `json:"storage,omitempty" yaml:"storage,omitempty"`
}

type RedisPorts struct {
	Client int `json:"client,omitempty" yaml:"client,omitempty"`
}

type RedisAuth struct {
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
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
	Plugins    []KafkaConnectPlugin    `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	Connectors []KafkaConnectConnector `json:"connectors,omitempty" yaml:"connectors,omitempty"`
}

// KafkaConnectPlugin is one artifact installed into the worker's plugin
// directory <name> before it starts. Repeating a name is allowed and
// meaningful: Kafka Connect isolates each plugin directory in its own
// classloader, so a transform that needs a connector's classes has to land in
// the same directory as the connector.
type KafkaConnectPlugin struct {
	Name   string `json:"name" yaml:"name"`
	URL    string `json:"url,omitempty" yaml:"url,omitempty"`
	SHA256 string `json:"sha256,omitempty" yaml:"sha256,omitempty"`
	File   string `json:"file,omitempty" yaml:"file,omitempty"`
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
	// Optional marks a connector whose source (a table, a publication, a stream)
	// only exists after the application has booted once. Registration failure
	// during `pyahu up` is a warning instead of an error; `pyahu connectors
	// apply` registers it later.
	Optional *bool `json:"optional,omitempty" yaml:"optional,omitempty"`
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
