package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func buildApiServerMqttBrokerConfigFuncList(brokerUrl string, tlsConfig *tls.Config, cfg *config.Config) ([]mqtt.MqttClientOptionsFunc, error) {

	u, err := url.Parse(brokerUrl)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to determine protocol for the MQTT connection")
		return nil, err
	}

	brokerConfigFuncs := []mqtt.MqttClientOptionsFunc{}

	if tlsConfig != nil {
		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithTlsConfig(tlsConfig))
	}

	// FIXME:
	useHostnameAsClientId := false

	fmt.Println("\nFIXME!!!!  Set the client id to be the hostname??")
	if useHostnameAsClientId == true {
		hostname, err := os.Hostname()
		if err != nil {
			panic(err)
		}

		cfg.MqttClientId = hostname // FIXME:  hack
	}

	if cfg.MqttClientId != "" {
		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithClientID(cfg.MqttClientId))
	}

	// Gotta set the client id before handing the config object off to the jwt generator

	if u.Scheme == "wss" { //Rethink this check - jwt also works over TLS
		jwtGenerator, err := jwt_utils.NewJwtGenerator(cfg.MqttBrokerJwtGeneratorImpl, cfg)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to instantiate a JWT generator for the MQTT connection")
			return nil, err
		}
		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithJwtAsHttpHeader(jwtGenerator))
		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithJwtReconnectingHandler(jwtGenerator))
	}

	brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithCleanSession(cfg.MqttCleanSession))

	brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithResumeSubs(cfg.MqttResumeSubs))

	return brokerConfigFuncs, nil
}

func startCloudConnectorApiServer(mgmtAddr string) {

	logger.InitLogger()

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

	brokerOptions, err := buildApiServerMqttBrokerConfigFuncList(cfg.MqttBrokerAddress, tlsConfig, cfg)
	if err != nil {
		logger.LogFatalError("Unable to configure MQTT Broker connection", err)
	}

	connectedChan := make(chan struct{})

	mqttClient, err := mqtt.CreateBrokerConnection(cfg.MqttBrokerAddress,
		func(MQTT.Client) {
			fmt.Println("CONNECTED!!")
			connectedChan <- struct{}{}
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

	mqttTopicBuilder := mqtt.NewTopicBuilder(cfg.MqttTopicPrefix)

	proxyFactory, err := mqtt.NewReceptorMQTTProxyFactory(cfg, mqttClient, mqttTopicBuilder)
	if err != nil {
		logger.LogFatalError("Unable to create proxy factory", err)
	}

	sqlConnectionLocator, err := controller.NewSqlConnectionLocator(cfg, proxyFactory)
	if err != nil {
		logger.LogFatalError("Failed to create SQL Connection Locator", err)
	}

	apiMux := mux.NewRouter()
	apiMux.Use(request_id.ConfiguredRequestID("x-rh-insights-request-id"))

	apiSpecServer := api.NewApiSpecServer(apiMux, cfg.UrlBasePath, cfg.OpenApiSpecFilePath)
	apiSpecServer.Routes()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	mgmtServer := api.NewManagementServer(sqlConnectionLocator, apiMux, cfg.UrlBasePath, cfg)
	mgmtServer.Routes()

	jr := api.NewMessageReceiver(sqlConnectionLocator, apiMux, cfg.UrlBasePath, cfg)
	jr.Routes()

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
