package queue

import (
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func StartProducer(cfg *kafka.ConfigMap) *kafka.Producer {
	logger.Log.Info("Starting Kafka Message producer...")
	logger.Log.Info("Kafka producer configureation: ", cfg)

	producer, err := kafka.NewProducer(cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Kafka producer", err)
	}

	return producer
}
