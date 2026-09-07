package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/pyahu/cli/pkg/schema"
)

func (c *Client) applyKafkaConnect(ctx context.Context, stack *schema.Stack, stackDir string) error {
	namespace := stack.Cluster.Namespace
	serviceLabels := baseLabels(stack, "kafka-connect")
	serviceLabels["app.kubernetes.io/name"] = "kafka-connect"
	selector := map[string]string{"app.kubernetes.io/name": "kafka-connect", "pyahu.io/stack": stack.Metadata.Name}

	if err := c.applyKafkaConnectTopicJobs(ctx, stack); err != nil {
		return err
	}
	files, err := kafkaConnectPluginFiles(stack, stackDir)
	if err != nil {
		return err
	}
	if err := c.applySecret(ctx, kafkaConnectPluginFilesSecret(stack, serviceLabels, files)); err != nil {
		return err
	}
	if err := c.applyService(ctx, kafkaConnectService(namespace, serviceLabels, selector)); err != nil {
		return err
	}
	if err := c.applyDeployment(ctx, kafkaConnectDeployment(stack, serviceLabels, selector)); err != nil {
		return err
	}
	return c.applyKafkaConnectConnectorJobs(ctx, stack)
}

const (
	kafkaConnectPluginFilesSecretName = "kafka-connect-plugin-files"
	// The worker image ships its own plugins in /kafka/connect; declared plugins
	// are installed alongside it so neither hides the other.
	kafkaConnectBuiltinPluginPath = "/kafka/connect"
	kafkaConnectExtraPluginPath   = "/kafka/connect-extra"
	// 1 MiB: `file` exists for a small jar with no public URL yet, not as a way
	// to push release archives through a Secret.
	kafkaConnectPluginFileLimit = 1 << 20
)

// kafkaConnectPluginFiles reads the `file` artifacts from disk, keyed the way the
// initContainer expects to find them under /plugin-files.
func kafkaConnectPluginFiles(stack *schema.Stack, stackDir string) (map[string][]byte, error) {
	files := map[string][]byte{}
	for i, plugin := range stack.KafkaConnectPlugins() {
		if plugin.File == "" {
			continue
		}
		path := filepath.Join(stackDir, plugin.File)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("read services.kafkaConnect.plugins[%d].file for plugin %s: %w", i, plugin.Name, err)
		}
		if info.Size() > kafkaConnectPluginFileLimit {
			return nil, fmt.Errorf("services.kafkaConnect.plugins[%d].file for plugin %s is %d bytes; the limit is 1 MiB — publish it and use url + sha256 instead", i, plugin.Name, info.Size())
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read services.kafkaConnect.plugins[%d].file for plugin %s: %w", i, plugin.Name, err)
		}
		files[kafkaConnectPluginFileKey(i, plugin)] = content
	}
	return files, nil
}

// kafkaConnectPluginFileKey prefixes the index so two plugins can carry files
// with the same base name.
func kafkaConnectPluginFileKey(index int, plugin schema.KafkaConnectPlugin) string {
	return fmt.Sprintf("%02d-%s", index, kubeName(schema.PluginArtifactBase(plugin), 200))
}

func kafkaConnectPluginFilesSecret(stack *schema.Stack, serviceLabels map[string]string, files map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: kafkaConnectPluginFilesSecretName, Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Type:       corev1.SecretTypeOpaque,
		Data:       files,
	}
}

