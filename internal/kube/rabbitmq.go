package kube

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyRabbitMQ(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "rabbitmq")
	serviceLabels["app.kubernetes.io/name"] = "rabbitmq"
	selector := map[string]string{"app.kubernetes.io/name": "rabbitmq", "pyahu.io/stack": stack.Metadata.Name}
	definitions, err := rabbitMQDefinitions(stack)
	if err != nil {
		return err
	}

	if err := c.applyConfigMap(ctx, rabbitMQConfigMap(namespace, serviceLabels)); err != nil {
		return err
	}
	secret := rabbitMQDefinitionsSecret(stack, serviceLabels, definitions)
	if err := c.applySecret(ctx, secret); err != nil {
		return err
	}
	if err := c.applyService(ctx, rabbitMQService(namespace, serviceLabels, selector)); err != nil {
		return err
	}
	if err := c.applyStatefulSet(ctx, rabbitMQStatefulSet(stack, serviceLabels, selector, definitions)); err != nil {
		return err
	}
	if stack.RabbitMQManagementEnabled() {
		return c.applyIngress(ctx, httpIngress("rabbitmq", namespace, serviceLabels, "rabbitmq.localhost", "rabbitmq", 15672, stack.LocalTLSEnabled(), stack.LocalTLSSecretName()))
	}
	return nil
}

func rabbitMQConfigMap(namespace string, serviceLabels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "rabbitmq-config", Namespace: namespace, Labels: serviceLabels},
		Data: map[string]string{
			"10-pyahu.conf": "definitions.import_backend = local_filesystem\ndefinitions.local.path = /etc/rabbitmq/definitions.json\nloopback_users = none\n",
		},
	}
}

func rabbitMQDefinitionsSecret(stack *schema.Stack, serviceLabels map[string]string, definitions string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "rabbitmq-definitions", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"definitions.json": definitions},
	}
}

func rabbitMQService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "rabbitmq", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{
				{Name: "amqp", Port: 5672, TargetPort: intstr.FromInt(5672), NodePort: nodePortRabbitMQ},
				{Name: "management", Port: 15672, TargetPort: intstr.FromInt(15672), NodePort: nodePortRabbitMQManagement},
			},
		},
	}
}

func rabbitMQStatefulSet(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string, definitions string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rabbitmq", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "rabbitmq",
			Replicas:    int32p(int32(stack.Services.RabbitMQ.Replicas)),
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      selector,
					Annotations: map[string]string{"pyahu.io/rabbitmq-definitions-sha256": sha256Hex(definitions)},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "rabbitmq",
						Image: "rabbitmq:" + stack.Services.RabbitMQ.Version,
						Ports: []corev1.ContainerPort{
							{Name: "amqp", ContainerPort: 5672},
							{Name: "management", ContainerPort: 15672},
						},
						Env: []corev1.EnvVar{
							{Name: "RABBITMQ_DEFAULT_USER", ValueFrom: secretRef("pyahu-local-credentials", "RABBITMQ_USER")},
							{Name: "RABBITMQ_DEFAULT_PASS", ValueFrom: secretRef("pyahu-local-credentials", "RABBITMQ_PASSWORD")},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/var/lib/rabbitmq"},
							{Name: "definitions", MountPath: "/etc/rabbitmq/definitions.json", SubPath: "definitions.json", ReadOnly: true},
							{Name: "config", MountPath: "/etc/rabbitmq/conf.d/10-pyahu.conf", SubPath: "10-pyahu.conf", ReadOnly: true},
						},
						ReadinessProbe: tcpProbe(5672, 10, 10),
						LivenessProbe:  tcpProbe(5672, 30, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "definitions",
							VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
								SecretName: "rabbitmq-definitions",
							}},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "rabbitmq-config"},
							}},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(stack.Services.RabbitMQ.Storage)}},
				},
			}},
		},
	}
}

type rabbitMQDefinitionFile struct {
	Users       []rabbitMQDefinitionUser       `json:"users"`
	VHosts      []rabbitMQDefinitionVHost      `json:"vhosts"`
	Permissions []rabbitMQDefinitionPermission `json:"permissions"`
	Parameters  []any                          `json:"parameters"`
	Policies    []any                          `json:"policies"`
	Queues      []any                          `json:"queues"`
	Exchanges   []any                          `json:"exchanges"`
	Bindings    []any                          `json:"bindings"`
}

type rabbitMQDefinitionUser struct {
	Name             string   `json:"name"`
	PasswordHash     string   `json:"password_hash"`
	HashingAlgorithm string   `json:"hashing_algorithm"`
	Tags             []string `json:"tags"`
}

type rabbitMQDefinitionVHost struct {
	Name string `json:"name"`
}

type rabbitMQDefinitionPermission struct {
	User      string `json:"user"`
	VHost     string `json:"vhost"`
	Configure string `json:"configure"`
	Write     string `json:"write"`
	Read      string `json:"read"`
}

func rabbitMQDefinitions(stack *schema.Stack) (string, error) {
	if stack.Services.RabbitMQ == nil {
		return "", fmt.Errorf("rabbitmq service is not configured")
	}
	definitions := rabbitMQDefinitionFile{
		Users:       []rabbitMQDefinitionUser{},
		VHosts:      []rabbitMQDefinitionVHost{},
		Permissions: []rabbitMQDefinitionPermission{},
		Parameters:  []any{},
		Policies:    []any{},
		Queues:      []any{},
		Exchanges:   []any{},
		Bindings:    []any{},
	}
	for _, vhost := range stack.Services.RabbitMQ.VHosts {
		definitions.VHosts = append(definitions.VHosts, rabbitMQDefinitionVHost{Name: vhost.Name})
	}
	for _, user := range stack.Services.RabbitMQ.Users {
		definitions.Users = append(definitions.Users, rabbitMQDefinitionUser{
			Name:             user.Name,
			PasswordHash:     rabbitMQPasswordHash(user.Name, user.Password),
			HashingAlgorithm: "rabbit_password_hashing_sha256",
			Tags:             rabbitMQTags(user.Tags),
		})
		for _, permission := range user.Permissions {
			definitions.Permissions = append(definitions.Permissions, rabbitMQDefinitionPermission{
				User:      user.Name,
				VHost:     permission.VHost,
				Configure: permission.Configure,
				Write:     permission.Write,
				Read:      permission.Read,
			})
		}
	}
	data, err := json.MarshalIndent(definitions, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func rabbitMQPasswordHash(username string, password string) string {
	seed := sha256.Sum256([]byte(username + "\x00" + password))
	salt := seed[:4]
	input := append([]byte{}, salt...)
	input = append(input, []byte(password)...)

	digest := sha256.Sum256(input)
	hash := append([]byte{}, salt...)
	hash = append(hash, digest[:]...)
	return base64.StdEncoding.EncodeToString(hash)
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

func rabbitMQTags(raw string) []string {
	var tags []string
	for _, tag := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' '
	}) {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return []string{"administrator"}
	}
	return tags
}
