package mqtt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	MQTT "github.com/eclipse/paho.mqtt.golang"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"

	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	Topic      string
	EntryPoint MQTT.MessageHandler
	Qos        byte
}

func buildBrokerConfigFuncList(brokerUrl string, tlsConfig *tls.Config, cfg *config.Config) ([]MqttClientOptionsFunc, error) {

	u, err := url.Parse(brokerUrl)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to determine protocol for the MQTT connection")
		return nil, err
	}

	brokerConfigFuncs := []MqttClientOptionsFunc{}

	if tlsConfig != nil {
		brokerConfigFuncs = append(brokerConfigFuncs, WithTlsConfig(tlsConfig))
	}

	if u.Scheme == "wss" { //Rethink this check - jwt also works over TLS
		jwtGenerator, err := jwt_utils.NewJwtGenerator(cfg.MqttBrokerJwtGeneratorImpl, cfg)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to instantiate a JWT generator for the MQTT connection")
			return nil, err
		}
		brokerConfigFuncs = append(brokerConfigFuncs, WithJwtAsHttpHeader(jwtGenerator))
		brokerConfigFuncs = append(brokerConfigFuncs, WithJwtReconnectingHandler(jwtGenerator))
	}

	if cfg.MqttClientId != "" {
		brokerConfigFuncs = append(brokerConfigFuncs, WithClientID(cfg.MqttClientId))
	}

	brokerConfigFuncs = append(brokerConfigFuncs, WithCleanSession(cfg.MqttCleanSession))

	brokerConfigFuncs = append(brokerConfigFuncs, WithResumeSubs(cfg.MqttResumeSubs))

	return brokerConfigFuncs, nil
}

func RegisterSubscribers(brokerUrl string, tlsConfig *tls.Config, cfg *config.Config, subscribers []Subscriber, defaultMessageHandler func(MQTT.Client, MQTT.Message)) (MQTT.Client, error) {

	brokerConfigFuncs, err := buildBrokerConfigFuncList(brokerUrl, tlsConfig, cfg)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("MQTT Broker configuration error")
		return nil, err
	}

	// Add a default publish message handler as some messages will get delivered before the topic
	// subscriptions are setup completely
	// See "Common Problems" here: https://github.com/eclipse/paho.mqtt.golang#common-problems
	brokerConfigFuncs = append(brokerConfigFuncs, WithDefaultPublishHandler(defaultMessageHandler))

	connOpts, err := NewBrokerOptions(brokerUrl, brokerConfigFuncs...)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to build MQTT ClientOptions")
		return nil, err
	}

	connOpts.SetOnConnectHandler(func(client MQTT.Client) {
		for _, subscriber := range subscribers {
			logger.Log.Infof("Subscribing to MQTT topic: %s - QOS: %d\n", subscriber.Topic, subscriber.Qos)
			if token := client.Subscribe(subscriber.Topic, subscriber.Qos, subscriber.EntryPoint); token.Wait() && token.Error() != nil {
				logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Fatalf("Subscribing to MQTT topic (%s) failed", subscriber.Topic)
			}
		}
	})

	mqttClient := MQTT.NewClient(connOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Error("Unable to connect to MQTT broker")
		return nil, token.Error()
	}

	logger.Log.Info("Connected to MQTT broker: ", brokerUrl)

	return mqttClient, nil
}

func ControlMessageHandler(topicVerifier *TopicVerifier, connectionRegistrar controller.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received control message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

		_, clientID, err := topicVerifier.VerifyIncomingTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

		if message.Payload() == nil || len(message.Payload()) == 0 {
			// This will happen when a retained message is removed
			logger.Trace("client sent an empty payload")
			return
		}

		var controlMsg ControlMessage

		if err := json.Unmarshal(message.Payload(), &controlMsg); err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Failed to unmarshal control message")
			return
		}

		logger.Debug("Got a control message:", controlMsg)

		switch controlMsg.MessageType {
		case "connection-status":
			handleConnectionStatusMessage(client, clientID, controlMsg, connectionRegistrar, accountResolver, connectedClientRecorder, sourcesRecorder)
		case "event":
			handleEventMessage(client, clientID, controlMsg)
		default:
			logger.Debug("Received an invalid message type:", controlMsg.MessageType)
		}
	}
}

