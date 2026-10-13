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
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logger.InitLogger()
}

func TestRecordMqttConsumerCertificateExpiry(t *testing.T) {
	certPath := writeMqttTestCertificate(t, time.Now().Add(45*24*time.Hour))

	RecordMqttConsumerCertificateExpiry(certPath)

	value := testutil.ToFloat64(metrics.certificateExpiryDays)
	if value < 44 || value > 45 {
		t.Fatalf("expected ~45 days remaining, got %v", value)
	}
}

func TestRecordMqttConsumerCertificateExpiryEmptyPath(t *testing.T) {
	hook := test.NewLocal(logger.Log)
	defer hook.Reset()

	metrics.certificateExpiryDays.Set(99)

	RecordMqttConsumerCertificateExpiry("")

	if got := testutil.ToFloat64(metrics.certificateExpiryDays); got != 99 {
		t.Fatalf("empty path should not change gauge, got %v", got)
	}

	if len(hook.Entries) == 0 {
		t.Fatal("expected error log for empty cert path")
	}

	last := hook.LastEntry()
	if last.Level != logrus.ErrorLevel {
		t.Fatalf("expected error level log, got %s", last.Level)
	}
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
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("unable to write cert file: %v", err)
	}

	return path
}
