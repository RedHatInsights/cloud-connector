package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	Connector "github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func buildIdentityHeader(orgId string, accountNumber string) string {
	s := fmt.Sprintf(`{"identity": {"account_number": "%s", "org_id": "%s", "internal": {}, "service_account": {"client_id": "0000", "username": "jdoe"}, "type": "Associate"}}`, accountNumber, orgId)
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func startConcurrentLoadTestClient(broker string, certFile string, keyFile string, connectionCount int, cloudConnectorUrl string, orgId string, accountNumber string, credRetrieverImpl string, redisAddr string) {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	/*
		   MQTT.ERROR = logger
		   MQTT.CRITICAL = logger
		MQTT.DEBUG = logger
	*/
	MQTT.WARN = logger

	identityHeader := buildIdentityHeader(orgId, accountNumber)

	redisClient := createRedisClient(redisAddr)

	onClientConnected := registerClientConnectedWithRedis(redisClient)
	onMessageReceived := registerMessageReceivedWithRedis(redisClient)

	var credRetriever retrieveCredentialsFunc = retrieveFakeCredentials()

	if credRetrieverImpl == "redis" {
		credRetriever = retrieveCredentialsFromRedis(redisClient)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	for i := 0; i < connectionCount; i++ {
		go startClientRegisterWithRedisController(certFile, keyFile, broker, cloudConnectorUrl, i, identityHeader, credRetriever, onClientConnected, onMessageReceived)
	}

	<-c
	fmt.Println("Client going down...disconnecting from mqtt uncleanly")
}

func startClientRegisterWithRedisController(certFile string, keyFile string, broker string, cloudConnectorUrl string, i int, identityHeader string, credRetriever retrieveCredentialsFunc, onClientConnected func(string), onMessageReceived func(MQTT.Client, MQTT.Message)) {

	startProducer(certFile, keyFile, broker, onClientConnected, onMessageReceived, i, credRetriever)
}

func startLocalTestController(certFile string, keyFile string, broker string, cloudConnectorUrl string, i int, identityHeader string, credRetriever retrieveCredentialsFunc, onClientConnected func(string), onMessageReceived func(MQTT.Client, MQTT.Message)) {
	var numMessagesSent int
	var messageReceived chan string
	messageReceived = make(chan string)

	/*
		onClientConnected := printClientConnected()
		onMessageReceived := notifyOnMessageReceived(messageReceived)
	*/
	sendMessageToClient := sendMessageToClientViaCloudConnector(cloudConnectorUrl, identityHeader)

	clientId, mqttClient, _ := startProducer(certFile, keyFile, broker, onClientConnected, onMessageReceived, i, credRetriever)

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
			clientId, mqttClient, _ = startProducer(certFile, keyFile, broker, onClientConnected, onMessageReceived, i, credRetriever)
			time.Sleep(1 * time.Second)
			verifyClientIsRegistered(cloudConnectorUrl, clientId, identityHeader)
			numMessagesSent = 0
		}

		messageId, _ := sendMessageToClient(clientId)
		verifyMessageWasReceived(messageReceived, messageId)
		numMessagesSent++
	}
}

func startProducer(certFile string, keyFile string, broker string, onClientConnected func(string), onMessageReceived func(MQTT.Client, MQTT.Message), i int, retrieveCredentials retrieveCredentialsFunc) (string, MQTT.Client, error) {
	tlsconfig, _ := NewTLSConfig(certFile, keyFile)

	clientID := generateUUID()

	username, password := retrieveCredentials()

	controlReadTopic := fmt.Sprintf("redhat/insights/%s/control/in", clientID)
	controlWriteTopic := fmt.Sprintf("redhat/insights/%s/control/out", clientID)
	dataReadTopic := fmt.Sprintf("redhat/insights/%s/data/in", clientID)
	fmt.Println("control consumer topic: ", controlReadTopic)

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(broker)
	connOpts.SetClientID(clientID)
	connOpts.SetTLSConfig(tlsconfig)
	connOpts.SetAutoReconnect(false)

	if username != "" {
		connOpts.SetUsername(username)
		connOpts.SetPassword(password)
	}

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

	tags["tag_key_1"] = "tag_value_1"
	dispatchers["dispatcher_1"] = map[string]string{
		"key_1": "value_1",
	}

	time.Sleep(1 * time.Second)

	publishConnectionStatusMessage(client, controlWriteTopic, qos, retained, generateUUID(), cf, dispatchers, tags, sentTime)

	onClientConnected(clientID)

	return clientID, client, nil
}

type onClientConnectedFunc func(string)
type onMessageReceivedFunc func(client MQTT.Client, message MQTT.Message)
type sendMessageToClientFunc func(string) (uuid.UUID, error)
type retrieveCredentialsFunc func() (string, string)

func printClientConnected() func(string) {
	return func(clientID string) {
		fmt.Println("CONNECTED ", clientID)
	}
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

func sendMessageToClientViaCloudConnector(cloudConnectorUrl string, identityHeader string) sendMessageToClientFunc {
	return func(clientID string) (uuid.UUID, error) {
		return sendMessageToClient(cloudConnectorUrl, clientID, identityHeader)
	}
}

func createRedisClient(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	fmt.Printf("%s\n", rdb.Ping(context.TODO()).String())

	return rdb
}

var initRedisConnectionRegistrar sync.Once
var connectedClientChan chan string

func registerClientConnectedWithRedis(redisClient *redis.Client) func(string) {

	initRedisConnectionRegistrar.Do(func() {
		fmt.Println("Starting go routine to register connections with redis")
		connectedClientChan = make(chan string)
		go func() {
			for clientId := range connectedClientChan {
				fmt.Printf("Register client %s with redis\n", clientId)

				notifyTestControllerOfNewConnection(redisClient, clientId)
			}
		}()
	})

	return func(clientID string) {
		fmt.Println("CONNECTED ", clientID)
		connectedClientChan <- clientID
	}
}

func notifyTestControllerOfNewConnection(rdb *redis.Client, clientId string) {
	event := ConnectionEvent{
		Event:    "connected",
		ClientId: clientId,
	}

	msgPayload, _ := json.Marshal(event)

	_, err := rdb.RPush(context.TODO(), "connected_client_list", msgPayload).Result()
	if err != nil {
		fmt.Printf("Error adding connected client id to list %s - err: %s\n", clientId, err)
	}
}

func registerMessageReceivedWithRedis(redisClient *redis.Client) func(MQTT.Client, MQTT.Message) {
	return func(client MQTT.Client, message MQTT.Message) {
		fmt.Printf("**** Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())
		fmt.Println("Registering message received with redis")
	}
}

func retrieveCredentialsFromRedis(redisClient *redis.Client) func() (string, string) {
	return func() (string, string) {
		fmt.Println("Retrieving creds from redis")
		u, p, err := retrieveUserFromRedis(redisClient)
		if err != nil {
			panic(err)
		}
		return u, p
	}
}

func retrieveFakeCredentials() func() (string, string) {
	return func() (string, string) {
		fmt.Println("Retrieving fake creds from nowhere...")
		return "", ""
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
