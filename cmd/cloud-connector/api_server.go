package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"
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

	logger.Log.Info("Starting Cloud-Connector service")

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
		logger.Log.Fatal("Failed to connect")
	}

	mqttTopicBuilder := mqtt.NewTopicBuilder(cfg.MqttTopicPrefix)

	proxyFactory, err := mqtt.NewConnectorClientMQTTProxyFactory(cfg, mqttClient, mqttTopicBuilder)
	if err != nil {
		logger.LogFatalError("Unable to create proxy factory", err)
	}

	sqlConnectionLocator, err := connection_repository.NewSqlConnectionLocator(cfg, database, proxyFactory)
	if err != nil {
		logger.LogFatalError("Failed to create SQL Connection Locator", err)
	}

	tenantTranslator, err := buildTenantTranslatorInstance(cfg)
	if err != nil {
		logger.LogFatalError("Unable to create tenant translator", err)
	}

	managementGetConnectionByOrgID, err := connection_repository.NewSqlGetConnectionByClientID(cfg, database)
	if err != nil {
		logger.LogFatalError("Unable to create getConnectionByClientID impl", err)
	}

	apiMux := mux.NewRouter()
	apiMux.Use(request_id.ConfiguredRequestID("x-rh-insights-request-id"))

	apiSpecServer := api.NewApiSpecServer(apiMux, cfg.UrlBasePath, cfg.OpenApiSpecFilePath)
	apiSpecServer.Routes()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	var v1ConnectionLocator connection_repository.ConnectionLocator
	var getConnectionFunction connection_repository.GetConnectionByClientID

	v1ConnectionLocator, getConnectionFunction = buildConnectionLookupInstances(cfg, database, proxyFactory)

	jr := api.NewMessageReceiver(v1ConnectionLocator, getConnectionFunction, tenantTranslator, apiMux, cfg.UrlBasePath, cfg)
	jr.Routes()

	getConnectionListByOrgIDFunction, err := connection_repository.NewSqlGetConnectionsByOrgID(cfg, database)
	if err != nil {
		logger.LogFatalError("Unable to create connection_repository.GetConnectionsByOrgID() function", err)
	}

	mgmtServer := api.NewManagementServer(sqlConnectionLocator, managementGetConnectionByOrgID, tenantTranslator, proxyFactory, apiMux, cfg.UrlBasePath, cfg)
	mgmtServer.Routes()

	connectionMediator := api.NewConnectionMediatorV2(getConnectionFunction, getConnectionListByOrgIDFunction, proxyFactory, apiMux, cfg.UrlBasePath, cfg)
	connectionMediator.Routes()

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

func buildConnectionLookupInstances(cfg *config.Config, database *sql.DB, proxyFactory controller.ConnectorClientProxyFactory) (connection_repository.ConnectionLocator, connection_repository.GetConnectionByClientID) {

	var v1ConnectionLocator connection_repository.ConnectionLocator
	var getConnectionFunction connection_repository.GetConnectionByClientID
	var err error

	if cfg.ApiServerConnectionLookupImpl == "relaxed" {
		logger.Log.Info("Using \"relaxed\" connection lookup mechanism")

		v1ConnectionLocator, err = connection_repository.NewPermittedTenantConnectionLocator(cfg, database, proxyFactory)
		if err != nil {
			logger.LogFatalError("Failed to create Permitted Account Connection Locator", err)
		}

		getConnectionFunction, err = connection_repository.NewPermittedTenantSqlGetConnectionByClientID(cfg, database)
		if err != nil {
			logger.LogFatalError("Unable to create connection_repository.GetConnection() function", err)
		}
	} else {

		logger.Log.Info("Using \"strict\" connection lookup mechanism")

		v1ConnectionLocator, err = connection_repository.NewSqlConnectionLocator(cfg, database, proxyFactory)
		if err != nil {
			logger.LogFatalError("Failed to create Permitted Account Connection Locator", err)
		}

		getConnectionFunction, err = connection_repository.NewSqlGetConnectionByClientID(cfg, database)
		if err != nil {
			logger.LogFatalError("Unable to create connection_repository.GetConnection() function", err)
		}
	}

	tenantTranslator, err := buildTenantTranslatorInstance(cfg)
	if err != nil {
		logger.LogFatalError("Unable to create tenant translator", err)
	}

	v1ConnectionLocator, err = connection_repository.NewTenantTranslatorDecorator(cfg, tenantTranslator, v1ConnectionLocator)
	if err != nil {
		logger.LogFatalError("Unable to create tenant translator decorator", err)
	}

	return v1ConnectionLocator, getConnectionFunction
}

func buildTenantTranslatorInstance(cfg *config.Config) (tenantid.Translator, error) {

	logger.Log.Infof("Using \"%s\" tenant translator impl", cfg.TenantTranslatorImpl)

	if cfg.TenantTranslatorImpl == "translator-service" {

		return tenantid.NewTranslator(cfg.TenantTranslatorURL, tenantid.WithTimeout(cfg.TenantTranslatorTimeout)), nil
	}

	if cfg.TenantTranslatorImpl == "mock" {

		mapping := buildTenantTranslatorMockMapping(cfg.TenantTranslatorMockMapping)

		return tenantid.NewTranslatorMockWithMapping(mapping), nil
	}

	logger.Log.Fatalf("Invalid tenant translator impl - %s", cfg.TenantTranslatorImpl)

	return nil, fmt.Errorf("Invalid tenant translator impl")
}

func buildTenantTranslatorMockMapping(mappingFromConfig map[string]interface{}) map[string]*string {
	mapping := make(map[string]*string)

	for k, v := range mappingFromConfig {
		value := v.(string)
		if value == "" {
			mapping[k] = nil
		} else {
			mapping[k] = &value
		}
	}

	return mapping
}
