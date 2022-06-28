package queue

import (
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
)

func StartConsumer(cfg *ConsumerConfig) *kafka.Reader {
	logger.Log.Info("Starting a new kafka consumer...")
	logger.Log.Info("Kafka consumer configuration: ", cfg)

	var kafkaDialer *kafka.Dialer
	var err error

	globalConfig := config.GetConfig()

	if globalConfig.KafkaUsername != "" {

		kafkaDialer, err = saslDialer(&SaslConfig{
			SaslMechanism: globalConfig.KafkaSASLMechanism,
			SaslUsername:  globalConfig.KafkaUsername,
			SaslPassword:  globalConfig.KafkaPassword,
			KafkaCA:       globalConfig.KafkaCA,
		})
		if err != nil {
			logger.Log.Error("Failed to create a new Kafka dialer: ", err)
			panic(err)
		}

	}

	readerConfig := kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		StartOffset: cfg.ConsumerOffset,
	}

	if kafkaDialer != nil {
		readerConfig.Dialer = kafkaDialer
	}

	r := kafka.NewReader(readerConfig)

	logger.Log.Info("Kafka consumer config: ", r.Config())

	return r
}
