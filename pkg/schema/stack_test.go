package schema

import (
	"net/url"
	"testing"
)

func TestSetDefaultsUsesSupportedK3SImage(t *testing.T) {
	stack := &Stack{Metadata: Metadata{Name: "demo"}}

	stack.SetDefaults()

	if stack.Cluster.K3SVersion != DefaultK3SImage {
		t.Fatalf("cluster k3sVersion = %q, want %q", stack.Cluster.K3SVersion, DefaultK3SImage)
	}
}

func TestConnectionURLsEscapeCredentials(t *testing.T) {
	stack := &Stack{
		Metadata: Metadata{Name: "demo"},
		Services: Services{
			Postgres: &PostgresService{
				Enabled:   Bool(true),
				Auth:      AuthConfig{Username: "app", Password: "p@ss:/word"},
				Databases: []DatabaseConfig{{Name: "orders"}},
			},
			RabbitMQ: &RabbitMQService{
				Enabled: Bool(true),
				Auth:    AuthConfig{Username: "app@local", Password: "p@ss:/word"},
			},
			Redis: &RedisService{
				Enabled: Bool(true),
				Auth:    RedisAuth{Password: "p@ss:/word"},
			},
		},
	}
	stack.SetDefaults()

	env := stack.ConnectionEnv()
	tests := []struct {
		name     string
		rawURL   string
		username string
		password string
	}{
		{name: "postgres", rawURL: env["POSTGRES_URL"], username: "app", password: "p@ss:/word"},
		{name: "rabbitmq", rawURL: env["RABBITMQ_URL"], username: "app@local", password: "p@ss:/word"},
		{name: "redis", rawURL: env["REDIS_URL"], username: "", password: "p@ss:/word"},
		{name: "internal postgres", rawURL: stack.PostgresInternalURL("zitadel"), username: "app", password: "p@ss:/word"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.User.Username() != tt.username {
				t.Fatalf("username = %q", parsed.User.Username())
			}
			password, ok := parsed.User.Password()
			if !ok || password != tt.password {
				t.Fatalf("password = %q, present = %t", password, ok)
			}
			if parsed.User.String() == tt.username+":"+tt.password {
				t.Fatalf("credentials were not escaped in %q", tt.rawURL)
			}
		})
	}
}

func TestLocalTLSRequiredForEveryIngressService(t *testing.T) {
	tests := []struct {
		name     string
		services Services
		want     bool
	}{
		{name: "no ingress", services: Services{Postgres: &PostgresService{}}, want: false},
		{name: "zitadel", services: Services{Zitadel: &ZitadelService{}}, want: true},
		{name: "rabbitmq management", services: Services{RabbitMQ: &RabbitMQService{}}, want: true},
		{name: "kafka ui", services: Services{Kafka: &KafkaService{}, KafkaUI: &KafkaUIService{}}, want: true},
		{name: "disabled TLS", services: Services{Kafka: &KafkaService{}, KafkaUI: &KafkaUIService{}}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stack := &Stack{Metadata: Metadata{Name: "demo"}, Services: test.services}
			if test.name == "disabled TLS" {
				stack.LocalTLS.Enabled = Bool(false)
			}
			stack.SetDefaults()
			if got := stack.LocalTLSRequired(); got != test.want {
				t.Fatalf("LocalTLSRequired() = %t, want %t", got, test.want)
			}
		})
	}
}
