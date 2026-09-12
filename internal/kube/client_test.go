package kube

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

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

func TestWaitForServiceReturnsAuthorizationErrorsImmediately(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			runtimeschema.GroupResource{Resource: "pods"},
			"",
			errors.New("access denied"),
		)
	})
	client := &Client{clientset: clientset}

	started := time.Now()
	err := client.WaitForService(context.Background(), "demo", "postgres", time.Minute)
	if err == nil || !apierrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("authorization error was delayed for %s", elapsed)
	}
}

func TestWaitForServicePreservesLastTransientError(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	clientset.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("temporary transport failure")
	})
	client := &Client{clientset: clientset}

	err := client.WaitForService(context.Background(), "demo", "postgres", 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "temporary transport failure") {
		t.Fatalf("last transient error was not preserved: %v", err)
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

func TestApplyStackPrunesObsoleteManagedResources(t *testing.T) {
	const namespace = "demo-dev"
	managedLabels := map[string]string{
		"app.kubernetes.io/managed-by": "pyahu-cli",
		"pyahu.io/stack":               "demo",
	}
	client := &Client{clientset: fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "obsolete", Namespace: namespace, Labels: managedLabels}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "user-owned", Namespace: namespace}},
	)}
	stack := &schema.Stack{
		Metadata: schema.Metadata{Name: "demo"},
		ConfigMaps: map[string]schema.ConfigMapDefinition{
			"desired": {Data: map[string]string{"key": "value"}},
		},
		Secrets: map[string]schema.SecretDefinition{
			"desired": {},
		},
	}
	stack.SetDefaults()

	if err := client.ApplyStack(context.Background(), stack, t.TempDir()); err != nil {
		t.Fatal(err)
	}

	selector := "app.kubernetes.io/managed-by=pyahu-cli,pyahu.io/stack=demo"
	listOptions := metav1.ListOptions{LabelSelector: selector}
	configMaps, err := client.clientset.CoreV1().ConfigMaps(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(configMaps.Items) != 1 || configMaps.Items[0].Name != "desired" {
		t.Fatalf("managed configmaps = %#v", configMaps.Items)
	}
	secrets, err := client.clientset.CoreV1().Secrets(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets.Items) != 2 {
		t.Fatalf("managed secrets = %#v", secrets.Items)
	}
	services, err := client.clientset.CoreV1().Services(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(services.Items) != 0 {
		t.Fatalf("managed services = %#v", services.Items)
	}
	statefulSets, err := client.clientset.AppsV1().StatefulSets(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(statefulSets.Items) != 0 {
		t.Fatalf("managed statefulsets = %#v", statefulSets.Items)
	}
	deployments, err := client.clientset.AppsV1().Deployments(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(deployments.Items) != 0 {
		t.Fatalf("managed deployments = %#v", deployments.Items)
	}
	ingresses, err := client.clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(ingresses.Items) != 0 {
		t.Fatalf("managed ingresses = %#v", ingresses.Items)
	}
	jobs, err := client.clientset.BatchV1().Jobs(namespace).List(context.Background(), listOptions)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("managed jobs = %#v", jobs.Items)
	}
	if _, err := client.clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), "user-owned", metav1.GetOptions{}); err != nil {
		t.Fatalf("unmanaged resource was removed: %v", err)
	}
}
