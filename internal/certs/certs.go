package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pyahu/cli/pkg/schema"
)

const (
	caCertFile     = "ca.crt"
	caKeyFile      = "ca.key"
	serverCertFile = "localhost.crt"
	serverKeyFile  = "localhost.key"
)

var (
	caValidity     = 10 * 365 * 24 * time.Hour
	serverValidity = 397 * 24 * time.Hour
	renewBefore    = 30 * 24 * time.Hour
)

type Paths struct {
	CADir      string `json:"caDir"`
	CACert     string `json:"caCert"`
	CAKey      string `json:"caKey"`
	ProjectDir string `json:"projectDir"`
	Cert       string `json:"cert"`
	Key        string `json:"key"`
}

type Bundle struct {
	Paths              Paths     `json:"paths"`
	Domains            []string  `json:"domains"`
	CertificatePEM     []byte    `json:"-"`
	LeafCertificatePEM []byte    `json:"-"`
	PrivateKeyPEM      []byte    `json:"-"`
	CACertificatePEM   []byte    `json:"-"`
	ExpiresAt          time.Time `json:"expiresAt"`
}

type Status struct {
	Paths       Paths      `json:"paths"`
	Domains     []string   `json:"domains"`
	CA          CertStatus `json:"ca"`
	Certificate CertStatus `json:"certificate"`
	HostTrusted bool       `json:"hostTrusted"`
}

