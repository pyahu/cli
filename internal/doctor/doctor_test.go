package doctor

import (
	"net"
	"testing"

	"github.com/pyahu/cli/pkg/schema"
)

func TestPortInUseDetectsListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	if !portInUse(port) {
		t.Fatalf("expected port %d to be detected as in use", port)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if portInUse(port) {
		t.Fatalf("expected port %d to be free after closing the listener", port)
	}
}

func TestCheckPortReflectsListenerState(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	if busy := checkPort("test", port, "field"); busy.OK {
		t.Fatalf("expected occupied port to be reported busy: %#v", busy)
	}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	if free := checkPort("test", port, "field"); !free.OK {
		t.Fatalf("expected free port to be reported available: %#v", free)
	}
}

func TestParseClusterLines(t *testing.T) {
	got := parseClusterLines("k3d", "pyahu-local 1/1 0/0 true\nother 1/1 0/0 true\n")
	if len(got) != 2 {
		t.Fatalf("clusters = %#v", got)
	}
	if got[0].Runtime != "k3d" || got[0].Name != "pyahu-local" {
		t.Fatalf("first cluster = %#v", got[0])
	}
	if got[1].Runtime != "k3d" || got[1].Name != "other" {
		t.Fatalf("second cluster = %#v", got[1])
	}
}

func TestParseClusterLinesSkipsKindEmptyMessage(t *testing.T) {
	got := parseClusterLines("kind", "No kind clusters found.\n")
	if len(got) != 0 {
		t.Fatalf("clusters = %#v", got)
	}
}

func TestFormatLocalClusters(t *testing.T) {
	got := formatLocalClusters([]localCluster{
		{Runtime: "kind", Name: "demo"},
		{Runtime: "k3d", Name: "other"},
	})
	want := "k3d/other, kind/demo"
	if got != want {
		t.Fatalf("formatLocalClusters = %q, want %q", got, want)
	}
}

func TestCheckPortsIncludesPostgresReadWhenReadReplicasEnabled(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Postgres: &schema.PostgresService{
				Enabled:      schema.Bool(true),
				ReadReplicas: 1,
			},
		},
	}
	stack.SetDefaults()

	checks := portChecks(stack)
	found := false
	for _, check := range checks {
		if check.name == "postgres-read" && check.enabled {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("postgres-read port check not found: %#v", checks)
	}
}

func TestCheckPortsIncludesKafkaConnectWhenEnabled(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Kafka:        &schema.KafkaService{Enabled: schema.Bool(true)},
			KafkaConnect: &schema.KafkaConnectService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()

	checks := portChecks(stack)
	found := false
	for _, check := range checks {
		if check.name == "kafka-connect" && check.enabled && check.port == 8083 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("kafka-connect port check not found: %#v", checks)
	}
}

func TestCheckPortsUsesTraefikEntrypointForHTTPServices(t *testing.T) {
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Kafka:   &schema.KafkaService{Enabled: schema.Bool(true)},
			KafkaUI: &schema.KafkaUIService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()

	checks := portChecks(stack)
	httpFound := false
	for _, check := range checks {
		// Kafka UI is served through Traefik, not its own host port.
		if check.name == "kafka-ui" {
			t.Fatalf("kafka-ui should not have its own host port check: %#v", checks)
		}
		if check.name == "http" && check.enabled && check.port == 80 {
			httpFound = true
		}
	}
	if !httpFound {
		t.Fatalf("traefik http entrypoint check not found: %#v", checks)
	}
}
