package kube

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pyahu/cli/pkg/schema"
)

type Client struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
}

type ServiceStatus struct {
	Name    string      `json:"name"`
	Enabled bool        `json:"enabled"`
	Ready   bool        `json:"ready"`
	Pods    []PodStatus `json:"pods,omitempty"`
	Message string      `json:"message,omitempty"`
}

type PodStatus struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Phase  string `json:"phase"`
	Reason string `json:"reason,omitempty"`
}

func New(kubeconfig string) (*Client, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("build Kubernetes config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return &Client{clientset: clientset, restConfig: restConfig}, nil
}

func (c *Client) WaitForAPI(ctx context.Context, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, err := c.clientset.Discovery().ServerVersion()
		return err == nil, nil
	})
}

func (c *Client) ApplyStack(ctx context.Context, stack *schema.Stack, stackDir string) error {
	if err := c.applyNamespace(ctx, stack); err != nil {
		return err
	}
	if err := c.applyCredentials(ctx, stack); err != nil {
		return err
	}
	if err := c.applyUserConfigMaps(ctx, stack, stackDir); err != nil {
		return err
	}
	if err := c.applyUserSecrets(ctx, stack, stackDir); err != nil {
		return err
	}
	if err := c.applyLocalTLS(ctx, stack, stackDir); err != nil {
		return err
	}
	if stack.PostgresEnabled() {
		if err := c.applyPostgres(ctx, stack, stackDir); err != nil {
			return err
		}
	}
	if stack.RabbitMQEnabled() {
		if err := c.applyRabbitMQ(ctx, stack); err != nil {
			return err
		}
	}
	if stack.KafkaEnabled() {
		if err := c.applyKafka(ctx, stack); err != nil {
			return err
		}
	}
	if stack.KafkaConnectEnabled() {
		if err := c.applyKafkaConnect(ctx, stack); err != nil {
			return err
		}
	}
	if stack.KafkaUIEnabled() {
		if err := c.applyKafkaUI(ctx, stack); err != nil {
			return err
		}
	}
	if stack.ZitadelEnabled() {
		if err := c.applyZitadel(ctx, stack); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) DeleteNamespace(ctx context.Context, namespace string) error {
	err := c.clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) WaitForStack(ctx context.Context, stack *schema.Stack) error {
	if stack.PostgresEnabled() {
		if err := c.WaitForService(ctx, stack.Cluster.Namespace, "postgres", 3*time.Minute); err != nil {
			return fmt.Errorf("wait for postgres: %w", err)
		}
	}
	if stack.RabbitMQEnabled() {
		if err := c.WaitForService(ctx, stack.Cluster.Namespace, "rabbitmq", 3*time.Minute); err != nil {
			return fmt.Errorf("wait for rabbitmq: %w", err)
		}
	}
	if stack.KafkaEnabled() {
		if err := c.WaitForService(ctx, stack.Cluster.Namespace, "kafka", 4*time.Minute); err != nil {
			return fmt.Errorf("wait for kafka: %w", err)
		}
		if err := c.applyKafkaTopicJobs(ctx, stack); err != nil {
			return fmt.Errorf("apply kafka topic jobs: %w", err)
		}
		if err := c.waitForJobs(ctx, stack.Cluster.Namespace, kafkaTopicJobNames(stack), 2*time.Minute); err != nil {
			return fmt.Errorf("wait for kafka topic jobs: %w", err)
		}
		if stack.KafkaConnectEnabled() {
			if err := c.applyKafkaConnectTopicJobs(ctx, stack); err != nil {
				return fmt.Errorf("apply kafka connect topic jobs: %w", err)
			}
			if err := c.waitForJobs(ctx, stack.Cluster.Namespace, kafkaConnectTopicJobNames(stack), 2*time.Minute); err != nil {
				return fmt.Errorf("wait for kafka connect topic jobs: %w", err)
			}
		}
	}
	if stack.KafkaConnectEnabled() {
		if err := c.WaitForService(ctx, stack.Cluster.Namespace, "kafka-connect", 4*time.Minute); err != nil {
			return fmt.Errorf("wait for kafka-connect: %w", err)
		}
		if err := c.applyKafkaConnectConnectorJobs(ctx, stack); err != nil {
			return fmt.Errorf("apply kafka connect connector jobs: %w", err)
		}
		names, err := kafkaConnectConnectorJobNames(stack)
		if err != nil {
			return err
		}
		if err := c.waitForJobs(ctx, stack.Cluster.Namespace, names, 2*time.Minute); err != nil {
			return fmt.Errorf("wait for kafka connect connector jobs: %w", err)
		}
	}
	if stack.KafkaUIEnabled() {
		if err := c.WaitForService(ctx, stack.Cluster.Namespace, "kafka-ui", 3*time.Minute); err != nil {
			return fmt.Errorf("wait for kafka-ui: %w", err)
		}
	}
	if stack.ZitadelEnabled() {
		if err := c.WaitForService(ctx, stack.Cluster.Namespace, "zitadel", 6*time.Minute); err != nil {
			return fmt.Errorf("wait for zitadel: %w", err)
		}
	}
	return nil
}

func (c *Client) WaitForService(ctx context.Context, namespace string, service string, timeout time.Duration) error {
	selector := labels.SelectorFromSet(labels.Set{"app.kubernetes.io/name": service}).String()
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return false, nil
		}
		if len(pods.Items) == 0 {
			return false, nil
		}
		for _, pod := range pods.Items {
			if !podReady(&pod) {
				return false, nil
			}
		}
		return true, nil
	})
}

