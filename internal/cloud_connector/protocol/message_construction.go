package protocol

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	//	"github.com/prometheus/client_golang/prometheus"
	//	"github.com/sirupsen/logrus"
)

func BuildReconnectMessage(delay int) (*uuid.UUID, *ControlMessage, error) {

	args := map[string]string{"delay": strconv.Itoa(delay)}

	content := CommandMessageContent{Command: "reconnect", Arguments: args}

	return BuildControlMessage("command", &content)
}

func BuildControlMessage(messageType string, content *CommandMessageContent) (*uuid.UUID, *ControlMessage, error) {

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

func BuildDataMessage(directive string, metadata interface{}, payload interface{}) (*uuid.UUID, *DataMessage, error) {

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

	//metrics.sentMessageDirectiveCounter.With(prometheus.Labels{"directive": directive}).Inc()

	return &messageID, &message, err
}
