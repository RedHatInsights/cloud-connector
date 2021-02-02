package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func logFatalError(msg string, err error) {
	logger.Log.WithFields(logrus.Fields{"error": err}).Fatal(msg)
}

// mutual auth
// JWT header
//   from env
//   from file
// client id

func main() {
	var mgmtAddr = flag.String("mgmtAddr", ":8081", "Hostname:port of the management server")

	flag.Parse()

	logger.InitLogger()

	logger.Log.Info("Starting Cloud-Connector service")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	tlsConfigFuncs, err := buildBrokerTlsConfigFuncList(cfg)
	if err != nil {
		logFatalError("TLS configuration error for MQTT Broker connection", err)
	}

	tlsConfig, err := tls_utils.NewTlsConfig(tlsConfigFuncs...)
	if err != nil {
		logFatalError("Unable to configure TLS for MQTT Broker connection", err)
	}

	localConnectionManager := controller.NewLocalConnectionManager()

	accountResolver, err := controller.NewAccountIdResolver(cfg.ClientIdToAccountIdImpl, cfg)
	if err != nil {
		logFatalError("Failed to create Account ID Resolver", err)
	}

	connectedClientRecorder := &controller.InventoryBasedConnectedClientRecorder{}

	controlMsgHandler := mqtt.ControlMessageHandler(localConnectionManager, accountResolver, connectedClientRecorder)
	dataMsgHandler := mqtt.DataMessageHandler()

	defaultMsgHandler := mqtt.DefaultMessageHandler(controlMsgHandler, dataMsgHandler)

	subscribers := []mqtt.Subscriber{
		mqtt.Subscriber{
			Topic:      "redhat/insights/+/control/out",
			EntryPoint: controlMsgHandler,
			Qos:        1,
		},
		mqtt.Subscriber{
			Topic:      "redhat/insights/+/data/out",
			EntryPoint: dataMsgHandler,
			Qos:        0,
		},
	}

	err = mqtt.RegisterSubscribers(cfg.MqttBrokerAddress, tlsConfig, cfg, subscribers, defaultMsgHandler)
	if err != nil {
		logFatalError("Failed to connect to MQTT broker", err)
	}

	apiMux := mux.NewRouter()
	apiMux.Use(request_id.ConfiguredRequestID("x-rh-insights-request-id"))

	apiSpecServer := api.NewApiSpecServer(apiMux, cfg.UrlBasePath, cfg.OpenApiSpecFilePath)
	apiSpecServer.Routes()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	mgmtServer := api.NewManagementServer(localConnectionManager, apiMux, cfg.UrlBasePath, cfg)
	mgmtServer.Routes()

	jr := api.NewMessageReceiver(localConnectionManager, apiMux, cfg.UrlBasePath, cfg)
	jr.Routes()

	apiSrv := utils.StartHTTPServer(*mgmtAddr, "management", apiMux)

	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-signalChan
	logger.Log.Info("Received signal to shutdown: ", sig)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HttpShutdownTimeout)
	defer cancel()

	utils.ShutdownHTTPServer(ctx, "management", apiSrv)

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