func (c *Client) applyKafkaConnectTopicJobs(ctx context.Context, stack *schema.Stack) error {
	for _, topic := range stack.KafkaConnectInternalTopics() {
		if err := c.applyJob(ctx, kafkaConnectTopicJob(stack, topic)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) applyKafkaConnectConnectorJobs(ctx context.Context, stack *schema.Stack) error {
	serviceLabels := baseLabels(stack, "kafka-connect")
	serviceLabels["app.kubernetes.io/name"] = "kafka-connect"
	for _, connector := range stack.Services.KafkaConnect.Connectors {
		payload, err := kafkaConnectConnectorPayload(stack, connector)
		if err != nil {
			return err
		}
		secret := kafkaConnectConnectorSecret(stack, serviceLabels, connector, payload)
		if err := c.applySecret(ctx, secret); err != nil {
			return err
		}
		if err := c.applyJob(ctx, kafkaConnectConnectorJob(stack, connector, secret.Name, payload)); err != nil {
			return err
		}
	}
	return nil
}

func kafkaConnectTopicJobNames(stack *schema.Stack) []string {
	topics := stack.KafkaConnectInternalTopics()
	names := make([]string, 0, len(topics))
	for _, topic := range topics {
		names = append(names, kafkaConnectTopicJobName(topic.Name))
	}
	return names
}

func kafkaConnectConnectorJobNames(stack *schema.Stack) ([]string, error) {
	names := make([]string, 0, len(stack.Services.KafkaConnect.Connectors))
	for _, connector := range stack.Services.KafkaConnect.Connectors {
		payload, err := kafkaConnectConnectorPayload(stack, connector)
		if err != nil {
			return nil, err
		}
		names = append(names, kafkaConnectConnectorResourceName(connector, payload))
	}
	return names, nil
}

func kafkaConnectService(namespace string, serviceLabels map[string]string, selector map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-connect", Namespace: namespace, Labels: serviceLabels},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selector,
			Ports: []corev1.ServicePort{{
				Name:       "rest",
				Port:       8083,
				TargetPort: intstr.FromInt(8083),
				NodePort:   nodePortKafkaConnect,
			}},
		},
	}
}

// kafkaConnectPluginInstallScript builds the initContainer script that installs
// the declared artifacts into /plugins/<name>. Release archives nest their jars
// under lib/, so every *.jar found is flattened into the plugin directory: Kafka
// Connect only treats the immediate children of a plugin.path entry as a plugin.
func kafkaConnectPluginInstallScript(stack *schema.Stack) string {
	plugins := stack.KafkaConnectPlugins()
	if len(plugins) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("work=$(mktemp -d)\n")
	b.WriteString("trap 'rm -rf \"$work\"' EXIT\n")
	for i, plugin := range plugins {
		format, err := schema.PluginArtifactFormat(plugin)
		if err != nil {
			// Validation rejects this before apply; keep the artifact out of the
			// script rather than emitting something the shell cannot run.
			continue
		}
		dir := shellQuote(kafkaConnectExtraPluginInstallDir(plugin.Name))
		artifact := fmt.Sprintf("$work/%d", i)

		b.WriteString(fmt.Sprintf("\necho \"installing plugin %s\"\n", plugin.Name))
		b.WriteString(fmt.Sprintf("mkdir -p %s\n", dir))
		if plugin.URL != "" {
			b.WriteString(fmt.Sprintf("wget -q -O %s %s || { echo \"plugin %s: download failed: %s\" >&2; exit 1; }\n",
				artifact, shellQuote(plugin.URL), plugin.Name, plugin.URL))
		} else {
			source := "/plugin-files/" + kafkaConnectPluginFileKey(i, plugin)
			b.WriteString(fmt.Sprintf("cp %s %s || { echo \"plugin %s: missing file artifact\" >&2; exit 1; }\n",
				shellQuote(source), artifact, plugin.Name))
		}
		if plugin.SHA256 != "" {
			b.WriteString(fmt.Sprintf("echo \"%s  %s\" | sha256sum -c - >/dev/null || { echo \"plugin %s: sha256 mismatch\" >&2; exit 1; }\n",
				plugin.SHA256, artifact, plugin.Name))
		}
		switch format {
		case "jar":
			b.WriteString(fmt.Sprintf("cp %s %s/%s\n", artifact, dir, shellQuote(schema.PluginArtifactBase(plugin))))
		default:
			extractDir := fmt.Sprintf("$work/x%d", i)
			b.WriteString(fmt.Sprintf("mkdir -p %s\n", extractDir))
			if format == "zip" {
				b.WriteString(fmt.Sprintf("unzip -q -o %s -d %s || { echo \"plugin %s: cannot unzip artifact\" >&2; exit 1; }\n", artifact, extractDir, plugin.Name))
			} else {
				b.WriteString(fmt.Sprintf("tar -xzf %s -C %s || { echo \"plugin %s: cannot untar artifact\" >&2; exit 1; }\n", artifact, extractDir, plugin.Name))
			}
			b.WriteString(fmt.Sprintf("find %s -name '*.jar' -type f -exec cp {} %s/ \\;\n", extractDir, dir))
			b.WriteString(fmt.Sprintf("[ -n \"$(ls -A %s)\" ] || { echo \"plugin %s: archive contained no jar\" >&2; exit 1; }\n", dir, plugin.Name))
		}
		b.WriteString(fmt.Sprintf("echo \"plugin %s: $(ls %s | wc -l) jar(s)\"\n", plugin.Name, dir))
	}
	return b.String()
}

