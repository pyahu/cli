package cloud

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ExecAPIVersion is the client-go exec credential contract this CLI answers on.
const ExecAPIVersion = "client.authentication.k8s.io/v1"

// KubeconfigPath is the file this CLI merges into: $KUBECONFIG when set, the default otherwise.
//
// Honouring $KUBECONFIG matters more than it looks: somebody who points it at a throwaway file is
// deliberately keeping this out of their real config, and writing to the default anyway would override
// a decision they made on purpose.
func KubeconfigPath() string {
	if explicit := os.Getenv("KUBECONFIG"); explicit != "" {
		// A list means "merge these"; writing to the first is what kubectl itself does.
		if first := filepath.SplitList(explicit); len(first) > 0 && first[0] != "" {
			return first[0]
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// WriteContext merges one environment into the kubeconfig and returns where it wrote and what it
// backed up.
//
// It MERGES rather than replaces, and backs the file up before writing. A kubeconfig is not this CLI's
// file: it is where somebody keeps access to every cluster they work with, and a tool that rewrites it
// wholesale eventually destroys a context nobody else has a copy of.
//
// The entry it writes carries NO credential. It records where the cluster is, which namespace to
// default to, and that this binary should be asked for a token when one is needed. A kubeconfig built
// this way can be copied, committed by accident or read off a stolen laptop without granting anything.
func WriteContext(env Environment, path string) (backup string, err error) {
	if path == "" {
		return "", fmt.Errorf("cannot locate a kubeconfig to write")
	}

	existing, err := clientcmd.LoadFromFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	if existing == nil {
		existing = clientcmdapi.NewConfig()
	} else {
		backup, err = backupFile(path)
		if err != nil {
			return "", err
		}
	}

	name := env.ContextName
	existing.Clusters[name] = &clientcmdapi.Cluster{Server: env.Server}
	existing.AuthInfos[name] = &clientcmdapi.AuthInfo{
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: ExecAPIVersion,
			Command:    "pyahu",
			Args: []string{
				"kube", "credentials",
				"--org", env.OrganizationID,
				"--tenant", env.TenantID,
			},
			// The plugin prints one JSON object and exits; it never needs the terminal, and asking for
			// it would make `kubectl` in a script hang waiting for one that is not there.
			InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
			InstallHint: "The Pyahu CLI is required to reach this cluster.\n" +
				"Install it with: curl -fsSL https://cli.pyahu.io/install.sh | sh",
		},
	}
	existing.Contexts[name] = &clientcmdapi.Context{
		Cluster:   name,
		AuthInfo:  name,
		Namespace: env.Namespace,
	}
	existing.CurrentContext = name

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return backup, err
	}
	if err := clientcmd.WriteToFile(*existing, path); err != nil {
		return backup, fmt.Errorf("write %s: %w", path, err)
	}
	return backup, nil
}

// backupFile copies a file next to itself with a timestamp, and returns the copy's name.
func backupFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	backup := fmt.Sprintf("%s.bak.%s", path, time.Now().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		return "", fmt.Errorf("back up %s: %w", path, err)
	}
	return backup, nil
}

// ExecCredential is the object `kubectl` reads from this CLI's stdout.
type ExecCredential struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Status     ExecCredentialInfo `json:"status"`
}

// ExecCredentialInfo carries the token and, crucially, when it stops working.
//
// `expirationTimestamp` is what makes this cheap: kubectl caches the token in memory until then and
// only calls this binary again afterwards, so a ten-minute credential costs one exchange per ten
// minutes rather than one per command.
type ExecCredentialInfo struct {
	Token               string    `json:"token"`
	ExpirationTimestamp time.Time `json:"expirationTimestamp"`
}

// NewExecCredential shapes a credential the way client-go expects it.
func NewExecCredential(c ClusterCredential) ExecCredential {
	return ExecCredential{
		APIVersion: ExecAPIVersion,
		Kind:       "ExecCredential",
		Status: ExecCredentialInfo{
			Token:               c.Token,
			ExpirationTimestamp: c.ExpiresAt,
		},
	}
}
