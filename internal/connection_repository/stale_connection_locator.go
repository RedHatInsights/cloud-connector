package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type ConnectionProcessor func(context.Context, domain.ConnectorClientState) error

func ProcessStaleConnections(ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, staleTimeCutoff time.Time, chunkSize int, processConnection ConnectionProcessor) error {

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	statement, err := databaseConn.Prepare(
		`SELECT account, client_id, canonical_facts, tags FROM connections
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
		var tagsString string

		if err := rows.Scan(&account, &clientID, &canonicalFactsString, &tagsString); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		var canonicalFacts interface{}
		err = json.Unmarshal([]byte(canonicalFactsString), &canonicalFacts)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts.  Skipping connection.", err, account, clientID)
			continue
		}

		var tags domain.Tags
		err = json.Unmarshal([]byte(tagsString), &tags)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to parse tags.  Skipping connection.", err, account, clientID)
			continue
		}

		rhcClient := domain.ConnectorClientState{
			Account:        account,
			ClientID:       clientID,
			CanonicalFacts: canonicalFacts,
			Tags:           tags,
		}

		processConnection(ctx, rhcClient)
	}

	return nil
}

func UpdateStaleTimestampInDB(log *logrus.Entry, ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, rhcClient domain.ConnectorClientState) {

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
