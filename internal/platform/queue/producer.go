package queue

import (
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
)

func StartProducer(cfg *ProducerConfig) *kafka.Writer {
	logger.Log.Info("Starting a new Kafka producer..")
	logger.Log.Info("Kafka producer configuration: ", cfg)

	writerConfig := kafka.WriterConfig{
		Brokers:    cfg.Brokers,
		Topic:      cfg.Topic,
		BatchSize:  cfg.BatchSize,
		BatchBytes: cfg.BatchBytes,
	}

	if cfg.Balancer == "hash" {
		writerConfig.Balancer = &kafka.Hash{}
	}

	w := kafka.NewWriter(writerConfig)

	logger.Log.Info("Producing messages to topic: ", cfg.Topic)

	return w
}
