package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyKafkaConnect(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "kafka-connect")
	serviceLabels["app.kubernetes.io/name"] = "kafka-connect"
	selector := map[string]string{"app.kubernetes.io/name": "kafka-connect", "pyahu.io/stack": stack.Metadata.Name}

	if err := c.applyKafkaConnectTopicJobs(ctx, stack); err != nil {
		return err
	}
	if err := c.applyService(ctx, kafkaConnectService(namespace, serviceLabels, selector)); err != nil {
		return err
	}
	if err := c.applyDeployment(ctx, kafkaConnectDeployment(stack, serviceLabels, selector)); err != nil {
		return err
	}
	return c.applyKafkaConnectConnectorJobs(ctx, stack)
}

func (c *Client) applyKafkaConnectTopicJobs(ctx context.Context, stack *schema.Stack) error {
	for _, topic := range stack.KafkaConnectInternalTopics() {
		if err := c.applyJob(ctx, kafkaConnectTopicJob(stack, topic)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) applyKafkaConnectConnectorJobs(ctx context.Context, stack *schema.Stack) error {
	serviceLabels := baseLabels(stack, "kafka-connect")
	serviceLabels["app.kubernetes.io/name"] = "kafka-connect"
	for _, connector := range stack.Services.KafkaConnect.Connectors {
		payload, err := kafkaConnectConnectorPayload(stack, connector)
		if err != nil {
			return err
		}
		secret := kafkaConnectConnectorSecret(stack, serviceLabels, connector, payload)
		if err := c.applySecret(ctx, secret); err != nil {
			return err
		}
		if err := c.applyJob(ctx, kafkaConnectConnectorJob(stack, connector, secret.Name, payload)); err != nil {
			return err
		}
	}
	return nil
}

func kafkaConnectTopicJobNames(stack *schema.Stack) []string {
	topics := stack.KafkaConnectInternalTopics()
	names := make([]string, 0, len(topics))
	for _, topic := range topics {
		names = append(names, kafkaConnectTopicJobName(topic.Name))
	}
	return names
}

func kafkaConnectConnectorJobNames(stack *schema.Stack) ([]string, error) {
	names := make([]string, 0, len(stack.Services.KafkaConnect.Connectors))
	for _, connector := range stack.Services.KafkaConnect.Connectors {
		payload, err := kafkaConnectConnectorPayload(stack, connector)
		if err != nil {
			return nil, err
		}
		names = append(names, kafkaConnectConnectorResourceName(connector, payload))
	}
	return names, nil
}

func kafkaConnectService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-connect", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "rest",
				Port:       8083,
				TargetPort: intstr.FromInt(8083),
				NodePort:   nodePortKafkaConnect,
			}},
		},
	}
}

func kafkaConnectDeployment(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-connect", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32p(int32(stack.Services.KafkaConnect.Replicas)),
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "kafka-connect",
						Image: stack.KafkaConnectImage(),
						Ports: []corev1.ContainerPort{{Name: "rest", ContainerPort: 8083}},
						Env: []corev1.EnvVar{
							{Name: "BOOTSTRAP_SERVERS", Value: stack.KafkaInternalBootstrapServers()},
							{Name: "GROUP_ID", Value: stack.Metadata.Name + "-connect"},
							{Name: "CONFIG_STORAGE_TOPIC", Value: stack.KafkaConnectConfigTopic()},
							{Name: "OFFSET_STORAGE_TOPIC", Value: stack.KafkaConnectOffsetTopic()},
							{Name: "STATUS_STORAGE_TOPIC", Value: stack.KafkaConnectStatusTopic()},
							{Name: "HOST_NAME", Value: "0.0.0.0"},
							{Name: "ADVERTISED_HOST_NAME", Value: "kafka-connect"},
							{Name: "ADVERTISED_PORT", Value: "8083"},
							{Name: "KEY_CONVERTER", Value: "org.apache.kafka.connect.json.JsonConverter"},
							{Name: "VALUE_CONVERTER", Value: "org.apache.kafka.connect.json.JsonConverter"},
							{Name: "HEAP_OPTS", Value: "-Xms256m -Xmx768m"},
						},
						ReadinessProbe: httpProbe("/connectors", 8083, 30, 10),
						LivenessProbe:  httpProbe("/", 8083, 60, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
						},
					}},
				},
			},
		},
	}
}

