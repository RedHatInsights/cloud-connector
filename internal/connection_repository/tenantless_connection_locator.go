package connection_repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

func ProcessTenantlessConnections(ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, staleTimeCutoff time.Time, chunkSize int, processConnection ConnectionProcessor) error {

	// FIXME: :w
	maxTenantLookupFailures := 4

	queryCtx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	statement, err := databaseConn.Prepare(
		`SELECT account, org_id, client_id, canonical_facts, tags, dispatchers, tenant_lookup_timestamp, tenant_lookup_failure_count FROM connections
           WHERE org_id = '' AND
             tenant_lookup_timestamp IS NOT NULL AND
             tenant_lookup_timestamp < $1 AND 
             tenant_lookup_failure_count < $2
             order by tenant_lookup_timestamp asc
             limit $3`)
	if err != nil {
		logger.LogFatalError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.QueryContext(queryCtx, staleTimeCutoff, maxTenantLookupFailures, chunkSize)
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
		var tenantLookupTimestamp time.Time
		var tenantCheckCount int

		if err := rows.Scan(&account, &orgId, &clientId, &serializedCanonicalFacts, &serializedTags, &serializedDispatchers, &tenantLookupTimestamp, &tenantCheckCount); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		log := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": orgId, "client_id": clientId})

		canonicalFacts := deserializeCanonicalFacts(log, serializedCanonicalFacts)
		dispatchers := deserializeDispatchers(log, serializedDispatchers)
		tags := deserializeTags(log, serializedTags)

		connectorClientState := domain.ConnectorClientState{
			OrgID:                 domain.OrgID(orgId.String),
			ClientID:              domain.ClientID(clientId),
			CanonicalFacts:        canonicalFacts,
			Dispatchers:           dispatchers,
			Tags:                  tags,
			TenantLookupTimestamp: tenantLookupTimestamp,
		}

		if account.Valid {
			connectorClientState.Account = domain.AccountID(account.String)
		}

		processConnection(ctx, connectorClientState)
	}

	return nil
}

func RecordSuccessfulTenantLookup(ctx context.Context, databaseConn *sql.DB, sqlTimeout time.Duration, rhcClient domain.ConnectorClientState) error {

	log := logger.Log.WithFields(logrus.Fields{"account": rhcClient.Account, "org_id": rhcClient.OrgID, "client_id": rhcClient.ClientID})

	log.Debug("Recording successful tenant lookup")

	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()

	update := "UPDATE connections SET org_id = $1, account = $2, tenant_lookup_failure_count = 0, tenant_lookup_timestamp = null WHERE client_id=$3"

	statement, err := databaseConn.Prepare(update)
	if err != nil {
		return err
	}
	defer statement.Close()

	results, err := statement.ExecContext(ctx, rhcClient.OrgID, rhcClient.Account, rhcClient.ClientID)
	if err != nil {
		return err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return err
	}

	log.Debug("rowsAffected:", rowsAffected)
	return nil
}