func kafkaConnectExtraPluginInstallDir(name string) string {
	return "/plugins/" + name
}

func kafkaConnectEnv(stack *schema.Stack) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: "BOOTSTRAP_SERVERS", Value: stack.KafkaInternalBootstrapServers()},
		{Name: "GROUP_ID", Value: stack.Metadata.Name + "-connect"},
		{Name: "CONFIG_STORAGE_TOPIC", Value: stack.KafkaConnectConfigTopic()},
		{Name: "OFFSET_STORAGE_TOPIC", Value: stack.KafkaConnectOffsetTopic()},
		{Name: "STATUS_STORAGE_TOPIC", Value: stack.KafkaConnectStatusTopic()},
		{Name: "HOST_NAME", Value: "0.0.0.0"},
		{Name: "ADVERTISED_HOST_NAME", Value: "kafka-connect"},
		{Name: "ADVERTISED_PORT", Value: "8083"},
		{Name: "KEY_CONVERTER", Value: "org.apache.kafka.connect.json.JsonConverter"},
		{Name: "VALUE_CONVERTER", Value: "org.apache.kafka.connect.json.JsonConverter"},
		{Name: "HEAP_OPTS", Value: "-Xms256m -Xmx768m"},
	}
	if len(stack.KafkaConnectPlugins()) > 0 {
		// The Debezium entrypoint maps CONNECT_* env onto worker properties, so
		// this becomes plugin.path; the worker scans each listed directory's
		// immediate children as plugins.
		env = append(env, corev1.EnvVar{Name: "CONNECT_PLUGIN_PATH", Value: kafkaConnectBuiltinPluginPath + "," + kafkaConnectExtraPluginPath})
	}
	return env
}

func kafkaConnectDeployment(stack *schema.Stack, serviceLabels map[string]string, selector map[string]string) *appsv1.Deployment {
	script := kafkaConnectPluginInstallScript(stack)
	annotations := map[string]string{}
	var initContainers []corev1.Container
	var volumes []corev1.Volume
	var workerMounts []corev1.VolumeMount
	if script != "" {
		// Roll the pod when the plugin set changes; the artifacts are installed at
		// pod start, so an unchanged pod would keep serving the previous set.
		annotations["pyahu.io/kafka-connect-plugins-sha256"] = sha256Hex(script)
		initContainers = append(initContainers, corev1.Container{
			Name:    "install-plugins",
			Image:   "alpine:3.21",
			Command: []string{"sh", "-ec"},
			Args:    []string{script},
			VolumeMounts: []corev1.VolumeMount{
				{Name: "plugins", MountPath: "/plugins"},
				{Name: "plugin-files", MountPath: "/plugin-files", ReadOnly: true},
			},
		})
		volumes = append(volumes,
			corev1.Volume{Name: "plugins", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			corev1.Volume{Name: "plugin-files", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: kafkaConnectPluginFilesSecretName}}},
		)
		workerMounts = append(workerMounts, corev1.VolumeMount{Name: "plugins", MountPath: kafkaConnectExtraPluginPath, ReadOnly: true})
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "kafka-connect", Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32p(int32(stack.Services.KafkaConnect.Replicas)),
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector, Annotations: annotations},
				Spec: corev1.PodSpec{
					InitContainers: initContainers,
					Volumes:        volumes,
					Containers: []corev1.Container{{
						Name:           "kafka-connect",
						Image:          stack.KafkaConnectImage(),
						Ports:          []corev1.ContainerPort{{Name: "rest", ContainerPort: 8083}},
						VolumeMounts:   workerMounts,
						Env:            kafkaConnectEnv(stack),
						ReadinessProbe: httpProbe("/connectors", 8083, 30, 10),
						LivenessProbe:  httpProbe("/", 8083, 60, 20),
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
							Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1Gi")},
						},
					}},
				},
			},
		},
	}
}

