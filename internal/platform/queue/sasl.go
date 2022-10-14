package queue

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	kafka "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

func createDialer(cfg *SaslConfig) (*kafka.Dialer, error) {

	if cfg == nil {
		logger.Log.Info("Using the default Kafka dialer")
		return kafka.DefaultDialer, nil
	}

	tlsConfig, err := createTLSConfig(cfg.KafkaCA)
	if err != nil {
		return nil, err
	}

	saslMechanism, err := createSaslMechanism(cfg.SaslMechanism, cfg.SaslUsername, cfg.SaslPassword)
	if err != nil {
		return nil, err
	}

	logger.Log.Info("Creating custom Kafka dialer")

	return &kafka.Dialer{
		Timeout:       10 * time.Second,
		DualStack:     true,
		SASLMechanism: saslMechanism,
		TLS:           tlsConfig,
	}, nil
}

func createTLSConfig(pathToCert string) (*tls.Config, error) {

	tlsConfigFuncs := []tls_utils.TlsConfigFunc{}

	if pathToCert != "" {
		tlsConfigFuncs = append(tlsConfigFuncs, tls_utils.WithCACerts(pathToCert))
	}

	tlsConfig, err := tls_utils.NewTlsConfig(tlsConfigFuncs...)
	if err != nil {
		logger.LogFatalError("Unable to configure TLS for Kafka Broker connection", err)
		return nil, err
	}

	return tlsConfig, nil
}

func createSaslMechanism(saslMechanismName string, username string, password string) (sasl.Mechanism, error) {

	switch strings.ToLower(saslMechanismName) {
	case "plain":
		return plain.Mechanism{
			Username: username,
			Password: password,
		}, nil
	case "scram-sha-512":
		mechanism, err := scram.Mechanism(scram.SHA512, username, password)
		if err != nil {
			return nil, fmt.Errorf("unable to create scram-sha-512 mechanism: %w", err)
		}

		return mechanism, nil
	case "scram-sha-256":
		mechanism, err := scram.Mechanism(scram.SHA256, username, password)
		if err != nil {
			return nil, fmt.Errorf("unable to create scram-sha-256 mechanism: %w", err)
		}

		return mechanism, nil
	default:

		return nil, fmt.Errorf("unable to configure sasl mechanism (%s)", saslMechanismName)
	}
}
