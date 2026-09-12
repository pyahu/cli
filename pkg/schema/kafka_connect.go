package schema

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

func PluginArtifactFormat(plugin KafkaConnectPlugin) (string, error) {
	name := plugin.File
	if name == "" {
		name = pluginURLPath(plugin.URL)
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return "zip", nil
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz", nil
	case strings.HasSuffix(lower, ".jar"):
		return "jar", nil
	default:
		return "", fmt.Errorf("artifact %q must be a .zip, .tar.gz, .tgz, or .jar", name)
	}
}

// PluginArtifactBase is the artifact file name, used to name the copy inside the
// plugin directory and the Secret key for a local file.
func PluginArtifactBase(plugin KafkaConnectPlugin) string {
	if plugin.File != "" {
		return path.Base(filepath.ToSlash(plugin.File))
	}
	return path.Base(pluginURLPath(plugin.URL))
}

// pluginURLPath strips the query and fragment so a signed or versioned download
// URL still reveals the artifact extension.
func pluginURLPath(raw string) string {
	if parsed, err := url.Parse(raw); err == nil && parsed.Path != "" {
		return parsed.Path
	}
	if index := strings.IndexAny(raw, "?#"); index >= 0 {
		return raw[:index]
	}
	return raw
}

// KafkaConnectPluginDirs lists the distinct plugin directories, in declaration order.
func (s *Stack) KafkaConnectPluginDirs() []string {
	if s.Services.KafkaConnect == nil {
		return nil
	}
	seen := map[string]bool{}
	dirs := []string{}
	for _, plugin := range s.Services.KafkaConnect.Plugins {
		if seen[plugin.Name] {
			continue
		}
		seen[plugin.Name] = true
		dirs = append(dirs, plugin.Name)
	}
	return dirs
}

// ConnectorOptional reports whether a registration failure during `pyahu up` is
// a warning rather than an error.
func ConnectorOptional(connector KafkaConnectConnector) bool {
	return connector.Optional != nil && *connector.Optional
}

func (s *Stack) KafkaConnectPlugins() []KafkaConnectPlugin {
	if s.Services.KafkaConnect == nil {
		return nil
	}
	return s.Services.KafkaConnect.Plugins
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
