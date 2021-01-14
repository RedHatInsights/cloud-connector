package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	//"os"
	"strings"

	MQTT "github.com/eclipse/paho.mqtt.golang"

	"github.com/RedHatInsights/cloud-connector/internal/controller"
)

const CONNECTION_STATUS_TOPIC = "redhat/insights/in/+"

func NewTLSConfig(certFilePath string, keyFilePath string) *tls.Config {
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
		panic(err)
	}

	// Just to print out the client certificate..
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		panic(err)
	}
	fmt.Println(cert.Leaf)
	fmt.Println(cert.Leaf.Subject.ToRDNSequence())
	fmt.Println(cert.Leaf.Subject.CommonName)

	// Create tls.Config with desired tls properties
	return &tls.Config{
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
}

func NewConnectionRegistrar(brokerUri string, certFilePath string, certKeyPath string, connectionRegistrar controller.ConnectionRegistrar) {

	startSubscriber(brokerUri, certFilePath, certKeyPath, connectionRegistrar)
}

func startSubscriber(brokerUri string, certFilePath string, keyFilePath string, connectionRegistrar controller.ConnectionRegistrar) {

	tlsconfig := NewTLSConfig(certFilePath, keyFilePath)

	connOpts := MQTT.NewClientOptions()

	connOpts.AddBroker(brokerUri)

	connOpts.SetTLSConfig(tlsconfig)

	recordConnection := messageHandler(connectionRegistrar)

	connOpts.OnConnect = func(c MQTT.Client) {
		fmt.Println("subscribing to topic: ", CONNECTION_STATUS_TOPIC)
		if token := c.Subscribe(CONNECTION_STATUS_TOPIC, 0, recordConnection); token.Wait() && token.Error() != nil {
			// FIXME:
			panic(token.Error())
		}
	}

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connected to broker", brokerUri)
}

func messageHandler(connectionRegistrar controller.ConnectionRegistrar) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		fmt.Printf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

		//verify the MQTT topic
		clientID, err := verifyTopic(message.Topic())
		if err != nil {
			log.Println(err)
			return
		}

		var connMsg ControlMessage

		if message.Payload() == nil || len(message.Payload()) == 0 {
			fmt.Printf("client %s sent an empty payload\n", clientID)
			return
		}

		if err := json.Unmarshal(message.Payload(), &connMsg); err != nil {
			fmt.Println("unmarshal of message failed, err:", err)
			panic(err)
		}

		fmt.Println("Got a connection:", connMsg)

		switch connMsg.MessageType {
		case "connection-status":
			handleControlMessage(client, clientID, connMsg, connectionRegistrar)
		case "event":
			handleEvent(client, clientID, connMsg)
		default:
			fmt.Println("Invalid message type!")
		}
	}
}

func handleControlMessage(client MQTT.Client, clientID string, msg ControlMessage, connectionRegistrar controller.ConnectionRegistrar) error {

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

func handleEvent(client MQTT.Client, clientID string, msg ControlMessage) {
	fmt.Println("FIXME: Got an event: %+v", msg.Content)
}

func connectionEvent(account string, clientID string, canonicalFacts interface{}) {
	fmt.Println("FIXME: send new connection kafka message")
}

func disconnectionEvent(account string, clientID string) {
	fmt.Println("FIXME: send lost connection kafka message")
}
