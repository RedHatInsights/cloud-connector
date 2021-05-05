package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

const serviceName = "Cloud-Connector Inventory Stale Timestamp Updater"

type connectionProcessor func(domain.RhcClient) error

func processStaleConnections(cfg *config.Config, databaseConn *sql.DB, processConnection connectionProcessor) error {

	windowToUpdateBeforeGoingStale := 1 * time.Hour
	offset := cfg.InventoryStaleTimestampOffset - windowToUpdateBeforeGoingStale
	tooOldIfBeforeThisTime := time.Now().Add(-1 * offset)

	logger.Log.Debug("Host's should be updated if their stale_timestamp is before ", tooOldIfBeforeThisTime)

	statement, err := databaseConn.Prepare("SELECT account, client_id, canonical_facts FROM connections WHERE canonical_facts != '{}' AND dispatchers ? 'rhc-worker-playbook' AND stale_timestamp < $1 order by stale_timestamp asc limit $2")
	if err != nil {
		logger.LogFatalError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.Query(tooOldIfBeforeThisTime, cfg.InventoryStaleTimestampUpdaterChunkSize)
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
			logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts", err, account, clientID)
			return err
		}

		rhcClient := domain.RhcClient{Account: account, ClientID: clientID, CanonicalFacts: canonicalFacts} // FIXME: build this from the database

		processConnection(rhcClient)
	}

	return nil
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

	connectedClientRecorder, err := controller.NewConnectedClientRecorder(cfg.ConnectedClientRecorderImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Connected Client Recorder", err)
	}

	processStaleConnections(cfg, databaseConn,
		func(rhcClient domain.RhcClient) error {

			log := logger.Log.WithFields(logrus.Fields{"client_id": rhcClient.ClientID, "account": rhcClient.Account})

			log.Debug("Processing stale connection")

			identity, _, err := accountResolver.MapClientIdToAccountId(context.TODO(), rhcClient.ClientID)
			if err != nil {
				// FIXME: Send disconnect here??  Need to determine the type of failure!
				logger.LogErrorWithAccountAndClientId("Unable to retrieve identity for connection", err, rhcClient.Account, rhcClient.ClientID)
				return err
			}

			err = connectedClientRecorder.RecordConnectedClient(context.TODO(), identity, rhcClient)
			fmt.Println("err:", err)

			updateStaleTimestampInDB(log, databaseConn, rhcClient)

			return nil
		})
}

func updateStaleTimestampInDB(log *logrus.Entry, databaseConn *sql.DB, rhcClient domain.RhcClient) {

	log.Debug("Updating stale timestamp")

	update := "UPDATE connections SET stale_timestamp = NOW() WHERE account=$1 AND client_id=$2"

	statement, err := databaseConn.Prepare(update)
	if err != nil {
		log.Fatal(err)
	}
	defer statement.Close()

	results, err := statement.Exec(rhcClient.Account, rhcClient.ClientID)
	if err != nil {
		log.Fatal(err)
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("rowsAffected:", rowsAffected)
}
