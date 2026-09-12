package schema

import "testing"

func TestSetDefaultsUsesSupportedK3SImage(t *testing.T) {
	stack := &Stack{Metadata: Metadata{Name: "demo"}}

	stack.SetDefaults()

	if stack.Cluster.K3SVersion != DefaultK3SImage {
		t.Fatalf("cluster k3sVersion = %q, want %q", stack.Cluster.K3SVersion, DefaultK3SImage)
	}
}
