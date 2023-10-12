package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/cloud_connector"
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

func startKafkaMessageConsumer(mgmtAddr string) {

	logger.Log.Info("Starting Cloud-Connector Kafka Message consumer")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Unable to connect to database: ", err)
	}

	tlsConfigFuncs, err := buildBrokerTlsConfigFuncList(cfg)
	if err != nil {
		logger.LogFatalError("TLS configuration error for MQTT Broker connection", err)
	}

	tlsConfig, err := tls_utils.NewTlsConfig(tlsConfigFuncs...)
	if err != nil {
		logger.LogFatalError("Unable to configure TLS for MQTT Broker connection", err)
	}

	connectionRegistrar, err := connection_repository.NewSqlConnectionRegistrar(cfg, database)
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

	kafkaReader, err := queue.StartConsumer(buildRhcMessageKafkaConsumerConfig(cfg))
	if err != nil {
		logger.LogFatalError("Unable to start kafka consumer", err)
	}

	brokerOptions, err := buildDefaultMqttBrokerConfigFuncList(cfg.MqttBrokerAddress, tlsConfig, cfg)
	if err != nil {
		logger.LogFatalError("Unable to configure MQTT Broker connection", err)
	}

	connectedChan := make(chan struct{})
	brokerOptions = append(brokerOptions, mqtt.WithOnConnectHandler(notifyOnIntialMqttConnection(connectedChan)))

	mqttConnectionFailedChan := make(chan error)
	brokerOptions = buildOnConnectionLostMqttOptions(cfg, mqttConnectionFailedChan, brokerOptions)

	mqttClient, err := mqtt.CreateBrokerConnection(cfg.MqttBrokerAddress, brokerOptions...)
	if err != nil {
		logger.LogFatalError("Unable to establish MQTT broker connection", err)
	}

	select {
	case <-connectedChan:
		logger.Log.Debug("Successfully connected to MQTT broker")
		break
	case <-time.After(2 * time.Second):
		logger.Log.Fatal("Failed to connect to MQTT broker")
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

	shutdownCtx, shutdownCtxCancel := context.WithCancel(context.Background())
	// If the kafka consumer runs into a fatal error, notify the
	// main thread so that it can shutdown the process
	fatalProcessingError := make(chan struct{})

	go consumeMqttMessagesFromKafka(kafkaReader, messageProcessor, shutdownCtx, fatalProcessingError)

	apiMux := mux.NewRouter()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	apiSrv := utils.StartHTTPServer(mgmtAddr, "management", apiMux)

	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		logger.Log.Info("Received signal to shutdown: ", sig)
		shutdownCtxCancel() // Notify the consumer to shutdown
	case <-fatalProcessingError:
		logger.Log.Info("Received a fatal processing error...shutting down!")
	case err = <-mqttConnectionFailedChan:
		logger.Log.Info("MQTT connection dropped: ", err)
		shutdownCtxCancel() // Notify the consumer to shutdown
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HttpShutdownTimeout)
	defer cancel()

	utils.ShutdownHTTPServer(ctx, "management", apiSrv)

	mqttClient.Disconnect(cfg.MqttDisconnectQuiesceTime)

	logger.Log.Info("Cloud-Connector shutting down")
}

func getHeaderValueAsString(headers []kafka.Header, headerName string) string {

	for _, header := range headers {
		if header.Key == headerName {
			return string(header.Value)
		}
	}

	return ""
}

