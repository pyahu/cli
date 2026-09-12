package catalog

import (
	"reflect"
	"testing"

	"github.com/pyahu/cli/internal/kube"
	"github.com/pyahu/cli/pkg/schema"
)

func TestBuildMapsServiceStateAndConnectionMetadata(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Postgres: &schema.PostgresService{
				Databases: []schema.DatabaseConfig{{Name: "zeta"}, {Name: "alpha", Owner: "owner"}},
			},
			Redis: &schema.RedisService{},
		},
	}
	stack.SetDefaults()
	statuses := []kube.ServiceStatus{{
		Name:    "postgres",
		Ready:   true,
		Message: "1/1 pods ready",
		Pods:    []kube.PodStatus{{Name: "postgres-0", Ready: true}},
	}}

	services := Build(stack, statuses, true)
	if len(services) != 7 {
		t.Fatalf("Build returned %d services, want 7", len(services))
	}
	if got := serviceNames(services); !reflect.DeepEqual(got, []string{"postgres", "zitadel", "rabbitmq", "redis", "kafka", "kafka-connect", "kafka-ui"}) {
		t.Fatalf("unexpected service order: %v", got)
	}

	postgres := services[0]
	if !postgres.Enabled || !postgres.Ready || postgres.Status != "ready" || postgres.Message != "1/1 pods ready" {
		t.Fatalf("unexpected PostgreSQL status: %#v", postgres)
	}
	if postgres.Namespace != "demo-dev" || postgres.Endpoints[0].Port != schema.DefaultPostgresPort {
		t.Fatalf("unexpected PostgreSQL endpoint: %#v", postgres.Endpoints[0])
	}
	if postgres.Env["POSTGRES_URL"] == "" || postgres.Details["databases"] != "alpha(owner=owner), zeta(owner=pyahu)" {
		t.Fatalf("unexpected PostgreSQL metadata: env=%v details=%v", postgres.Env, postgres.Details)
	}

	redis := services[3]
	if redis.Status != "waiting" || redis.Message != "waiting for pods" {
		t.Fatalf("unexpected Redis status: %#v", redis)
	}
	if services[1].Status != "disabled" {
		t.Fatalf("disabled ZITADEL was reported as %q", services[1].Status)
	}
}

func TestBuildReportsEnabledServicesAsStoppedWithoutCluster(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{Postgres: &schema.PostgresService{}},
	}
	stack.SetDefaults()

	service := Build(stack, []kube.ServiceStatus{{Name: "postgres", Ready: true}}, false)[0]
	if service.Ready || service.Status != "stopped" || service.Message != "cluster is not running" {
		t.Fatalf("unexpected stopped service: %#v", service)
	}
}

func TestParseEndpointAndSortedKeys(t *testing.T) {
	fallback := parsedEndpoint{Scheme: "http", Host: "fallback.localhost", Port: 8080}
	if got := parseEndpoint("https://identity.localhost/path", fallback); got != (parsedEndpoint{Scheme: "https", Host: "identity.localhost", Port: 443}) {
		t.Fatalf("unexpected HTTPS endpoint: %#v", got)
	}
	if got := parseEndpoint("http://identity.localhost:9080/path", fallback); got != (parsedEndpoint{Scheme: "http", Host: "identity.localhost", Port: 9080}) {
		t.Fatalf("unexpected explicit endpoint: %#v", got)
	}
	if got := EnvKeys(map[string]string{"Z": "", "A": ""}); !reflect.DeepEqual(got, []string{"A", "Z"}) {
		t.Fatalf("environment keys are not sorted: %v", got)
	}
	if got := DetailKeys(map[string]string{"two": "", "one": ""}); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("detail keys are not sorted: %v", got)
	}
}

func serviceNames(services []Service) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, service.Name)
	}
	return names
}
