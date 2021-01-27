package tls_utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

type TlsConfigFunc func(*tls.Config) error

func WithCert(certFilePath string, certKeyPath string) TlsConfigFunc {
	return func(tlsConfig *tls.Config) error {
		fmt.Println("TLS CONFIG - SETTING THE PUB/PRIV KEY PAIR")

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
		fmt.Println("TLS CONFIG - SETTING CA CERTS")

		certpool := x509.NewCertPool()
		pemCerts, err := ioutil.ReadFile(caCertFilePath)
		if err != nil {
			return err
		}

		certpool.AppendCertsFromPEM(pemCerts)

		tlsConfig.RootCAs = certpool

		return nil
	}
}

func WithSkipVerify() TlsConfigFunc {
	return func(tlsConfig *tls.Config) error {
		fmt.Println("TLS CONFIG - SETTING INSECURE SKIP VERIFY")

		tlsConfig.InsecureSkipVerify = true

		return nil
	}
}

func NewTlsConfig(configOpts ...TlsConfigFunc) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	for _, opt := range configOpts {
		err := opt(tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	return tlsConfig, nil
}
