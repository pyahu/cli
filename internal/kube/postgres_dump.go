package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/pyahu/cli/pkg/schema"
)

type PostgresRestoreOptions struct {
	Clean bool
}

func (c *Client) BackupPostgres(ctx context.Context, stack *schema.Stack, database string, out io.Writer) error {
	if !stack.PostgresEnabled() {
		return fmt.Errorf("services.postgres is not enabled")
	}
	if strings.TrimSpace(database) == "" {
		return fmt.Errorf("database is required")
	}
	command := []string{
		"sh",
		"-ec",
		fmt.Sprintf(`PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U "$POSTGRES_USER" -d %s --format=custom --no-owner --no-acl`, shellQuote(database)),
	}
	return c.execPostgresPrimary(ctx, stack, command, nil, out)
}

func (c *Client) RestorePostgres(ctx context.Context, stack *schema.Stack, database string, in io.Reader, opts PostgresRestoreOptions) error {
	if !stack.PostgresEnabled() {
		return fmt.Errorf("services.postgres is not enabled")
	}
	if strings.TrimSpace(database) == "" {
		return fmt.Errorf("database is required")
	}
	args := []string{
		`PGPASSWORD="$POSTGRES_PASSWORD"`,
		"pg_restore",
		`-U "$POSTGRES_USER"`,
		"-d " + shellQuote(database),
		"--no-owner",
		"--no-acl",
		"--single-transaction",
	}
	if opts.Clean {
		args = append(args, "--clean", "--if-exists")
	}
	command := []string{"sh", "-ec", strings.Join(args, " ")}
	return c.execPostgresPrimary(ctx, stack, command, in, io.Discard)
}

func (c *Client) execPostgresPrimary(ctx context.Context, stack *schema.Stack, command []string, stdin io.Reader, stdout io.Writer) error {
	if c.restConfig == nil {
		return fmt.Errorf("Kubernetes REST config is not available")
	}
	pod, err := c.postgresPrimaryPod(ctx, stack)
	if err != nil {
		return err
	}
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(stack.Cluster.Namespace).
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "postgres",
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("create pod exec stream: %w", err)
	}
	var stderr bytes.Buffer
	if stdout == nil {
		stdout = io.Discard
	}
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: &stderr,
		Tty:    false,
	}); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("postgres exec failed: %w: %s", err, message)
		}
		return fmt.Errorf("postgres exec failed: %w", err)
	}
	return nil
}

func (c *Client) postgresPrimaryPod(ctx context.Context, stack *schema.Stack) (*corev1.Pod, error) {
	selector := labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/name": "postgres",
		"pyahu.io/stack":         stack.Metadata.Name,
		"pyahu.io/postgres-role": "primary",
	}).String()
	pods, err := c.clientset.CoreV1().Pods(stack.Cluster.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list postgres pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no postgres primary pod found in namespace %s", stack.Cluster.Namespace)
	}
	for i := range pods.Items {
		if podReady(&pods.Items[i]) {
			return &pods.Items[i], nil
		}
	}
	return nil, fmt.Errorf("postgres primary pod is not ready")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
