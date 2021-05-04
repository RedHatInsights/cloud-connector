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
)

const serviceName = "Cloud-Connector Inventory Stale Timestamp Updater"

// FIXME: remember "limit" to chunk size

type connectionProcessor func(domain.AccountID, domain.ClientID, string) error

func processStaleConnections(cfg *config.Config, databaseConn *sql.DB, processConnection connectionProcessor) error {

	windowToUpdateBeforeGoingStale := 1 * time.Hour
	offset := cfg.InventoryStaleTimestampOffset - windowToUpdateBeforeGoingStale
	tooOldIfBeforeThisTime := time.Now().Add(-1 * offset)
	fmt.Println("tooOldIfBeforeThisTime: ", tooOldIfBeforeThisTime)

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

		processConnection(account, clientID, canonicalFactsString)
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

	logger.Log.Info(accountResolver)

	connectedClientRecorder, err := controller.NewConnectedClientRecorder(cfg.ConnectedClientRecorderImpl, cfg)
	if err != nil {
		logger.LogFatalError("Failed to create Connected Client Recorder", err)
	}

	logger.Log.Info(connectedClientRecorder)

	processStaleConnections(cfg, databaseConn,
		func(account domain.AccountID, clientID domain.ClientID, canonicalFactsString string) error {
			logger.Log.Infof("FOUND STALE CONNECTION: %s %s %s\n", account, clientID, canonicalFactsString)

			identity, account2, err := accountResolver.MapClientIdToAccountId(context.TODO(), clientID)

			fmt.Println("account2:", account2)

			var canonicalFacts interface{}
			err = json.Unmarshal([]byte(canonicalFactsString), &canonicalFacts)
			fmt.Println("err:", err)

			err = connectedClientRecorder.RecordConnectedClient(context.TODO(), identity, account, clientID, canonicalFacts)
			fmt.Println("err:", err)

			updateStaleTimestampInDB(databaseConn, account, clientID)

			return nil
		})
}

func updateStaleTimestampInDB(databaseConn *sql.DB, account domain.AccountID, clientID domain.ClientID) {
	fmt.Println("Updating the timestamp in the db!")
	update := "UPDATE connections SET updated_at = NOW() WHERE account=$1 AND client_id=$2"

	statement, err := databaseConn.Prepare(update)
	if err != nil {
		logger.Log.Fatal(err)
	}
	defer statement.Close()

	results, err := statement.Exec(account, clientID)
	if err != nil {
		logger.Log.Fatal(err)
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		logger.Log.Fatal(err)
	}

	fmt.Println("rowsAffected:", rowsAffected)
}
