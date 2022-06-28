package queue

import (
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	kafka "github.com/segmentio/kafka-go"
)

func StartProducer(cfg *ProducerConfig) *kafka.Writer {
	logger.Log.Info("Starting a new Kafka producer..")
	logger.Log.Info("Kafka producer configuration: ", cfg)

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

	writerConfig := kafka.WriterConfig{
		Brokers:    cfg.Brokers,
		Topic:      cfg.Topic,
		BatchSize:  cfg.BatchSize,
		BatchBytes: cfg.BatchBytes,
	}

	if kafkaDialer != nil {
		writerConfig.Dialer = kafkaDialer
	}

	if cfg.Balancer == "hash" {
		writerConfig.Balancer = &kafka.Hash{}
	}

	w := kafka.NewWriter(writerConfig)

	logger.Log.Info("Producing messages to topic: ", cfg.Topic)

	return w
}