func kafkaConnectTopicJob(stack *schema.Stack, topic schema.TopicConfig) *batchv1.Job {
	name := kafkaConnectTopicJobName(topic.Name)
	labels := baseLabels(stack, name)
	labels["app.kubernetes.io/name"] = "kafka-connect-topic"
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stack.Cluster.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32p(6),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:    "create-topic",
						Image:   "apache/kafka:" + stack.Services.Kafka.Version,
						Command: []string{"sh", "-ec"},
						Args: []string{fmt.Sprintf(
							"/opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka.%s.svc.cluster.local:9092 --create --if-not-exists --topic %s --partitions %d --replication-factor %d --config cleanup.policy=compact",
							stack.Cluster.Namespace,
							topic.Name,
							topic.Partitions,
							topic.Replicas,
						)},
					}},
				},
			},
		},
	}
}

func kafkaConnectTopicJobName(topicName string) string {
	return "kafka-connect-topic-" + kubeName(topicName, 34)
}

func kafkaConnectConnectorSecret(stack *schema.Stack, serviceLabels map[string]string, connector schema.KafkaConnectConnector, payload string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: kafkaConnectConnectorResourceName(connector, payload), Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"connector.json": payload},
	}
}

func kafkaConnectConnectorJob(stack *schema.Stack, connector schema.KafkaConnectConnector, secretName string, payload string) *batchv1.Job {
	name := kafkaConnectConnectorResourceName(connector, payload)
	labels := baseLabels(stack, name)
	labels["app.kubernetes.io/name"] = "kafka-connect-connector"
	script := fmt.Sprintf(`until curl -fsS %[1]s/connectors >/dev/null; do sleep 2; done
curl -fsS -X PUT -H 'Content-Type: application/json' --data-binary @/connector/connector.json %[1]s/connectors/%[2]s/config
for i in $(seq 1 60); do
  status="$(curl -fsS %[1]s/connectors/%[2]s/status)"
  echo "$status"
  if printf '%%s' "$status" | grep -q '"state":"FAILED"'; then exit 1; fi
  running_count="$(printf '%%s' "$status" | grep -o '"state":"RUNNING"' | wc -l | tr -d ' ')"
  if [ "$running_count" -ge 2 ]; then exit 0; fi
  sleep 2
done
echo "connector %[2]s did not become healthy" >&2
exit 1
`, stack.KafkaConnectInternalURL(), connector.Name)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stack.Cluster.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32p(6),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:         "apply-connector",
						Image:        stack.KafkaConnectImage(),
						Command:      []string{"sh", "-ec"},
						Args:         []string{script},
						VolumeMounts: []corev1.VolumeMount{{Name: "connector", MountPath: "/connector", ReadOnly: true}},
					}},
					Volumes: []corev1.Volume{{
						Name: "connector",
						VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
							SecretName: secretName,
						}},
					}},
				},
			},
		},
	}
}

func kafkaConnectConnectorPayload(stack *schema.Stack, connector schema.KafkaConnectConnector) (string, error) {
	data, err := json.MarshalIndent(stack.KafkaConnectConnectorConfig(connector), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func kafkaConnectConnectorResourceName(connector schema.KafkaConnectConnector, payload string) string {
	hash := sha256Hex(payload)
	return "kafka-connect-" + kubeName(connector.Name, 33) + "-" + hash[:8]
}

func kubeName(value string, max int) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		name = "item"
	}
	if len(name) > max {
		name = strings.TrimRight(name[:max], "-")
	}
	return name
}
