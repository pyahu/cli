package kube

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pyahu/cli/internal/certs"
	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyLocalTLS(ctx context.Context, stack *schema.Stack, stackDir string) error {
	if !stack.LocalTLSRequired() {
		return nil
	}
	bundle, err := certs.Ensure(stackDir, stack.LocalTLSDomains())
	if err != nil {
		return err
	}
	labels := baseLabels(stack, "local-tls")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stack.LocalTLSSecretName(),
			Namespace: stack.Cluster.Namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeTLS,
		StringData: map[string]string{
			corev1.TLSCertKey:       string(bundle.CertificatePEM),
			corev1.TLSPrivateKeyKey: string(bundle.PrivateKeyPEM),
		},
	}
	if err := c.applySecret(ctx, secret); err != nil {
		return err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      stack.LocalTLSCAConfigMapName(),
			Namespace: stack.Cluster.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"ca.crt": string(bundle.CACertificatePEM),
		},
	}
	return c.applyConfigMap(ctx, cm)
}
