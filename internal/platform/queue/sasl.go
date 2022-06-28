package queue

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"io/ioutil"
	"strings"
	"time"
)

func saslDialer(cfg *SaslConfig) (*kafka.Dialer, error) {

	var dialer *kafka.Dialer

	caCert, err := ioutil.ReadFile(cfg.KafkaCA)
	if err != nil {
		logger.Log.Fatal("Unable to read kafka cert", err)
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	switch strings.ToLower(cfg.SaslMechanism) {
	case "plain":
		mechanism := plain.Mechanism{
			Username: cfg.SaslUsername,
			Password: cfg.SaslPassword,
		}

		dialer = &kafka.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: mechanism,
			TLS: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	case "scram-sha-512":
		scramMechanism, err := scram.Mechanism(scram.SHA512, cfg.SaslUsername, cfg.SaslPassword)
		if err != nil {
			logger.Log.Error("Failed to create SCRAM-SHA-512 SASL mechanism: ", err)
			return nil, err
		}

		dialer = &kafka.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: scramMechanism,
			TLS: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	case "scra-sha-256":
		scramMechanism, err := scram.Mechanism(scram.SHA256, cfg.SaslUsername, cfg.SaslPassword)
		if err != nil {
			logger.Log.Error("Failed to create SCRAM-SHA-256 SASL mechanism: ", err)
			return nil, err
		}

		dialer = &kafka.Dialer{
			Timeout:       10 * time.Second,
			DualStack:     true,
			SASLMechanism: scramMechanism,
			TLS: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	}

	return dialer, nil
}
