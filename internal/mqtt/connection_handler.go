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

const TOPIC = "redhat/insights"

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

var m MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("rec TOPIC: %s MSG:%s\n", msg.Topic(), msg.Payload())
}

func startSubscriber(brokerUri string, certFilePath string, keyFilePath string, connectionRegistrar controller.ConnectionRegistrar) {

	tlsconfig := NewTLSConfig(certFilePath, keyFilePath)

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(brokerUri)

	connOpts.SetTLSConfig(tlsconfig)

	//lastWill := fmt.Sprintf("{'client': '%s'}", clientID)
	//connOpts.SetWill(ACCOUNT_TOPIC+"/leaving", lastWill, 0, false)

	connOpts.SetDefaultPublishHandler(m)

	recordConnection := messageHandler(connectionRegistrar)

	connOpts.OnConnect = func(c MQTT.Client) {
		topic := fmt.Sprintf("%s/+/in", TOPIC)
		fmt.Println("subscribing to topic: ", topic)
		if token := c.Subscribe(topic, 0, recordConnection); token.Wait() && token.Error() != nil {
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
		clientIDFromTopic, err := verifyTopic(message.Topic())
		if err != nil {
			log.Println(err)
			return
		}

		var connMsg ConnectorMessage

        fmt.Printf("payload type: %T\n", message.Payload())
        fmt.Printf("payload type: |%s|\n", message.Payload())

        if message.Payload() == nil || len(message.Payload()) == 0 {
            fmt.Println("empty payload")
            return
        }

		if err := json.Unmarshal(message.Payload(), &connMsg); err != nil {
			fmt.Println("unmarshal of message failed, err:", err)
			panic(err)
		}

		fmt.Println("Got a connection:", connMsg)

		if clientIDFromTopic != connMsg.ClientID {
			fmt.Println("Potentially malicious connection attempt")
			return
		}

		switch connMsg.MessageType {
		case "handshake":
			handleHandshake(client, connMsg, connectionRegistrar)
		case "disconnect":
			handleDisconnect(client, connMsg, connectionRegistrar)
		case "processing_error":
			handleProcessingError(connMsg)
		default:
			fmt.Println("Invalid message type!")
		}
	}
}

func handleHandshake(client MQTT.Client, msg ConnectorMessage, connectionRegistrar controller.ConnectionRegistrar) error {

	account, err := getAccountNumberFromBop(msg.ClientID)

	if err != nil {
		return err
	}

	handshakePayload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		// FIXME: Return an error response
		return errors.New("Unable to parse payload")
	}

	handshakeType := handshakePayload["type"]

	if handshakeType == "host" {
		handleHostHandshake(client, msg, account, connectionRegistrar)
	} else if handshakeType == "proxy" {
		handleProxyHandshake(client, msg, account, connectionRegistrar)
	} else {
		// FIXME: Return an error response
		return errors.New("Invalid handshake payload type")
	}

	proxy := ReceptorMQTTProxy{ClientID: msg.ClientID, Client: client}

	connectionRegistrar.Register(context.Background(), account, msg.ClientID, &proxy)
	// FIXME: check for error, but ignore duplicate registration errors

	return nil
}

func handleHostHandshake(client MQTT.Client, msg ConnectorMessage, account string, connectionRegistrar controller.ConnectionRegistrar) {

	handshakePayload := msg.Payload.(map[string]interface{})

	canonicalFacts, gotCanonicalFacts := handshakePayload["canonical_facts"]

	if gotCanonicalFacts == false {
		fmt.Println("FIXME: error!  hangup")
		return
	}

	registerConnectionInInventory(account, msg.ClientID, canonicalFacts)

	connectionEvent(account, msg.ClientID, msg.Payload)
}

func handleProxyHandshake(client MQTT.Client, msg ConnectorMessage, account string, connectionRegistrar controller.ConnectionRegistrar) {
	connectionEvent(account, msg.ClientID, msg.Payload)
}

func handleDisconnect(client MQTT.Client, msg ConnectorMessage, connectionRegistrar controller.ConnectionRegistrar) {

	account, err := getAccountNumberFromBop(msg.ClientID)

	if err != nil {
		fmt.Println("Couldn't determine account number...ignoring connection")
		// FIXME: Disconnect client??  How??
		return
	}

	connectionRegistrar.Unregister(context.Background(), account, msg.ClientID)

	disconnectionEvent(account, msg.ClientID)
}

func handleProcessingError(connMsg ConnectorMessage) {
	fmt.Println("PROCESSING ERROR")
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

	if items[0] != "redhat" || items[1] != "insights" || items[3] != "in" {
		return "", errors.New("MQTT topic needs to be redhat/insights/<clientID>/in")
	}

	return items[2], nil
}

func registerConnectionInInventory(account string, clientID string, canonicalFacts interface{}) {
	fmt.Println("FIXME: send inventory kafka message - ", account, clientID, canonicalFacts)
}

func connectionEvent(account string, clientID string, canonicalFacts interface{}) {
	fmt.Println("FIXME: send new connection kafka message")
}

func disconnectionEvent(account string, clientID string) {
	fmt.Println("FIXME: send lost connection kafka message")
}
