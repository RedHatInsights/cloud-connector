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
	} else if cfg.Balancer == "crc32" {
		// crc32 should match the hash that librdkafka is using
		writerConfig.Balancer = &kafka.CRC32Balancer{}
	}

	w := kafka.NewWriter(writerConfig)

	return w, nil
}
