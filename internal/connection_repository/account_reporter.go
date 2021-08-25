package connection_repository

import (
	"context"
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func StartConnectedAccountReport(accountsToExcludeCmdLineArg string, processorFunc ConnectionCountProcessor) {

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

	ProcessConnectionCounts(
		context.TODO(),
		databaseConn,
		sqlTimeout,
		accountsToExclude,
		processorFunc)
}
