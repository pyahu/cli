package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pyahu/cli/internal/config"
	"github.com/pyahu/cli/internal/doctor"
	"github.com/pyahu/cli/internal/kube"
	"github.com/pyahu/cli/internal/update"
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
	defer func() { _ = reader.Close() }()
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
	if hint := errorHint(err); hint != "" {
		t.Fatalf("preflight failure has circular hint: %q", hint)
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

func TestUpSummaryRedactsSecrets(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	rt := &fakeRuntime{installed: true, exists: false, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return fakeKube{}, nil }
		a.deps.runDoctor = func(ctx context.Context, stack *schema.Stack, clusterExists bool) []doctor.Check {
			return []doctor.Check{{Name: "k3d", OK: true, Message: "ok"}}
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "up", "--skip-wait", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "pyahu_local") {
		t.Fatalf("human up summary leaked a password:\n%s", stdout)
	}
	for _, want := range []string{
		"POSTGRES_PASSWORD            <hidden>",
		"postgresql://pyahu:hidden@localhost:5432/app?sslmode=disable",
		"next: eval \"$(pyahu env)\"",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("human up summary does not contain %q:\n%s", want, stdout)
		}
	}

	stdout, _, err = executeTestCommand(t, mutate, "--file", stackPath, "up", "--skip-wait", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if result.Env["POSTGRES_PASSWORD"] != "<hidden>" {
		t.Fatalf("POSTGRES_PASSWORD = %q", result.Env["POSTGRES_PASSWORD"])
	}
	if strings.Contains(result.Env["POSTGRES_URL"], "pyahu_local") {
		t.Fatalf("POSTGRES_URL leaked a password: %q", result.Env["POSTGRES_URL"])
	}
}

func TestDownPreservesDataByDefault(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	rt := &fakeRuntime{exists: true}
	removeCalled := false
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.removeAll = func(path string) error {
			removeCalled = true
			return nil
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "down", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if !rt.deleteCalled {
		t.Fatal("cluster was not deleted")
	}
	if removeCalled {
		t.Fatal("data was removed without --purge-data")
	}
	if !strings.Contains(stdout, "data retained at") {
		t.Fatalf("retention path is not reported:\n%s", stdout)
	}
}

func TestDownPurgesDataWithYes(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	rt := &fakeRuntime{exists: true}
	removedPath := ""
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.removeAll = func(path string) error {
			removedPath = path
			return nil
		}
	}

	_, _, err := executeTestCommand(t, mutate, "--file", stackPath, "down", "--purge-data", "--yes", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if !rt.deleteCalled {
		t.Fatal("cluster was not deleted")
	}
	wantSuffix := filepath.Join(".pyahu", "clusters", "pyahu-local", "storage")
	if !strings.HasSuffix(removedPath, wantSuffix) {
		t.Fatalf("removed path = %q, want suffix %q", removedPath, wantSuffix)
	}
}

func TestDownPurgeRequiresExplicitNonInteractiveConfirmation(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")
	rt := &fakeRuntime{exists: true}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
	}

	_, _, err := executeTestCommand(t, mutate, "--file", stackPath, "--no-input", "down", "--purge-data")
	if err == nil {
		t.Fatal("expected confirmation error")
	}
	if !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.deleteCalled {
		t.Fatal("cluster was deleted before purge confirmation")
	}
}

func TestDownRejectsPurgeWhileKeepingCluster(t *testing.T) {
	stackPath := writePresetStack(t, "minimal")

	_, _, err := executeTestCommand(t, nil, "--file", stackPath, "down", "--keep-cluster", "--purge-data", "--yes")
	if err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("unexpected error: %v", err)
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
	warnings    []string
}

func (k fakeKube) WaitForAPI(ctx context.Context, timeout time.Duration) error { return nil }
func (k fakeKube) ApplyStack(ctx context.Context, stack *schema.Stack, stackDir string) error {
	return nil
}
func (k fakeKube) WaitForStack(ctx context.Context, stack *schema.Stack) ([]string, error) {
	return k.warnings, nil
}

func (k fakeKube) ApplyConnectors(ctx context.Context, stack *schema.Stack, name string) error {
	return nil
}
func (k fakeKube) CaptureZitadelPAT(ctx context.Context, stack *schema.Stack) error { return nil }
func (k fakeKube) DeleteNamespace(ctx context.Context, namespace string) error      { return nil }
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

// connectStatusJSON is the shape of GET /connectors?expand=status.
const connectStatusJSON = `{
  "allpick-core-outbox": {
    "status": {
      "name": "allpick-core-outbox",
      "type": "source",
      "connector": {"state": "RUNNING", "worker_id": "kafka-connect:8083"},
      "tasks": [{"id": 0, "state": "RUNNING", "worker_id": "kafka-connect:8083"}]
    }
  },
  "checkout-outbox": {
    "status": {
      "name": "checkout-outbox",
      "type": "source",
      "connector": {"state": "RUNNING", "worker_id": "kafka-connect:8083"},
      "tasks": [{"id": 0, "state": "FAILED", "worker_id": "kafka-connect:8083", "trace": "ERR no such key\nat com.redis..."}]
    }
  }
}`

// writeConnectStackFile points services.kafkaConnect.ports.rest at a fake worker,
// so `connectors status` talks to it over the host port like it does for real.
func writeConnectStackFile(t *testing.T, restPort int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pyahu.yaml")
	content := fmt.Sprintf(`apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo
services:
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    ports:
      rest: %d
    connectors:
      - name: allpick-core-outbox
        kind: custom
        optional: true
        config:
          connector.class: io.debezium.connector.postgresql.PostgresConnector
      - name: checkout-outbox
        kind: custom
        optional: true
        config:
          connector.class: com.redis.kafka.connect.RedisStreamSourceConnector
`, restPort)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func startFakeConnect(t *testing.T, body string) int {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestConnectorsStatusFailsOnAFailedTask(t *testing.T) {
	stackPath := writeConnectStackFile(t, startFakeConnect(t, connectStatusJSON))

	stdout, _, err := executeTestCommand(t, nil, "--file", stackPath, "connectors", "status", "--no-color")
	if err == nil {
		t.Fatal("expected a non-zero exit when a task is not RUNNING")
	}
	if got := exitCode(err); got != 5 {
		t.Fatalf("exit code = %d", got)
	}
	want := `CONNECTOR            TASK  STATE    DETAIL
allpick-core-outbox  -     RUNNING  source
                     0     RUNNING  -
checkout-outbox      -     RUNNING  source
                     0     FAILED   ERR no such key
`
	if stdout != want {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestConnectorsStatusJSONReportsEachTask(t *testing.T) {
	stackPath := writeConnectStackFile(t, startFakeConnect(t, connectStatusJSON))

	stdout, _, err := executeTestCommand(t, nil, "--file", stackPath, "connectors", "status", "--format", "json")
	if err == nil {
		t.Fatal("expected a non-zero exit when a task is not RUNNING")
	}
	var got struct {
		Healthy    bool `json:"healthy"`
		Connectors []struct {
			Name  string `json:"name"`
			State string `json:"state"`
			Tasks []struct {
				ID    int    `json:"id"`
				State string `json:"state"`
			} `json:"tasks"`
		} `json:"connectors"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Healthy {
		t.Fatal("healthy should be false when a task is FAILED")
	}
	if len(got.Connectors) != 2 || got.Connectors[0].Name != "allpick-core-outbox" {
		t.Fatalf("connectors = %#v", got.Connectors)
	}
	// The connector row stays RUNNING while its only task is FAILED — the trap
	// this command exists to catch.
	if got.Connectors[1].State != "RUNNING" || got.Connectors[1].Tasks[0].State != "FAILED" {
		t.Fatalf("checkout-outbox = %#v", got.Connectors[1])
	}
}

func TestConnectorsStatusSucceedsWhenEveryTaskRuns(t *testing.T) {
	body := `{"app-cdc": {"status": {"name": "app-cdc", "type": "source",
	  "connector": {"state": "RUNNING"}, "tasks": [{"id": 0, "state": "RUNNING"}]}}}`
	stackPath := writeConnectStackFile(t, startFakeConnect(t, body))

	if _, _, err := executeTestCommand(t, nil, "--file", stackPath, "connectors", "status"); err != nil {
		t.Fatalf("expected success: %v", err)
	}
}

func TestConnectorsStatusFailsWhenAConnectorHasNoTasks(t *testing.T) {
	body := `{"app-cdc": {"status": {"name": "app-cdc", "type": "source",
	  "connector": {"state": "RUNNING"}, "tasks": []}}}`
	stackPath := writeConnectStackFile(t, startFakeConnect(t, body))

	stdout, _, err := executeTestCommand(t, nil, "--file", stackPath, "connectors", "status", "--no-color")
	if err == nil {
		t.Fatal("expected a non-zero exit for a connector with no tasks")
	}
	if !strings.Contains(stdout, "NO TASKS") {
		t.Fatalf("stdout does not report the missing tasks:\n%s", stdout)
	}
}

func TestConnectorsApplyRejectsAnUndeclaredName(t *testing.T) {
	stackPath := writeConnectStackFile(t, 8083)
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return fakeKube{}, nil }
	}

	_, _, err := executeTestCommand(t, mutate, "--file", stackPath, "connectors", "apply", "--name", "nope")
	if err == nil || !strings.Contains(err.Error(), `connector "nope" is not declared`) {
		t.Fatalf("error = %v", err)
	}
	if got := exitCode(err); got != 2 {
		t.Fatalf("exit code = %d", got)
	}
}

func TestConnectorsApplyJSONListsTheAppliedConnectors(t *testing.T) {
	stackPath := writeConnectStackFile(t, 8083)
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return fakeKube{}, nil }
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "connectors", "apply", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Connectors []string `json:"connectors"`
		Applied    bool     `json:"applied"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Applied || strings.Join(got.Connectors, ",") != "allpick-core-outbox,checkout-outbox" {
		t.Fatalf("apply result = %#v", got)
	}
}

func TestUpWarnsAboutPendingOptionalConnectors(t *testing.T) {
	stackPath := writeConnectStackFile(t, 8083)
	rt := &fakeRuntime{installed: true, exists: true, kubeconfig: "/tmp/kubeconfig"}
	warning := "connector checkout-outbox is not healthy yet — run `pyahu connectors apply` after the application has started"
	mutate := func(a *app) {
		a.deps.newRuntime = func(opts options) localRuntime { return rt }
		a.deps.newKube = func(kubeconfig string) (localKube, error) { return fakeKube{warnings: []string{warning}}, nil }
		a.deps.runDoctor = func(ctx context.Context, stack *schema.Stack, clusterExists bool) []doctor.Check {
			return []doctor.Check{{Name: "k3d", OK: true, Message: "ok"}}
		}
	}

	stdout, _, err := executeTestCommand(t, mutate, "--file", stackPath, "up", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, warning) {
		t.Fatalf("up summary does not carry the optional-connector warning:\n%s", stdout)
	}
}

func stubUpgrade(current string, latest string) func(time.Duration) (update.Result, bool) {
	return func(time.Duration) (update.Result, bool) {
		return update.Result{Current: current, Latest: latest}, true
	}
}

func TestUpgradeNoticeGoesToStderrNotStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	a := newApp("0.4.0", "commit", "date", &stdout, &stderr)
	a.opts.noColor = true

	a.printUpgradeNotice(stubUpgrade("0.4.0", "0.6.1"))

	// stdout is consumed by `eval "$(pyahu env)"`; a banner there would be
	// evaluated as shell.
	if stdout.Len() != 0 {
		t.Fatalf("upgrade notice leaked to stdout:\n%s", stdout.String())
	}
	got := stderr.String()
	for _, want := range []string{
		"pyahu 0.4.0 is out of date — 0.6.1 is available",
		"https://github.com/pyahu/cli/releases/tag/v0.6.1",
		update.OptOutEnv,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stderr does not contain %q:\n%s", want, got)
		}
	}
}

func TestUpgradeNoticeIsSuppressedForMachineOutput(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*app)
	}{
		// JSON output is parsed by scripts; --quiet asked for silence.
		{"json", func(a *app) { a.opts.output = "json" }},
		{"quiet", func(a *app) { a.opts.quiet = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			a := newApp("0.4.0", "commit", "date", &stdout, &stderr)
			tc.mutate(a)

			a.printUpgradeNotice(stubUpgrade("0.4.0", "0.6.1"))

			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("expected silence, got stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestUpgradeNoticeIsSilentWhenUpToDate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	a := newApp("0.6.1", "commit", "date", &stdout, &stderr)

	a.printUpgradeNotice(func(time.Duration) (update.Result, bool) { return update.Result{}, false })

	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silence, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func upgradeApp(t *testing.T, version string, latest string, replaced *[]byte) func(*app) {
	t.Helper()
	return func(a *app) {
		a.opts.version = version
		a.deps.latestRelease = func(context.Context) (string, error) { return latest, nil }
		a.deps.downloadRelease = func(context.Context, update.Release) ([]byte, error) {
			return []byte("new binary"), nil
		}
		a.deps.replaceBinary = func(path string, binary []byte) error {
			if replaced != nil {
				*replaced = binary
			}
			return nil
		}
	}
}

func TestCheckUpdateReportsOutdatedAsJSON(t *testing.T) {
	stdout, _, err := executeTestCommand(t, upgradeApp(t, "0.4.0", "0.7.0", nil), "check-update", "--output", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Current  string `json:"current"`
		Latest   string `json:"latest"`
		Outdated bool   `json:"outdated"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Current != "0.4.0" || got.Latest != "0.7.0" || !got.Outdated {
		t.Fatalf("check-update = %#v", got)
	}
}

func TestCheckUpdateExitsZeroUnlessAskedOtherwise(t *testing.T) {
	// Being behind is not a command failure, so the default exit stays 0.
	if _, _, err := executeTestCommand(t, upgradeApp(t, "0.4.0", "0.7.0", nil), "check-update"); err != nil {
		t.Fatalf("default exit should be 0: %v", err)
	}
	_, _, err := executeTestCommand(t, upgradeApp(t, "0.4.0", "0.7.0", nil), "check-update", "--exit-code")
	if err == nil {
		t.Fatal("--exit-code should fail when a release is available")
	}
	if code := exitCode(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestCheckUpdateSaysWhenCurrent(t *testing.T) {
	stdout, _, err := executeTestCommand(t, upgradeApp(t, "0.7.0", "0.7.0", nil), "check-update", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "pyahu 0.7.0 is the latest release") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestUpgradeReplacesTheBinary(t *testing.T) {
	var replaced []byte
	stdout, _, err := executeTestCommand(t, upgradeApp(t, "0.4.0", "0.7.0", &replaced), "upgrade", "--yes", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if string(replaced) != "new binary" {
		t.Fatalf("replaced with %q", replaced)
	}
	if !strings.Contains(stdout, "pyahu 0.7.0 installed") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestUpgradeIsANoOpWhenCurrent(t *testing.T) {
	var replaced []byte
	stdout, _, err := executeTestCommand(t, upgradeApp(t, "0.7.0", "0.7.0", &replaced), "upgrade", "--yes", "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if replaced != nil {
		t.Fatal("an up-to-date binary must not be replaced")
	}
	if !strings.Contains(stdout, "already the latest release") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestUpgradeRefusesWithoutConfirmation(t *testing.T) {
	var replaced []byte
	// --no-input means nobody is there to answer; replacing the binary anyway
	// would be a surprise edit to the user's machine.
	_, _, err := executeTestCommand(t, upgradeApp(t, "0.4.0", "0.7.0", &replaced), "upgrade", "--no-input")
	if err == nil || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("error = %v", err)
	}
	if replaced != nil {
		t.Fatal("the binary must not be replaced without confirmation")
	}
}

func TestUpgradeInstallsAPinnedVersion(t *testing.T) {
	var replaced []byte
	mutate := func(a *app) {
		upgradeApp(t, "0.7.0", "0.7.0", &replaced)(a)
		// A pinned version must not consult the releases endpoint at all, so a
		// downgrade works even when the lookup would say "you are current".
		a.deps.latestRelease = func(context.Context) (string, error) {
			t.Error("--version must not need the releases endpoint")
			return "", nil
		}
		a.deps.downloadRelease = func(_ context.Context, release update.Release) ([]byte, error) {
			if release.Version != "0.5.0" {
				t.Errorf("release version = %q, want 0.5.0", release.Version)
			}
			return []byte("pinned binary"), nil
		}
	}

	if _, _, err := executeTestCommand(t, mutate, "upgrade", "--yes", "--version", "v0.5.0", "--no-color"); err != nil {
		t.Fatal(err)
	}
	if string(replaced) != "pinned binary" {
		t.Fatalf("replaced with %q", replaced)
	}
}

func TestUpgradeLeavesAManagedBinaryAlone(t *testing.T) {
	// The real binary under test lives in the Go test cache, so point GOPATH at
	// it to make DetectManager see a `go install` binary.
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOBIN", filepath.Dir(executable))
	t.Setenv("GOPATH", "")

	var replaced []byte
	_, _, err = executeTestCommand(t, upgradeApp(t, "0.4.0", "0.7.0", &replaced), "upgrade", "--yes")
	if err == nil || !strings.Contains(err.Error(), "managed by another tool") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "go install github.com/pyahu/cli/cmd/pyahu@latest") {
		t.Fatalf("the message must name the command to run: %v", err)
	}
	if replaced != nil {
		t.Fatal("a binary owned by another tool must never be replaced")
	}
}

func TestUpgradeRefusesOnADevBuild(t *testing.T) {
	var replaced []byte
	_, _, err := executeTestCommand(t, upgradeApp(t, "dev", "0.7.0", &replaced), "upgrade", "--yes")
	if err == nil || !strings.Contains(err.Error(), "locally built binary") {
		t.Fatalf("error = %v", err)
	}
	if replaced != nil {
		t.Fatal("a dev build must not be replaced")
	}
}