func kafkaConnectTopicJob(stack *schema.Stack, topic schema.TopicConfig) *batchv1.Job {
	name := kafkaConnectTopicJobName(topic.Name)
	labels := baseLabels(stack, name)
	labels["app.kubernetes.io/name"] = "kafka-connect-topic"
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stack.Cluster.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32p(6),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:    "create-topic",
						Image:   "apache/kafka:" + stack.Services.Kafka.Version,
						Command: []string{"sh", "-ec"},
						Args: []string{fmt.Sprintf(
							"/opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka.%s.svc.cluster.local:9092 --create --if-not-exists --topic %s --partitions %d --replication-factor %d --config cleanup.policy=compact",
							stack.Cluster.Namespace,
							topic.Name,
							topic.Partitions,
							topic.Replicas,
						)},
					}},
				},
			},
		},
	}
}

func kafkaConnectTopicJobName(topicName string) string {
	return "kafka-connect-topic-" + kubeName(topicName, 34)
}

func kafkaConnectConnectorSecret(stack *schema.Stack, serviceLabels map[string]string, connector schema.KafkaConnectConnector, payload string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: kafkaConnectConnectorResourceName(connector, payload), Namespace: stack.Cluster.Namespace, Labels: serviceLabels},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{"connector.json": payload},
	}
}

func kafkaConnectConnectorJob(stack *schema.Stack, connector schema.KafkaConnectConnector, secretName string, payload string) *batchv1.Job {
	name := kafkaConnectConnectorResourceName(connector, payload)
	labels := baseLabels(stack, name)
	labels["app.kubernetes.io/name"] = "kafka-connect-connector"
	// Waiting for the class to show up in /connector-plugins separates "the
	// plugin was never installed" from "the connector itself is unhealthy".
	script := fmt.Sprintf(`until curl -fsS %[1]s/connectors >/dev/null; do sleep 2; done
for i in $(seq 1 60); do
  if curl -fsS '%[1]s/connector-plugins?connectorsOnly=false' | grep -q '"%[3]s"'; then break; fi
  if [ "$i" -eq 60 ]; then
    echo "connector class %[3]s is not loaded by the Kafka Connect worker - check services.kafkaConnect.plugins" >&2
    exit 1
  fi
  sleep 2
done
curl -fsS -X PUT -H 'Content-Type: application/json' --data-binary @/connector/connector.json %[1]s/connectors/%[2]s/config
for i in $(seq 1 60); do
  status="$(curl -fsS %[1]s/connectors/%[2]s/status)"
  echo "$status"
  if printf '%%s' "$status" | grep -q '"state":"FAILED"'; then exit 1; fi
  running_count="$(printf '%%s' "$status" | grep -o '"state":"RUNNING"' | wc -l | tr -d ' ')"
  if [ "$running_count" -ge 2 ]; then exit 0; fi
  sleep 2
done
echo "connector %[2]s did not become healthy" >&2
exit 1
`, stack.KafkaConnectInternalURL(), connector.Name, stack.KafkaConnectConnectorConfig(connector)["connector.class"])
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: stack.Cluster.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32p(6),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{{
						Name:         "apply-connector",
						Image:        stack.KafkaConnectImage(),
						Command:      []string{"sh", "-ec"},
						Args:         []string{script},
						VolumeMounts: []corev1.VolumeMount{{Name: "connector", MountPath: "/connector", ReadOnly: true}},
					}},
					Volumes: []corev1.Volume{{
						Name: "connector",
						VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
							SecretName: secretName,
						}},
					}},
				},
			},
		},
	}
}

func kafkaConnectConnectorPayload(stack *schema.Stack, connector schema.KafkaConnectConnector) (string, error) {
	data, err := json.MarshalIndent(stack.KafkaConnectConnectorConfig(connector), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func kafkaConnectConnectorResourceName(connector schema.KafkaConnectConnector, payload string) string {
	hash := sha256Hex(payload)
	return "kafka-connect-" + kubeName(connector.Name, 33) + "-" + hash[:8]
}

func kubeName(value string, max int) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		name = "item"
	}
	if len(name) > max {
		name = strings.TrimRight(name[:max], "-")
	}
	return name
}
