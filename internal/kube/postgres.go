package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyPostgres(ctx context.Context, stack *schema.Stack, stackDir string) error {
	namespace := stack.Cluster.Namespace
	primaryLabels := postgresLabels(stack, "postgres", "primary")
	primarySelector := postgresSelector(stack, "primary")
	replicaLabels := postgresLabels(stack, "postgres-read", "replica")
	replicaSelector := postgresSelector(stack, "replica")
	replicationUser := ""
	replicationPassword := ""
	if stack.PostgresReadReplicas() > 0 {
		replicationUser = stack.PostgresReplicationUser()
		replicationPassword = stack.PostgresReplicationPassword()
	}

	initScript, err := postgresInitScriptWithSeeds(stack.Services.Postgres.Databases, stack.PostgresUser(), replicationUser, replicationPassword, stackDir)
	if err != nil {
		return err
	}
	if err := c.applyConfigMap(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-init", Namespace: namespace, Labels: primaryLabels},
		Data: map[string]string{
			"create-databases.sh": initScript,
		},
	}); err != nil {
		return err
	}
	if err := c.applyConfigMap(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-config", Namespace: namespace, Labels: primaryLabels},
		Data:       map[string]string{"pg_hba.conf": postgresHBA(stack)},
	}); err != nil {
		return err
	}

	if err := c.applyService(ctx, postgresService(namespace, primaryLabels, primarySelector)); err != nil {
		return err
	}
	if err := c.applyStatefulSet(ctx, postgresStatefulSet(stack, primaryLabels, primarySelector)); err != nil {
		return err
	}
	if stack.PostgresReadReplicas() == 0 {
		if err := c.deleteStatefulSetIfExists(ctx, namespace, "postgres-read"); err != nil {
			return err
		}
		return c.deleteServiceIfExists(ctx, namespace, "postgres-read")
	}
	if err := c.applyService(ctx, postgresReadService(namespace, replicaLabels, replicaSelector)); err != nil {
		return err
	}
	return c.applyStatefulSet(ctx, postgresReadStatefulSet(stack, replicaLabels, replicaSelector))
}

func postgresService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "postgres",
				Port:       5432,
				TargetPort: intstr.FromInt(5432),
				NodePort:   nodePortPostgres,
			}},
		},
	}
}

func postgresReadService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-read", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "postgres",
				Port:       5432,
				TargetPort: intstr.FromInt(5432),
				NodePort:   nodePortPostgresRead,
			}},
		},
	}
}

func postgresLabels(stack *schema.Stack, component string, role string) map[string]string {
	labels := baseLabels(stack, component)
	labels["app.kubernetes.io/name"] = "postgres"
	labels["pyahu.io/postgres-role"] = role
	return labels
}

func postgresSelector(stack *schema.Stack, role string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": "postgres",
		"pyahu.io/stack":         stack.Metadata.Name,
		"pyahu.io/postgres-role": role,
	}
}

func postgresStatefulSet(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "postgres",
			Replicas:    int32p(int32(stack.Services.Postgres.Instances)),
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "postgres",
						Image: "postgres:" + stack.Services.Postgres.Version + "-alpine",
						Args: []string{
							"-c", "wal_level=logical",
							"-c", "max_wal_senders=10",
							"-c", "max_replication_slots=10",
							"-c", "wal_keep_size=256MB",
							"-c", "hot_standby=on",
							"-c", "hba_file=/etc/postgresql/pg_hba.conf",
						},
						Ports: []corev1.ContainerPort{{Name: "postgres", ContainerPort: 5432}},
						Env: []corev1.EnvVar{
							{Name: "POSTGRES_USER", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_USER")},
							{Name: "POSTGRES_PASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_PASSWORD")},
							{Name: "POSTGRES_REPLICATION_USER", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_REPLICATION_USER")},
							{Name: "POSTGRES_REPLICATION_PASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_REPLICATION_PASSWORD")},
							{Name: "POSTGRES_DB", Value: "postgres"},
							{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/var/lib/postgresql/data"},
							{Name: "init", MountPath: "/docker-entrypoint-initdb.d"},
							{Name: "config", MountPath: "/etc/postgresql/pg_hba.conf", SubPath: "pg_hba.conf", ReadOnly: true},
						},
						ReadinessProbe: execProbe([]string{"pg_isready", "-U", stack.PostgresUser()}, 5, 5),
						LivenessProbe:  execProbe([]string{"pg_isready", "-U", stack.PostgresUser()}, 15, 10),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "init",
						VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-init"},
							DefaultMode:          int32p(0o755),
						}},
					}, {
						Name: "config",
						VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
						}},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(stack.Services.Postgres.Storage)}},
				},
			}},
		},
	}
}

