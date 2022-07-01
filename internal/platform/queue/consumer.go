package queue

import (
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
)

func StartConsumer(cfg *ConsumerConfig) (*kafka.Reader, error) {
	logger.Log.Info("Starting a new kafka consumer...")
	logger.Log.Info("Kafka consumer configuration: ", cfg)

	kafkaDialer, err := createDialer(cfg.SaslConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka dialer: %w", err)
	}

	readerConfig := kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		StartOffset: cfg.ConsumerOffset,
		Dialer:      kafkaDialer,
	}

	r := kafka.NewReader(readerConfig)

	return r, nil
}
