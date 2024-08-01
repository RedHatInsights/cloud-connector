package main

import (
	"context"
	"fmt"
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

	logger.Log.Info("Starting ", serviceName)

	cfg := config.GetConfig()
	logger.Log.Info("Cloud-Connector configuration:\n", cfg)

	databaseConn, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		logger.LogFatalError("Failed to connect to the database", err)
	}

	connectionRegistrar, err := connection_repository.NewSqlConnectionRegistrar(cfg, databaseConn)
	if err != nil {
		logger.LogFatalError("Failed to create connection registrar", err)
	}

	accountResolver, err := controller.NewAccountIdResolver(cfg.ClientIdToAccountIdImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Account ID Resolver", err)
	}

	var kafkaSaslCfg *queue.SaslConfig

	if cfg.KafkaSASLMechanism != "" {
		kafkaSaslCfg = &queue.SaslConfig{
			SaslMechanism: cfg.KafkaSASLMechanism,
			SaslUsername:  cfg.KafkaUsername,
			SaslPassword:  cfg.KafkaPassword,
			KafkaCA:       cfg.KafkaCA,
		}
	}

	kafkaProducerCfg := &queue.ProducerConfig{
		Brokers:    cfg.InventoryKafkaBrokers,
		SaslConfig: kafkaSaslCfg,
		Topic:      cfg.InventoryKafkaTopic,
		BatchSize:  cfg.InventoryKafkaBatchSize,
		BatchBytes: cfg.InventoryKafkaBatchBytes,
	}

	kafkaProducer, err := queue.StartProducer(kafkaProducerCfg)
	if err != nil {
		logger.LogFatalError("Unable to start kafka producer", err)
	}

	connectedClientRecorder, err := controller.NewInventoryBasedConnectedClientRecorder(controller.BuildInventoryMessageProducer(kafkaProducer), cfg.InventoryStaleTimestampOffset, cfg.InventoryReporterName)
	if err != nil {
		logger.LogFatalError("Failed to create Connected Client Recorder", err)
	}

	sqlTimeout := cfg.ConnectionDatabaseQueryTimeout
	tooOldIfBeforeThisTime := calculateStaleCutoffTime(cfg.InventoryStaleTimestampOffset)
	chunkSize := cfg.InventoryStaleTimestampUpdaterChunkSize

	logger.Log.Debug("Host's should be updated if their stale_timestamp is before ", tooOldIfBeforeThisTime)

	connection_repository.ProcessStaleConnections(context.TODO(), databaseConn, sqlTimeout, tooOldIfBeforeThisTime, chunkSize,
		func(ctx context.Context, rhcClient domain.ConnectorClientState) error {

			log := logger.Log.WithFields(logrus.Fields{"client_id": rhcClient.ClientID, "account": rhcClient.Account, "org_id": rhcClient.OrgID})

			log.Debug("Processing stale connection")

			if rhcClient.TenantLookupFailureCount >= cfg.PurgeConnectionOnFailedTenantLookupCount {
				logger.LogErrorWithAccountAndClientId("Tenant lookup failed for connection too many times.  Purging connection...", err, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
				connectionRegistrar.Unregister(ctx, rhcClient.ClientID)
				return fmt.Errorf("Tenant lookup failed to many times")
			}

			identity, _, _, err := accountResolver.MapClientIdToAccountId(context.Background(), rhcClient.ClientID)
			if err != nil {

				logger.LogErrorWithAccountAndClientId("Unable to retrieve identity for connection", err, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)

				dberr := connection_repository.RecordFailedTenantLookup(ctx, databaseConn, sqlTimeout, rhcClient)
				if dberr != nil {
					logger.LogErrorWithAccountAndClientId("Unable to record failed tenant lookup for connection", err, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
				}

				return err
			}

			err = connectedClientRecorder.RecordConnectedClient(ctx, identity, rhcClient)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to sent host info to inventory", err, rhcClient.Account, rhcClient.OrgID, rhcClient.ClientID)
				return err
			}

			connection_repository.RecordUpdatedStaleTimestamp(ctx, databaseConn, sqlTimeout, rhcClient)

			return nil
		})

	// Explicitly close the kafka producer...this should cause a flush of any buffered messages
	if err := kafkaProducer.Close(); err != nil {
		logger.LogFatalError("Failed to close the kafka writer", err)
	}
}
