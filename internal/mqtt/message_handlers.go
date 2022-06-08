package mqtt

import (
	"context"
	"errors"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	TopicKafkaHeaderKey     = "topic"
	MessageIDKafkaHeaderKey = "mqtt_message_id"
)

func ControlMessageHandler(ctx context.Context, kafkaWriter *kafka.Producer, topicVerifier *TopicVerifier) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {

		metrics.kafkaWriterGoRoutineGauge.Inc()
		defer metrics.kafkaWriterGoRoutineGauge.Dec()

		metrics.controlMessageReceivedCounter.Inc()

		mqttMessageID := fmt.Sprintf("%d", message.MessageID())
		cfg := config.GetConfig()

		_, clientID, err := topicVerifier.VerifyIncomingTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		log := logger.Log.WithFields(logrus.Fields{"client_id": clientID,
			"mqtt_message_id": mqttMessageID,
			"duplicate":       message.Duplicate()})

		if len(message.Payload()) == 0 {
			// This will happen when a retained message is removed
			// This can also happen when rhcd is "priming the pump" as required by the akamai broker
			log.Trace("client sent an empty payload")
			return
		}

		kafkwWriteDurationTimer := prometheus.NewTimer(metrics.kafkaWriterPublishDuration)

		// Use the client id as the message key.  All messages with the same key,
		// get sent to the same partitions.  This is important so that the ordering
		// of the messages is retained.
		err = kafkaWriter.Produce(
			&kafka.Message{
				Headers: []kafka.Header{
					{TopicKafkaHeaderKey, []byte(message.Topic())},
					{MessageIDKafkaHeaderKey, []byte(mqttMessageID)},
				},
				TopicPartition: kafka.TopicPartition{Topic: &cfg.RhcMessageKafkaTopic, Partition: kafka.PartitionAny},
				Key:            []byte(clientID),
				Value:          message.Payload(),
			}, nil)

		kafkwWriteDurationTimer.ObserveDuration()

		log.Debug("MQTT message written to kafka")

		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Error writing MQTT message to kafka")

			if errors.Is(err, context.Canceled) == true {
				// The context was canceled.  This likely happened due to the process shutting down,
				// so just return and allow things to shutdown cleanly
				return
			}

			// This is gross, but we need to try to push the log messages to cloudwatch
			// before the panic is triggered below.
			logger.FlushLogger()

			// If writing to kafka fails, then just fall over and do not read anymore
			// messages from the mqtt broker.  We need to panic here so that the mqtt broker
			// is not sent an ACK for the message.
			log.Fatal("Failed writing to kafka")
		}
	}
}

func DataMessageHandler() func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received data message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

		metrics.dataMessageReceivedCounter.Inc()

		if message.Payload() == nil || len(message.Payload()) == 0 {
			logger.Log.Trace("Received empty data message")
			return
		}
	}
}

func DefaultMessageHandler(topicVerifier *TopicVerifier, controlMessageHandler, dataMessageHandler func(MQTT.Client, MQTT.Message)) func(client MQTT.Client, message MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

		topicType, _, err := topicVerifier.VerifyIncomingTopic(message.Topic())

		if err != nil {
			logger.Log.Debugf("Topic verification failed : %s\nMessage: %s\n", message.Topic(), message.Payload())
			return
		}

		if topicType == ControlTopicType {
			controlMessageHandler(client, message)
		} else if topicType == DataTopicType {
			dataMessageHandler(client, message)
		} else {
			logger.Log.Debugf("Received message on unknown topic: %s\nMessage: %s\n", message.Topic(), message.Payload())
		}
	}
}
