package main

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func buildMessageHandlerMqttBrokerConfigFuncList(brokerUrl string, tlsConfig *tls.Config, cfg *config.Config) ([]mqtt.MqttClientOptionsFunc, error) {

	brokerConfigFuncs, err := buildDefaultMqttBrokerConfigFuncList(brokerUrl, tlsConfig, cfg)
	if err != nil {
		return nil, err
	}

	brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithCleanSession(cfg.MqttCleanSession))

	brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithResumeSubs(cfg.MqttResumeSubs))

	return brokerConfigFuncs, nil
}

func startMqttMessageConsumer(mgmtAddr string) {

	logger.Log.Info("Starting Cloud-Connector service")

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

	mqttTopicBuilder := mqtt.NewTopicBuilder(cfg.MqttTopicPrefix)
	mqttTopicVerifier := mqtt.NewTopicVerifier(cfg.MqttTopicPrefix)

	kafkaProducer, err := queue.StartProducer(buildRhcMessageKafkaProducerConfig(cfg))
	if err != nil {
		logger.LogFatalError("Unable to start kafka producer", err)
	}

	controlMsgHandler := mqtt.ControlMessageHandler(context.TODO(), kafkaProducer, mqttTopicVerifier)
	dataMsgHandler := mqtt.DataMessageHandler()

	defaultMsgHandler := mqtt.DefaultMessageHandler(mqttTopicVerifier, controlMsgHandler, dataMsgHandler)

	subscribers := []mqtt.Subscriber{
		mqtt.Subscriber{
			Topic:      mqttTopicBuilder.BuildIncomingWildcardControlTopic(),
			EntryPoint: defaultMsgHandler,
			Qos:        cfg.MqttControlSubscriptionQoS,
		},
		/*
			mqtt.Subscriber{
				Topic:      mqttTopicBuilder.BuildIncomingWildcardDataTopic(),
				EntryPoint: dataMsgHandler,
				Qos:        cfg.MqttDataSubscriptionQoS,
			},
		*/
	}

	brokerOptions, err := buildMessageHandlerMqttBrokerConfigFuncList(cfg.MqttBrokerAddress, tlsConfig, cfg)
	if err != nil {
		logger.LogFatalError("Unable to configure MQTT Broker connection", err)
	}

	mqttConnectionFailedChan := make(chan error)

	brokerOptions = buildOnConnectionLostMqttOptions(cfg, mqttConnectionFailedChan, brokerOptions)

	brokerOptions = append(brokerOptions, mqtt.WithOnConnectHandler(subscribeOnMqttConnectHandler(subscribers)))

	// Add a default publish message handler as some messages will get delivered before the topic
	// subscriptions are setup completely
	// See "Common Problems" here: https://github.com/eclipse/paho.mqtt.golang#common-problems
	brokerOptions = append(brokerOptions, mqtt.WithDefaultPublishHandler(defaultMsgHandler))

	mqttClient, err := mqtt.CreateBrokerConnection(cfg.MqttBrokerAddress, brokerOptions...)
	if err != nil {
		logger.LogFatalError("Failed to connect to MQTT broker", err)
	}

	apiMux := mux.NewRouter()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	apiSrv := utils.StartHTTPServer(mgmtAddr, "management", apiMux)

	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		logger.Log.Info("Received signal to shutdown: ", sig)
	case err = <-mqttConnectionFailedChan:
		logger.Log.Info("MQTT connection dropped: ", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HttpShutdownTimeout)
	defer cancel()

	utils.ShutdownHTTPServer(ctx, "management", apiSrv)

	mqttClient.Disconnect(cfg.MqttDisconnectQuiesceTime)

	kafkaProducer.Close()

	logger.FlushLogger()

	// This is kind of gross.  The idea here is to flush the logs and then
	// sleep for a bit to make sure the logs are sent to cloudwatch.
	time.Sleep(cfg.MqttConsumerShutdownSleepTime)

	logger.Log.Info("Cloud-Connector shutting down")
}

