package cloud_connector

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func init() {
	logger.InitLogger()
}

type mockConnectionRegistrar struct {
	clients map[domain.ClientID]domain.ConnectorClientState
}

func (mcr *mockConnectionRegistrar) Register(ctx context.Context, rhcClient domain.ConnectorClientState) error {
	mcr.clients[rhcClient.ClientID] = rhcClient
	return nil
}

func (mcr *mockConnectionRegistrar) Unregister(ctx context.Context, clientID domain.ClientID) error {
	return nil
}

func (mcr *mockConnectionRegistrar) FindConnectionByClientID(ctx context.Context, clientID domain.ClientID) (domain.ConnectorClientState, error) {
	return mcr.clients[clientID], nil
}

type mockAccountIdResolver struct {
	accounts map[domain.ClientID]domain.AccountID
}

func (mar *mockAccountIdResolver) MapClientIdToAccountId(ctx context.Context, clientID domain.ClientID) (domain.Identity, domain.AccountID, error) {
	return domain.Identity("1111"), domain.AccountID("000111"), nil
}

func TestHandleOnlineMessagesNoExistingConnection(t *testing.T) {

	var mqttClient MQTT.Client
	var clientID domain.ClientID = "1234"
	var cfg config.Config
	var topicBuilder mqtt.TopicBuilder
	var accountResolver = &mockAccountIdResolver{}
	var connectionRegistrar = &mockConnectionRegistrar{
		clients: make(map[domain.ClientID]domain.ConnectorClientState),
	}
	var connectedClientRecorder controller.ConnectedClientRecorder
	var sourcesRecorder controller.SourcesRecorder

	incomingMessage := buildOnlineMessage(t, "56789", time.Now())

	err := handleOnlineMessage(mqttClient, clientID, incomingMessage, &cfg, &topicBuilder, accountResolver, connectionRegistrar, connectedClientRecorder, sourcesRecorder)

	if err != nil {
		t.Fatal("handleOnlineMessage should not have returned an error")
	}

	recordedConnectionState, err := connectionRegistrar.FindConnectionByClientID(context.Background(), clientID)

	if recordedConnectionState.MessageMetadata.LatestMessageID != incomingMessage.MessageID {
		t.Error("incoming messages does not appear to have been stored")
	}
}

func TestHandleOnlineMessagesUpdateExistingConnection(t *testing.T) {

	var mqttClient MQTT.Client
	var clientID domain.ClientID = "1234"
	var cfg config.Config
	var topicBuilder mqtt.TopicBuilder
	var accountResolver = &mockAccountIdResolver{}
	var connectionRegistrar = &mockConnectionRegistrar{
		clients: make(map[domain.ClientID]domain.ConnectorClientState),
	}
	var connectedClientRecorder controller.ConnectedClientRecorder
	var sourcesRecorder controller.SourcesRecorder

	var connectionState = domain.ConnectorClientState{
		Account:  "000111",
		ClientID: clientID,
		MessageMetadata: domain.MessageMetadata{
			LatestMessageID: "12345",
			LatestTimestamp: time.Now().Add(-2 * time.Second),
		},
	}

	if err := connectionRegistrar.Register(context.TODO(), connectionState); err != nil {
		t.Fatal(err)
	}

	expectedMessageID := "56789"
	expecteMessageSentTime := time.Now()

	incomingMessage := buildOnlineMessage(t, expectedMessageID, expecteMessageSentTime)

	err := handleOnlineMessage(mqttClient, clientID, incomingMessage, &cfg, &topicBuilder, accountResolver, connectionRegistrar, connectedClientRecorder, sourcesRecorder)

	if err != nil {
		t.Fatal("handleOnlineMessage should not have returned an error")
	}

	recordedConnectionState, err := connectionRegistrar.FindConnectionByClientID(context.Background(), clientID)

	if recordedConnectionState.MessageMetadata.LatestMessageID != incomingMessage.MessageID {
		t.Error("incoming messages does not appear to have been stored")
	}

	if recordedConnectionState.MessageMetadata.LatestTimestamp != incomingMessage.Sent {
		t.Error("incoming messages does not appear to have been stored")
	}

}

func TestHandleDuplicateAndOldOnlineMessages(t *testing.T) {

	var mqttClient MQTT.Client
	var clientID domain.ClientID = "1234"
	var cfg config.Config
	var topicBuilder mqtt.TopicBuilder
	var accountResolver = &mockAccountIdResolver{}
	var connectedClientRecorder controller.ConnectedClientRecorder
	var sourcesRecorder controller.SourcesRecorder

	now := time.Now()

	testCases := []struct {
		testCaseName      string
		currentMessageID  string
		currentSentTime   time.Time
		incomingMessageID string
		incomingSentTime  time.Time
		expectedError     error
	}{
		{"duplicate", "1234-56789", now, "1234-56789", now, errDuplicateOrOldMQTTMessage},
		{"old message", "1234-56789", now, "1234-98765", now.Add(-2 * time.Second), errDuplicateOrOldMQTTMessage},
	}

	for _, tc := range testCases {
		t.Run(tc.testCaseName, func(t *testing.T) {

			var connectionRegistrar = &mockConnectionRegistrar{
				clients: make(map[domain.ClientID]domain.ConnectorClientState),
			}

			var connectionState = domain.ConnectorClientState{
				Account:  "000111",
				ClientID: clientID,
				MessageMetadata: domain.MessageMetadata{
					LatestMessageID: tc.currentMessageID,
					LatestTimestamp: tc.currentSentTime,
				},
			}

			if err := connectionRegistrar.Register(context.TODO(), connectionState); err != nil {
				t.Fatal(err)
			}

			incomingMessage := buildOnlineMessage(t, tc.incomingMessageID, tc.incomingSentTime)

			err := handleOnlineMessage(mqttClient, clientID, incomingMessage, &cfg, &topicBuilder, accountResolver, connectionRegistrar, connectedClientRecorder, sourcesRecorder)

			if err != tc.expectedError {
				t.Fatal("handleOnlineMesssage did not return the expected error!")
			}

		})
	}
}

func buildOnlineMessage(t *testing.T, messageID string, sentTime time.Time) protocol.ControlMessage {
	var connectionStatusPayload = "{\"state\":\"online\"}"
	var content map[string]interface{}

	if err := json.Unmarshal([]byte(connectionStatusPayload), &content); err != nil {
		t.Fatal(err)
	}

	var incomingMessage = protocol.ControlMessage{
		MessageType: "connection-status",
		MessageID:   messageID,
		Version:     1,
		Sent:        sentTime,
		Content:     content,
	}

	return incomingMessage
}
