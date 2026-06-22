package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pyahu/cli/internal/config"
	"github.com/pyahu/cli/internal/doctor"
	"github.com/pyahu/cli/internal/kube"
	"github.com/pyahu/cli/pkg/schema"
)

func TestEnvCommandFormats(t *testing.T) {
	stackPath := writePresetStack(t, "platform")

	tests := []struct {
		name    string
		args    []string
		want    string
		wantKey string
	}{
		{
			name: "dotenv",
			args: []string{"--file", stackPath, "env", "--format", "dotenv"},
			want: "POSTGRES_URL=postgresql://pyahu:pyahu_local@localhost:5432/app?sslmode=disable\n",
		},
		{
			name: "shell",
			args: []string{"--file", stackPath, "env", "--format", "shell"},
			want: "export KAFKA_BOOTSTRAP_SERVERS='localhost:9092'\n",
		},
		{
			name:    "json",
			args:    []string{"--file", stackPath, "env", "--format", "json"},
			wantKey: "RABBITMQ_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := executeTestCommand(t, nil, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if tt.want != "" && !strings.Contains(stdout, tt.want) {
				t.Fatalf("stdout does not contain %q:\n%s", tt.want, stdout)
			}
			if tt.wantKey != "" {
				var got map[string]string
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatal(err)
				}
				if got[tt.wantKey] == "" {
					t.Fatalf("missing key %s in %#v", tt.wantKey, got)
				}
			}
		})
	}
}

func TestEnvCommandShellQuotesUnsafeValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pyahu.yaml")
	data := []byte(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  postgres:
    enabled: true
    auth:
      username: app
      password: pa$HOME'x
    databases:
      - name: app
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := executeTestCommand(t, nil, "--file", path, "env", "--format", "shell")
	if err != nil {
		t.Fatal(err)
	}
	want := `export POSTGRES_PASSWORD='pa$HOME'"'"'x'`
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout does not contain %q:\n%s", want, stdout)
	}
}

func TestRejectsInvalidGlobalOutputFormat(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")

	_, _, err := executeTestCommand(t, nil, "--file", stackPath, "--output", "xml", "status")
	if err == nil {
		t.Fatal("expected invalid output error")
	}
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d", exitCode(err))
	}
	if !strings.Contains(err.Error(), "--output must be human or json") {
		t.Fatalf("unexpected error: %v", err)
	}
	if hint := errorHint(err); hint != "" {
		t.Fatalf("unexpected hint: %q", hint)
	}
}

func TestInitCommandWritesPreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pyahu.yaml")

	stdout, _, err := executeTestCommand(t, nil, "--file", path, "init", "--preset", "minimal")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "created "+path) {
		t.Fatalf("unexpected stdout: %s", stdout)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Data.PostgresEnabled() {
		t.Fatal("expected postgres to be enabled")
	}
}

