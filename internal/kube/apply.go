package kube

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/pyahu/cli/pkg/schema"
)

const (
	nodePortPostgres           = 30543
	nodePortPostgresRead       = 30544
	nodePortKafka              = 30092
	nodePortKafkaConnect       = 30083
	nodePortRabbitMQ           = 30672
	nodePortRabbitMQManagement = 31672
)

func (c *Client) applyConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	existing, err := c.clientset.CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.CoreV1().ConfigMaps(cm.Namespace).Create(ctx, cm, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	cm.ResourceVersion = existing.ResourceVersion
	_, err = c.clientset.CoreV1().ConfigMaps(cm.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (c *Client) applySecret(ctx context.Context, secret *corev1.Secret) error {
	existing, err := c.clientset.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		normalizeSecretData(secret)
		_, err = c.clientset.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	secret.ResourceVersion = existing.ResourceVersion
	normalizeSecretData(secret)
	_, err = c.clientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

func normalizeSecretData(secret *corev1.Secret) {
	if len(secret.StringData) == 0 {
		return
	}
	secret.Data = map[string][]byte{}
	for key, value := range secret.StringData {
		secret.Data[key] = []byte(value)
	}
	secret.StringData = nil
}

func (c *Client) applyService(ctx context.Context, service *corev1.Service) error {
	existing, err := c.clientset.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.CoreV1().Services(service.Namespace).Create(ctx, service, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	service.ResourceVersion = existing.ResourceVersion
	service.Spec.ClusterIP = existing.Spec.ClusterIP
	service.Spec.ClusterIPs = existing.Spec.ClusterIPs
	service.Spec.IPFamilies = existing.Spec.IPFamilies
	service.Spec.IPFamilyPolicy = existing.Spec.IPFamilyPolicy
	_, err = c.clientset.CoreV1().Services(service.Namespace).Update(ctx, service, metav1.UpdateOptions{})
	return err
}

func (c *Client) deleteServiceIfExists(ctx context.Context, namespace string, name string) error {
	err := c.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) applyStatefulSet(ctx context.Context, sts *appsv1.StatefulSet) error {
	existing, err := c.clientset.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.AppsV1().StatefulSets(sts.Namespace).Create(ctx, sts, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	sts.ResourceVersion = existing.ResourceVersion
	_, err = c.clientset.AppsV1().StatefulSets(sts.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}

func (c *Client) deleteStatefulSetIfExists(ctx context.Context, namespace string, name string) error {
	err := c.clientset.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) applyDeployment(ctx context.Context, deploy *appsv1.Deployment) error {
	existing, err := c.clientset.AppsV1().Deployments(deploy.Namespace).Get(ctx, deploy.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.AppsV1().Deployments(deploy.Namespace).Create(ctx, deploy, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	deploy.ResourceVersion = existing.ResourceVersion
	_, err = c.clientset.AppsV1().Deployments(deploy.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	return err
}

func (c *Client) applyIngress(ctx context.Context, ingress *networkingv1.Ingress) error {
	existing, err := c.clientset.NetworkingV1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.NetworkingV1().Ingresses(ingress.Namespace).Create(ctx, ingress, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	ingress.ResourceVersion = existing.ResourceVersion
	_, err = c.clientset.NetworkingV1().Ingresses(ingress.Namespace).Update(ctx, ingress, metav1.UpdateOptions{})
	return err
}

func (c *Client) applyJob(ctx context.Context, job *batchv1.Job) error {
	existing, err := c.clientset.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	if existing.Status.Succeeded > 0 {
		return nil
	}
	propagation := metav1.DeletePropagationBackground
	if err := c.clientset.BatchV1().Jobs(job.Namespace).Delete(ctx, job.Name, metav1.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return wait.PollUntilContextTimeout(ctx, time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := c.clientset.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			_, err = c.clientset.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
			return err == nil, err
		}
		return false, nil
	})
}

// httpIngress builds a Traefik Ingress for a plain HTTP backend served under a
// *.localhost hostname, with optional TLS using the shared local certificate.
func httpIngress(name string, namespace string, labels map[string]string, host string, serviceName string, servicePort int32, tlsEnabled bool, secretName string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: serviceName,
						Port: networkingv1.ServiceBackendPort{Number: servicePort},
					}},
				}}}},
			}},
		},
	}
	if tlsEnabled {
		ingress.Spec.TLS = []networkingv1.IngressTLS{{Hosts: []string{host}, SecretName: secretName}}
	}
	return ingress
}

func baseLabels(stack *schema.Stack, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/part-of":    "pyahu",
		"app.kubernetes.io/managed-by": "pyahu-cli",
		"app.kubernetes.io/component":  component,
		"pyahu.io/stack":               stack.Metadata.Name,
	}
}

func merge(left map[string]string, right map[string]string) map[string]string {
	if left == nil {
		left = map[string]string{}
	}
	for key, value := range right {
		left[key] = value
	}
	return left
}

func podReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func secretRef(name string, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}}
}

func execProbe(command []string, initialDelay int32, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: command}},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
	}
}

func tcpProbe(port int, initialDelay int32, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(port)}},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
	}
}

func httpProbe(path string, port int, initialDelay int32, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
			Path: path,
			Port: intstr.FromInt(port),
		}},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
	}
}

func int32p(v int32) *int32 {
	return &v
}