type CertStatus struct {
	Exists    bool      `json:"exists"`
	Valid     bool      `json:"valid"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type TrustOptions struct {
	NoInput bool
	Verbose bool
	Out     io.Writer
	Err     io.Writer
}

func PathsFor(stackDir string) (Paths, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	if stackDir == "" {
		stackDir = "."
	}
	caDir := filepath.Join(configDir, "pyahu", "certs")
	projectDir := filepath.Join(stackDir, schema.DefaultLocalStateDir, "certs")
	return Paths{
		CADir:      caDir,
		CACert:     filepath.Join(caDir, caCertFile),
		CAKey:      filepath.Join(caDir, caKeyFile),
		ProjectDir: projectDir,
		Cert:       filepath.Join(projectDir, serverCertFile),
		Key:        filepath.Join(projectDir, serverKeyFile),
	}, nil
}

func EnsureCA() (Paths, []byte, error) {
	paths, err := PathsFor("")
	if err != nil {
		return Paths{}, nil, err
	}
	caCert, _, caPEM, err := ensureCA(paths)
	if err != nil {
		return Paths{}, nil, err
	}
	if !caCert.IsCA {
		return Paths{}, nil, errors.New("local CA certificate is not a CA")
	}
	return paths, caPEM, nil
}

func Ensure(stackDir string, domains []string) (*Bundle, error) {
	paths, err := PathsFor(stackDir)
	if err != nil {
		return nil, err
	}
	caCert, caKey, caPEM, err := ensureCA(paths)
	if err != nil {
		return nil, err
	}
	leafPEM, keyPEM, expiresAt, err := ensureServerCertificate(paths, caCert, caKey, normalizeDomains(domains))
	if err != nil {
		return nil, err
	}
	chainPEM := append(append([]byte{}, leafPEM...), caPEM...)
	return &Bundle{
		Paths:              paths,
		Domains:            normalizeDomains(domains),
		CertificatePEM:     chainPEM,
		LeafCertificatePEM: leafPEM,
		PrivateKeyPEM:      keyPEM,
		CACertificatePEM:   caPEM,
		ExpiresAt:          expiresAt,
	}, nil
}

func Rotate(stackDir string, domains []string) (*Bundle, error) {
	paths, err := PathsFor(stackDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(paths.CADir, 0o755); err != nil {
		return nil, fmt.Errorf("create CA directory: %w", err)
	}
	if err := os.MkdirAll(paths.ProjectDir, 0o755); err != nil {
		return nil, fmt.Errorf("create certificate directory: %w", err)
	}
	caCert, caKey, caPEM, err := createCA()
	if err != nil {
		return nil, err
	}
	if err := writePEM(paths.CACert, caPEM, 0o644); err != nil {
		return nil, err
	}
	caKeyPEM, err := encodePrivateKey(caKey)
	if err != nil {
		return nil, err
	}
	if err := writePEM(paths.CAKey, caKeyPEM, 0o600); err != nil {
		return nil, err
	}
	leafPEM, keyPEM, expiresAt, err := createServerCertificate(caCert, caKey, normalizeDomains(domains))
	if err != nil {
		return nil, err
	}
	if err := writePEM(paths.Cert, leafPEM, 0o644); err != nil {
		return nil, err
	}
	if err := writePEM(paths.Key, keyPEM, 0o600); err != nil {
		return nil, err
	}
	return &Bundle{
		Paths:              paths,
		Domains:            normalizeDomains(domains),
		CertificatePEM:     append(append([]byte{}, leafPEM...), caPEM...),
		LeafCertificatePEM: leafPEM,
		PrivateKeyPEM:      keyPEM,
		CACertificatePEM:   caPEM,
		ExpiresAt:          expiresAt,
	}, nil
}

func Inspect(stackDir string, domains []string) (Status, error) {
	paths, err := PathsFor(stackDir)
	if err != nil {
		return Status{}, err
	}
	status := Status{Paths: paths, Domains: normalizeDomains(domains)}

	caPEM, err := os.ReadFile(paths.CACert)
	if err != nil {
		if os.IsNotExist(err) {
			status.CA.Message = "missing"
			status.Certificate.Message = "missing"
			return status, nil
		}
		return Status{}, fmt.Errorf("read local CA certificate: %w", err)
	}
	status.CA.Exists = true
	caCert, err := parseCertificatePEM(caPEM)
	if err != nil {
		status.CA.Message = err.Error()
	} else {
		status.CA.ExpiresAt = caCert.NotAfter
		status.CA.Valid = caCert.IsCA && time.Now().Before(caCert.NotAfter)
		if !status.CA.Valid {
			status.CA.Message = "expired or not a CA"
		}
		trusted, trustErr := CATrusted(caPEM)
		if trustErr == nil {
			status.HostTrusted = trusted
		}
	}

	certPEM, err := os.ReadFile(paths.Cert)
	if err != nil {
		if os.IsNotExist(err) {
			status.Certificate.Message = "missing"
			return status, nil
		}
		return Status{}, fmt.Errorf("read local TLS certificate: %w", err)
	}
	status.Certificate.Exists = true
	cert, err := parseCertificatePEM(certPEM)
	if err != nil {
		status.Certificate.Message = err.Error()
		return status, nil
	}
	status.Certificate.ExpiresAt = cert.NotAfter
	if caCert == nil {
		status.Certificate.Message = "local CA is invalid"
		return status, nil
	}
	keyPEM, err := os.ReadFile(paths.Key)
	if err != nil {
		status.Certificate.Message = err.Error()
		return status, nil
	}
	key, err := parsePrivateKeyPEM(keyPEM)
	if err != nil {
		status.Certificate.Message = err.Error()
		return status, nil
	}
	if serverCertificateUsable(cert, key, caCert, normalizeDomains(domains)) {
		status.Certificate.Valid = true
	} else {
		status.Certificate.Message = "expired, unsigned by the local CA, missing a required domain, or key mismatch"
	}
	return status, nil
}

func CATrusted(caPEM []byte) (bool, error) {
	caCert, err := parseCertificatePEM(caPEM)
	if err != nil {
		return false, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return false, err
	}
	_, err = caCert.Verify(x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	return err == nil, nil
}

func TrustHost(ctx context.Context, caPath string, opts TrustOptions) error {
	switch runtime.GOOS {
	case "linux":
		if _, err := exec.LookPath("update-ca-certificates"); err == nil {
			if err := runPrivileged(ctx, opts, caPath, "install", "-m", "0644", caPath, "/usr/local/share/ca-certificates/pyahu-local-ca.crt"); err != nil {
				return err
			}
			return runPrivileged(ctx, opts, caPath, "update-ca-certificates")
		}
		if _, err := exec.LookPath("trust"); err == nil {
			return runPrivileged(ctx, opts, caPath, "trust", "anchor", caPath)
		}
		return fmt.Errorf("no supported Linux trust tool found; install ca-certificates or p11-kit and run again")
	case "darwin":
		return runPrivileged(ctx, opts, caPath, "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", caPath)
	case "windows":
		return runCommand(ctx, opts, "certutil", "-addstore", "-f", "Root", caPath)
	default:
		return fmt.Errorf("installing the host trust store is not supported on %s", runtime.GOOS)
	}
}

func TrustInstructions(caPath string) string {
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("sudo install -m 0644 %s /usr/local/share/ca-certificates/pyahu-local-ca.crt\nsudo update-ca-certificates", caPath)
	case "darwin":
		return fmt.Sprintf("sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s", caPath)
	case "windows":
		return fmt.Sprintf("certutil -addstore -f Root %s", caPath)
	default:
		return fmt.Sprintf("install %s as a trusted root CA for this host", caPath)
	}
}

func ensureCA(paths Paths) (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	certPEM, certErr := os.ReadFile(paths.CACert)
	keyPEM, keyErr := os.ReadFile(paths.CAKey)
	if certErr == nil && keyErr == nil {
		cert, err := parseCertificatePEM(certPEM)
		if err == nil && cert.IsCA && time.Now().Add(renewBefore).Before(cert.NotAfter) {
			key, err := parsePrivateKeyPEM(keyPEM)
			if err == nil {
				return cert, key, certPEM, nil
			}
		}
	}
	if certErr != nil && !os.IsNotExist(certErr) {
		return nil, nil, nil, fmt.Errorf("read local CA certificate: %w", certErr)
	}
	if keyErr != nil && !os.IsNotExist(keyErr) {
		return nil, nil, nil, fmt.Errorf("read local CA key: %w", keyErr)
	}
	if err := os.MkdirAll(paths.CADir, 0o755); err != nil {
		return nil, nil, nil, fmt.Errorf("create CA directory: %w", err)
	}
	cert, key, certPEM, err := createCA()
	if err != nil {
		return nil, nil, nil, err
	}
	keyPEM, err = encodePrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := writePEM(paths.CACert, certPEM, 0o644); err != nil {
		return nil, nil, nil, err
	}
	if err := writePEM(paths.CAKey, keyPEM, 0o600); err != nil {
		return nil, nil, nil, err
	}
	return cert, key, certPEM, nil
}

func runPrivileged(ctx context.Context, opts TrustOptions, caPath string, name string, args ...string) error {
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		if opts.NoInput {
			return fmt.Errorf("trust installation may require sudo; rerun without --no-input or run manually:\n%s", TrustInstructions(caPath))
		}
		if _, err := exec.LookPath("sudo"); err != nil {
			return fmt.Errorf("sudo is required to update the host trust store: %w", err)
		}
		args = append([]string{name}, args...)
		name = "sudo"
	}
	return runCommand(ctx, opts, name, args...)
}

func runCommand(ctx context.Context, opts TrustOptions, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	if opts.Verbose {
		cmd.Stdout = writerOrDiscard(opts.Out)
	} else {
		cmd.Stdout = io.Discard
	}
	cmd.Stderr = writerOrDiscard(opts.Err)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func writerOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

func ensureServerCertificate(paths Paths, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, domains []string) ([]byte, []byte, time.Time, error) {
	certPEM, certErr := os.ReadFile(paths.Cert)
	keyPEM, keyErr := os.ReadFile(paths.Key)
	if certErr == nil && keyErr == nil {
		cert, certParseErr := parseCertificatePEM(certPEM)
		key, keyParseErr := parsePrivateKeyPEM(keyPEM)
		if certParseErr == nil && keyParseErr == nil && serverCertificateUsable(cert, key, caCert, domains) {
			return certPEM, keyPEM, cert.NotAfter, nil
		}
	}
	if certErr != nil && !os.IsNotExist(certErr) {
		return nil, nil, time.Time{}, fmt.Errorf("read local TLS certificate: %w", certErr)
	}
	if keyErr != nil && !os.IsNotExist(keyErr) {
		return nil, nil, time.Time{}, fmt.Errorf("read local TLS key: %w", keyErr)
	}
	if err := os.MkdirAll(paths.ProjectDir, 0o755); err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("create certificate directory: %w", err)
	}
	certPEM, keyPEM, expiresAt, err := createServerCertificate(caCert, caKey, domains)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	if err := writePEM(paths.Cert, certPEM, 0o644); err != nil {
		return nil, nil, time.Time{}, err
	}
	if err := writePEM(paths.Key, keyPEM, 0o600); err != nil {
		return nil, nil, time.Time{}, err
	}
	return certPEM, keyPEM, expiresAt, nil
}

func createCA() (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("generate local CA key: %w", err)
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now().Add(-time.Minute)
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Pyahu Local Development CA"},
		NotBefore:             now,
		NotAfter:              now.Add(caValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create local CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parse generated local CA certificate: %w", err)
	}
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

func createServerCertificate(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, domains []string) ([]byte, []byte, time.Time, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("generate local TLS key: %w", err)
	}
	keyPEM, err := encodePrivateKey(key)
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	now := time.Now().Add(-time.Minute)
	expiresAt := now.Add(serverValidity)
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now,
		NotAfter:     expiresAt,
		DNSNames:     domains,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, time.Time{}, fmt.Errorf("create local TLS certificate: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), keyPEM, expiresAt, nil
}

func serverCertificateUsable(cert *x509.Certificate, key *ecdsa.PrivateKey, caCert *x509.Certificate, domains []string) bool {
	now := time.Now()
	if now.Before(cert.NotBefore) || now.Add(renewBefore).After(cert.NotAfter) {
		return false
	}
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		return false
	}
	if !reflect.DeepEqual(cert.PublicKey, &key.PublicKey) {
		return false
	}
	for _, domain := range domains {
		if !containsString(cert.DNSNames, domain) {
			return false
		}
	}
	return true
}

func parseCertificatePEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("missing PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PEM certificate: %w", err)
	}
	return cert, nil
}

func parsePrivateKeyPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("missing PEM private key")
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse EC private key: %w", err)
		}
		return key, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS#8 private key: %w", err)
		}
		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not ECDSA")
		}
		return ecdsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported private key PEM type %q", block.Type)
	}
}

func encodePrivateKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("encode EC private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

func serialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial number: %w", err)
	}
	return serial, nil
}

func writePEM(path string, data []byte, perm os.FileMode) error {
	if len(data) == 0 {
		return fmt.Errorf("refuse to write empty PEM file %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func normalizeDomains(domains []string) []string {
	seen := map[string]bool{}
	normalized := []string{}
	add := func(domain string) {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" || seen[domain] {
			return
		}
		seen[domain] = true
		normalized = append(normalized, domain)
	}
	for _, domain := range schema.DefaultLocalTLSDomains() {
		add(domain)
	}
	for _, domain := range domains {
		add(domain)
	}
	sort.Strings(normalized)
	if len(normalized) == 0 {
		return schema.DefaultLocalTLSDomains()
	}
	return normalized
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func PEMBlockCount(data []byte, blockType string) int {
	count := 0
	rest := data
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			return count
		}
		if block.Type == blockType {
			count++
		}
		rest = remaining
	}
}
