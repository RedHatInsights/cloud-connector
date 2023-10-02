package tls_utils

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"path/filepath"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

type TlsConfigFunc func(*tls.Config) error

func WithCert(certFilePath string, certKeyPath string) TlsConfigFunc {
	return func(tlsConfig *tls.Config) error {
		logger.Log.Trace("TLS config - setting the pub/priv key pair")

		cert, err := tls.LoadX509KeyPair(certFilePath, certKeyPath)
		if err != nil {
			return err
		}

		tlsConfig.Certificates = []tls.Certificate{cert}

		return nil
	}
}

func WithCACerts(caCertFilePath string) TlsConfigFunc {
	return func(tlsConfig *tls.Config) error {
		logger.Log.Trace("TLS config - setting ca certs")

		caCertFilePath = filepath.Clean(caCertFilePath)

		pemCerts, err := ioutil.ReadFile(caCertFilePath)
		if err != nil {
			return err
		}

		certpool := x509.NewCertPool()

		certpool.AppendCertsFromPEM(pemCerts)

		tlsConfig.RootCAs = certpool

		return nil
	}
}

func WithSkipVerify() TlsConfigFunc {
	return func(tlsConfig *tls.Config) error {
		logger.Log.Trace("TLS config - setting insecure skip verify")

		tlsConfig.InsecureSkipVerify = true

		return nil
	}
}

func NewTlsConfig(configOpts ...TlsConfigFunc) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	for _, opt := range configOpts {
		err := opt(tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	return tlsConfig, nil
}
