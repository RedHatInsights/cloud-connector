package mqtt

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

func buildReconnectMessage(delay int) (*uuid.UUID, *ControlMessage, error) {

	args := map[string]string{"delay": strconv.Itoa(delay)}

	content := CommandMessageContent{Command: "reconnect", Arguments: args}

	return buildControlMessage("command", &content)
}

func buildControlMessage(messageType string, content *CommandMessageContent) (*uuid.UUID, *ControlMessage, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, nil, err
	}

	message := ControlMessage{
		MessageType: messageType,
		MessageID:   messageID.String(),
		Version:     1,
		Sent:        time.Now(),
		Content:     content,
	}

	return &messageID, &message, err
}

func buildDataMessage(directive string, metadata interface{}, payload interface{}) (*uuid.UUID, *DataMessage, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, nil, err
	}

	message := DataMessage{
		MessageType: "data",
		MessageID:   messageID.String(),
		Version:     1,
		Sent:        time.Now(),
		Metadata:    metadata,
		Directive:   directive,
		Content:     payload,
	}

	metrics.sentMessageDirectiveCounter.With(prometheus.Labels{"directive": directive}).Inc()

	return &messageID, &message, err
}

func sendReconnectMessageToClient(mqttClient MQTT.Client, logger *logrus.Entry, topicBuilder *TopicBuilder, qos byte, clientID domain.ClientID, delay int) error {

	messageID, message, err := buildReconnectMessage(delay)

	if err != nil {
		return err
	}

	logger = logger.WithFields(logrus.Fields{"message_id": messageID, "client_id": clientID})

	logger.Debug("Sending reconnect message to connected client")

	topic := topicBuilder.BuildOutgoingControlTopic(clientID)

	err = sendMessage(mqttClient, logger, clientID, messageID, topic, qos, message)

	return err
}

func sendControlMessage(mqttClient MQTT.Client, logger *logrus.Entry, topic string, qos byte, clientID domain.ClientID, messageType string, content *CommandMessageContent) (*uuid.UUID, error) {

	messageID, message, err := buildControlMessage(messageType, content)

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

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	logger.Debug("Sending message to connected client on topic: ", topic, " qos: ", qos)

	t := mqttClient.Publish(topic, qos, false, messageBytes)
	go func() {
		_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
		if t.Error() != nil {
			logger := logger.WithFields(logrus.Fields{"error": t.Error()})
			logger.Error("Error sending a message to MQTT broker")
			metrics.messagePublishedFailureCounter.Inc()

			// FIXME:  This will bring down the service!  This was added to work around an
			// issue we are seeing with the production mqtt broker.  We are running into an issue in prod where
			// cloud-connector cannot send or receive messages.  On the sending side, we are getting an
			// timeout error.  BUT...things never recover.  So fall over and allow openshift to restart
			// the service.  This Fatal call needs to be removed after the mqtt broker starts behaving better.
			go func() {
				logger.Warn("cloud-connector is about to fall over...FIXME later!!")
				time.Sleep(1 * time.Second) // Give us some time send the log message...to give the humans a clue to figure out what happened here...
				logger.Fatal("ran into an mqtt error...going down")
			}()

			return
		}

		metrics.messagePublishedSuccessCounter.Inc()
	}()

	return nil
}
