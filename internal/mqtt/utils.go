package mqtt

import (
	"bytes"
	"encoding/json"

	"github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	//	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

func SendReconnectMessageToClient(mqttClient MQTT.Client, logger *logrus.Entry, topicBuilder *TopicBuilder, qos byte, clientID domain.ClientID, delay int) error {

	messageID, message, err := protocol.BuildReconnectMessage(delay)

	if err != nil {
		return err
	}

	logger = logger.WithFields(logrus.Fields{"message_id": messageID, "client_id": clientID})

	logger.Debug("Sending reconnect message to connected client")

	topic := topicBuilder.BuildOutgoingControlTopic(clientID)

	err = sendMessage(mqttClient, logger, clientID, messageID, topic, qos, message)

	return err
}

func sendControlMessage(mqttClient MQTT.Client, logger *logrus.Entry, topic string, qos byte, clientID domain.ClientID, messageType string, content *protocol.CommandMessageContent) (*uuid.UUID, error) {

	messageID, message, err := protocol.BuildControlMessage(messageType, content)

	if err != nil {
		return nil, err
	}

	logger = logger.WithFields(logrus.Fields{"message_id": messageID, "client_id": clientID})

	logger.Debug("Sending control message to connected client")

	err = sendMessage(mqttClient, logger, clientID, messageID, topic, qos, message)

	return messageID, err
}

func sendMessage(mqttClient MQTT.Client, logger *logrus.Entry, clientID domain.ClientID, messageID *uuid.UUID, topic string, qos byte, message interface{}) error {

	logger = logger.WithFields(logrus.Fields{"message_id": messageID, "client_id": clientID})

	messageBuffer := &bytes.Buffer{}
	encoder := json.NewEncoder(messageBuffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(message); err != nil {
		return err
	}

	logger.Debug("Sending message to connected client on topic: ", topic, " qos: ", qos)

	token := mqttClient.Publish(topic, qos, false, messageBuffer.Bytes())
	if token.Wait() && token.Error() != nil {
		logger := logger.WithFields(logrus.Fields{"error": token.Error()})
		logger.Error("Error sending a message to MQTT broker")
		metrics.messagePublishedFailureCounter.Inc()
		return token.Error()
	}

	metrics.messagePublishedSuccessCounter.Inc()

	return nil
}
