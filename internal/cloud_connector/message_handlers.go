package cloud_connector

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

var (
	errDuplicateOrOldMQTTMessage = errors.New("duplicate or old message")
)

const (
	canonicalFactsKey           = "canonical_facts"
	dispatchersKey              = "dispatchers"
	tagsKey                     = "tags"
	catalogDispatcherKey        = "catalog"
	catalogApplicationType      = "ApplicationType"
	catalogSourceName           = "SrcName"
	catalogSourceRef            = "SourceRef"
	catalogSourceType           = "SrcType"
	playbookWorkerDispatcherKey = "rhc-worker-playbook"
)

func HandleControlMessage(cfg *config.Config, mqttClient MQTT.Client, topicBuilder *mqtt.TopicBuilder, connectionRegistrar connection_repository.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) func(MQTT.Client, domain.ClientID, string) {

	return func(client MQTT.Client, clientID domain.ClientID, payload string) {

		metrics.controlMessageReceivedCounter.Inc()

		logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

		if len(payload) == 0 {
			// This will happen when a retained message is removed
			logger.Trace("client sent an empty payload")
			return
		}

		var controlMsg protocol.ControlMessage

		if err := json.Unmarshal([]byte(payload), &controlMsg); err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Failed to unmarshal control message")
			return
		}

		logger.Debug("Got a control message:", controlMsg)

		switch controlMsg.MessageType {
		case "connection-status":
			handleConnectionStatusMessage(client, clientID, controlMsg, cfg, topicBuilder, connectionRegistrar, accountResolver, connectedClientRecorder, sourcesRecorder)
		case "event":
			handleEventMessage(client, clientID, controlMsg)
		default:
			logger.Debug("Received an invalid message type:", controlMsg.MessageType)
		}
	}
}

func handleConnectionStatusMessage(client MQTT.Client, clientID domain.ClientID, msg protocol.ControlMessage, cfg *config.Config, topicBuilder *mqtt.TopicBuilder, connectionRegistrar connection_repository.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) error {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debug("handling connection status control message")

	handshakePayload := msg.Content.(map[string]interface{})

	connectionState, gotConnectionState := handshakePayload["state"]

	if gotConnectionState == false {
		// FIXME: Close down the connection
		return errors.New("Invalid connection state")
	}

	if connectionState == "online" {
		return handleOnlineMessage(client, clientID, msg, cfg, topicBuilder, accountResolver, connectionRegistrar, connectedClientRecorder, sourcesRecorder)
	} else if connectionState == "offline" {
		return handleOfflineMessage(client, clientID, msg, connectionRegistrar)
	} else {
		logger.Debug("Invalid connection state from connection-status message.")
		return errors.New("Invalid connection state")
	}

	return nil
}

func handleOnlineMessage(client MQTT.Client, clientID domain.ClientID, msg protocol.ControlMessage, cfg *config.Config, topicBuilder *mqtt.TopicBuilder, accountResolver controller.AccountIdResolver, connectionRegistrar connection_repository.ConnectionRegistrar, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) error {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "message_id": msg.MessageID})

	logger.Debug("handling online connection-status message")

	connectionState, err := connectionRegistrar.FindConnectionByClientID(context.Background(), clientID)

	if isDuplicateOrOldMessage(connectionState, msg) {
		logger.Debug("ignoring message - duplicate or old message")
		return errDuplicateOrOldMQTTMessage
	}

	identity, account, err := accountResolver.MapClientIdToAccountId(context.Background(), clientID)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to resolve client id to account number")

		mqtt.SendReconnectMessageToClient(client, logger, topicBuilder, cfg.MqttControlPublishQoS, clientID, cfg.InvalidHandshakeReconnectDelay)

		return err
	}

	logger = logger.WithFields(logrus.Fields{"account": account})

	handshakePayload := msg.Content.(map[string]interface{})

	rhcClient := domain.ConnectorClientState{ClientID: clientID,
		Account:        account,
		Dispatchers:    handshakePayload[dispatchersKey],
		CanonicalFacts: handshakePayload[canonicalFactsKey],
		Tags:           handshakePayload[tagsKey],
		MessageMetadata: domain.MessageMetadata{LatestMessageID: msg.MessageID,
			LatestTimestamp: msg.Sent},
	}

	_, err = connectionRegistrar.Register(context.Background(), rhcClient)
	if err != nil {
		mqtt.SendReconnectMessageToClient(client, logger, topicBuilder, cfg.MqttControlPublishQoS, clientID, cfg.InvalidHandshakeReconnectDelay)
		return err
	}

	if shouldHostBeRegisteredWithInventory(handshakePayload) == true {

		err = connectedClientRecorder.RecordConnectedClient(context.Background(), identity, rhcClient)

		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Failed to record client id within the platform")

			// If we cannot "register" the connection with inventory, then send a disconnect message
			mqtt.SendReconnectMessageToClient(client, logger, topicBuilder, cfg.MqttControlPublishQoS, clientID, cfg.InvalidHandshakeReconnectDelay)

			return err
		}
	}

	processDispatchers(sourcesRecorder, identity, account, clientID, handshakePayload)

	return nil
}

