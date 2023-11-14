package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
)

func startCloudConnectorTokenGeneratorServer(mgmtAddr string) {

	logger.Log.Info("Starting Cloud-Connector token generator service")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	apiMux := mux.NewRouter()
	apiMux.Use(request_id.ConfiguredRequestID("x-rh-insights-request-id"))

	/*
		apiSpecServer := api.NewApiSpecServer(apiMux, cfg.UrlBasePath, cfg.OpenApiSpecFilePath)
		apiSpecServer.Routes()
	*/

	monitoringServer := api.NewTokenGeneratorServer(apiMux, cfg.UrlBasePath, cfg)
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
