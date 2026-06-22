package kube

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyKafka(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "kafka")
	serviceLabels["app.kubernetes.io/name"] = "kafka"
	selector := map[string]string{"app.kubernetes.io/name": "kafka", "pyahu.io/stack": stack.Metadata.Name}

	for _, svc := range kafkaServices(namespace, serviceLabels, selector) {
		if err := c.applyService(ctx, svc); err != nil {
			return err
		}
	}
	if err := c.applyStatefulSet(ctx, kafkaStatefulSet(stack, serviceLabels, selector)); err != nil {
		return err
	}
	return c.applyKafkaTopicJobs(ctx, stack)
}

func kafkaServices(namespace string, serviceLabels map[string]string, selector map[string]string) []*corev1.Service {
	return []*corev1.Service{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "kafka-headless", Namespace: namespace, Labels: serviceLabels},
			Spec: corev1.ServiceSpec{
				ClusterIP: "None",
				Selector:  selector,
				Ports: []corev1.ServicePort{
					{Name: "plaintext", Port: 9092, TargetPort: intstr.FromInt(9092)},
					{Name: "controller", Port: 9093, TargetPort: intstr.FromInt(9093)},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "kafka", Namespace: namespace, Labels: serviceLabels},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports:    []corev1.ServicePort{{Name: "plaintext", Port: 9092, TargetPort: intstr.FromInt(9092)}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "kafka-external", Namespace: namespace, Labels: serviceLabels},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeNodePort,
				Selector: selector,
				Ports:    []corev1.ServicePort{{Name: "external", Port: 9094, TargetPort: intstr.FromInt(9094), NodePort: nodePortKafka}},
			},
		},
	}
}

func kafkaStatefulSet(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.StatefulSet {
	namespace := stack.Cluster.Namespace
	internalHost := fmt.Sprintf("kafka.%s.svc.cluster.local", namespace)
	controllerHost := fmt.Sprintf("kafka-0.kafka-headless.%s.svc.cluster.local", namespace)
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka", Namespace: namespace, Labels: serviceLabels},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "kafka-headless",
			Replicas:    int32p(int32(stack.Services.Kafka.Replicas)),
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:  "kafka",
					Image: "apache/kafka:" + stack.Services.Kafka.Version,
					Ports: []corev1.ContainerPort{
						{Name: "plaintext", ContainerPort: 9092},
						{Name: "controller", ContainerPort: 9093},
						{Name: "external", ContainerPort: 9094},
					},
					Env: []corev1.EnvVar{
						{Name: "KAFKA_NODE_ID", Value: "1"},
						{Name: "KAFKA_PROCESS_ROLES", Value: "broker,controller"},
						{Name: "KAFKA_LISTENERS", Value: "PLAINTEXT://:9092,CONTROLLER://:9093,EXTERNAL://:9094"},
						{Name: "KAFKA_ADVERTISED_LISTENERS", Value: fmt.Sprintf("PLAINTEXT://%s:9092,EXTERNAL://localhost:%d", internalHost, stack.KafkaPort())},
						{Name: "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP", Value: "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT,EXTERNAL:PLAINTEXT"},
						{Name: "KAFKA_CONTROLLER_LISTENER_NAMES", Value: "CONTROLLER"},
						{Name: "KAFKA_CONTROLLER_QUORUM_VOTERS", Value: fmt.Sprintf("1@%s:9093", controllerHost)},
						{Name: "KAFKA_INTER_BROKER_LISTENER_NAME", Value: "PLAINTEXT"},
						{Name: "KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR", Value: "1"},
						{Name: "KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR", Value: "1"},
						{Name: "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR", Value: "1"},
						{Name: "KAFKA_AUTO_CREATE_TOPICS_ENABLE", Value: "true"},
						{Name: "KAFKA_LOG_DIRS", Value: "/var/lib/kafka/data"},
						{Name: "CLUSTER_ID", Value: "MkU3OEVBNTcwNTJENDM2Qk"},
					},
					VolumeMounts:   []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/kafka/data"}},
					ReadinessProbe: tcpProbe(9092, 30, 10),
					LivenessProbe:  tcpProbe(9092, 60, 20),
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
						Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("768Mi")},
					},
				}}},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(stack.Services.Kafka.Storage)}},
				},
			}},
		},
	}
}

func (c *Client) applyKafkaTopicJobs(ctx context.Context, stack *schema.Stack) error {
	if len(stack.Services.Kafka.Topics) == 0 {
		return nil
	}
	for _, topic := range stack.Services.Kafka.Topics {
		if err := c.applyJob(ctx, kafkaTopicJob(stack, topic)); err != nil {
			return err
		}
	}
	return nil
}

func kafkaTopicJobNames(stack *schema.Stack) []string {
	names := make([]string, 0, len(stack.Services.Kafka.Topics))
	for _, topic := range stack.Services.Kafka.Topics {
		names = append(names, kafkaTopicJobName(topic.Name))
	}
	return names
}

func kafkaTopicJob(stack *schema.Stack, topic schema.TopicConfig) *batchv1.Job {
	name := kafkaTopicJobName(topic.Name)
	labels := baseLabels(stack, name)
	labels["app.kubernetes.io/name"] = "kafka-topic"
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
						Command: []string{"sh", "-c"},
						Args:    []string{fmt.Sprintf("/opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka.%s.svc.cluster.local:9092 --create --if-not-exists --topic %s --partitions %d --replication-factor %d", stack.Cluster.Namespace, topic.Name, topic.Partitions, topic.Replicas)},
					}},
				},
			},
		},
	}
}

func kafkaTopicJobName(topicName string) string {
	return "kafka-topic-" + kubeName(topicName, 40) + "-" + sha256Hex(topicName)[:8]
}