func postgresReadStatefulSet(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-read", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "postgres-read",
			Replicas:    int32p(int32(stack.PostgresReadReplicas())),
			Selector:    &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Name:    "basebackup",
						Image:   "postgres:" + stack.Services.Postgres.Version + "-alpine",
						Command: []string{"sh", "-ec"},
						Args:    []string{postgresReplicaInitScript()},
						Env: []corev1.EnvVar{
							{Name: "POSTGRES_REPLICATION_USER", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_REPLICATION_USER")},
							{Name: "POSTGRES_REPLICATION_PASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_REPLICATION_PASSWORD")},
							{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
						},
						VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/var/lib/postgresql/data"}},
					}},
					Containers: []corev1.Container{{
						Name:  "postgres",
						Image: "postgres:" + stack.Services.Postgres.Version + "-alpine",
						Args: []string{
							"-c", "hot_standby=on",
							"-c", "hba_file=/etc/postgresql/pg_hba.conf",
						},
						Ports: []corev1.ContainerPort{{Name: "postgres", ContainerPort: 5432}},
						Env: []corev1.EnvVar{
							{Name: "POSTGRES_USER", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_USER")},
							{Name: "PGPASSWORD", ValueFrom: secretRef("pyahu-local-credentials", "POSTGRES_PASSWORD")},
							{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "data", MountPath: "/var/lib/postgresql/data"},
							{Name: "config", MountPath: "/etc/postgresql/pg_hba.conf", SubPath: "pg_hba.conf", ReadOnly: true},
						},
						ReadinessProbe: execProbe([]string{"pg_isready", "-U", stack.PostgresUser()}, 10, 5),
						LivenessProbe:  execProbe([]string{"pg_isready", "-U", stack.PostgresUser()}, 30, 10),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("512Mi")},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "config",
						VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "postgres-config"},
						}},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(stack.Services.Postgres.Storage)}},
				},
			}},
		},
	}
}

func postgresReplicaInitScript() string {
	return strings.Join([]string{
		`if [ -s "$PGDATA/PG_VERSION" ]; then exit 0; fi`,
		`rm -rf "$PGDATA"`,
		`mkdir -p "$PGDATA"`,
		`until PGPASSWORD="$POSTGRES_REPLICATION_PASSWORD" pg_isready -h postgres -U "$POSTGRES_REPLICATION_USER"; do sleep 2; done`,
		`PGPASSWORD="$POSTGRES_REPLICATION_PASSWORD" pg_basebackup -h postgres -D "$PGDATA" -U "$POSTGRES_REPLICATION_USER" -R -X stream -c fast`,
		`chown -R 999:999 "$PGDATA"`,
		`chmod 700 "$PGDATA"`,
	}, "\n")
}

func postgresInitScript(databases []schema.DatabaseConfig, defaultOwner string, replicationUser string, replicationPassword string) string {
	script, _ := postgresInitScriptWithSeeds(databases, defaultOwner, replicationUser, replicationPassword, "")
	return script
}

func postgresInitScriptWithSeeds(databases []schema.DatabaseConfig, defaultOwner string, replicationUser string, replicationPassword string, stackDir string) (string, error) {
	var builder strings.Builder
	builder.WriteString("#!/bin/sh\nset -eu\n")
	if replicationUser != "" {
		password := postgresSQLLiteral(replicationPassword)
		builder.WriteString("psql -v ON_ERROR_STOP=1 --username \"$POSTGRES_USER\" --dbname postgres <<'SQL'\n")
		builder.WriteString(fmt.Sprintf("DO $$\nBEGIN\n  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = %s) THEN\n    CREATE ROLE \"%s\" WITH REPLICATION LOGIN PASSWORD %s;\n  ELSE\n    ALTER ROLE \"%s\" WITH REPLICATION LOGIN PASSWORD %s;\n  END IF;\nEND\n$$;\n", postgresSQLLiteral(replicationUser), replicationUser, password, replicationUser, password))
		builder.WriteString("SQL\n")
	}
	for i, db := range databases {
		owner := db.Owner
		if owner == "" {
			owner = defaultOwner
		}
		if owner != defaultOwner {
			builder.WriteString(fmt.Sprintf("psql -v ON_ERROR_STOP=1 --username \"$POSTGRES_USER\" --dbname postgres -tc \"SELECT 1 FROM pg_roles WHERE rolname = '%s'\" | grep -q 1 || psql -v ON_ERROR_STOP=1 --username \"$POSTGRES_USER\" --dbname postgres -c 'CREATE ROLE \"%s\";'\n", owner, owner))
		}
		builder.WriteString(fmt.Sprintf("psql -v ON_ERROR_STOP=1 --username \"$POSTGRES_USER\" --dbname postgres -tc \"SELECT 1 FROM pg_database WHERE datname = '%s'\" | grep -q 1 || psql -v ON_ERROR_STOP=1 --username \"$POSTGRES_USER\" --dbname postgres -c 'CREATE DATABASE \"%s\" OWNER \"%s\";'\n", db.Name, db.Name, owner))
		if db.Seed != "" {
			if stackDir == "" {
				continue
			}
			content, err := os.ReadFile(filepath.Join(stackDir, db.Seed))
			if err != nil {
				return "", fmt.Errorf("read seed for database %s: %w", db.Name, err)
			}
			marker := fmt.Sprintf("PYAHU_SEED_%d", i)
			builder.WriteString(fmt.Sprintf("psql -v ON_ERROR_STOP=1 --username \"$POSTGRES_USER\" --dbname \"%s\" <<'%s'\n", db.Name, marker))
			builder.Write(content)
			if !strings.HasSuffix(string(content), "\n") {
				builder.WriteByte('\n')
			}
			builder.WriteString(marker + "\n")
		}
	}
	return builder.String(), nil
}

func postgresHBA(stack *schema.Stack) string {
	lines := []string{
		"local all all trust",
		"host all all 127.0.0.1/32 scram-sha-256",
		"host all all ::1/128 scram-sha-256",
		"host all all 0.0.0.0/0 scram-sha-256",
	}
	if stack.PostgresReadReplicas() > 0 {
		lines = append(lines, fmt.Sprintf("host replication %s 0.0.0.0/0 scram-sha-256", stack.PostgresReplicationUser()))
	}
	return strings.Join(lines, "\n") + "\n"
}

func postgresSQLLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
