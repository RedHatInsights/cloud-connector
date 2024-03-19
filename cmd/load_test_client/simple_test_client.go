package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	//"flag"
	"fmt"
	"log"
	//"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	Connector "github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

func NewTLSConfig(certFile string, keyFile string) (*tls.Config, string) {

	// Import client certificate/key pair
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		panic(err)
	}

	// Just to print out the client certificate..
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		panic(err)
	}
	//fmt.Println(cert.Leaf)
	fmt.Println(cert.Leaf.Subject.CommonName)

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

	return tlsConfig, cert.Leaf.Subject.CommonName
}

func startLoadTestClient(broker string, certFile string, keyFile string) {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	/*
	   MQTT.ERROR = logger
	   MQTT.CRITICAL = logger
	   MQTT.WARN = logger
	*/
	MQTT.DEBUG = logger

	/*
		broker := flag.String("broker", "tcp://eclipse-mosquitto:1883", "hostname / port of broker")
		certFile := flag.String("cert", "cert.pem", "path to cert file")
		keyFile := flag.String("key", "key.pem", "path to key file")
		flag.Parse()
	*/

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go startTestClient(certFile, keyFile, broker)

	panicTimer := time.NewTimer(60 * time.Second)

	select {
	case <-c:
		fmt.Println("Received signal...")
		break
	case <-panicTimer.C:
		panic("I'm out!!")
	}
	fmt.Println("Client going down...disconnecting from mqtt uncleanly")
}

var m MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("default handler rec TOPIC: %s MSG:%s\n", msg.Topic(), msg.Payload())
}

func startTestClient(certFile string, keyFile string, broker string) {
	startProducer(certFile, keyFile, broker)
}

func startProducer(certFile string, keyFile string, broker string) (string, MQTT.Client, error) {
	tlsconfig, _ := NewTLSConfig(certFile, keyFile)

	clientID := generateUUID()

	controlReadTopic := fmt.Sprintf("redhat/insights/%s/control/in", clientID)
	controlWriteTopic := fmt.Sprintf("redhat/insights/%s/control/out", clientID)
	dataReadTopic := fmt.Sprintf("redhat/insights/%s/data/in", clientID)
	fmt.Println("control consumer topic: ", controlReadTopic)
	fmt.Println("data consumer topic: ", dataReadTopic)

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(broker)
	connOpts.SetClientID(clientID)
	connOpts.SetTLSConfig(tlsconfig)
	connOpts.SetAutoReconnect(false)

	connectionStatusMsgPayload := Connector.ConnectionStatusMessageContent{ConnectionState: "offline"}
	lastWillMsg := Connector.ControlMessage{
		MessageType: "connection-status",
		MessageID:   generateUUID(),
		Version:     1,
		Sent:        time.Now(),
		Content:     connectionStatusMsgPayload,
	}
	payload, err := json.Marshal(lastWillMsg)

	if err != nil {
		fmt.Println("marshal of message failed, err:", err)
		panic(err)
	}

	retained := false
	qos := byte(1)

	connOpts.SetWill(controlWriteTopic, string(payload), qos, retained)

	connOpts.SetOnConnectHandler(func(client MQTT.Client) {
		if token := client.Subscribe(controlReadTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}

		if token := client.Subscribe(dataReadTopic, 0, onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	})

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	fmt.Println("Connected to server ", broker)

	sentTime := time.Now()

	cf := Connector.CanonicalFacts{
		InsightsID:            generateUUID(),
		MachineID:             generateUUID(),
		BiosID:                generateUUID(),
		SubscriptionManagerID: generateUUID(),
		SatelliteID:           generateUUID(),
		IpAddresses:           []string{"192.168.68.101"},
		MacAddresses:          []string{"54:54:45:45:62:26"},
		Fqdn:                  "fred.flintstone.com",
	}

	dispatchers := make(Connector.Dispatchers)
	tags := make(Connector.Tags)

	publishConnectionStatusMessage(client, controlWriteTopic, qos, retained, generateUUID(), cf, dispatchers, tags, sentTime)

	fmt.Printf("CONNECTED %s\n", clientID)

	return clientID, client, nil
}

func publishConnectionStatusMessage(client MQTT.Client, topic string, qos byte, retained bool, messageID string, cf Connector.CanonicalFacts, dispatchers Connector.Dispatchers, tags Connector.Tags, sentTime time.Time) {

	connectionStatusPayload := Connector.ConnectionStatusMessageContent{
		CanonicalFacts:  cf,
		Dispatchers:     dispatchers,
		ConnectionState: "online",
		Tags:            tags,
		ClientName:      "fred.flintstone",
		ClientVersion:   "11.0.2",
	}
	connMsg := Connector.ControlMessage{
		MessageType: "connection-status",
		MessageID:   messageID,
		Version:     1,
		Sent:        sentTime,
		Content:     connectionStatusPayload,
	}

	payload, err := json.Marshal(connMsg)

	if err != nil {
		fmt.Println("marshal of message failed, err:", err)
		panic(err)
	}

	client.Publish(topic, qos, retained, payload)
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	//fmt.Printf("**** Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

	var dataMsg Connector.DataMessage

	if message.Payload() == nil || len(message.Payload()) == 0 {
		fmt.Println("empty payload")
		return
	}

	if err := json.Unmarshal(message.Payload(), &dataMsg); err != nil {
		fmt.Println("unmarshal of message failed, err:", err)
		panic(err)
	}

	//fmt.Println("Got a message:", dataMsg)

	fmt.Printf("MESSAGE_RECEIVED %s\n", dataMsg.MessageID)
}

func generateUUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
}
