package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pyahu/cli/internal/config"
	"github.com/pyahu/cli/pkg/schema"
)

func (a *app) loadStack() (*config.LoadedStack, error) {
	return a.deps.loadStack(a.opts.file)
}

func (a *app) reportConfig(loaded *config.LoadedStack) {
	if loaded == nil || a.opts.output != "human" || a.opts.quiet {
		return
	}
	switch {
	case loaded.GlobalPath != "" && loaded.ProjectPath != "":
		a.step("config", "loaded %s, then %s (project wins)", displayPath(loaded.GlobalPath), displayPath(loaded.ProjectPath))
	case loaded.GlobalPath != "":
		a.step("config", "loaded global config %s", displayPath(loaded.GlobalPath))
	case loaded.ProjectPath != "":
		a.step("config", "loaded project config %s", displayPath(loaded.ProjectPath))
	default:
		a.step("config", "loaded config %s", displayPath(loaded.Path))
	}
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return path
	}
	return filepath.Join("~", rel)
}

func defaultDoctorStack() *schema.Stack {
	stack := &schema.Stack{
		APIVersion: schema.APIVersion,
		Kind:       schema.Kind,
		Metadata:   schema.Metadata{Name: "pyahu-local"},
		Services: schema.Services{
			Postgres:     &schema.PostgresService{Enabled: schema.Bool(true)},
			Zitadel:      &schema.ZitadelService{Enabled: schema.Bool(true)},
			RabbitMQ:     &schema.RabbitMQService{Enabled: schema.Bool(true)},
			Redis:        &schema.RedisService{Enabled: schema.Bool(true)},
			Kafka:        &schema.KafkaService{Enabled: schema.Bool(true)},
			KafkaConnect: &schema.KafkaConnectService{Enabled: schema.Bool(true)},
			KafkaUI:      &schema.KafkaUIService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()
	return stack
}

// validServiceNames lists the services `describe` and `logs` accept, in the order
// they are shown to the user.
func validServiceNames() []string {
	return []string{"postgres", "zitadel", "rabbitmq", "redis", "kafka", "kafka-connect", "kafka-ui"}
}

func validService(service string) bool {
	for _, name := range validServiceNames() {
		if name == service {
			return true
		}
	}
	return false
}

func unknownServiceError() error {
	return usageError("service must be one of: " + strings.Join(validServiceNames(), ", "))
}
