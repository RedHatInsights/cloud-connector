package main

import (
	"context"
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
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
)

func verifyConfiguration(cfg *config.Config) error {
	return nil
}

func main() {
	var mgmtAddr = flag.String("mgmtAddr", ":8081", "Hostname:port of the management server")
	var broker = flag.String("broker", "ssl://localhost:8883", "uri of broker")
	var certFile = flag.String("cert", "connector-service-cert.pem", "path to cert file")
	var keyFile = flag.String("key", "connector-service-key.pem", "path to key file")

	flag.Parse()

	logger.InitLogger()

	logger.Log.Info("Starting Receptor-Controller Job-Receiver service")

	cfg := config.GetConfig()
	logger.Log.Info("Receptor Controller configuration:\n", cfg)

	err := verifyConfiguration(cfg)
	if err != nil {
		logger.Log.Fatal("Configuration error encountered during startup: ", err)
	}

	localConnectionManager := controller.NewLocalConnectionManager()
	//accountResolver := &controller.BOPAccountIdResolver{}
	accountResolver := &controller.ConfigurableAccountIdResolver{}
	connectedClientRecorder := &controller.InventoryBasedConnectedClientRecorder{}

	err = mqtt.NewConnectionRegistrar(*broker, *certFile, *keyFile, localConnectionManager, accountResolver, connectedClientRecorder)
	if err != nil {
		logger.Log.Fatal("Failed to connect to MQTT broker: ", err)
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

	logger.Log.Info("Receptor-Controller shutting down")
}
