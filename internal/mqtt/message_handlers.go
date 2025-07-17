package mqtt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const (
	TopicKafkaHeaderKey     = "topic"
	MessageIDKafkaHeaderKey = "mqtt_message_id"
	DateReceivedHeaderKey   = "date_received"
)

func ControlMessageHandler(ctx context.Context, kafkaWriter *kafka.Writer, topicVerifier *TopicVerifier) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {

		metrics.kafkaWriterGoRoutineGauge.Inc()
		defer metrics.kafkaWriterGoRoutineGauge.Dec()

		metrics.controlMessageReceivedCounter.Inc()

		mqttMessageID := fmt.Sprintf("%d", message.MessageID())

		_, clientID, err := topicVerifier.VerifyIncomingTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		log := logger.Log.WithFields(logrus.Fields{"client_id": clientID,
			"mqtt_message_id": mqttMessageID,
			"duplicate":       message.Duplicate(),
			"topic":           message.Topic()})

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
		err = kafkaWriter.WriteMessages(ctx,
			kafka.Message{
				Headers: []kafka.Header{
					{TopicKafkaHeaderKey, []byte(message.Topic())},
					{MessageIDKafkaHeaderKey, []byte(mqttMessageID)},
					{DateReceivedHeaderKey, []byte(time.Now().UTC().Format(time.RFC3339Nano))},
				},
				Key:   []byte(clientID),
				Value: message.Payload(),
			})

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
		logger.Log.Tracef("Received data message on topic: %s\n", message.Topic())

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
