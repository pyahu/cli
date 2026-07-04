package kube

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/pyahu/cli/pkg/schema"
)

// CaptureZitadelPAT reads the FirstInstance service-account PAT that ZITADEL
// wrote to the shared emptyDir and stores it in the iam-admin-pat Secret so
// headless provisioning tools can authenticate to the management API. It is
// idempotent: once the Secret holds a PAT the call is a no-op, which keeps the
// credential durable across pod restarts (the emptyDir is lost, FirstInstance
// does not re-run, but the Secret persists).
func (c *Client) CaptureZitadelPAT(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace

	if existing, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, zitadelPATSecretName, metav1.GetOptions{}); err == nil {
		if len(existing.Data["pat"]) > 0 {
			return nil
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	var pat string
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := c.zitadelPodName(ctx, namespace)
		if err != nil {
			return false, nil
		}
		out, err := c.execCapture(ctx, namespace, pod, zitadelSABootstrapContainer, []string{"cat", zitadelPATPath})
		if err != nil {
			return false, nil
		}
		out = strings.TrimSpace(out)
		if out == "" {
			return false, nil
		}
		pat = out
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("read zitadel service-account PAT: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: zitadelPATSecretName, Namespace: namespace, Labels: baseLabels(stack, "zitadel")},
		Data:       map[string][]byte{"pat": []byte(pat)},
	}
	return c.applySecret(ctx, secret)
}

func (c *Client) zitadelPodName(ctx context.Context, namespace string) (string, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=zitadel"})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if podReady(&pod) {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no ready zitadel pod")
}

func (c *Client) execCapture(ctx context.Context, namespace, pod, container string, command []string) (string, error) {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).Namespace(namespace).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr}); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
