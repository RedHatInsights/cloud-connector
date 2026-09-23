package mqtt

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

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func init() {
	logger.InitLogger()
}

func TestRecordMqttConsumerCertificateExpiry(t *testing.T) {
	certPath := writeMqttTestCertificate(t, time.Now().Add(45*24*time.Hour))

	RecordMqttConsumerCertificateExpiry(certPath)

	value := testutil.ToFloat64(metrics.certificateExpiryDays.WithLabelValues(mqttConsumerCertificateLabel))
	if value < 44 || value > 45 {
		t.Fatalf("expected ~45 days remaining, got %v", value)
	}
}

func TestRecordMqttConsumerCertificateExpiryEmptyPath(t *testing.T) {
	RecordMqttConsumerCertificateExpiry("")
}

func writeMqttTestCertificate(t *testing.T, notAfter time.Time) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("unable to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "cloud-connector-mqtt-test"},
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
