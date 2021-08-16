package mqtt

import (
	"context"
	"errors"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	Topic      string
	EntryPoint MQTT.MessageHandler
	Qos        byte
}

func CreateBrokerConnection(brokerUrl string, onConnectHandler func(MQTT.Client), brokerConfigFuncs ...MqttClientOptionsFunc) (MQTT.Client, error) {

	connOpts, err := NewBrokerOptions(brokerUrl, brokerConfigFuncs...)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to build MQTT ClientOptions")
		return nil, err
	}

	connOpts.SetOnConnectHandler(onConnectHandler)

	mqttClient := MQTT.NewClient(connOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Error("Unable to connect to MQTT broker")
		return nil, token.Error()
	}

	logger.Log.Info("Connected to MQTT broker: ", brokerUrl)

	return mqttClient, nil
}

func RegisterSubscribers(brokerUrl string, subscribers []Subscriber, defaultMessageHandler func(MQTT.Client, MQTT.Message), brokerConfigFuncs ...MqttClientOptionsFunc) (MQTT.Client, error) {

	// Add a default publish message handler as some messages will get delivered before the topic
	// subscriptions are setup completely
	// See "Common Problems" here: https://github.com/eclipse/paho.mqtt.golang#common-problems
	brokerConfigFuncs = append(brokerConfigFuncs, WithDefaultPublishHandler(defaultMessageHandler))

	return CreateBrokerConnection(
		brokerUrl,
		func(client MQTT.Client) {
			for _, subscriber := range subscribers {
				logger.Log.Infof("Subscribing to MQTT topic: %s - QOS: %d\n", subscriber.Topic, subscriber.Qos)
				if token := client.Subscribe(subscriber.Topic, subscriber.Qos, subscriber.EntryPoint); token.Wait() && token.Error() != nil {
					logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Fatalf("Subscribing to MQTT topic (%s) failed", subscriber.Topic)
				}
			}
		},
		brokerConfigFuncs...)
}

func ControlMessageHandler(ctx context.Context, kafkaWriter *kafka.Writer, topicVerifier *TopicVerifier) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {

		metrics.controlMessageReceivedCounter.Inc()

		mqttMessageID := fmt.Sprintf("%d", message.MessageID())

		_, clientID, err := topicVerifier.VerifyIncomingTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "mqtt_message_id": mqttMessageID})

		if len(message.Payload()) == 0 {
			// This will happen when a retained message is removed
			// This can also happen when rhcd is "priming the pump" as required by the akamai broker
			logger.Trace("client sent an empty payload")
			return
		}

		go func() {
			metrics.kafkaWriterGoRoutineGauge.Inc()
			defer metrics.kafkaWriterGoRoutineGauge.Dec()

			err := kafkaWriter.WriteMessages(ctx,
				kafka.Message{
					Headers: []kafka.Header{
						{"topic", []byte(message.Topic())},
						{"mqtt_message_id", []byte(mqttMessageID)},
					}, // FIXME:  hard coded string??
					Value: message.Payload(),
				})

			logger.Debug("MQTT message written to kafka")

			if err != nil {
				logger.WithFields(logrus.Fields{"error": err}).Error("Error writing MQTT message to kafka")

				if errors.Is(err, context.Canceled) == true {
					// The context was canceled.  This likely happened due to the process shutting down,
					// so just return and allow things to shutdown cleanly
					return
				}

				// If writing to kafka fails, then just fall over and do not read anymore
				// messages from the mqtt broker
				logger.Fatal("Failed writing to kafka")
			}
		}()
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
