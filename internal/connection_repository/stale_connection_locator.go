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
		`SELECT account, org_id, client_id, canonical_facts, tags, dispatchers FROM connections
           WHERE canonical_facts != '{}' AND
           ( dispatchers ? 'rhc-worker-playbook' OR dispatchers ? 'package-manager' ) AND
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
		var orgID sql.NullString
		var canonicalFactsString sql.NullString
		var tagsString sql.NullString
		var dispatchersString sql.NullString

		var connectorClient domain.ConnectorClientState

		if err := rows.Scan(&connectorClient.Account, &orgID, &connectorClient.ClientID, &canonicalFactsString, &tagsString, &dispatchersString); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		if orgID.Valid {
			connectorClient.OrgID = domain.OrgID(orgID.String)
		}

		if dispatchersString.Valid {
			err = json.Unmarshal([]byte(dispatchersString.String), &connectorClient.Dispatchers)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse dispatchers from database.  Skipping connection.", err, connectorClient.Account, connectorClient.OrgID, connectorClient.ClientID)
				continue
			}
		}

		if canonicalFactsString.Valid {
			err = json.Unmarshal([]byte(canonicalFactsString.String), &connectorClient.CanonicalFacts)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts from database.  Skipping connection.", err, connectorClient.Account, connectorClient.OrgID, connectorClient.ClientID)
				continue
			}
		}

		if tagsString.Valid {
			err = json.Unmarshal([]byte(tagsString.String), &connectorClient.Tags)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse tags from database.  Skipping connection.", err, connectorClient.Account, connectorClient.OrgID, connectorClient.ClientID)
				continue
			}
		}

		processConnection(ctx, connectorClient)
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
