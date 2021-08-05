package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/cloud_connector"
	"github.com/RedHatInsights/cloud-connector/internal/config"
    "github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

func startKafkaMessageConsumer(mgmtAddr string) {

	logger.InitLogger()

	logger.Log.Info("Starting Cloud-Connector Kafka Message consumer")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	tlsConfigFuncs, err := buildBrokerTlsConfigFuncList(cfg)
	if err != nil {
		logger.LogFatalError("TLS configuration error for MQTT Broker connection", err)
	}

	tlsConfig, err := tls_utils.NewTlsConfig(tlsConfigFuncs...)
	if err != nil {
		logger.LogFatalError("Unable to configure TLS for MQTT Broker connection", err)
	}

	connectionRegistrar, err := connection_repository.NewSqlConnectionRegistrar(cfg)
	if err != nil {
		logger.LogFatalError("Failed to create SQL Connection Registrar", err)
	}

	accountResolver, err := controller.NewAccountIdResolver(cfg.ClientIdToAccountIdImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Account ID Resolver", err)
	}

	connectedClientRecorder, err := controller.NewConnectedClientRecorder(cfg.ConnectedClientRecorderImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Connected Client Recorder", err)
	}

	sourcesRecorder, err := controller.NewSourcesRecorder(cfg.SourcesRecorderImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Sources Recorder", err)
	}

	mqttTopicBuilder := mqtt.NewTopicBuilder(cfg.MqttTopicPrefix)
	mqttTopicVerifier := mqtt.NewTopicVerifier(cfg.MqttTopicPrefix)

	// FIXME:
	//start kafka consumer
	rhcMessageKafkaConsumer := queue.ConsumerConfig{
		Brokers: cfg.InventoryKafkaBrokers,                // FIXME:
		Topic:   "platform.cloud-connector.mqtt_messages", // FIXME: configurable
		GroupID: "cloud-connector-rhc-message-consumer",   // FIXME:
	}
	kafkaReader := queue.StartConsumer(&rhcMessageKafkaConsumer)

	brokerOptions, err := buildDefaultMqttBrokerConfigFuncList(cfg.MqttBrokerAddress, tlsConfig, cfg)
	if err != nil {
		logger.LogFatalError("Unable to configure MQTT Broker connection", err)
	}

	connectedChan := make(chan struct{})
	var initialConnection sync.Once

	mqttClient, err := mqtt.CreateBrokerConnection(cfg.MqttBrokerAddress,
		func(MQTT.Client) {
			fmt.Println("CONNECTED!!")
			initialConnection.Do(func() {
				connectedChan <- struct{}{}
			})
			fmt.Println("LEAVING CONNECTED!!")
		},
		brokerOptions...,
	)
	if err != nil {
		logger.LogFatalError("Unable to establish MQTT Broker connection", err)
	}

	select {
	case <-connectedChan:
		fmt.Println("CONNECTED!!")
		break
	case <-time.After(2 * time.Second):
		logger.Log.Fatal("Failed ot connect")
	}

	messageProcessor := handleMessage(
		cfg,
		mqttClient,
		mqttTopicVerifier,
		mqttTopicBuilder,
		connectionRegistrar,
		accountResolver,
		connectedClientRecorder,
		sourcesRecorder)

	go consumeMqttMessagesFromKafka(kafkaReader, messageProcessor)

	apiMux := mux.NewRouter()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	apiSrv := utils.StartHTTPServer(mgmtAddr, "management", apiMux)

	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	logger.Log.Info("Received signal to shutdown: ", sig)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HttpShutdownTimeout)
	defer cancel()

	utils.ShutdownHTTPServer(ctx, "management", apiSrv)

	logger.Log.Info("Cloud-Connector shutting down")
}

func handleMessage(cfg *config.Config, mqttClient MQTT.Client, topicVerifier *mqtt.TopicVerifier, topicBuilder *mqtt.TopicBuilder, connectionRegistrar connection_repository.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) func(*kafka.Message) error {

	handler := cloud_connector.HandleControlMessage(
		cfg,
		mqttClient,
		topicBuilder,
		connectionRegistrar,
		accountResolver,
		connectedClientRecorder,
		sourcesRecorder)

	return func(msg *kafka.Message) error {

		// FIXME: move all this ugly header handling logic to a helper method

		fmt.Printf("%% Message %s\n", string(msg.Value))
		if msg.Headers == nil {
			return errors.New("FIXME: no headers in kafka message!!")
		}

		var topic string
		var mqttMessageID string

		for _, header := range msg.Headers {
			if header.Key == "topic" {
				topic = string(header.Value)
			} else if header.Key == "mqtt_message_id" {
				mqttMessageID = string(header.Value)
			}
		}

		logger := logger.Log.WithFields(logrus.Fields{"mqtt_message_id": mqttMessageID})

		logger.Debug("Read message off of kafka topic")

		if len(topic) == 0 {
			logger.Debug("Unable to process message.  Message does not have topic header")
			return nil
		}

		payload := string(msg.Value)

		logger.Debugf("Received control message on topic: %s\nMessage: %s\n", topic, payload)

		topicType, clientID, err := topicVerifier.VerifyIncomingTopic(topic)

		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Debug("Error during topic parsing")
		}

		if topicType != mqtt.ControlTopicType {
			logger.Debug("Invalid topic type read from kafka.  Skipping message...")
			return nil
		}

		handler(mqttClient, clientID, payload)

		return nil
	}
}

func consumeMqttMessagesFromKafka(kafkaReader *kafka.Reader, process func(*kafka.Message) error) {

	for {
		m, err := kafkaReader.ReadMessage(context.Background())
		if err != nil {
			logger.LogError("Failed to read message from kafka", err)
			break
		}
		logger.Log.Infof("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))

		process(&m)
	}

	logger.Log.Infof("Stopped reading kafka messages")

	if err := kafkaReader.Close(); err != nil {
		logger.LogError("Failed to close kafka reader", err)
	}
}
