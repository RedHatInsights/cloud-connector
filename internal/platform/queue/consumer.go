package queue

import (
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
)

func StartConsumer(cfg *ConsumerConfig) *kafka.Reader {
	logger.Log.Info("Starting a new kafka consumer...")
	logger.Log.Info("Kafka consumer configuration: ", cfg)

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		StartOffset: cfg.ConsumerOffset,
	})

	logger.Log.Info("Kafka consumer config: ", r.Config())

	return r
}
