package certs

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCreatesLocalCAAndWildcardCertificate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stackDir := t.TempDir()

	bundle, err := Ensure(stackDir, []string{"zitadel.localhost"})
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{bundle.Paths.CACert, bundle.Paths.CAKey, bundle.Paths.Cert, bundle.Paths.Key} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if got := PEMBlockCount(bundle.CertificatePEM, "CERTIFICATE"); got != 2 {
		t.Fatalf("certificate chain blocks = %d, want 2", got)
	}

	cert := parseTestCertificate(t, bundle.LeafCertificatePEM)
	for _, host := range []string{"localhost", "zitadel.localhost", "api.localhost"} {
		if err := cert.VerifyHostname(host); err != nil {
			t.Fatalf("certificate does not verify %s: %v", host, err)
		}
	}

	status, err := Inspect(stackDir, []string{"zitadel.localhost"})
	if err != nil {
		t.Fatal(err)
	}
	if !status.CA.Valid {
		t.Fatalf("CA status = %#v", status.CA)
	}
	if !status.Certificate.Valid {
		t.Fatalf("certificate status = %#v", status.Certificate)
	}
}

func TestEnsureIsIdempotent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stackDir := t.TempDir()

	first, err := Ensure(stackDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Ensure(stackDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if string(first.LeafCertificatePEM) != string(second.LeafCertificatePEM) {
		t.Fatal("Ensure regenerated a valid certificate")
	}
	if filepath.Dir(second.Paths.Cert) != filepath.Join(stackDir, ".pyahu", "local", "certs") {
		t.Fatalf("project cert dir = %s", filepath.Dir(second.Paths.Cert))
	}
}

func TestRotateReplacesLocalCA(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	stackDir := t.TempDir()

	first, err := Ensure(stackDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := Rotate(stackDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	if string(first.CACertificatePEM) == string(rotated.CACertificatePEM) {
		t.Fatal("Rotate did not replace the CA certificate")
	}
}

func parseTestCertificate(t *testing.T, data []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("missing certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
