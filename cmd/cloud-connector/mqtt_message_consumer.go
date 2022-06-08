package main

import (
	"context"
	"crypto/tls"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"
	"github.com/confluentinc/confluent-kafka-go/kafka"

	"github.com/gorilla/mux"
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

	kafkaProducerCfg := &kafka.ConfigMap{
		"bootstrap.servers":  strings.Join(cfg.RhcMessageKafkaBrokers, ","),
		"batch.num.messages": cfg.RhcMessageKafkaBatchSize,
		"batch.size":         cfg.RhcMessageKafkaBatchBytes,
	}

	kafkaProducer := queue.StartProducer(kafkaProducerCfg)

	controlMsgHandler := mqtt.ControlMessageHandler(context.TODO(), kafkaProducer, mqttTopicVerifier)
	dataMsgHandler := mqtt.DataMessageHandler()

	defaultMsgHandler := mqtt.DefaultMessageHandler(mqttTopicVerifier, controlMsgHandler, dataMsgHandler)

	subscribers := []mqtt.Subscriber{
		mqtt.Subscriber{
			Topic:      mqttTopicBuilder.BuildIncomingWildcardControlTopic(),
			EntryPoint: controlMsgHandler,
			Qos:        cfg.MqttControlSubscriptionQoS,
		},
		mqtt.Subscriber{
			Topic:      mqttTopicBuilder.BuildIncomingWildcardDataTopic(),
			EntryPoint: dataMsgHandler,
			Qos:        cfg.MqttDataSubscriptionQoS,
		},
	}

	brokerOptions, err := buildMessageHandlerMqttBrokerConfigFuncList(cfg.MqttBrokerAddress, tlsConfig, cfg)
	if err != nil {
		logger.LogFatalError("Unable to configure MQTT Broker connection", err)
	}

	mqttClient, err := mqtt.RegisterSubscribers(cfg.MqttBrokerAddress, subscribers, defaultMsgHandler, brokerOptions...)
	if err != nil {
		logger.LogFatalError("Failed to connect to MQTT broker", err)
	}

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

	mqttClient.Disconnect(cfg.MqttDisconnectQuiesceTime)

	kafkaProducer.Close()

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
