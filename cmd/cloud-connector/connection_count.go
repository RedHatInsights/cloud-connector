package main

import (
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	connectionCountMetric = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cloud_connector_connection_count",
			Help: "Number of connections",
		},
	)
)

func startConnectionCount(mgmtAddr string) {

	logger.Log.Info("Starting Connection Count")

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Unable to connect to database:", err)
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM connections").Scan(&count)
	if err != nil {
		logger.LogFatalError("Error Executing the query:", err)
	}

	connectionCountMetric.Add(float64(count))

	if err := push.New(cfg.PrometheusPushGateway, "cloud_connector").
		Collector(connectionCountMetric).
		Push(); err != nil {
		logger.LogFatalError("Error pushing metric to the Push Gateway:", err)
	}

	logger.Log.Info("Connection Count:", count)
	logger.Log.Info("Metric pushed to the Push Gateway successfully")
}
