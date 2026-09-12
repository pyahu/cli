package kube

import corev1 "k8s.io/api/core/v1"

func hardenPodSpec(spec *corev1.PodSpec) {
	serviceAccountToken := false
	spec.AutomountServiceAccountToken = &serviceAccountToken

	if spec.SecurityContext == nil {
		spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if spec.SecurityContext.SeccompProfile == nil {
		spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	for index := range spec.InitContainers {
		hardenContainer(&spec.InitContainers[index])
	}
	for index := range spec.Containers {
		hardenContainer(&spec.Containers[index])
	}
}

func hardenContainer(container *corev1.Container) {
	if container.SecurityContext == nil {
		container.SecurityContext = &corev1.SecurityContext{}
	}
	allowPrivilegeEscalation := false
	container.SecurityContext.AllowPrivilegeEscalation = &allowPrivilegeEscalation
	container.SecurityContext.Capabilities = &corev1.Capabilities{
		Drop: []corev1.Capability{"ALL"},
	}
}
