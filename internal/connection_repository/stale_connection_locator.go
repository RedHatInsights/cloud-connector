package connection_repository

import (
	"context"
	"database/sql"
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
		var account sql.NullString
		var orgId sql.NullString
		var clientId domain.ClientID
		var serializedCanonicalFacts sql.NullString
		var serializedDispatchers sql.NullString
		var serializedTags sql.NullString

		if err := rows.Scan(&account, &orgId, &clientId, &serializedCanonicalFacts, &serializedTags, &serializedDispatchers); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		log := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": orgId, "client_id": clientId})

		canonicalFacts := deserializeCanonicalFacts(log, serializedCanonicalFacts)
		dispatchers := deserializeDispatchers(log, serializedDispatchers)
		tags := deserializeTags(log, serializedTags)

		connectorClientState := domain.ConnectorClientState{
			OrgID:          domain.OrgID(orgId.String),
			ClientID:       domain.ClientID(clientId),
			CanonicalFacts: canonicalFacts,
			Dispatchers:    dispatchers,
			Tags:           tags,
		}

		if account.Valid {
			connectorClientState.Account = domain.AccountID(account.String)
		}

		processConnection(ctx, connectorClientState)
	}

	return nil
}

func UpdateStaleTimestampInDB(log *logrus.Entry, ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, rhcClient domain.ConnectorClientState) {

	log.Debug("Updating stale timestamp")

	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	update := "UPDATE connections SET stale_timestamp = NOW() WHERE org_id=$1 AND client_id=$2"

	statement, err := databaseConn.Prepare(update)
	if err != nil {
		log.Fatal(err)
	}
	defer statement.Close()

	results, err := statement.ExecContext(ctx, rhcClient.OrgID, rhcClient.ClientID)
	if err != nil {
		log.Fatal(err)
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("rowsAffected:", rowsAffected)
}
