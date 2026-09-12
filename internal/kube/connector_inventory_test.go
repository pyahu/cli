package kube

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pyahu/cli/pkg/schema"
)

func TestReconcileKafkaConnectorsDeletesOnlyPreviouslyManagedNames(t *testing.T) {
	deleted := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Fatalf("unexpected method: %s", request.Method)
		}
		deleted = append(deleted, request.URL.Path)
		response.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, rawPort, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}

	const namespace = "demo-dev"
	client := &Client{clientset: fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: kafkaConnectInventoryConfigMapName, Namespace: namespace},
		Data: map[string]string{
			kafkaConnectInventoryDataKey: "orders-source\nremoved-source",
		},
	})}
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		Cluster:  schema.ClusterConfig{Namespace: namespace},
		Services: schema.Services{
			Kafka: &schema.KafkaService{},
			KafkaConnect: &schema.KafkaConnectService{
				Ports:      schema.KafkaConnectPorts{REST: port},
				Connectors: []schema.KafkaConnectConnector{{Name: "orders-source"}, {Name: "new-source"}},
			},
		},
	}
	stack.SetDefaults()

	if err := client.reconcileKafkaConnectors(context.Background(), stack); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deleted, []string{"/connectors/removed-source"}) {
		t.Fatalf("deleted connectors = %v", deleted)
	}
	inventory, err := client.clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), kafkaConnectInventoryConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := inventory.Data[kafkaConnectInventoryDataKey], "new-source\norders-source"; got != want {
		t.Fatalf("inventory = %q, want %q", got, want)
	}
}

func TestDesiredInventoryRetainsKafkaConnectOwnershipWhileDisabled(t *testing.T) {
	stack := &schema.Stack{Metadata: schema.Metadata{Name: "demo"}}
	stack.SetDefaults()

	desired, err := desiredResourceInventory(stack)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := desired.configMaps[kafkaConnectInventoryConfigMapName]; !ok {
		t.Fatal("Kafka Connect ownership inventory should be retained")
	}
}
