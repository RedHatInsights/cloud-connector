package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	Connector "github.com/RedHatInsights/cloud-connector/internal/mqtt"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

func NewTLSConfig(certFile string, keyFile string) (*tls.Config, string) {
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
	flag.Parse()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	for i := 0; i < *connectionCount; i++ {
		go startProducer(*certFile, *keyFile, *broker, i)
	}

	<-c
	fmt.Println("Client going down...disconnecting from mqtt uncleanly")
}

var m MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	fmt.Printf("default handler rec TOPIC: %s MSG:%s\n", msg.Topic(), msg.Payload())
}

func startProducer(certFile string, keyFile string, broker string, i int) {
	tlsconfig, clientID := NewTLSConfig(certFile, keyFile)

	controlReadTopic := fmt.Sprintf("redhat/insights/%s/control/in", clientID)
	controlWriteTopic := fmt.Sprintf("redhat/insights/%s/control/out", clientID)
	dataReadTopic := fmt.Sprintf("redhat/insights/%s/data/in", clientID)
	fmt.Println("control consumer topic: ", controlReadTopic)

	connOpts := MQTT.NewClientOptions()
	connOpts.AddBroker(broker)
	connOpts.SetClientID(clientID)
	connOpts.SetTLSConfig(tlsconfig)

	connectionStatusMsgPayload := Connector.ConnectionStatusMessageContent{ConnectionState: "offline"}
	lastWillMsg := Connector.ControlMessage{
		MessageType: "connection-status",
		MessageID:   "5678",
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

	cf := Connector.CanonicalFacts{
		InsightsID:            generateUUID(),
		MachineID:             generateUUID(),
		BiosID:                generateUUID(),
		SubscriptionManagerID: generateUUID(),
		SatelliteID:           generateUUID(),
		IpAddresses:           []string{"192.168.68.101"},
		MacAddresses:          []string{"54.54.45.45.62.26"},
		Fqdn:                  "fred.flintstone.com",
	}

	dispatchers := make(Connector.Dispatchers)
	dispatchers["playbook"] = make(map[string]string)
	dispatchers["playbook"]["ansible-runner-version"] = "1.2.3"
	dispatchers["echo"] = make(map[string]string)

	dispatchers["catalog"] = make(map[string]string)
	dispatchers["catalog"]["ApplicationType"] = "/insights/platform/catalog"
	dispatchers["catalog"]["SourceRef"] = "df2bac3e-c7b4-4a8b-8226-b943b9a12eaf"
	dispatchers["catalog"]["SrcName"] = "dehort Testing Bulk Create"
	dispatchers["catalog"]["SrcType"] = "ansible-tower"
	dispatchers["catalog"]["WorkerBuild"] = "2021-02-19 10:18:24"
	dispatchers["catalog"]["WorkerSHA"] = "48d28791e3b59f7334d2671c07978113b0d40374"
	dispatchers["catalog"]["WorkerVersion"] = "v0.1.0"

	connectionStatusPayload := Connector.ConnectionStatusMessageContent{
		CanonicalFacts:  cf,
		Dispatchers:     dispatchers,
		ConnectionState: "online"}
	connMsg := Connector.ControlMessage{
		MessageType: "connection-status",
		MessageID:   "1234",
		Version:     1,
		Sent:        time.Now(),
		Content:     connectionStatusPayload,
	}

	payload, err = json.Marshal(connMsg)

	if err != nil {
		fmt.Println("marshal of message failed, err:", err)
		panic(err)
	}

	fmt.Println("publishing to topic:", controlWriteTopic)
	fmt.Println("retained: ", retained)
	fmt.Println("qos: ", qos)
	client.Publish(controlWriteTopic, qos, retained, payload)
	fmt.Printf("Published message %s... Sleeping...\n", payload)
}

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	fmt.Printf("Received message on topic: %s\nMessage: %s\n", message.Topic(), message.Payload())

	var connMsg Connector.ControlMessage

	if message.Payload() == nil || len(message.Payload()) == 0 {
		fmt.Println("empty payload")
		return
	}

	if err := json.Unmarshal(message.Payload(), &connMsg); err != nil {
		fmt.Println("unmarshal of message failed, err:", err)
		panic(err)
	}

	fmt.Println("Got a message:", connMsg)

	switch connMsg.MessageType {
	case "command":
		fmt.Println("Got a command message")
		commandPayload := connMsg.Content.(map[string]interface{})

		if commandPayload["command"] == "ping" {
			fmt.Println("Got a ping command")

			messageID, _ := uuid.NewRandom()

			pongMessage := Connector.EventMessage{
				MessageType: "event",
				MessageID:   messageID.String(),
				Version:     1,
				Sent:        time.Now(),
				Content:     "pong",
			}

			messageBytes, err := json.Marshal(pongMessage)
			if err != nil {
				fmt.Println("ERROR marshalling pong message: ", err)
				return
			}

			topic := "redhat/insights/client-1/control/out"
			fmt.Println("sending pong response on ", topic)
			t := client.Publish(topic, byte(0), false, messageBytes)
			go func() {
				_ = t.Wait() // Can also use '<-t.Done()' in releases > 1.2.0
				if t.Error() != nil {
					fmt.Println("public error:", t.Error())
				}
			}()
		}

	default:
		fmt.Println("Invalid message type!")
	}
}

func buildDisconnectMessage(clientID string) ([]byte, error) {
	connMsg := Connector.ControlMessage{
		MessageType: "disconnect",
		MessageID:   "4321",
		Version:     1,
	}

	return json.Marshal(connMsg)
}

func generateUUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
}