func (c *Client) Status(ctx context.Context, stack *schema.Stack) ([]ServiceStatus, error) {
	services := []struct {
		name    string
		enabled bool
	}{
		{name: "postgres", enabled: stack.PostgresEnabled()},
		{name: "zitadel", enabled: stack.ZitadelEnabled()},
		{name: "rabbitmq", enabled: stack.RabbitMQEnabled()},
		{name: "kafka", enabled: stack.KafkaEnabled()},
		{name: "kafka-connect", enabled: stack.KafkaConnectEnabled()},
		{name: "kafka-ui", enabled: stack.KafkaUIEnabled()},
	}

	statuses := make([]ServiceStatus, 0, len(services))
	for _, service := range services {
		status := ServiceStatus{Name: service.name, Enabled: service.enabled}
		if !service.enabled {
			status.Message = "disabled"
			statuses = append(statuses, status)
			continue
		}
		pods, err := c.podStatuses(ctx, stack.Cluster.Namespace, service.name)
		if err != nil {
			status.Message = err.Error()
			statuses = append(statuses, status)
			continue
		}
		status.Pods = pods
		status.Ready = len(pods) > 0
		for _, pod := range pods {
			if !pod.Ready {
				status.Ready = false
				break
			}
		}
		if status.Ready {
			status.Message = "ready"
		} else if len(pods) == 0 {
			status.Message = "no pods"
		} else {
			status.Message = "waiting"
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (c *Client) Logs(ctx context.Context, namespace string, service string, follow bool, tail int64) (io.ReadCloser, error) {
	selector := labels.SelectorFromSet(labels.Set{"app.kubernetes.io/name": service}).String()
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for service %s", service)
	}
	sort.Slice(pods.Items, func(i, j int) bool {
		return pods.Items[i].CreationTimestamp.Before(&pods.Items[j].CreationTimestamp)
	})
	opts := &corev1.PodLogOptions{Follow: follow, TailLines: &tail}
	return c.clientset.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, opts).Stream(ctx)
}

func (c *Client) waitForJobs(ctx context.Context, namespace string, names []string, timeout time.Duration) error {
	for _, name := range names {
		if err := c.waitForJob(ctx, namespace, name, timeout); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) waitForJob(ctx context.Context, namespace string, name string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		job, err := c.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if jobComplete(job) {
			return true, nil
		}
		if err := jobFailure(job); err != nil {
			return false, err
		}
		return false, nil
	})
}

func jobComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return job.Status.Succeeded > 0
}

func jobFailure(job *batchv1.Job) error {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return fmt.Errorf("job %s failed: %s", job.Name, jobConditionMessage(condition))
		}
	}
	if job.Spec.BackoffLimit != nil && job.Status.Failed > *job.Spec.BackoffLimit {
		return fmt.Errorf("job %s failed after %d attempts", job.Name, job.Status.Failed)
	}
	return nil
}

func jobConditionMessage(condition batchv1.JobCondition) string {
	parts := []string{}
	if condition.Reason != "" {
		parts = append(parts, condition.Reason)
	}
	if condition.Message != "" {
		parts = append(parts, condition.Message)
	}
	if len(parts) == 0 {
		return "unknown reason"
	}
	return strings.Join(parts, ": ")
}

func (c *Client) podStatuses(ctx context.Context, namespace string, service string) ([]PodStatus, error) {
	selector := labels.SelectorFromSet(labels.Set{"app.kubernetes.io/name": service}).String()
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	statuses := make([]PodStatus, 0, len(pods.Items))
	for _, pod := range pods.Items {
		status := PodStatus{Name: pod.Name, Phase: string(pod.Status.Phase), Ready: podReady(&pod)}
		for _, container := range pod.Status.ContainerStatuses {
			if container.State.Waiting != nil {
				status.Reason = container.State.Waiting.Reason
				break
			}
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool { return statuses[i].Name < statuses[j].Name })
	return statuses, nil
}
