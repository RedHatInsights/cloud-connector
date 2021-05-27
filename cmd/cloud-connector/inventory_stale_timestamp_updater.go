package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"

	"github.com/sirupsen/logrus"
)

const serviceName = "Cloud-Connector Inventory Stale Timestamp Updater"

type connectionProcessor func(context.Context, domain.RhcClient) error

func processStaleConnections(ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, staleTimeCutoff time.Time, chunkSize int, processConnection connectionProcessor) error {

	logger.Log.Debug("Host's should be updated if their stale_timestamp is before ", staleTimeCutoff)

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	statement, err := databaseConn.Prepare(
		`SELECT account, client_id, canonical_facts FROM connections
           WHERE canonical_facts != '{}' AND
             dispatchers ? 'rhc-worker-playbook' AND
             stale_timestamp < $1
             order by stale_timestamp asc
             limit $2`)
	if err != nil {
		logger.LogFatalError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.QueryContext(queryCtx, staleTimeCutoff, chunkSize)
	if err != nil {
		logger.LogFatalError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var account domain.AccountID
		var clientID domain.ClientID
		var canonicalFactsString string

		if err := rows.Scan(&account, &clientID, &canonicalFactsString); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		var canonicalFacts interface{}
		err = json.Unmarshal([]byte(canonicalFactsString), &canonicalFacts)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts.  Skipping connection.", err, account, clientID)
			continue
		}

		rhcClient := domain.RhcClient{Account: account, ClientID: clientID, CanonicalFacts: canonicalFacts} // FIXME: build this from the database

		processConnection(ctx, rhcClient)
	}

	return nil
}

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

	processStaleConnections(context.TODO(), databaseConn, sqlTimeout, tooOldIfBeforeThisTime, chunkSize,
		func(ctx context.Context, rhcClient domain.RhcClient) error {

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

			updateStaleTimestampInDB(log, ctx, databaseConn, sqlTimeout, rhcClient)

			return nil
		})

	// Explicitly close the kafka producer...this should cause a flush of any buffered messages
	if err := kafkaProducer.Close(); err != nil {
		logger.LogFatalError("Failed to close the kafka writer", err)
	}
}

func updateStaleTimestampInDB(log *logrus.Entry, ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, rhcClient domain.RhcClient) {

	log.Debug("Updating stale timestamp")

	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	update := "UPDATE connections SET stale_timestamp = NOW() WHERE account=$1 AND client_id=$2"

	statement, err := databaseConn.Prepare(update)
	if err != nil {
		log.Fatal(err)
	}
	defer statement.Close()

	results, err := statement.ExecContext(ctx, rhcClient.Account, rhcClient.ClientID)
	if err != nil {
		log.Fatal(err)
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("rowsAffected:", rowsAffected)
}
