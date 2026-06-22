package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyNamespace(ctx context.Context, stack *schema.Stack) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   stack.Cluster.Namespace,
			Labels: baseLabels(stack, "namespace"),
		},
	}
	existing, err := c.clientset.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	existing.Labels = merge(existing.Labels, ns.Labels)
	_, err = c.clientset.CoreV1().Namespaces().Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func (c *Client) applyCredentials(ctx context.Context, stack *schema.Stack) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pyahu-local-credentials",
			Namespace: stack.Cluster.Namespace,
			Labels:    baseLabels(stack, "credentials"),
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"POSTGRES_USER":                 stack.PostgresUser(),
			"POSTGRES_PASSWORD":             stack.PostgresPassword(),
			"POSTGRES_REPLICATION_USER":     stack.PostgresReplicationUser(),
			"POSTGRES_REPLICATION_PASSWORD": stack.PostgresReplicationPassword(),
			"RABBITMQ_USER":                 stack.RabbitMQUser(),
			"RABBITMQ_PASSWORD":             stack.RabbitMQPassword(),
			"ZITADEL_ADMIN_USER":            stack.ZitadelAdminUser(),
			"ZITADEL_ADMIN_PASSWORD":        stack.ZitadelAdminPassword(),
			"ZITADEL_MASTERKEY":             stack.ZitadelMasterKey(),
		},
	}
	return c.applySecret(ctx, secret)
}

func (c *Client) applyUserConfigMaps(ctx context.Context, stack *schema.Stack, stackDir string) error {
	for name, definition := range stack.ConfigMaps {
		data := map[string]string{}
		for key, value := range definition.Data {
			data[key] = value
		}
		for key, rel := range definition.Files {
			content, err := os.ReadFile(filepath.Join(stackDir, rel))
			if err != nil {
				return fmt.Errorf("read configMaps.%s.files.%s: %w", name, key, err)
			}
			data[key] = string(content)
		}
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stack.Cluster.Namespace, Labels: baseLabels(stack, name)},
			Data:       data,
		}
		if err := c.applyConfigMap(ctx, cm); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) applyUserSecrets(ctx context.Context, stack *schema.Stack, stackDir string) error {
	for name, definition := range stack.Secrets {
		data := map[string]string{}
		for key, value := range definition.StringData {
			switch {
			case value.FromEnv != "":
				env, ok := os.LookupEnv(value.FromEnv)
				if !ok {
					return fmt.Errorf("secrets.%s.stringData.%s requires environment variable %s", name, key, value.FromEnv)
				}
				data[key] = env
			case value.FromFile != "":
				content, err := os.ReadFile(filepath.Join(stackDir, value.FromFile))
				if err != nil {
					return fmt.Errorf("read secrets.%s.stringData.%s: %w", name, key, err)
				}
				data[key] = string(content)
			}
		}
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stack.Cluster.Namespace, Labels: baseLabels(stack, name)},
			Type:       corev1.SecretTypeOpaque,
			StringData: data,
		}
		if err := c.applySecret(ctx, secret); err != nil {
			return err
		}
	}
	return nil
}
