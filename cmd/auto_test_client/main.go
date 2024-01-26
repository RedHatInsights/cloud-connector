package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
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

func main() {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	/*
	   MQTT.ERROR = logger
	   MQTT.CRITICAL = logger
	   MQTT.WARN = logger
	*/
	MQTT.DEBUG = logger

	connectionCount := flag.Int("connection_count", 1, "number of connections to create")
	broker := flag.String("broker", "tcp://eclipse-mosquitto:1883", "hostname / port of broker")
	certFile := flag.String("cert", "cert.pem", "path to cert file")
	keyFile := flag.String("key", "key.pem", "path to key file")
	cloudConnectorUrl := flag.String("cloud-connector", "http://localhost:10000", "protocol / hostname / port of cloud-connector")
	orgId := flag.String("org-id", "10001", "org-id")
	accountNumber := flag.String("account", "010101", "account number")
	flag.Parse()

	identityHeader := buildIdentityHeader(*orgId, *accountNumber)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	for i := 0; i < *connectionCount; i++ {
		go startTestClient(*certFile, *keyFile, *broker, *cloudConnectorUrl, i, identityHeader)
	}

	<-c
	fmt.Println("Client going down...disconnecting from mqtt uncleanly")
}

var m MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("default handler rec TOPIC: %s MSG:%s\n", msg.Topic(), msg.Payload())
}

func startTestClient(certFile string, keyFile string, broker string, cloudConnectorUrl string, i int, identityHeader string) {
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
	tlsconfig, clientID := NewTLSConfig(certFile, keyFile)

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

	return clientID, client, nil
}

func publishConnectionStatusMessage(client MQTT.Client, topic string, qos byte, retained bool, messageID string, cf Connector.CanonicalFacts, dispatchers Connector.Dispatchers, tags Connector.Tags, sentTime time.Time) {
	fmt.Println("sentTime: ", sentTime)

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

	fmt.Println("publishing to topic:", topic)
	fmt.Println("retained: ", retained)
	fmt.Println("qos: ", qos)

	client.Publish(topic, qos, retained, payload)
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

		fmt.Println("Got a message:", dataMsg)

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

func verifyClientIsRegistered(cloudConnectorUrl string, clientId string, identityHeader string) {
	fmt.Printf("Verifying client (%s) is registered with cloud-connector!!", clientId)
    status := getClientStatusFromCloudConnector(cloudConnectorUrl, clientId, identityHeader)
    if status != "connected" {
        fmt.Println("***  ERROR:  status should have been connected")
    }
}

func verifyClientIsUnregistered(cloudConnectorUrl string, clientId string, identityHeader string) {
	fmt.Printf("Verifying client (%s) is unregistered with cloud-connector!!", clientId)
    status := getClientStatusFromCloudConnector(cloudConnectorUrl, clientId, identityHeader)
    if status != "disconnected" {
        fmt.Println("***  ERROR:  status should have been disconnected")
    }
}

func getClientStatusFromCloudConnector(cloudConnectorUrl string, clientId string, identityHeader string) string {
	type connectionStatus struct {
		Status string `json:"status"`
	}

	url := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/status", cloudConnectorUrl, clientId)

	req, err := http.NewRequest(http.MethodGet, url, nil)

	req.Close = true

	requestId := fmt.Sprint(time.Now().Unix())

	req.Header.Set("x-rh-identity", identityHeader)
	req.Header.Set("x-rh-insights-request-id", requestId)

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("res: ", res)
		fmt.Println("err: ", err)
		time.Sleep(2 * time.Second)
		panic(err)
	}

	connStatus := connectionStatus{}
	json.NewDecoder(res.Body).Decode(&connStatus)

	fmt.Println("Connection Status: ", connStatus.Status)
	return connStatus.Status
}

func sendMessageToClient(cloudConnectorUrl string, clientId string, identityHeader string) (uuid.UUID, error) {
	fmt.Printf("Sending message to client (%s)!!", clientId)

	jsonBody := []byte(`{"directive": "imadirective", "payload": "imapayload"}`)
	bodyReader := bytes.NewReader(jsonBody)

	type sendMessageResponse struct {
		Id string `json:"id"`
	}

	url := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/message", cloudConnectorUrl, clientId)

	req, err := http.NewRequest(http.MethodPost, url, bodyReader)

	req.Close = true

	requestId := fmt.Sprint(time.Now().Unix())

	fmt.Println("request id:", requestId)

	req.Header.Set("x-rh-identity", identityHeader)
	req.Header.Set("x-rh-insights-request-id", requestId)

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("res: ", res)
		fmt.Println("err: ", err)
		time.Sleep(2 * time.Second)

		panic(err)
	}

	sendMsgResponse := sendMessageResponse{}
	json.NewDecoder(res.Body).Decode(&sendMsgResponse)

	fmt.Println("Message response: ", sendMsgResponse)

	return uuid.Parse(sendMsgResponse.Id)
}

func generateUUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
}
