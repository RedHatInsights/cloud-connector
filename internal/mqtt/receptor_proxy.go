package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	errUnableToSendMessage = errors.New("unable to send message")
)

type ReceptorMQTTProxy struct {
	AccountID domain.AccountID
	ClientID  domain.ClientID
	Client    MQTT.Client
}

func (rhp *ReceptorMQTTProxy) SendMessage(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	messageID, err := rhp.sendDataMessage(ctx, directive, metadata, payload)

	return messageID, err
}

func (rhp *ReceptorMQTTProxy) Ping(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID) error {

	commandMessageContent := CommandMessageContent{Command: "ping"}

	_, err := rhp.sendControlMessage(ctx, "command", commandMessageContent)

	return err
}

func (rhp *ReceptorMQTTProxy) Reconnect(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID, delay int) error {

	args := map[string]int{"delay": delay}

	commandMessageContent := CommandMessageContent{Command: "reconnect", Arguments: args}

	_, err := rhp.sendControlMessage(ctx, "command", commandMessageContent)

	return err
}

func (rhp *ReceptorMQTTProxy) Close(ctx context.Context) error {

	commandMessageContent := CommandMessageContent{Command: "disconnect"}

	_, err := rhp.sendControlMessage(ctx, "command", commandMessageContent)

	return err
}

func (rhp *ReceptorMQTTProxy) sendControlMessage(ctx context.Context, msgType string, content CommandMessageContent) (*uuid.UUID, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	logger := logger.Log.WithFields(logrus.Fields{"message_id": messageID, "account": rhp.AccountID, "client_id": rhp.ClientID})

	logger.Debug("Sending control message to connected client")

	message := ControlMessage{
		MessageType: msgType,
		MessageID:   messageID.String(),
		Version:     1,
		Sent:        time.Now(),
		Content:     content,
	}

	topic := fmt.Sprintf(CONTROL_MESSAGE_OUTGOING_TOPIC, rhp.ClientID)

	err = rhp.sendMessage(logger, topic, message)

	return &messageID, err
}

func (rhp *ReceptorMQTTProxy) sendDataMessage(ctx context.Context, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	logger := logger.Log.WithFields(logrus.Fields{"message_id": messageID, "account": rhp.AccountID, "client_id": rhp.ClientID})

	go func() {
		var sleepTime time.Duration = 5
		logger.Debugf("Sleeping for %d second before sending data message to connected client\n", sleepTime)
		time.Sleep(sleepTime * time.Second)
		logger.Debug("Sending data message to connected client")

		topic := fmt.Sprintf(DATA_MESSAGE_OUTGOING_TOPIC, rhp.ClientID)

		message := DataMessage{
			MessageType: "data",
			MessageID:   messageID.String(),
			Version:     1,
			Sent:        time.Now(),
			Metadata:    metadata,
			Directive:   directive,
			Content:     payload,
		}

		err = rhp.sendMessage(logger, topic, message)
	}()

	return &messageID, err
}

func (rhp *ReceptorMQTTProxy) sendMessage(logger *logrus.Entry, topic string, message interface{}) error {

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	logger.Trace("Sending message to connected client on topic: ", topic)

	t := rhp.Client.Publish(topic, byte(0), false, messageBytes)
	go func() {
		_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
		if t.Error() != nil {
			logger := logger.WithFields(logrus.Fields{"error": t.Error()})
			logger.Error("Error sending a message to MQTT broker")
		}
	}()

	return nil
}
