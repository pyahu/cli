package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pyahu/cli/internal/connect"
	"github.com/pyahu/cli/pkg/schema"
)

const (
	kafkaConnectInventoryConfigMapName = "pyahu-kafka-connect-inventory"
	kafkaConnectInventoryDataKey       = "connectors"
)

// reconcileKafkaConnectors deletes only registrations previously recorded as
// managed by this stack. Connectors registered manually against the same worker
// are deliberately left alone.
func (c *Client) reconcileKafkaConnectors(ctx context.Context, stack *schema.Stack) error {
	managed, err := c.managedKafkaConnectors(ctx, stack.Cluster.Namespace)
	if err != nil {
		return err
	}
	desiredNames := make([]string, 0, len(stack.Services.KafkaConnect.Connectors))
	desired := make(map[string]struct{}, len(stack.Services.KafkaConnect.Connectors))
	for _, connector := range stack.Services.KafkaConnect.Connectors {
		desiredNames = append(desiredNames, connector.Name)
		desired[connector.Name] = struct{}{}
	}
	sort.Strings(desiredNames)

	client := connect.New(fmt.Sprintf("http://localhost:%d", stack.KafkaConnectPort()))
	for _, name := range managed {
		if _, ok := desired[name]; ok {
			continue
		}
		if err := client.Delete(ctx, name); err != nil {
			return fmt.Errorf("delete obsolete connector %s: %w", name, err)
		}
	}

	labels := baseLabels(stack, "kafka-connect")
	labels["app.kubernetes.io/name"] = "kafka-connect"
	return c.applyConfigMap(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kafkaConnectInventoryConfigMapName,
			Namespace: stack.Cluster.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{kafkaConnectInventoryDataKey: strings.Join(desiredNames, "\n")},
	})
}

func (c *Client) managedKafkaConnectors(ctx context.Context, namespace string) ([]string, error) {
	inventory, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, kafkaConnectInventoryConfigMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Kafka Connect ownership inventory: %w", err)
	}
	names := strings.Fields(inventory.Data[kafkaConnectInventoryDataKey])
	sort.Strings(names)
	return names, nil
}
