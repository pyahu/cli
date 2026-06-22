package kube

import (
	"context"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pyahu/cli/pkg/schema"
)

func TestWaitForJobReturnsFailureCondition(t *testing.T) {
	client := &Client{clientset: fake.NewSimpleClientset(&batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "bootstrap", Namespace: "demo"},
		Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
			Type:    batchv1.JobFailed,
			Status:  corev1.ConditionTrue,
			Reason:  "BackoffLimitExceeded",
			Message: "command exited with status 1",
		}}},
	})}

	err := client.waitForJob(context.Background(), "demo", "bootstrap", time.Second)
	if err == nil {
		t.Fatal("expected job failure")
	}
	for _, want := range []string{"bootstrap", "BackoffLimitExceeded", "command exited with status 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error does not contain %q: %v", want, err)
		}
	}
}

func TestApplyLocalTLSCreatesSecretAndCAConfigMap(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stackDir := t.TempDir()
	stack := &schema.Stack{
		APIVersion: schema.APIVersion,
		Kind:       schema.Kind,
		Metadata:   schema.Metadata{Name: "demo"},
		Services: schema.Services{
			Postgres: &schema.PostgresService{Enabled: schema.Bool(true)},
			Zitadel:  &schema.ZitadelService{Enabled: schema.Bool(true)},
		},
	}
	stack.SetDefaults()
	client := &Client{clientset: fake.NewSimpleClientset()}

	if err := client.applyLocalTLS(context.Background(), stack, stackDir); err != nil {
		t.Fatal(err)
	}

	secret, err := client.clientset.CoreV1().Secrets(stack.Cluster.Namespace).Get(context.Background(), stack.LocalTLSSecretName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if secret.Type != corev1.SecretTypeTLS {
		t.Fatalf("secret type = %q", secret.Type)
	}
	if len(secret.Data[corev1.TLSCertKey]) == 0 || len(secret.Data[corev1.TLSPrivateKeyKey]) == 0 {
		t.Fatalf("secret missing TLS data: %#v", secret.Data)
	}

	cm, err := client.clientset.CoreV1().ConfigMaps(stack.Cluster.Namespace).Get(context.Background(), stack.LocalTLSCAConfigMapName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cm.Data["ca.crt"], "BEGIN CERTIFICATE") {
		t.Fatalf("ConfigMap missing CA certificate: %#v", cm.Data)
	}
}
