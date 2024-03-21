package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	Connector "github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

func buildIdentityHeader(orgId string, accountNumber string) string {
	s := fmt.Sprintf(`{"identity": {"account_number": "%s", "org_id": "%s", "internal": {}, "service_account": {"client_id": "0000", "username": "jdoe"}, "type": "Associate"}}`, accountNumber, orgId)
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func startConcurrentLoadTestClient(broker string, certFile string, keyFile string, connectionCount int, cloudConnectorUrl string, orgId string, accountNumber string) {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	/*
	   MQTT.ERROR = logger
	   MQTT.CRITICAL = logger
	MQTT.DEBUG = logger
	*/
	MQTT.WARN = logger

	identityHeader := buildIdentityHeader(orgId, accountNumber)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	for i := 0; i < connectionCount; i++ {
		go startConcurrentTestClient(certFile, keyFile, broker, cloudConnectorUrl, i, identityHeader)
	}

	<-c
	fmt.Println("Client going down...disconnecting from mqtt uncleanly")
}

func startConcurrentTestClient(certFile string, keyFile string, broker string, cloudConnectorUrl string, i int, identityHeader string) {
	var numMessagesSent int
	var messageReceived chan string
	messageReceived = make(chan string)

	onMessageReceived := notifyOnMessageReceived(messageReceived)

	clientId, mqttClient, _ := startProducer(certFile, keyFile, broker, onMessageReceived, i)

	time.Sleep(1 * time.Second)
	verifyClientIsRegistered(cloudConnectorUrl, clientId, identityHeader)

	rand.Seed(time.Now().UnixNano())

	for {
		randomSleepTime := rand.Intn(1000)
		randomSleepTime = 10
		fmt.Println("Sleeping for ", randomSleepTime)
		time.Sleep(time.Duration(randomSleepTime) * time.Second)

		if numMessagesSent > 10 {
			disconnectMqttClient(mqttClient)
			time.Sleep(1 * time.Second)
			verifyClientIsUnregistered(cloudConnectorUrl, clientId, identityHeader)
			clientId, mqttClient, _ = startProducer(certFile, keyFile, broker, onMessageReceived, i)
			time.Sleep(1 * time.Second)
			verifyClientIsRegistered(cloudConnectorUrl, clientId, identityHeader)
			numMessagesSent = 0
		}

		messageId, _ := sendMessageToClient(cloudConnectorUrl, clientId, identityHeader)
		verifyMessageWasReceived(messageReceived, messageId)
		numMessagesSent++
	}
}

func startProducer(certFile string, keyFile string, broker string, onMessageReceived func(MQTT.Client, MQTT.Message), i int) (string, MQTT.Client, error) {
	tlsconfig, _ := NewTLSConfig(certFile, keyFile)

	clientID := generateUUID()

	controlReadTopic := fmt.Sprintf("redhat/insights/%s/control/in", clientID)
	controlWriteTopic := fmt.Sprintf("redhat/insights/%s/control/out", clientID)
	dataReadTopic := fmt.Sprintf("redhat/insights/%s/data/in", clientID)
	fmt.Println("control consumer topic: ", controlReadTopic)

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

	fmt.Println("last-will - publishing to topic:", controlWriteTopic)
	fmt.Println("last-will -  retained: ", retained)
	fmt.Println("last-will - qos: ", qos)

	connOpts.SetWill(controlWriteTopic, string(payload), qos, retained)

	connOpts.SetOnConnectHandler(func(client MQTT.Client) {
		fmt.Println("*** OnConnect - subscribing to topic:", controlReadTopic)
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

    fmt.Println("CONNECTED ", clientID)

	return clientID, client, nil
}

func notifyOnMessageReceived(messageReceived chan string) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		fmt.Printf("**** Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

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

        fmt.Println("MESSAGE_RECEIVED ", dataMsg.MessageID)

		messageReceived <- dataMsg.MessageID
	}
}

func disconnectMqttClient(mqttClient MQTT.Client) {
	fmt.Println("DISCONNECTING!!")
	mqttClient.Disconnect(1000)
}

func verifyMessageWasReceived(messageReceived chan string, expectedMessageId uuid.UUID) {
	fmt.Println("Checking to see if message was received")

	select {
	case receivedMessageId := <-messageReceived:
		fmt.Println("Got message: ", receivedMessageId)
		if receivedMessageId != expectedMessageId.String() {
			fmt.Println("Epic fail...got wrong message!")
		}
	case <-time.After(10 * time.Second):
		fmt.Println("Epic Fail...did not get message after 10 second")
	}
}