func buildBrokerTlsConfigFuncList(cfg *config.Config) ([]tls_utils.TlsConfigFunc, error) {

	tlsConfigFuncs := []tls_utils.TlsConfigFunc{}

	if cfg.MqttBrokerTlsCertFile != "" && cfg.MqttBrokerTlsKeyFile != "" {
		tlsConfigFuncs = append(tlsConfigFuncs, tls_utils.WithCert(cfg.MqttBrokerTlsCertFile, cfg.MqttBrokerTlsKeyFile))
	} else if cfg.MqttBrokerTlsCertFile != "" || cfg.MqttBrokerTlsKeyFile != "" {
		return tlsConfigFuncs, errors.New("Cert or key file specified without the other")
	}

	if cfg.MqttBrokerTlsCACertFile != "" {
		tlsConfigFuncs = append(tlsConfigFuncs, tls_utils.WithCACerts(cfg.MqttBrokerTlsCACertFile))
	}

	if cfg.MqttBrokerTlsSkipVerify == true {
		tlsConfigFuncs = append(tlsConfigFuncs, tls_utils.WithSkipVerify())
	}

	return tlsConfigFuncs, nil
}

func buildRhcMessageKafkaProducerConfig(cfg *config.Config) *queue.ProducerConfig {
	var kafkaSaslCfg *queue.SaslConfig

	if cfg.KafkaSASLMechanism != "" {
		kafkaSaslCfg = &queue.SaslConfig{
			SaslMechanism: cfg.KafkaSASLMechanism,
			SaslUsername:  cfg.KafkaUsername,
			SaslPassword:  cfg.KafkaPassword,
			KafkaCA:       cfg.KafkaCA,
		}
	}

	kafkaProducerCfg := &queue.ProducerConfig{
		Brokers:    cfg.RhcMessageKafkaBrokers,
		SaslConfig: kafkaSaslCfg,
		Topic:      cfg.RhcMessageKafkaTopic,
		BatchSize:  cfg.RhcMessageKafkaBatchSize,
		BatchBytes: cfg.RhcMessageKafkaBatchBytes,
		Balancer:   "hash",
	}

	return kafkaProducerCfg
}

func buildOnConnectionLostMqttOptions(cfg *config.Config, mqttConnectionFailedChan chan error, brokerOptions []mqtt.MqttClientOptionsFunc) []mqtt.MqttClientOptionsFunc {

	var autoReconnect = true
	var onConnectionLostHandler func(MQTT.Client, error)

	if cfg.ShutdownOnMqttConnectionLost {
		autoReconnect = false
		onConnectionLostHandler = notifyOnMqttConnectionLostHandler(mqttConnectionFailedChan)
	} else {
		autoReconnect = true
		onConnectionLostHandler = logMqttConnectionLostHandler
	}

	return append(brokerOptions, mqtt.WithAutoReconnect(autoReconnect), mqtt.WithConnectionLostHandler(onConnectionLostHandler))
}

func notifyOnMqttConnectionLostHandler(mqttConnectionFailedChan chan error) func(MQTT.Client, error) {
	return func(client MQTT.Client, err error) {
		logger.Log.Infof("MQTT connection dropped, err: %s", err)
		client.Disconnect(1000) // FIXME: If a connection is lost, do we really need to call disconnect??
		mqttConnectionFailedChan <- err
	}
}

func logMqttConnectionLostHandler(client MQTT.Client, err error) {
	logger.Log.Infof("MQTT connection dropped, err: %s", err)
}

func subscribeOnMqttConnectHandler(subscribers []mqtt.Subscriber) func(client MQTT.Client) {
	return func(client MQTT.Client) {
		for _, subscriber := range subscribers {
			logger.Log.Infof("Subscribing to MQTT topic: %s - QOS: %d\n", subscriber.Topic, subscriber.Qos)
			if token := client.Subscribe(subscriber.Topic, subscriber.Qos, subscriber.EntryPoint); token.Wait() && token.Error() != nil {
				logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Fatalf("Subscribing to MQTT topic (%s) failed", subscriber.Topic)
			}
		}
	}
}
