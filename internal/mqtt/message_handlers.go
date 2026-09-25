package mqtt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

// Note: sync.Once is imported for shutdownOnce in KafkaWriterState

const (
	TopicKafkaHeaderKey     = "topic"
	MessageIDKafkaHeaderKey = "mqtt_message_id"
	DateReceivedHeaderKey   = "date_received"
)

type KafkaWriterState struct {
	consecutiveWriteErrors  int
	maxConsecutiveErrors    int
	writeErrorBackoff       time.Duration
	fatalWriteError         chan struct{}
	shutdownOnce            sync.Once
}

func NewKafkaWriterState(maxErrors int, backoff time.Duration, fatalChan chan struct{}) *KafkaWriterState {
	return &KafkaWriterState{
		maxConsecutiveErrors: maxErrors,
		writeErrorBackoff:    backoff,
		fatalWriteError:      fatalChan,
	}
}

func ControlMessageHandler(ctx context.Context, kafkaWriter *kafka.Writer, topicVerifier *TopicVerifier, state *KafkaWriterState) func(MQTT.Client, MQTT.Message) {
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
		kafkaMsg := kafka.Message{
			Headers: []kafka.Header{
				{Key: TopicKafkaHeaderKey, Value: []byte(message.Topic())},
				{Key: MessageIDKafkaHeaderKey, Value: []byte(mqttMessageID)},
				{Key: DateReceivedHeaderKey, Value: []byte(time.Now().UTC().Format(time.RFC3339Nano))},
			},
			Key:   []byte(clientID),
			Value: message.Payload(),
		}

		// Retry loop with backoff - blocks until success or shutdown decision.
		// NOTE: With OrderMatters=true (default), handlers block the message dispatch loop. This creates
		// backpressure through unbuffered channels that prevents paho from reading PINGRESP from the socket,
		// causing "pingresp not received" disconnects. This blocking behavior existed before this change
		// (WriteMessages blocks on failure) and may be more pronounced with retry loop (~120s).
		for {
			err = kafkaWriter.WriteMessages(ctx, kafkaMsg)
			if err == nil {
				// Success - reset error counter
				state.consecutiveWriteErrors = 0

				kafkwWriteDurationTimer.ObserveDuration()
				log.Debug("MQTT message written to kafka")
				return
			}

			if errors.Is(err, context.Canceled) {
				// Context canceled - clean shutdown
				kafkwWriteDurationTimer.ObserveDuration()
				return
			}

			// Write failed - track consecutive errors
			state.consecutiveWriteErrors++
			currentErrors := state.consecutiveWriteErrors

			log.WithFields(logrus.Fields{
				"consecutive_errors": currentErrors,
				"max_errors":         state.maxConsecutiveErrors,
				"error":              err,
			}).Error("Failed to write MQTT message to kafka")

			if currentErrors >= state.maxConsecutiveErrors {
				log.Errorf("Reached %d consecutive kafka write errors, shutting down", state.maxConsecutiveErrors)
				logger.FlushLogger()

				// Signal shutdown (idempotent via sync.Once - reliable signaling)
				state.shutdownOnce.Do(func() {
					close(state.fatalWriteError)
				})

				// CRITICAL: We must NEVER return from this handler - block forever until process exits.
				// Returning from the handler causes the MQTT library to send PUBACK (QoS 1 ACK) to the broker,
				// which would acknowledge a message we failed to persist to Kafka. The old code achieved this
				// via log.Fatal() (immediate crash). The new code blocks forever - mqttClient.Disconnect() will
				// timeout after quiesceTime, forcibly close the connection without calling m.Ack(), and the
				// process will exit while this handler remains blocked. No PUBACK is sent.
				kafkwWriteDurationTimer.ObserveDuration()
				select {} // Block forever
			}

			// Backoff before retry
			backoffTimer := time.NewTimer(state.writeErrorBackoff)
			select {
			case <-backoffTimer.C:
				// Retry WriteMessages
			case <-ctx.Done():
				backoffTimer.Stop()
				kafkwWriteDurationTimer.ObserveDuration()
				return
			}
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