func TestCertsStatusShowsMissingLocalCertificate(t *testing.T) {
	stackPath := writePresetStack(t, "platform")

	stdout, _, err := executeTestCommand(t, nil, "--file", stackPath, "certs", "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"local CA:",
		"CA status:     missing",
		"cert status:   missing",
		"k8s secret:    pyahu-local-tls",
		"next: pyahu up",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestCertsRotateJSON(t *testing.T) {
	stackPath := writePresetStack(t, "platform")

	stdout, _, err := executeTestCommand(t, nil, "--file", stackPath, "--output", "json", "certs", "rotate")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Rotated     bool     `json:"rotated"`
		CA          string   `json:"ca"`
		Certificate string   `json:"certificate"`
		Domains     []string `json:"domains"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Rotated || got.CA == "" || got.Certificate == "" {
		t.Fatalf("unexpected rotate output: %#v", got)
	}
	if len(got.Domains) == 0 {
		t.Fatalf("missing domains: %#v", got)
	}
}

func TestBackupPostgresWritesDumpFile(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	backupDir := t.TempDir()
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) {
			return fakeKube{backupData: "dump-data"}, nil
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "backup", "postgres", "--dir", backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "backup written:") {
		t.Fatalf("stdout does not include backup path:\n%s", stdout)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("backup files = %d, want 1", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "dump-data" {
		t.Fatalf("backup data = %q", data)
	}
}

func TestRestorePostgresReadsDumpFile(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	dumpPath := filepath.Join(t.TempDir(), "app.dump")
	if err := os.WriteFile(dumpPath, []byte("dump-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	var restored bytes.Buffer
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) {
			return fakeKube{restoreSink: &restored}, nil
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "restore", "postgres", "--source", dumpPath, "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if restored.String() != "dump-data" {
		t.Fatalf("restored data = %q", restored.String())
	}
	if !strings.Contains(stdout, "restore completed:") {
		t.Fatalf("stdout does not include restore completion:\n%s", stdout)
	}
}

func TestRestorePostgresCleanRequiresConfirmation(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	dumpPath := filepath.Join(t.TempDir(), "app.dump")
	if err := os.WriteFile(dumpPath, []byte("dump-data"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) {
			return fakeKube{}, nil
		}
	}

	_, _, err := executeTestCommand(t, mutate, "--file", stackPath, "restore", "postgres", "--source", dumpPath)
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenRestoreSourceDownloadsS3WithCustomEndpoint(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	argsPath := filepath.Join(dir, "args.txt")
	awsPath := filepath.Join(binDir, "aws")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuoteForTest(argsPath) + "\nprintf 's3-dump' > \"$4\"\n"
	if err := os.WriteFile(awsPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	reader, path, cleanup, err := openRestoreSource(context.Background(), "s3://bucket/app.dump", "http://localhost:9000")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "s3-dump" {
		t.Fatalf("downloaded data = %q", data)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected temporary restore file: %v", err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"s3\n", "cp\n", "s3://bucket/app.dump\n", "--endpoint-url\n", "http://localhost:9000\n"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("aws args missing %q:\n%s", want, args)
		}
	}
}

func shellQuoteForTest(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func TestDoctorJSONFailureReturnsDependencyCode(t *testing.T) {
	mutate := func(a *app) {
		a.deps.loadStack = func(path string) (*config.LoadedStack, error) {
			return nil, errors.New("not found")
		}
		a.deps.clusterExists = func(ctx context.Context, stack *schema.Stack) bool {
			return false
		}
		a.deps.runDoctor = func(ctx context.Context, stack *schema.Stack, clusterExists bool) []doctor.Check {
			return []doctor.Check{{Name: "k3d", OK: false, Message: "missing"}}
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "doctor", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode(err) != 3 {
		t.Fatalf("exit code = %d", exitCode(err))
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q", stdout)
	}
	if got["ok"] != false {
		t.Fatalf("ok = %#v", got["ok"])
	}
}

func TestUpStopsAfterFailedPreflight(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	rt := &fakeRuntime{installed: true, exists: false, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime {
			return rt
		}
		a.deps.runDoctor = func(ctx context.Context, stack *schema.Stack, clusterExists bool) []doctor.Check {
			return []doctor.Check{{Name: "port:postgres", OK: false, Message: "busy"}}
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "up", "--skip-wait", "--output", "json")
	if err == nil {
		t.Fatal("expected error")
	}
	if exitCode(err) != 3 {
		t.Fatalf("exit code = %d", exitCode(err))
	}
	if rt.createCalled {
		t.Fatal("Create was called after failed preflight")
	}
	if strings.Contains(stdout, "[preflight]") {
		t.Fatalf("json output includes human progress: %q", stdout)
	}
}

func TestUpPrintsWarningsAndContinues(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	rt := &fakeRuntime{installed: true, exists: false, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime {
			return rt
		}
		a.deps.newKube = func(kubeconfig string) (localKube, error) {
			return fakeKube{}, nil
		}
		a.deps.runDoctor = func(ctx context.Context, stack *schema.Stack, clusterExists bool) []doctor.Check {
			return []doctor.Check{{
				Name:     "local-clusters",
				OK:       true,
				Severity: "warning",
				Message:  "found other local Kubernetes clusters: kind/demo",
			}}
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "up", "--skip-wait")
	if err != nil {
		t.Fatal(err)
	}
	if !rt.createCalled {
		t.Fatal("Create was not called")
	}
	if !strings.Contains(stdout, "local-clusters") || !strings.Contains(stdout, "warn") {
		t.Fatalf("stdout does not include warning:\n%s", stdout)
	}
}

func TestServicesCommandJSON(t *testing.T) {
	stackPath := writePresetStack(t, "platform")
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	k := fakeKube{statuses: []kube.ServiceStatus{{
		Name:    "postgres",
		Enabled: true,
		Ready:   true,
		Message: "ready",
		Pods:    []kube.PodStatus{{Name: "postgres-0", Ready: true, Phase: "Running"}},
	}}}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return k, nil }
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "services", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Running  bool `json:"running"`
		Services []struct {
			Name      string `json:"name"`
			Status    string `json:"status"`
			Endpoints []struct {
				URL string `json:"url"`
			} `json:"endpoints"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Running {
		t.Fatal("expected running cluster")
	}
	if got.Services[0].Name != "postgres" || got.Services[0].Status != "ready" {
		t.Fatalf("unexpected first service: %#v", got.Services[0])
	}
	if got.Services[0].Endpoints[0].URL == "" {
		t.Fatalf("expected postgres endpoint URL: %#v", got.Services[0].Endpoints)
	}
}

func TestDescribeCommandHumanOutput(t *testing.T) {
	stackPath := writePresetStack(t, "platform")
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	k := fakeKube{statuses: []kube.ServiceStatus{{
		Name:    "postgres",
		Enabled: true,
		Ready:   true,
		Message: "ready",
		Pods:    []kube.PodStatus{{Name: "postgres-0", Ready: true, Phase: "Running"}},
	}}}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return k, nil }
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "describe", "postgres")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"service:   PostgreSQL",
		"status:    ready",
		"POSTGRES_URL",
		"POSTGRES_PASSWORD            <hidden>",
		"postgresql://pyahu:hidden@localhost:5432/app?sslmode=disable",
		"postgres-0",
		"postgres.pyahu-local-dev.svc.cluster.local:5432",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "pyahu_local") {
		t.Fatalf("stdout leaks default password:\n%s", stdout)
	}
}