func handleConnectionStatusMessage(client MQTT.Client, clientID domain.ClientID, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) error {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debug("handling connection status control message")

	handshakePayload := msg.Content.(map[string]interface{})

	connectionState, gotConnectionState := handshakePayload["state"]

	if gotConnectionState == false {
		// FIXME: Close down the connection
		return errors.New("Invalid connection state")
	}

	if connectionState == "online" {
		return handleOnlineMessage(client, clientID, msg, accountResolver, connectionRegistrar, connectedClientRecorder, sourcesRecorder)
	} else if connectionState == "offline" {
		return handleOfflineMessage(client, clientID, msg, connectionRegistrar)
	} else {
		return errors.New("Invalid connection state")
	}

	return nil
}

func handleOnlineMessage(client MQTT.Client, clientID domain.ClientID, msg ControlMessage, accountResolver controller.AccountIdResolver, connectionRegistrar controller.ConnectionRegistrar, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) error {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debug("handling online connection-status message")

	identity, account, err := accountResolver.MapClientIdToAccountId(context.Background(), clientID)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to resolve client id to account number")
		// FIXME:  tell the client to disconnect
		return err
	}

	logger = logger.WithFields(logrus.Fields{"account": account})

	handshakePayload := msg.Content.(map[string]interface{}) // FIXME:

	canonicalFacts, gotCanonicalFacts := handshakePayload["canonical_facts"]

	if gotCanonicalFacts == false {
		fmt.Println("FIXME: error!  hangup")
		return errors.New("Invalid handshake")
	}

	err = connectedClientRecorder.RecordConnectedClient(context.Background(), identity, account, clientID, canonicalFacts)
	if err != nil {
		// FIXME:  If we cannot "register" the connection with inventory, then send a disconnect message
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to record client id within the platform")
		return err
	}

	proxy := ReceptorMQTTProxy{AccountID: account, ClientID: clientID, Client: client}

	connectionRegistrar.Register(context.Background(), account, clientID, &proxy)
	// FIXME: check for error, but ignore duplicate registration errors

	processDispatchers(sourcesRecorder, identity, account, clientID, handshakePayload)

	return nil
}

const (
	DISPATCHERS_KEY          = "dispatchers"
	CATALOG_DISPATCHER_KEY   = "catalog"
	CATALOG_APPLICATION_TYPE = "ApplicationType"
	CATALOG_SOURCE_NAME      = "SrcName"
	CATALOG_SOURCE_REF       = "SourceRef"
	CATALOG_SOURCE_TYPE      = "SrcType"
)

func processDispatchers(sourcesRecorder controller.SourcesRecorder, identity domain.Identity, account domain.AccountID, clientId domain.ClientID, handshakePayload map[string]interface{}) {

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientId, "account": account})

	dispatchers, gotDispatchers := handshakePayload[DISPATCHERS_KEY]

	if gotDispatchers == false {
		logger.Debug("No dispatchers found")
		return
	}

	dispatchersMap := dispatchers.(map[string]interface{})

	catalog, gotCatalog := dispatchersMap[CATALOG_DISPATCHER_KEY]

	if gotCatalog == false {
		logger.Debug("No catalog dispatcher found")
		return
	}

	catalogMap := catalog.(map[string]interface{})

	applicationType, gotApplicationType := catalogMap[CATALOG_APPLICATION_TYPE]
	sourceType, gotSourceType := catalogMap[CATALOG_SOURCE_TYPE]
	sourceRef, gotSourceRef := catalogMap[CATALOG_SOURCE_REF]
	sourceName, gotSourceName := catalogMap[CATALOG_SOURCE_NAME]

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

func handleOfflineMessage(client MQTT.Client, clientID domain.ClientID, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) error {
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debug("handling offline connection-status message")

	connectionRegistrar.Unregister(context.Background(), clientID)

	return nil
}

func handleEventMessage(client MQTT.Client, clientID domain.ClientID, msg ControlMessage) error {
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})
	logger.Debugf("Received an event message from client: %v\n", msg)
	return nil
}

func DataMessageHandler() func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received data message: %s\n", message.Payload())

		if message.Payload() == nil || len(message.Payload()) == 0 {
			logger.Log.Debugf("Received empty data message")
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
