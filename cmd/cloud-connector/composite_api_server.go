package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/gorilla/mux"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

func startCompositeCloudConnectorApiServer(mgmtAddr string) {

	logger.Log.Info("Starting Cloud-Connector Composite API service")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	// FIXME:  Make this configurable
	urls := []string{"http://cloud-connector-api:10000", "http://cloud-connector-aws-api:10000"}

	fmt.Println("PASS IN THE SHARED CONNECTION STATE CACHE")
	proxyFactory, err := api.NewConnectorClientHTTPProxyFactory(cfg, cache)
	if err != nil {
		logger.LogFatalError("Unable to create proxy factory", err)
	}

	tenantTranslator, err := buildTenantTranslatorInstance(cfg)
	if err != nil {
		logger.LogFatalError("Unable to create tenant translator", err)
	}

	apiMux := mux.NewRouter()
	apiMux.Use(request_id.ConfiguredRequestID("x-rh-insights-request-id"))

	apiSpecServer := api.NewApiSpecServer(apiMux, cfg.UrlBasePath, cfg.OpenApiSpecFilePath)
	apiSpecServer.Routes()

	monitoringServer := api.NewMonitoringServer(apiMux, cfg)
	monitoringServer.Routes()

	var getConnectionFunction connection_repository.GetConnectionByClientID
	getConnectionFunction, err = connection_repository.NewCompositeGetConnectionByClientID(cfg, urls, cache)

	if err != nil {
		logger.LogFatalError("Unable to create connection_repository.GetConnection() function", err)
	}

	jr := api.NewMessageReceiver(getConnectionFunction, tenantTranslator, proxyFactory, apiMux, cfg.UrlBasePath, cfg)
	jr.Routes()

	getConnectionListByOrgIDFunction, err := connection_repository.NewCompositeGetConnectionsByOrgID(cfg)
	if err != nil {
		logger.LogFatalError("Unable to create connection_repository.GetConnectionsByOrgID() function", err)
	}

	connectionMediator := api.NewConnectionMediatorV2(getConnectionFunction, getConnectionListByOrgIDFunction, proxyFactory, apiMux, cfg.UrlBasePath, cfg)
	connectionMediator.Routes()

	apiSrv := utils.StartHTTPServer(mgmtAddr, "management", apiMux)

	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalChan:
		logger.Log.Info("Received signal to shutdown: ", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HttpShutdownTimeout)
	defer cancel()

	utils.ShutdownHTTPServer(ctx, "management", apiSrv)

	logger.Log.Info("Cloud-Connector shutting down")
}
