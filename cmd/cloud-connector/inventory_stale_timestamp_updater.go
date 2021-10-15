package main

import (
	"context"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"

	"github.com/sirupsen/logrus"
)

const serviceName = "Cloud-Connector Inventory Stale Timestamp Updater"

func calculateStaleCutoffTime(staleTimeOffset time.Duration) time.Time {
	windowToUpdateBeforeGoingStale := 1 * time.Hour
	offset := staleTimeOffset - windowToUpdateBeforeGoingStale
	tooOldIfBeforeThisTime := time.Now().Add(-1 * offset)
	return tooOldIfBeforeThisTime
}

func startInventoryStaleTimestampUpdater() {

	logger.InitLogger()

	logger.Log.Info("Starting ", serviceName)

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	databaseConn, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Failed to connect to the database", err)
	}

	accountResolver, err := controller.NewAccountIdResolver(cfg.ClientIdToAccountIdImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Account ID Resolver", err)
	}

	kafkaProducerCfg := &queue.ProducerConfig{
		Brokers:    cfg.InventoryKafkaBrokers,
		Topic:      cfg.InventoryKafkaTopic,
		BatchSize:  cfg.InventoryKafkaBatchSize,
		BatchBytes: cfg.InventoryKafkaBatchBytes,
	}

	kafkaProducer := queue.StartProducer(kafkaProducerCfg)

	connectedClientRecorder, err := controller.NewInventoryBasedConnectedClientRecorder(kafkaProducer, cfg.InventoryStaleTimestampOffset, cfg.InventoryReporterName)
	if err != nil {
		logger.LogFatalError("Failed to create Connected Client Recorder", err)
	}

	sqlTimeout := cfg.ConnectionDatabaseQueryTimeout
	tooOldIfBeforeThisTime := calculateStaleCutoffTime(cfg.InventoryStaleTimestampOffset)
	chunkSize := cfg.InventoryStaleTimestampUpdaterChunkSize

	logger.Log.Debug("Host's should be updated if their stale_timestamp is before ", tooOldIfBeforeThisTime)

	connection_repository.ProcessStaleConnections(context.TODO(), databaseConn, sqlTimeout, tooOldIfBeforeThisTime, chunkSize,
		func(ctx context.Context, rhcClient domain.ConnectorClientState) error {

			log := logger.Log.WithFields(logrus.Fields{"client_id": rhcClient.ClientID, "account": rhcClient.Account})

			log.Debug("Processing stale connection")

			identity, _, err := accountResolver.MapClientIdToAccountId(ctx, rhcClient.ClientID)
			if err != nil {
				// FIXME: Send disconnect here??  Need to determine the type of failure!
				logger.LogErrorWithAccountAndClientId("Unable to retrieve identity for connection", err, rhcClient.Account, rhcClient.ClientID)
				return err
			}

			err = connectedClientRecorder.RecordConnectedClient(ctx, identity, rhcClient)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to sent host info to inventory", err, rhcClient.Account, rhcClient.ClientID)
				return err
			}

			connection_repository.UpdateStaleTimestampInDB(log, ctx, databaseConn, sqlTimeout, rhcClient)

			return nil
		})

	// Explicitly close the kafka producer...this should cause a flush of any buffered messages
	if err := kafkaProducer.Close(); err != nil {
		logger.LogFatalError("Failed to close the kafka writer", err)
	}
}
