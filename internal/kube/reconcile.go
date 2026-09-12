package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/pyahu/cli/pkg/schema"
)

type resourceInventory struct {
	configMaps   map[string]struct{}
	secrets      map[string]struct{}
	services     map[string]struct{}
	statefulSets map[string]struct{}
	deployments  map[string]struct{}
	ingresses    map[string]struct{}
	jobs         map[string]struct{}
}

func newResourceInventory() resourceInventory {
	return resourceInventory{
		configMaps:   map[string]struct{}{},
		secrets:      map[string]struct{}{},
		services:     map[string]struct{}{},
		statefulSets: map[string]struct{}{},
		deployments:  map[string]struct{}{},
		ingresses:    map[string]struct{}{},
		jobs:         map[string]struct{}{},
	}
}

func desiredResourceInventory(stack *schema.Stack) (resourceInventory, error) {
	desired := newResourceInventory()
	desired.secrets["pyahu-local-credentials"] = struct{}{}

	for name := range stack.ConfigMaps {
		desired.configMaps[name] = struct{}{}
	}
	for name := range stack.Secrets {
		desired.secrets[name] = struct{}{}
	}
	if stack.LocalTLSRequired() {
		desired.secrets[stack.LocalTLSSecretName()] = struct{}{}
		desired.configMaps[stack.LocalTLSCAConfigMapName()] = struct{}{}
	}
	if stack.PostgresEnabled() {
		addNames(desired.configMaps, "postgres-init", "postgres-config")
		desired.services["postgres"] = struct{}{}
		desired.statefulSets["postgres"] = struct{}{}
		if stack.PostgresReadReplicas() > 0 {
			desired.services["postgres-read"] = struct{}{}
			desired.statefulSets["postgres-read"] = struct{}{}
		}
	}
	if stack.RabbitMQEnabled() {
		desired.configMaps["rabbitmq-config"] = struct{}{}
		desired.secrets["rabbitmq-definitions"] = struct{}{}
		desired.services["rabbitmq"] = struct{}{}
		desired.statefulSets["rabbitmq"] = struct{}{}
		if stack.RabbitMQManagementEnabled() {
			desired.ingresses["rabbitmq"] = struct{}{}
		}
	}
	if stack.RedisEnabled() {
		desired.services["redis"] = struct{}{}
		desired.statefulSets["redis"] = struct{}{}
	}
	if stack.KafkaEnabled() {
		addNames(desired.services, "kafka-headless", "kafka", "kafka-external")
		desired.statefulSets["kafka"] = struct{}{}
		addNames(desired.jobs, kafkaTopicJobNames(stack)...)
	}
	if stack.KafkaConnectEnabled() {
		desired.secrets[kafkaConnectPluginFilesSecretName] = struct{}{}
		desired.services["kafka-connect"] = struct{}{}
		desired.deployments["kafka-connect"] = struct{}{}
		addNames(desired.jobs, kafkaConnectTopicJobNames(stack)...)
		connectorNames, err := kafkaConnectConnectorJobNames(stack)
		if err != nil {
			return resourceInventory{}, err
		}
		addNames(desired.jobs, connectorNames...)
		addNames(desired.secrets, connectorNames...)
	}
	if stack.KafkaUIEnabled() {
		desired.services["kafka-ui"] = struct{}{}
		desired.deployments["kafka-ui"] = struct{}{}
		desired.ingresses["kafka-ui"] = struct{}{}
	}
	if stack.ZitadelEnabled() {
		desired.services["zitadel"] = struct{}{}
		desired.deployments["zitadel"] = struct{}{}
		addNames(desired.ingresses, "zitadel", "zitadel-login")
	}
	return desired, nil
}

func addNames(set map[string]struct{}, names ...string) {
	for _, name := range names {
		set[name] = struct{}{}
	}
}

// pruneObsoleteResources removes namespaced Kubernetes objects previously
// created for this stack but no longer present in its desired configuration.
// PersistentVolumeClaims are deliberately retained to avoid deleting local data
// when a stateful service is temporarily disabled.
func (c *Client) pruneObsoleteResources(ctx context.Context, stack *schema.Stack) error {
	desired, err := desiredResourceInventory(stack)
	if err != nil {
		return fmt.Errorf("build desired resource inventory: %w", err)
	}
	namespace := stack.Cluster.Namespace
	selector := labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/managed-by": "pyahu-cli",
		"pyahu.io/stack":               stack.Metadata.Name,
	}).String()
	listOptions := metav1.ListOptions{LabelSelector: selector}
	deleteOptions := metav1.DeleteOptions{}

	jobs, err := c.clientset.BatchV1().Jobs(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed jobs: %w", err)
	}
	for _, item := range jobs.Items {
		if _, ok := desired.jobs[item.Name]; !ok {
			if err := c.clientset.BatchV1().Jobs(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete job %s: %w", item.Name, err)
			}
		}
	}

	ingresses, err := c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed ingresses: %w", err)
	}
	for _, item := range ingresses.Items {
		if _, ok := desired.ingresses[item.Name]; !ok {
			if err := c.clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete ingress %s: %w", item.Name, err)
			}
		}
	}

	deployments, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed deployments: %w", err)
	}
	for _, item := range deployments.Items {
		if _, ok := desired.deployments[item.Name]; !ok {
			if err := c.clientset.AppsV1().Deployments(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete deployment %s: %w", item.Name, err)
			}
		}
	}

	statefulSets, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed statefulsets: %w", err)
	}
	for _, item := range statefulSets.Items {
		if _, ok := desired.statefulSets[item.Name]; !ok {
			if err := c.clientset.AppsV1().StatefulSets(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete statefulset %s: %w", item.Name, err)
			}
		}
	}

	services, err := c.clientset.CoreV1().Services(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed services: %w", err)
	}
	for _, item := range services.Items {
		if _, ok := desired.services[item.Name]; !ok {
			if err := c.clientset.CoreV1().Services(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete service %s: %w", item.Name, err)
			}
		}
	}

	configMaps, err := c.clientset.CoreV1().ConfigMaps(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed configmaps: %w", err)
	}
	for _, item := range configMaps.Items {
		if _, ok := desired.configMaps[item.Name]; !ok {
			if err := c.clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete configmap %s: %w", item.Name, err)
			}
		}
	}

	secrets, err := c.clientset.CoreV1().Secrets(namespace).List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("list managed secrets: %w", err)
	}
	for _, item := range secrets.Items {
		if _, ok := desired.secrets[item.Name]; !ok {
			if err := c.clientset.CoreV1().Secrets(namespace).Delete(ctx, item.Name, deleteOptions); err != nil {
				return fmt.Errorf("delete obsolete secret %s: %w", item.Name, err)
			}
		}
	}

	return nil
}
