package schema

import (
	"fmt"
	"sort"
	"strings"
)

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
	if s.RedisEnabled() {
		values = append(values, hostPort{field: "services.redis.ports.client", port: s.RedisPort()})
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
