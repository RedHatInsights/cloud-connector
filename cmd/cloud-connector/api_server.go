package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
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

func buildJwtGenerator(cfg *config.Config, mqttClientId string) (jwt_utils.JwtGenerator, error) {

	if cfg.MqttBrokerJwtGeneratorImpl == jwt_utils.RsaTokenGenerator {
		return jwt_utils.NewRSABasedJwtGenerator(cfg.JwtPrivateKeyFile, mqttClientId, cfg.JwtTokenExpiry)
	} else if cfg.MqttBrokerJwtGeneratorImpl == jwt_utils.FileTokenGenerator {
		return jwt_utils.NewFileBasedJwtGenerator(cfg.MqttBrokerJwtFile)
	} else {
		errorMsg := "Invalid JWT generator configured for the MQTT connection"
		logger.Log.Error(errorMsg)
		return nil, errors.New(errorMsg)
	}
}

func buildMqttClientId(cfg *config.Config) (string, error) {
	if cfg.MqttUseHostnameAsClientId == true {
		hostname, err := os.Hostname()
		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to determine hostname to use as client_id for MQTT connection")
			return "", err
		}

		return hostname, nil
	} else if cfg.MqttClientId != "" {
		return cfg.MqttClientId, nil
	} else {
		errorMsg := "Unable to determine what to use as the client_id for MQTT connection"
		logger.Log.Error(errorMsg)
		return "", errors.New(errorMsg)
	}
}

func buildDefaultMqttBrokerConfigFuncList(brokerUrl string, tlsConfig *tls.Config, cfg *config.Config) ([]mqtt.MqttClientOptionsFunc, error) {

	u, err := url.Parse(brokerUrl)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to determine protocol for the MQTT connection")
		return nil, err
	}

	brokerConfigFuncs := []mqtt.MqttClientOptionsFunc{}

	if tlsConfig != nil {
		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithTlsConfig(tlsConfig))
	}

	mqttClientId, err := buildMqttClientId(cfg)
	if err != nil {
		return nil, err
	}

	brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithClientID(mqttClientId))

	if u.Scheme == "wss" { //Rethink this check - jwt also works over TLS

		jwtGenerator, err := buildJwtGenerator(cfg, mqttClientId)

		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to instantiate a JWT generator for the MQTT connection")
			return nil, err
		}

		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithJwtAsHttpHeader(jwtGenerator))
		brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithJwtReconnectingHandler(jwtGenerator))
	}

	brokerConfigFuncs = append(brokerConfigFuncs, mqtt.WithProtocolVersion(4))

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

	brokerOptions, err := buildDefaultMqttBrokerConfigFuncList(cfg.MqttBrokerAddress, tlsConfig, cfg)
	if err != nil {
		logger.LogFatalError("Unable to configure MQTT Broker connection", err)
	}

	connectedChan := make(chan struct{})
	var initialConnection sync.Once

	mqttClient, err := mqtt.CreateBrokerConnection(cfg.MqttBrokerAddress,
		func(MQTT.Client) {
			logger.Log.Trace("Connected to MQTT broker")
			initialConnection.Do(func() {
				connectedChan <- struct{}{}
			})
		},
		brokerOptions...,
	)
	if err != nil {
		logger.LogFatalError("Unable to establish MQTT Broker connection", err)
	}

	select {
	case <-connectedChan:
		break
	case <-time.After(2 * time.Second):
		logger.Log.Fatal("Failed ot connect")
	}

	mqttTopicBuilder := mqtt.NewTopicBuilder(cfg.MqttTopicPrefix)

	proxyFactory, err := mqtt.NewReceptorMQTTProxyFactory(cfg, mqttClient, mqttTopicBuilder)
	if err != nil {
		logger.LogFatalError("Unable to create proxy factory", err)
	}

	sqlConnectionLocator, err := connection_repository.NewSqlConnectionLocator(cfg, proxyFactory)
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

	mqttClient.Disconnect(cfg.MqttDisconnectQuiesceTime)

	logger.Log.Info("Cloud-Connector shutting down")
}
