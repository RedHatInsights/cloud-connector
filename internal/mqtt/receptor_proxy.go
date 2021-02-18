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
	fmt.Print("messageID:", messageID)
	fmt.Print("err:", err)

	return messageID, nil
}

func (rhp *ReceptorMQTTProxy) Ping(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID) error {

	commandMessageContent := CommandMessageContent{Command: "ping"}

	messageID, err := rhp.sendControlMessage(ctx, "command", commandMessageContent)
	fmt.Print("messageID:", messageID)
	fmt.Print("err:", err)

	return nil
}

func (rhp *ReceptorMQTTProxy) Close(ctx context.Context) error {

	commandMessageContent := CommandMessageContent{Command: "disconnect"}

	messageID, err := rhp.sendControlMessage(ctx, "command", commandMessageContent)
	fmt.Print("messageID:", messageID)
	fmt.Print("err:", err)

	return nil
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

	err = rhp.sendMessage(topic, message)
	fmt.Print("err:", err)

	return &messageID, err
}

func (rhp *ReceptorMQTTProxy) sendDataMessage(ctx context.Context, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	logger := logger.Log.WithFields(logrus.Fields{"message_id": messageID, "account": rhp.AccountID, "client_id": rhp.ClientID})

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

	err = rhp.sendMessage(topic, message)
	fmt.Print("err:", err)

	return &messageID, err
}

func (rhp *ReceptorMQTTProxy) sendMessage(topic string, message interface{}) error {

	messageBytes, err := json.Marshal(message)
	if err != nil {
		return err
	}

	fmt.Println("topic: ", topic)

	t := rhp.Client.Publish(topic, byte(0), false, messageBytes)
	go func() {
		_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
		if t.Error() != nil {
			fmt.Println("public error:", t.Error())
		}
	}()

	return nil
}
