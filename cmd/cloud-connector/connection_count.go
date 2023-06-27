package main

import (
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func startConnectionCount(mgmtAddr string) {

	logger.Log.Info("Starting Connection Count")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Unable to connect to database: ", err)
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM connections").Scan(&count)
	if err != nil {
		logger.LogFatalError("Error Executing the query: ", err)
	}

	logger.Log.Info("Connection Count:", count)
}
