package queue

import (
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
)

func StartProducer(cfg *ProducerConfig) (*kafka.Writer, error) {
	logger.Log.Info("Starting a new Kafka producer..")
	logger.Log.Info("Kafka producer configuration: ", cfg)

	kafkaDialer, err := createDialer(cfg.SaslConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka dialer: %w", err)
	}

	writerConfig := kafka.WriterConfig{
		Brokers:    cfg.Brokers,
		Topic:      cfg.Topic,
		BatchSize:  cfg.BatchSize,
		BatchBytes: cfg.BatchBytes,
		Dialer:     kafkaDialer,
	}

	if cfg.Balancer == "hash" {
		writerConfig.Balancer = &kafka.Hash{}
	}

	w := kafka.NewWriter(writerConfig)

	logger.Log.Info("Producing messages to topic: ", cfg.Topic)

	return w, nil
}
