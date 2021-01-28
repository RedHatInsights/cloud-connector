package mqtt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

func (rhp *ReceptorMQTTProxy) SendMessage(ctx context.Context, accountNumber string, recipient string, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	messageID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	fmt.Println("Sending message to connected client")

	topic := fmt.Sprintf(DATA_MESSAGE_OUTGOING_TOPIC, rhp.ClientID)
	fmt.Println("topic: ", topic)

	message := DataMessage{
		MessageType: "data",
		MessageID:   messageID.String(),
		Version:     1,
		Sent:        time.Now(),
		Metadata:    metadata,
		Directive:   directive,
		Content:     payload,
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

func (rhp *ReceptorMQTTProxy) Ping(ctx context.Context, accountNumber string, recipient string) error {
	messageID, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	fmt.Println("Sending ping message to connected client")

	topic := fmt.Sprintf(CONTROL_MESSAGE_OUTGOING_TOPIC, rhp.ClientID)
	fmt.Println("topic: ", topic)

	commandMessageContent := CommandMessageContent{Command: "ping"}

	message := ControlMessage{
		MessageType: "command",
		MessageID:   messageID.String(),
		Version:     1,
		Sent:        time.Now(),
		Content:     commandMessageContent,
	}

	messageBytes, err := json.Marshal(message)

	t := rhp.Client.Publish(topic, byte(0), false, messageBytes)
	go func() {
		_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
		if t.Error() != nil {
			fmt.Println("public error:", t.Error())
		}
	}()

	return nil
}

func (rhp *ReceptorMQTTProxy) Close(ctx context.Context) error {
	return nil
}
