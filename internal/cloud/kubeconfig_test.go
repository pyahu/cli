package cloud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

func environment() Environment {
	return Environment{
		OrganizationID: "org-1",
		TenantID:       "tenant-1",
		Name:           "production",
		Server:         "https://kube.prod.example.cloud",
		Namespace:      "pyahu-acme-production",
		ContextName:    "pyahu-acme-production",
		Level:          "FULL",
	}
}

const existingConfig = `apiVersion: v1
kind: Config
current-context: work
clusters:
  - name: work
    cluster:
      server: https://work.example
contexts:
  - name: work
    context:
      cluster: work
      user: work
users:
  - name: work
    user:
      token: a-token-somebody-needs
`

// The property that makes this file harmless to copy, commit by accident or read off a stolen laptop.
func TestWrittenContextCarriesNoCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")

	if _, err := WriteContext(environment(), path); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, forbidden := range []string{"token:", "client-key", "password", "client-certificate-data"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("the kubeconfig must carry no credential, found %q", forbidden)
		}
	}
	if !strings.Contains(string(raw), "command: pyahu") {
		t.Fatal("the kubeconfig must declare the exec plugin")
	}
}

// A kubeconfig is where somebody keeps access to every cluster they work with. A tool that rewrites it
// wholesale eventually destroys a context nobody else has a copy of.
func TestWritingMergesAndLeavesOtherContextsAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(existingConfig), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := WriteContext(environment(), path); err != nil {
		t.Fatalf("write: %v", err)
	}

	merged, err := clientcmd.LoadFromFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := merged.Contexts["work"]; !ok {
		t.Fatal("the pre-existing context was destroyed")
	}
	if merged.AuthInfos["work"].Token != "a-token-somebody-needs" {
		t.Fatal("the pre-existing credential was destroyed")
	}
	if _, ok := merged.Contexts["pyahu-acme-production"]; !ok {
		t.Fatal("the new context is missing")
	}
	if merged.CurrentContext != "pyahu-acme-production" {
		t.Fatalf("the new context must become current, got %q", merged.CurrentContext)
	}
	if merged.Contexts["pyahu-acme-production"].Namespace != "pyahu-acme-production" {
		t.Fatal("the namespace must default to the tenant's, so kubectl needs no -n")
	}
}

func TestWritingBacksUpWhatItFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(existingConfig), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	backup, err := WriteContext(environment(), path)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	if backup == "" {
		t.Fatal("an existing kubeconfig must be backed up before it is changed")
	}
	saved, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(saved) != existingConfig {
		t.Fatal("the backup must be what was there before")
	}
}

// Nothing to back up is not a failure, and must not produce an empty file pretending to be one.
func TestFirstWriteHasNothingToBackUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")

	backup, err := WriteContext(environment(), path)

	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if backup != "" {
		t.Fatalf("expected no backup, got %q", backup)
	}
}

// Somebody who points KUBECONFIG at a throwaway file is deliberately keeping this out of their real
// config. Writing to the default anyway would override a decision they made on purpose.
func TestKubeconfigPathHonoursTheEnvironment(t *testing.T) {
	t.Setenv("KUBECONFIG", "/tmp/one:/tmp/two")

	if got := KubeconfigPath(); got != "/tmp/one" {
		t.Fatalf("expected the first entry of KUBECONFIG, got %q", got)
	}
}

// The expiry is what makes this cheap: kubectl caches the token until then, so a ten-minute credential
// costs one exchange per ten minutes rather than one per command.
func TestExecCredentialCarriesItsExpiry(t *testing.T) {
	credential := ClusterCredential{Token: "t"}
	credential.ExpiresAt = credential.ExpiresAt.AddDate(2026, 0, 0)

	exec := NewExecCredential(credential)

	if exec.APIVersion != ExecAPIVersion || exec.Kind != "ExecCredential" {
		t.Fatalf("unexpected shape %+v", exec)
	}
	if exec.Status.ExpirationTimestamp != credential.ExpiresAt {
		t.Fatal("the expiry must travel, or kubectl calls this binary on every command")
	}
}
