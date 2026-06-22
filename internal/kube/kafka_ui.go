package kube

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyKafkaUI(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "kafka-ui")
	serviceLabels["app.kubernetes.io/name"] = "kafka-ui"
	selector := map[string]string{"app.kubernetes.io/name": "kafka-ui", "pyahu.io/stack": stack.Metadata.Name}

	if err := c.applyService(ctx, kafkaUIService(namespace, serviceLabels, selector)); err != nil {
		return err
	}
	if err := c.applyDeployment(ctx, kafkaUIDeployment(stack, serviceLabels, selector)); err != nil {
		return err
	}
	return c.applyIngress(ctx, httpIngress("kafka-ui", namespace, serviceLabels, "kafka-ui.localhost", "kafka-ui", 8080, stack.LocalTLSEnabled(), stack.LocalTLSSecretName()))
}

func kafkaUIService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-ui", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
		},
	}
}

func kafkaUIDeployment(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.Deployment {
	env := []corev1.EnvVar{
		{Name: "SERVER_PORT", Value: "8080"},
		{Name: "DYNAMIC_CONFIG_ENABLED", Value: "false"},
		{Name: "KAFKA_CLUSTERS_0_NAME", Value: stack.Metadata.Name},
		{Name: "KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS", Value: stack.KafkaInternalBootstrapServers()},
		{Name: "KAFKA_CLUSTERS_0_READONLY", Value: "false"},
	}
	if stack.KafkaConnectEnabled() {
		env = append(env,
			corev1.EnvVar{Name: "KAFKA_CLUSTERS_0_KAFKACONNECT_0_NAME", Value: "pyahu-connect"},
			corev1.EnvVar{Name: "KAFKA_CLUSTERS_0_KAFKACONNECT_0_ADDRESS", Value: stack.KafkaConnectInternalURL()},
		)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-ui", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32p(int32(stack.Services.KafkaUI.Replicas)),
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:           "kafka-ui",
						Image:          stack.KafkaUIImage(),
						Ports:          []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
						Env:            env,
						ReadinessProbe: tcpProbe(8080, 30, 10),
						LivenessProbe:  tcpProbe(8080, 60, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
						},
					}},
				},
			},
		},
	}
}
