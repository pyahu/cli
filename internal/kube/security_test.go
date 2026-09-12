package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestHardenPodSpecAppliesRestrictedDefaults(t *testing.T) {
	spec := corev1.PodSpec{
		InitContainers: []corev1.Container{{Name: "init"}},
		Containers:     []corev1.Container{{Name: "app"}},
	}

	hardenPodSpec(&spec)

	if spec.AutomountServiceAccountToken == nil || *spec.AutomountServiceAccountToken {
		t.Fatal("service account token should not be mounted")
	}
	if spec.SecurityContext == nil || spec.SecurityContext.SeccompProfile == nil ||
		spec.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatal("pod should use the runtime default seccomp profile")
	}
	for _, container := range append(spec.InitContainers, spec.Containers...) {
		if container.SecurityContext == nil || container.SecurityContext.AllowPrivilegeEscalation == nil ||
			*container.SecurityContext.AllowPrivilegeEscalation {
			t.Fatalf("container %q should disable privilege escalation", container.Name)
		}
		if container.SecurityContext.Capabilities == nil ||
			len(container.SecurityContext.Capabilities.Drop) != 1 ||
			container.SecurityContext.Capabilities.Drop[0] != corev1.Capability("ALL") {
			t.Fatalf("container %q should drop all Linux capabilities", container.Name)
		}
	}
}
