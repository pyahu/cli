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
	if err := c.applyIngress(ctx, zitadelIngress(namespace, serviceLabels, external.domain, stack.LocalTLSEnabled(), stack.LocalTLSSecretName())); err != nil {
		return err
	}
	return c.applyIngress(ctx, zitadelLoginIngress(namespace, serviceLabels, external.domain, stack.LocalTLSEnabled(), stack.LocalTLSSecretName()))
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
			Ports: []corev1.ServicePort{
				{Name: "http2", Port: 8080, TargetPort: intstr.FromInt(8080)},
				{Name: "login", Port: 3000, TargetPort: intstr.FromInt(3000)},
			},
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
							// Login V2 is the supported login UI in ZITADEL v4; the legacy v1 login
							// (/ui/login) silently fails the password step in v4.15.2. Enable v2 and
							// run its app (ghcr.io/zitadel/zitadel-login) as a sidecar (see below).
							// Applied at instance init, so a fresh database is required. FirstInstance
							// mints a login-client machine user (IAM_LOGIN_CLIENT) + PAT and writes it
							// to the shared emptyDir; the login sidecar reads it from the same path.
							{Name: "ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_REQUIRED", Value: "true"},
							{Name: "ZITADEL_DEFAULTINSTANCE_FEATURES_LOGINV2_BASEURI", Value: loginV2Prefix(external) + "/"},
							{Name: "ZITADEL_OIDC_DEFAULTLOGINURLV2", Value: loginV2Prefix(external) + "/login?authRequest="},
							{Name: "ZITADEL_OIDC_DEFAULTLOGOUTURLV2", Value: loginV2Prefix(external) + "/logout?post_logout_redirect="},
							{Name: "ZITADEL_FIRSTINSTANCE_LOGINCLIENTPATPATH", Value: zitadelLoginClientPATPath},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_MACHINE_USERNAME", Value: "login-client"},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_MACHINE_NAME", Value: "Automatically Initialized IAM_LOGIN_CLIENT"},
							{Name: "ZITADEL_FIRSTINSTANCE_ORG_LOGINCLIENT_PAT_EXPIRATIONDATE", Value: "2100-01-01T00:00:00Z"},
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
					}, {
						// Login V2 UI (Next.js). Same pod as zitadel so it can read the
						// login-client PAT from the shared emptyDir and reach the API over
						// loopback. CUSTOM_REQUEST_HEADERS carries the external Host so the
						// plaintext loopback call resolves the right instance (and avoids the
						// Node TLS trust problem of calling https://<domain> directly).
						Name:  "zitadel-login",
						Image: "ghcr.io/zitadel/zitadel-login:" + stack.Services.Zitadel.Version,
						Env: []corev1.EnvVar{
							{Name: "ZITADEL_API_URL", Value: "http://localhost:8080"},
							{Name: "NEXT_PUBLIC_BASE_PATH", Value: "/ui/v2/login"},
							{Name: "ZITADEL_SERVICE_USER_TOKEN_FILE", Value: zitadelLoginClientPATPath},
							{Name: "CUSTOM_REQUEST_HEADERS", Value: fmt.Sprintf("Host:%s,X-Forwarded-Proto:%s", external.domain, zitadelScheme(external))},
						},
						Ports:          []corev1.ContainerPort{{Name: "http", ContainerPort: 3000}},
						ReadinessProbe: httpProbe("/ui/v2/login/healthy", 3000, 20, 10),
						LivenessProbe:  httpProbe("/ui/v2/login/healthy", 3000, 90, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "pat-out", MountPath: zitadelPATDir, ReadOnly: true}},
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
	zitadelLoginClientPATPath   = "/pat-out/login-client.pat"
	zitadelSABootstrapContainer = "sa-bootstrap"
	zitadelPATSecretName        = "iam-admin-pat"
)

// zitadelScheme returns the external URL scheme (http/https).
func zitadelScheme(external zitadelExternal) string {
	if external.secure {
		return "https"
	}
	return "http"
}

// loginV2Prefix builds the browser-facing base URL of the Login V2 app
// (e.g. https://zitadel.localhost/ui/v2/login), omitting the default port.
func loginV2Prefix(external zitadelExternal) string {
	host := external.domain
	if !(external.port == "" || (external.secure && external.port == "443") || (!external.secure && external.port == "80")) {
		host = host + ":" + external.port
	}
	return zitadelScheme(external) + "://" + host + "/ui/v2/login"
}

// zitadelLoginIngress routes /ui/v2/login on the Zitadel host to the login
// sidecar (port 3000, plain HTTP/1.1 — no h2c, unlike the API backend). Traefik
// longest-prefix matching sends these paths here and everything else to the API
// ingress.
func zitadelLoginIngress(namespace string, serviceLabels map[string]string, domain string, tlsEnabled bool, secretName string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "zitadel-login", Namespace: namespace, Labels: serviceLabels},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				Host: domain,
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/ui/v2/login",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: "zitadel",
						Port: networkingv1.ServiceBackendPort{Number: 3000},
					}},
				}}}},
			}},
		},
	}
	if tlsEnabled {
		ingress.Spec.TLS = []networkingv1.IngressTLS{{Hosts: []string{domain}, SecretName: secretName}}
	}
	return ingress
}

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
