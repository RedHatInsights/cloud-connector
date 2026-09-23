package tls_utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCertificateExpiryDays(t *testing.T) {
	certPath := writeTestCertificate(t, time.Now().Add(10*24*time.Hour))

	days, err := CertificateExpiryDays(certPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if days < 9 || days > 10 {
		t.Fatalf("expected ~10 days remaining, got %v", days)
	}
}

func TestCertificateExpiryDaysExpired(t *testing.T) {
	certPath := writeTestCertificate(t, time.Now().Add(-48*time.Hour))

	days, err := CertificateExpiryDays(certPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if days >= 0 {
		t.Fatalf("expected negative days for expired cert, got %v", days)
	}
}

func TestCertificateNotAfterMissingFile(t *testing.T) {
	_, err := CertificateNotAfter(filepath.Join(t.TempDir(), "missing.pem"))
	if err == nil {
		t.Fatal("expected error for missing certificate file")
	}
}

func TestCertificateNotAfterNoCertificateBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(path, []byte("not a pem certificate\n"), 0600); err != nil {
		t.Fatalf("unable to write test file: %v", err)
	}

	_, err := CertificateNotAfter(path)
	if err == nil {
		t.Fatal("expected error when PEM has no certificate block")
	}
}

func writeTestCertificate(t *testing.T, notAfter time.Time) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("unable to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "cloud-connector-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("unable to create certificate: %v", err)
	}

	path := filepath.Join(t.TempDir(), "cert.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("unable to create cert file: %v", err)
	}
	defer f.Close()

	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("unable to write cert pem: %v", err)
	}

	return path
}
