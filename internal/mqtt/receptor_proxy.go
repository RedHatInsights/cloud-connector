package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

var (
	errUnableToSendMessage = errors.New("unable to send message")
)

type ReceptorMQTTProxy struct {
	ClientID string
	Client   MQTT.Client
}

func (rhp *ReceptorMQTTProxy) SendMessage(ctx context.Context, accountNumber string, recipient string, payload interface{}, directive string) (*uuid.UUID, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	fmt.Println("Sending message to connected client")

	topic := fmt.Sprintf("redhat/insights/%s/out", rhp.ClientID)
	fmt.Println("topic: ", topic)

	message := ConnectorMessage{
		MessageType: "message",
		MessageID:   messageID.String(),
		Version:     1,
		Payload:     payload,
	}

	messageBytes, err := json.Marshal(message)

	t := rhp.Client.Publish(topic, byte(0), false, messageBytes)
	go func() {
		_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
		if t.Error() != nil {
			fmt.Println("public error:", t.Error())
		}
	}()

	return &messageID, nil
}

func (rhp *ReceptorMQTTProxy) Close(ctx context.Context) error {
	return nil
}
