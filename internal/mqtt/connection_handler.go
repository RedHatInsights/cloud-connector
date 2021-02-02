package mqtt

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	MQTT "github.com/eclipse/paho.mqtt.golang"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"

	"github.com/sirupsen/logrus"
)

const (
	CONTROL_MESSAGE_INCOMING_TOPIC string = "redhat/insights/+/control/out"
	CONTROL_MESSAGE_OUTGOING_TOPIC string = "redhat/insights/%s/control/in"
	DATA_MESSAGE_INCOMING_TOPIC    string = "redhat/insights/+/data/out"
	DATA_MESSAGE_OUTGOING_TOPIC    string = "redhat/insights/%s/data/in"
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

	if u.Scheme == "wss" {
		jwtGenerator, err := jwt_utils.NewJwtGenerator(cfg.MqttBrokerJwtGeneratorImpl, cfg)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to instantiate a JWT generator for the MQTT connection")
			return nil, err
		}

		brokerConfigFuncs = append(brokerConfigFuncs, WithJwtAsHttpHeader(jwtGenerator))
	}

	return brokerConfigFuncs, nil
}

func RegisterSubscribers(brokerUrl string, tlsConfig *tls.Config, cfg *config.Config, subscribers []Subscriber) error {

	brokerConfigFuncs, err := buildBrokerConfigFuncList(brokerUrl, tlsConfig, cfg)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("MQTT Broker configuration error")
		return err
	}

	connOpts, err := NewBrokerOptions(brokerUrl, brokerConfigFuncs...)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to build MQTT ClientOptions")
		return err
	}

	mqttClient := MQTT.NewClient(connOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Error("Unable to connect to MQTT broker")
		return token.Error()
	}

	logger.Log.Info("Connected to MQTT broker: ", brokerUrl)

	for _, subscriber := range subscribers {
		logger.Log.Infof("Subscribing to MQTT topic: %s - QOS: %d\n", subscriber.Topic, subscriber.Qos)
		if token := mqttClient.Subscribe(subscriber.Topic, subscriber.Qos, subscriber.EntryPoint); token.Wait() && token.Error() != nil {
			logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Errorf("Subscribing to MQTT topic (%s) failed", subscriber.Topic)
			return token.Error()
		}
	}

	return nil
}

func ControlMessageHandler(connectionRegistrar controller.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

		clientID, err := verifyTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

		if message.Payload() == nil || len(message.Payload()) == 0 {
			// This will happen when a retained message is removed
			logger.Debugf("client sent an empty payload\n") // FIXME:  Remove me later on...
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
			handleConnectionStatusMessage(client, clientID, controlMsg, connectionRegistrar, accountResolver, connectedClientRecorder)
		case "event":
			handleEventMessage(client, clientID, controlMsg)
		default:
			logger.Debug("Received an invalid message type:", controlMsg.MessageType)
		}
	}
}

func handleConnectionStatusMessage(client MQTT.Client, clientID domain.ClientID, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder) error {

	// FIXME: pass the logger around
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID})

	logger.Debug("handling connection status control message")

	account, err := accountResolver.MapClientIdToAccountId(context.Background(), clientID)
	if err != nil {
		// FIXME:  tell the client to disconnect
		return err
	}

	logger = logger.WithFields(logrus.Fields{"account": account})

	handshakePayload := msg.Content.(map[string]interface{})

	connectionState, gotConnectionState := handshakePayload["state"]

	if gotConnectionState == false {
		// FIXME: Close down the connection
		return errors.New("Invalid connection state")
	}

	if connectionState == "online" {
		return handleOnlineMessage(client, account, clientID, msg, connectionRegistrar, connectedClientRecorder)
	} else if connectionState == "offline" {
		return handleOfflineMessage(client, account, clientID, msg, connectionRegistrar)
	} else {
		return errors.New("Invalid connection state")
	}

	return nil
}

func handleOnlineMessage(client MQTT.Client, account domain.AccountID, clientID domain.ClientID, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar, connectedClientRecorder controller.ConnectedClientRecorder) error {

	// FIXME: pass the logger around
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "account": account})

	logger.Debug("handling online connection-status message")

	handshakePayload := msg.Content.(map[string]interface{}) // FIXME:

	canonicalFacts, gotCanonicalFacts := handshakePayload["canonical_facts"]

	if gotCanonicalFacts == false {
		fmt.Println("FIXME: error!  hangup")
		return errors.New("Invalid handshake")
	}

	err := connectedClientRecorder.RecordConnectedClient(context.Background(), account, clientID, canonicalFacts)
	if err != nil {
		// FIXME:  If we cannot "register" the connection with inventory, then send a disconnect message
		return err
	}

	connectionEvent(account, clientID, msg.Content)

	proxy := ReceptorMQTTProxy{ClientID: string(clientID), Client: client, Logger: logger}

	connectionRegistrar.Register(context.Background(), string(account), string(clientID), &proxy)
	// FIXME: check for error, but ignore duplicate registration errors

	return nil
}

func handleOfflineMessage(client MQTT.Client, account domain.AccountID, clientID domain.ClientID, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) error {

	// FIXME: pass the logger around
	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "account": account})

	logger.Debug("handling offline connection-status message")

	connectionRegistrar.Unregister(context.Background(), string(account), string(clientID))

	disconnectionEvent(account, clientID)

	logger.Debug("Removing client's retained connection-status message")
	// FIXME:
	clientTopic := fmt.Sprintf("redhat/insights/%s/control/out", clientID)
	client.Publish(clientTopic, byte(0), true, "")

	return nil
}

func verifyTopic(topic string) (domain.ClientID, error) {
	items := strings.Split(topic, "/")
	if len(items) != 5 {
		return "", errors.New("MQTT topic requires 4 sections: redhat, insights, <clientID>, control, in")
	}

	if items[0] != "redhat" || items[1] != "insights" || items[4] != "out" {
		fmt.Println("topic: ", topic)
		return "", errors.New("MQTT topic needs to be redhat/insights/<clientID>/control/out")
	}

	return domain.ClientID(items[2]), nil
}

func registerConnectionInSources(account domain.AccountID, clientID domain.ClientID, catalogServiceFacts interface{}) error {
	fmt.Println("FIXME: adding entry to sources - ", account, clientID, catalogServiceFacts)
	return nil
}

func handleEventMessage(client MQTT.Client, clientID domain.ClientID, msg ControlMessage) error {
	fmt.Printf("FIXME: Got an event: %+v\n", msg.Content)
	return nil
}

func connectionEvent(account domain.AccountID, clientID domain.ClientID, canonicalFacts interface{}) {
	fmt.Println("FIXME: send new connection kafka message")
}

func disconnectionEvent(account domain.AccountID, clientID domain.ClientID) {
	fmt.Println("FIXME: send lost connection kafka message")
}

func DataMessageHandler() func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())
	}
}
