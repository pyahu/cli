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

func (c *Client) applyRedis(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "redis")
	serviceLabels["app.kubernetes.io/name"] = "redis"
	selector := map[string]string{"app.kubernetes.io/name": "redis", "pyahu.io/stack": stack.Metadata.Name}

	if err := c.applyService(ctx, redisService(namespace, serviceLabels, selector)); err != nil {
		return err
	}
	return c.applyStatefulSet(ctx, redisStatefulSet(stack, serviceLabels, selector))
}

func redisService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "client",
				Port:       6379,
				TargetPort: intstr.FromInt(6379),
				NodePort:   nodePortRedis,
			}},
		},
	}
}

func redisStatefulSet(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "redis",
			Replicas:    int32p(1),
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "redis",
						Image: stack.RedisImage(),
						Args:  redisArgs(stack),
						Ports: []corev1.ContainerPort{{Name: "client", ContainerPort: 6379}},
						Env: []corev1.EnvVar{
							{Name: "REDIS_PASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "REDIS_PASSWORD")},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
						// TCP probes keep the manifest image-agnostic: valkey ships
						// valkey-cli, redis ships redis-cli, and neither is guaranteed.
						ReadinessProbe: tcpProbe(6379, 5, 10),
						LivenessProbe:  tcpProbe(6379, 30, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("64Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
						},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(stack.Services.Redis.Storage)}},
				},
			}},
		},
	}
}

// redisArgs builds the server flags. The password is referenced as $(REDIS_PASSWORD)
// so Kubernetes expands it from the credentials Secret instead of writing it into
// the pod spec.
func redisArgs(stack *schema.Stack) []string {
	args := []string{}
	if stack.RedisAppendOnly() {
		args = append(args, "--appendonly", "yes", "--appendfsync", "everysec")
	}
	if stack.RedisPassword() != "" {
		args = append(args, "--requirepass", "$(REDIS_PASSWORD)")
	}
	return args
}
