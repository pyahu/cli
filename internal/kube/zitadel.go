package kube

import (
	"context"
	"fmt"
	"net/url"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyZitadel(ctx context.Context, stack *schema.Stack) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "zitadel")
	serviceLabels["app.kubernetes.io/name"] = "zitadel"
	selector := map[string]string{"app.kubernetes.io/name": "zitadel", "pyahu.io/stack": stack.Metadata.Name}
	external, err := parseZitadelExternal(stack.Services.Zitadel.ExternalURL)
	if err != nil {
		return err
	}

	if err := c.applyService(ctx, zitadelService(namespace, serviceLabels, selector)); err != nil {
		return err
	}
	if err := c.applyDeployment(ctx, zitadelDeployment(stack, serviceLabels, selector, external)); err != nil {
		return err
	}
	return c.applyIngress(ctx, zitadelIngress(namespace, serviceLabels, external.domain, stack.LocalTLSEnabled(), stack.LocalTLSSecretName()))
}

type zitadelExternal struct {
	domain string
	port   string
	secure bool
}

func parseZitadelExternal(rawURL string) (zitadelExternal, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return zitadelExternal{}, err
	}
	external := zitadelExternal{domain: parsed.Hostname(), port: parsed.Port(), secure: parsed.Scheme == "https"}
	if external.port == "" {
		if external.secure {
			external.port = "443"
		} else {
			external.port = "80"
		}
	}
	return external, nil
}

func zitadelService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "zitadel", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    []corev1.ServicePort{{Name: "http2", Port: 8080, TargetPort: intstr.FromInt(8080)}},
		},
	}
}

func zitadelDeployment(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string, external zitadelExternal) *appsv1.Deployment {
	namespace := stack.Cluster.Namespace
	postgresDSN := fmt.Sprintf("postgresql://%s:%s@postgres.%s.svc.cluster.local:5432/zitadel?sslmode=disable", stack.PostgresUser(), stack.PostgresPassword(), namespace)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "zitadel", Namespace: namespace, Labels: serviceLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32p(1),
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Name:    "wait-postgres",
						Image:   "postgres:" + stack.Services.Postgres.Version + "-alpine",
						Command: []string{"sh", "-c", "until pg_isready -h postgres -U \"$POSTGRES_USER\"; do sleep 2; done"},
						Env: []corev1.EnvVar{
							{Name: "POSTGRES_USER", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_USER")},
							{Name: "PGPASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_PASSWORD")},
						},
					}},
					Containers: []corev1.Container{{
						Name:  "zitadel",
						Image: "ghcr.io/zitadel/zitadel:" + stack.Services.Zitadel.Version,
						Args:  []string{"start-from-init", "--masterkey", stack.ZitadelMasterKey()},
						Ports: []corev1.ContainerPort{{Name: "http2", ContainerPort: 8080}},
						Env: []corev1.EnvVar{
							{Name: "ZITADEL_PORT", Value: "8080"},
							{Name: "ZITADEL_TLS_ENABLED", Value: "false"},
							{Name: "ZITADEL_DATABASE_POSTGRES_DSN", Value: postgresDSN},
							{Name: "ZITADEL_EXTERNALSECURE", Value: fmt.Sprintf("%t", external.secure)},
							{Name: "ZITADEL_EXTERNALDOMAIN", Value: external.domain},
							{Name: "ZITADEL_EXTERNALPORT", Value: external.port},
							// ZITADEL v4 defaults the first instance to the new Login V2, which is a
							// separate app (ghcr.io/zitadel/zitadel-login) that Pyahu does not deploy.
							// Keep the local stack single-container by serving the built-in v1 login
							// at /ui/login. Applied at instance init, so a fresh database is required.
							{Name: "ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED", Value: "false"},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_HUMAN_USERNAME", Value: stack.Services.Zitadel.Admin.Username},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "ZITADEL_ADMIN_PASSWORD")},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_HUMAN_PASSWORDCHANGEREQUIRED", Value: "false"},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_HUMAN_EMAIL_ADDRESS", Value: stack.Services.Zitadel.Admin.Username},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_HUMAN_EMAIL_VERIFIED", Value: "true"},
							// Service-account machine user + long-lived PAT for headless
							// provisioning. FirstInstance writes the PAT to PATPATH; the
							// sa-bootstrap sidecar exposes it and the CLI exports it to the
							// iam-admin-pat Secret (see CaptureZitadelPAT). Applied at instance
							// init, so a fresh database is required.
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_USERNAME", Value: zitadelServiceAccountUser},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_MACHINE_MACHINE_NAME", Value: "Pyahu Admin Service Account"},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_MACHINE_PAT_EXPIRATIONDATE", Value: "2100-01-01T00:00:00Z"},
							{Name: "ZITADEL_FIRSTINSTANCE_PATPATH", Value: zitadelPATPath},
						},
						ReadinessProbe: httpProbe("/debug/ready", 8080, 20, 10),
						LivenessProbe:  httpProbe("/debug/healthz", 8080, 60, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("768Mi")},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "pat-out", MountPath: zitadelPATDir}},
					}, {
						// Idle sidecar sharing the FirstInstance PAT emptyDir so the CLI can
						// `exec cat` and export the service-account PAT to a Secret — the
						// zitadel image ships no shell to exec into directly. See CaptureZitadelPAT.
						Name:         zitadelSABootstrapContainer,
						Image:        "busybox:1.37",
						Command:      []string{"sh", "-c", "while true; do sleep 3600; done"},
						VolumeMounts: []corev1.VolumeMount{{Name: "pat-out", MountPath: zitadelPATDir}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("16Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("32Mi")},
						},
					}},
					Volumes: []corev1.Volume{{Name: "pat-out", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
				},
			},
		},
	}
}

const (
	zitadelServiceAccountUser   = "pyahu-admin-sa"
	zitadelPATDir               = "/pat-out"
	zitadelPATPath              = "/pat-out/pat"
	zitadelSABootstrapContainer = "sa-bootstrap"
	zitadelPATSecretName        = "iam-admin-pat"
)

func zitadelIngress(namespace string, serviceLabels map[string]string, domain string, tlsEnabled bool, secretName string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zitadel",
			Namespace: namespace,
			Labels:    serviceLabels,
			Annotations: map[string]string{
				"traefik.ingress.kubernetes.io/service.serversscheme": "h2c",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: domain,
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: "zitadel",
						Port: networkingv1.ServiceBackendPort{Number: 8080},
					}},
				}}}},
			}},
		},
	}
	if tlsEnabled {
		ingress.Spec.TLS = []networkingv1.IngressTLS{{
			Hosts:      []string{domain},
			SecretName: secretName,
		}}
	}
	return ingress
}
