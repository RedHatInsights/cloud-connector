package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	MQTT "github.com/eclipse/paho.mqtt.golang"

	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

const CONNECTION_STATUS_TOPIC = "redhat/insights/in/+"

func NewTLSConfig(certFilePath string, keyFilePath string) (*tls.Config, error) {
	// Import trusted certificates from CAfile.pem.
	// Alternatively, manually add CA certificates to
	// default openssl CA bundle.
	/*
	   certpool := x509.NewCertPool()
	   pemCerts, err := ioutil.ReadFile("samplecerts/CAfile.pem")
	   if err == nil {
	       certpool.AppendCertsFromPEM(pemCerts)
	   }
	*/

	// Import client certificate/key pair
	cert, err := tls.LoadX509KeyPair(certFilePath, keyFilePath)
	if err != nil {
		return nil, err
	}

	// Just to print out the client certificate..
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, err
	}

	// Create tls.Config with desired tls properties
	tlsConfig := &tls.Config{
		// RootCAs = certs used to verify server cert.
		//RootCAs: certpool,
		// ClientAuth = whether to request cert from server.
		// Since the server is set up for SSL, this happens
		// anyways.
		//ClientAuth: tls.NoClientCert,
		// ClientCAs = certs used to validate client cert.
		//ClientCAs: nil,
		// InsecureSkipVerify = verify that cert contents
		// match server. IP matches what is in cert etc.
		InsecureSkipVerify: true,
		// Certificates = list of certs client sends to server.
		Certificates: []tls.Certificate{cert},
	}

	return tlsConfig, nil
}

func NewConnectionRegistrar(brokerUri string, certFilePath string, certKeyPath string, connectionRegistrar controller.ConnectionRegistrar) error {

	tlsconfig, err := NewTLSConfig(certFilePath, certKeyPath)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to config TLS for the MQTT broker connection")
		return err
	}

	connOpts := MQTT.NewClientOptions()

	connOpts.AddBroker(brokerUri)

	connOpts.SetTLSConfig(tlsconfig)

	recordConnection := controlMessageHandler(connectionRegistrar)

	connOpts.OnConnect = func(c MQTT.Client) {
		logger.Log.Info("Subscribing to topic: ", CONNECTION_STATUS_TOPIC)
		if token := c.Subscribe(CONNECTION_STATUS_TOPIC, 0, recordConnection); token.Wait() && token.Error() != nil {
			logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Fatal("Subscribing to topic failed")
		}
	}

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Error("Unable to connect to MQTT broker")
		return token.Error()
	}

	logger.Log.Info("Connected to broker: ", brokerUri)

	return nil
}

func controlMessageHandler(connectionRegistrar controller.ConnectionRegistrar) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		logger.Log.Debugf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

		//verify the MQTT topic
		clientID, err := verifyTopic(message.Topic())
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Failed to verify topic")
			return
		}

		logger := logger.Log.WithFields(logrus.Fields{"clientID": clientID})

		var connMsg ControlMessage

		if message.Payload() == nil || len(message.Payload()) == 0 {
			// FIXME: This will happen when a retained message is removed
			logger.Debugf("client sent an empty payload\n")
			return
		}

		if err := json.Unmarshal(message.Payload(), &connMsg); err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Failed to unmarshal control message")
			return
		}

		logger.Debug("Got a connection:", connMsg)

		switch connMsg.MessageType {
		case "connection-status":
			handleConnectionStatusMessage(client, clientID, connMsg, connectionRegistrar)
		case "event":
			handleEventMessage(client, clientID, connMsg)
		default:
			logger.Debug("Received an invalid message type:", connMsg.MessageType)
		}
	}
}

func handleConnectionStatusMessage(client MQTT.Client, clientID string, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) error {

	account, err := getAccountNumberFromBop(clientID)

	if err != nil {
		return err
	}

	handshakePayload := msg.Content.(map[string]interface{})

	connectionState, gotConnectionState := handshakePayload["state"]

	if gotConnectionState == false {
		// FIXME: Close down the connection
		return errors.New("Invalid connection state")
	}

	if connectionState == "online" {
		handleOnlineMessage(client, account, clientID, msg, connectionRegistrar)
	} else if connectionState == "offline" {
		handleOfflineMessage(client, account, clientID, msg, connectionRegistrar)
	} else {
		return errors.New("Invalid connection state")
	}

	return nil
}

func handleOnlineMessage(client MQTT.Client, account string, clientID string, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) error {

	handshakePayload := msg.Content.(map[string]interface{}) // FIXME:

	canonicalFacts, gotCanonicalFacts := handshakePayload["canonical_facts"]

	if gotCanonicalFacts == false {
		fmt.Println("FIXME: error!  hangup")
		return errors.New("Invalid handshake")
	}

	registerConnectionInInventory(account, clientID, canonicalFacts)

	connectionEvent(account, clientID, msg.Content)

	proxy := ReceptorMQTTProxy{ClientID: clientID, Client: client}

	connectionRegistrar.Register(context.Background(), account, clientID, &proxy)
	// FIXME: check for error, but ignore duplicate registration errors

	return nil
}

func handleOfflineMessage(client MQTT.Client, account string, clientID string, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) {

	connectionRegistrar.Unregister(context.Background(), account, clientID)

	disconnectionEvent(account, clientID)

	// FIXME:
	clientTopic := fmt.Sprintf("redhat/insights/in/%s", clientID)
	client.Publish(clientTopic, byte(0), true, "")
}

func registerCatalogWithSources(client MQTT.Client, account string, clientID string, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) error {

	return nil
}

func getAccountNumberFromBop(clientID string) (string, error) {
	// FIXME: need to lookup the account number for the connected client
	fmt.Println("FIXME: looking up the connection's account number in BOP")

	/*
		Required
		x-rh-apitoken *
		x-rh-clientid

		Optional
		x-rh-insights-env

		Cert auth
		x-rh-certauth-cn
		x-rh-certauth-issuer
		x-rh-insights-certauth-secret

		make http GET



	*/

	return "010101", nil
}

func verifyTopic(topic string) (string, error) {
	items := strings.Split(topic, "/")
	if len(items) != 4 {
		return "", errors.New("MQTT topic requires 4 sections: redhat, insights, <clientID>, in")
	}

	if items[0] != "redhat" || items[1] != "insights" || items[2] != "in" {
		return "", errors.New("MQTT topic needs to be redhat/insights/<clientID>/in")
	}

	return items[3], nil
}

func registerConnectionInInventory(account string, clientID string, canonicalFacts interface{}) {
	fmt.Println("FIXME: send inventory kafka message - ", account, clientID, canonicalFacts)
}

func registerConnectionInSources(account string, clientID string, catalogServiceFacts interface{}) {
	fmt.Println("FIXME: adding entry to sources - ", account, clientID, catalogServiceFacts)
}

func handleEventMessage(client MQTT.Client, clientID string, msg ControlMessage) {
	fmt.Printf("FIXME: Got an event: %+v\n", msg.Content)
}

func connectionEvent(account string, clientID string, canonicalFacts interface{}) {
	fmt.Println("FIXME: send new connection kafka message")
}

func disconnectionEvent(account string, clientID string) {
	fmt.Println("FIXME: send lost connection kafka message")
}