func isDuplicateOrOldMessage(currentConnectionState domain.ConnectorClientState, incomingMsg protocol.ControlMessage) bool {

	if currentConnectionState.MessageMetadata.LatestMessageID == incomingMsg.MessageID || incomingMsg.Sent.Before(currentConnectionState.MessageMetadata.LatestTimestamp) {
		// Duplicate or old message
		return true
	}

	return false
}

func shouldHostBeRegisteredWithInventory(handshakePayload map[string]interface{}) bool {
	return doesHostHaveCanonicalFacts(handshakePayload) && doesHostHavePlaybookWorker(handshakePayload)
}

func doesHostHaveCanonicalFacts(handshakePayload map[string]interface{}) bool {
	_, gotCanonicalFacts := handshakePayload[canonicalFactsKey]
	return gotCanonicalFacts
}

func doesHostHavePlaybookWorker(handshakePayload map[string]interface{}) bool {

	dispatchers, gotDispatchers := handshakePayload[dispatchersKey]

	if gotDispatchers == false {
		return false
	}

	dispatchersMap := dispatchers.(map[string]interface{})

	_, foundPlaybookDispatcher := dispatchersMap[playbookWorkerDispatcherKey]

	return foundPlaybookDispatcher
}

func processDispatchers(sourcesRecorder controller.SourcesRecorder, identity domain.Identity, account domain.AccountID, clientId domain.ClientID, handshakePayload map[string]interface{}) {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientId, "account": account})

	dispatchers, gotDispatchers := handshakePayload[dispatchersKey]

	if gotDispatchers == false {
		logger.Debug("No dispatchers found")
		return
	}

	dispatchersMap := dispatchers.(map[string]interface{})

	catalog, gotCatalog := dispatchersMap[catalogDispatcherKey]

	if gotCatalog == false {
		logger.Debug("No catalog dispatcher found")
		return
	}

	catalogMap := catalog.(map[string]interface{})

	applicationType, gotApplicationType := catalogMap[catalogApplicationType]
	sourceType, gotSourceType := catalogMap[catalogSourceType]
	sourceRef, gotSourceRef := catalogMap[catalogSourceRef]
	sourceName, gotSourceName := catalogMap[catalogSourceName]

	if gotApplicationType != true || gotSourceType != true || gotSourceRef != true || gotSourceName != true {
		// MISSING FIELDS
		logger.Debug("Found a catalog dispatcher, but missing some of the required fields")
		return
	}

	err := sourcesRecorder.RegisterWithSources(identity, account, clientId, sourceRef.(string), sourceName.(string), sourceType.(string), applicationType.(string))
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to register catalog with sources")
	}
}

func handleOfflineMessage(client MQTT.Client, clientID domain.ClientID, msg protocol.ControlMessage, connectionRegistrar connection_repository.ConnectionRegistrar) error {
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debug("handling offline connection-status message")

	connectionRegistrar.Unregister(context.Background(), clientID)

	return nil
}

func handleEventMessage(client MQTT.Client, clientID domain.ClientID, msg protocol.ControlMessage) error {
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})
	logger.Debugf("Received an event message from client: %v\n", msg)
	return nil
}
