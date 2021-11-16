package mqtt

import (
	"context"
	"errors"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

const (
	TopicKafkaHeaderKey     = "topic"
	MessageIDKafkaHeaderKey = "mqtt_message_id"
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

		metrics.kafkaWriterGoRoutineGauge.Inc()
		defer metrics.kafkaWriterGoRoutineGauge.Dec()

		metrics.controlMessageReceivedCounter.Inc()

		mqttMessageID := fmt.Sprintf("%d", message.MessageID())

		_, clientID, err := topicVerifier.VerifyIncomingTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID,
			"mqtt_message_id": mqttMessageID,
			"duplicate":       message.Duplicate()})

		if len(message.Payload()) == 0 {
			// This will happen when a retained message is removed
			// This can also happen when rhcd is "priming the pump" as required by the akamai broker
			logger.Trace("client sent an empty payload")
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
				},
				Key:   []byte(clientID),
				Value: message.Payload(),
			})

		kafkwWriteDurationTimer.ObserveDuration()

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

func ThrottlingMessageHandler(maxInFlight int, f MQTT.MessageHandler) MQTT.MessageHandler {
	// WARNING:  Messages buffered here can be lost if the process is restarted.  We probably need
	//  to come up with a better message buffering mechanism that allows control messages (at least)
	//  to not be lost when a restart occurs.

	fmt.Println("**** maxInFlight: ", maxInFlight)
	type messageWrapper struct {
		client  MQTT.Client
		message MQTT.Message
	}

	messagePipe := make(chan messageWrapper, maxInFlight)

	go func() {
		for msgWrapper := range messagePipe {
			metrics.mqttMessagesWaitingToBeProcessed.Dec()
			fmt.Println("**** GOT A MESSAGE ... calling f(c,m)")
			f(msgWrapper.client, msgWrapper.message)
			fmt.Println("**** FIXME!!!  i dont know how to shutdown!!")
		}
	}()

	return func(c MQTT.Client, m MQTT.Message) {
		metrics.mqttMessagesWaitingToBeProcessed.Inc()
		fmt.Println("**** BLOCKED PUTTING MESSAGE ON QUEUE...")
		messagePipe <- messageWrapper{c, m}
		fmt.Println("**** UNBLOCKED")
	}
}
