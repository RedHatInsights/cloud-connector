package tls_utils

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// CertificateExpiryDays returns the number of whole days remaining until the
// first certificate in the PEM file expires. Negative values mean the
// certificate has already expired.
func CertificateExpiryDays(certFilePath string) (float64, error) {
	notAfter, err := CertificateNotAfter(certFilePath)
	if err != nil {
		return 0, err
	}

	return math.Floor(time.Until(notAfter).Hours() / 24), nil
}

// CertificateNotAfter returns the NotAfter timestamp of the first certificate
// found in the PEM file at certFilePath.
func CertificateNotAfter(certFilePath string) (time.Time, error) {
	certFilePath = filepath.Clean(certFilePath)

	pemCerts, err := os.ReadFile(certFilePath)
	if err != nil {
		return time.Time{}, err
	}

	for {
		var block *pem.Block
		block, pemCerts = pem.Decode(pemCerts)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return time.Time{}, err
		}

		return cert.NotAfter, nil
	}

	return time.Time{}, fmt.Errorf("no certificate found in %s", certFilePath)
}