func TestDescribeCommandShowSecrets(t *testing.T) {
	stackPath := writePresetStack(t, "platform")
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return fakeKube{}, nil }
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "describe", "postgres", "--show-secrets")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "POSTGRES_PASSWORD            pyahu_local") {
		t.Fatalf("stdout does not show secrets:\n%s", stdout)
	}
}

func executeTestCommand(t *testing.T, mutate func(*app), args ...string) (string, string, error) {
	t.Helper()
	isolateUserConfigDir(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	a := newApp("test", "commit", "date", &stdout, &stderr)
	if mutate != nil {
		mutate(a)
	}
	cmd := a.newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func isolateUserConfigDir(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	t.Setenv("APPDATA", filepath.Join(root, "AppData", "Roaming"))
	t.Setenv("USERPROFILE", root)
}

func writePresetStack(t *testing.T, preset string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pyahu.yaml")
	content, err := config.Preset(preset)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type fakeRuntime struct {
	installed    bool
	exists       bool
	kubeconfig   string
	createCalled bool
	deleteCalled bool
}

func (r *fakeRuntime) CheckInstalled() error {
	if !r.installed {
		return errors.New("missing")
	}
	return nil
}

func (r *fakeRuntime) Exists(ctx context.Context, name string) (bool, error) {
	return r.exists, nil
}

func (r *fakeRuntime) Create(ctx context.Context, stack *schema.Stack, stackDir string) (bool, error) {
	r.createCalled = true
	return !r.exists, nil
}

func (r *fakeRuntime) Delete(ctx context.Context, name string) error {
	r.deleteCalled = true
	return nil
}

func (r *fakeRuntime) Kubeconfig(ctx context.Context, name string) (string, error) {
	return r.kubeconfig, nil
}

type fakeKube struct {
	statuses    []kube.ServiceStatus
	backupData  string
	restoreSink *bytes.Buffer
}

func (k fakeKube) WaitForAPI(ctx context.Context, timeout time.Duration) error { return nil }
func (k fakeKube) ApplyStack(ctx context.Context, stack *schema.Stack, stackDir string) error {
	return nil
}
func (k fakeKube) WaitForStack(ctx context.Context, stack *schema.Stack) error { return nil }
func (k fakeKube) DeleteNamespace(ctx context.Context, namespace string) error { return nil }
func (k fakeKube) Status(ctx context.Context, stack *schema.Stack) ([]kube.ServiceStatus, error) {
	return k.statuses, nil
}
func (k fakeKube) Logs(ctx context.Context, namespace string, service string, follow bool, tail int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (k fakeKube) BackupPostgres(ctx context.Context, stack *schema.Stack, database string, out io.Writer) error {
	_, err := io.WriteString(out, k.backupData)
	return err
}
func (k fakeKube) RestorePostgres(ctx context.Context, stack *schema.Stack, database string, in io.Reader, opts kube.PostgresRestoreOptions) error {
	if k.restoreSink == nil {
		_, err := io.Copy(io.Discard, in)
		return err
	}
	_, err := io.Copy(k.restoreSink, in)
	return err
}
