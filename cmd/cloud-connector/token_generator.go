package main

import (
	"context"
	"crypto/rsa"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller/api"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

func startCloudConnectorTokenGeneratorServer(mgmtAddr string) {

	logger.Log.Info("Starting Cloud-Connector token generator service")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	tokenSigningKey, err := loadPrivateKeyFromFile("newkey.pem") // FIXME:  make the file name configurable
	if err != nil {
		logger.LogFatalError("Unable to load token signing key: ", err)
	}

	apiMux := mux.NewRouter()
	apiMux.Use(request_id.ConfiguredRequestID("x-rh-insights-request-id"))

	tokenServiceUrlBasePath := cfg.UrlBasePath + "/auth"
	tokenServiceOpenApiSpecFilePath := "/tmp/apispec.json"

	apiSpecServer := api.NewApiSpecServer(apiMux, tokenServiceUrlBasePath, tokenServiceOpenApiSpecFilePath)
	apiSpecServer.Routes()

	monitoringServer := api.NewTokenGeneratorServer(apiMux, cfg.UrlBasePath, tokenSigningKey, cfg)
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

func loadPrivateKeyFromFile(pathToKey string) (*rsa.PrivateKey, error) {
	privateKeyFile := filepath.Clean(pathToKey)
	signBytes, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}

	return jwt.ParseRSAPrivateKeyFromPEM(signBytes)
}
