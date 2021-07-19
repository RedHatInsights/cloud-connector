package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func startConnectedAccountReport(accountsToExcludeCmdLineArg string) {

	logger.InitLogger()

	logger.Log.Info("Starting Cloud-Connector Connected Account Report")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	databaseConn, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Failed to connect to the database", err)
	}

	sqlTimeout := cfg.ConnectionDatabaseQueryTimeout

	accountsToExclude := strings.Split(accountsToExcludeCmdLineArg, ",")

	connection_repository.ProcessConnectionCounts(
		context.TODO(),
		databaseConn,
		sqlTimeout,
		accountsToExclude,
		stdoutConnectionCountProcessor)
}

func stdoutConnectionCountProcessor(ctx context.Context, account domain.AccountID, count int) error {
	fmt.Printf("%s - %d\n", account, count)
	return nil
}