func handleMessage(cfg *config.Config, mqttClient MQTT.Client, topicVerifier *mqtt.TopicVerifier, topicBuilder *mqtt.TopicBuilder, connectionRegistrar connection_repository.ConnectionRegistrar, accountResolver controller.AccountIdResolver, connectedClientRecorder controller.ConnectedClientRecorder, sourcesRecorder controller.SourcesRecorder) func(*kafka.Message) error {

	controlMessageHandler := cloud_connector.HandleControlMessage(
		cfg,
		mqttClient,
		topicBuilder,
		connectionRegistrar,
		accountResolver,
		connectedClientRecorder,
		sourcesRecorder)

	return func(msg *kafka.Message) error {

		logger.Log.Tracef("%% Message %s\n", string(msg.Value))

		if msg.Headers == nil {
			logger.Log.Debug("Unable to process message.  Message does not have headers!")
			return nil
		}

		topic := getHeaderValueAsString(msg.Headers, mqtt.TopicKafkaHeaderKey)
		mqttMessageID := getHeaderValueAsString(msg.Headers, mqtt.MessageIDKafkaHeaderKey)
		dateReceived := getHeaderValueAsString(msg.Headers, mqtt.DateReceivedHeaderKey)

		logger := logger.Log.WithFields(logrus.Fields{"mqtt_message_id": mqttMessageID,
			"client_id":     string(msg.Key),
			"date_received": dateReceived})

		logger.Debug("Read message off of kafka topic")

		if len(topic) == 0 {
			logger.Debug("Unable to process message.  Message does not have topic header!")
			return nil
		}

		payload := string(msg.Value)

		logger.Debugf("Received control message on topic: %s\nMessage: %s\n", topic, payload)

		topicType, clientID, err := topicVerifier.VerifyIncomingTopic(topic)

		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Debug("Unable to process message.  Unable to parse topic!")
			return nil
		}

		if topicType != mqtt.ControlTopicType {
			logger.Debug("Invalid topic type read from kafka.  Skipping message...")
			return nil
		}

		return controlMessageHandler(mqttClient, clientID, payload)
	}
}

func consumeMqttMessagesFromKafka(kafkaReader *kafka.Reader, process func(*kafka.Message) error, ctx context.Context, fatalProcessingError chan struct{}) {

	for {
		m, err := kafkaReader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) != true {
				logger.LogError("Failed to fetch message from kafka", err)
				// Notify the main thread to shutdown
				fatalProcessingError <- struct{}{}
			}
			break
		}

		logger.Log.Tracef("message from partition %d at offset %d: %s = %s\n", m.Partition, m.Offset, string(m.Key), string(m.Value))

		metrics.kafkaMessageReceivedCounter.Inc()

		err = process(&m)
		if err != nil {
			logger.LogError("Error handling message:", err)
			// Notify the main thread to shutdown
			fatalProcessingError <- struct{}{}
			break
		}

		// explicitly commit the message
		err = kafkaReader.CommitMessages(ctx, m)
		if err != nil {
			logger.LogError("Failed to commit message to kafka", err)
			// Notify the main thread to shutdown
			fatalProcessingError <- struct{}{}
			break
		}
	}

	logger.Log.Infof("Stopped reading kafka messages")

	if err := kafkaReader.Close(); err != nil {
		logger.LogError("Failed to close kafka reader", err)
	}
}

func buildRhcMessageKafkaConsumerConfig(cfg *config.Config) *queue.ConsumerConfig {
	var kafkaSaslCfg *queue.SaslConfig

	if cfg.KafkaSASLMechanism != "" {
		kafkaSaslCfg = &queue.SaslConfig{
			SaslMechanism: cfg.KafkaSASLMechanism,
			SaslUsername:  cfg.KafkaUsername,
			SaslPassword:  cfg.KafkaPassword,
			KafkaCA:       cfg.KafkaCA,
		}
	}

	rhcMessageKafkaConsumer := queue.ConsumerConfig{
		Brokers:    cfg.RhcMessageKafkaBrokers,
		SaslConfig: kafkaSaslCfg,
		Topic:      cfg.RhcMessageKafkaTopic,
		GroupID:    cfg.RhcMessageKafkaConsumerGroup,
	}

	return &rhcMessageKafkaConsumer
}

type mqttMetrics struct {
	kafkaMessageReceivedCounter prometheus.Counter
}

func newMqttMetrics() *mqttMetrics {
	metrics := new(mqttMetrics)

	metrics.kafkaMessageReceivedCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloud_connector_kafka_message_received_count",
		Help: "The number of kafka messages received",
	})

	return metrics
}

var (
	metrics = newMqttMetrics()
)
