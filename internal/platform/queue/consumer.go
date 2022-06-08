package queue

import (
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/confluentinc/confluent-kafka-go/kafka"
)

func StartConsumer(cfg *kafka.ConfigMap, topic string) (*kafka.Consumer, error) {
	logger.Log.Info("Starting Kafka Message consumer...")
	logger.Log.Info("Kafka consumer configureation: ", cfg)

	consumer, err := kafka.NewConsumer(cfg)

	if err != nil {
		return nil, err
	}

	err = consumer.SubscribeTopics([]string{topic}, nil)

	if err != nil {
		return nil, err
	}

	logger.Log.Info("Connected to Kafka")

	return consumer, nil
}
