package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

func NewSqlGetConnectionByClientID(cfg *config.Config, database *sql.DB) (GetConnectionByClientID, error) {

	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, clientId domain.ClientID) (domain.ConnectorClientState, error) {
		var clientState domain.ConnectorClientState
		var err error

		callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionByAccountAndClientIDDuration)
		defer callDurationTimer.ObserveDuration()

		ctx, cancel := context.WithTimeout(ctx, cfg.ConnectionDatabaseQueryTimeout)
		defer cancel()

		statement, err := database.Prepare(`SELECT  account, dispatchers, canonical_facts, tags
            FROM connections WHERE org_id = $1 AND client_id = $2`)
		if err != nil {
			logger.LogWithError(log, "SQL Prepare failed", err)
			return clientState, nil
		}
		defer statement.Close()

		var accountString sql.NullString
		var dispatchersString sql.NullString
		var canonicalFactsString sql.NullString
		var tagsString sql.NullString

		err = statement.QueryRowContext(ctx, orgId, clientId).Scan(&accountString, &dispatchersString, &canonicalFactsString, &tagsString)

		if err != nil {
			if err == sql.ErrNoRows {
				return clientState, NotFoundError
			}

			logger.LogWithError(log, "SQL query failed:", err)
			return clientState, err
		}

		clientState.OrgID = orgId
		clientState.ClientID = clientId

		if accountString.Valid {
			clientState.Account = domain.AccountID(accountString.String)
		}

		if dispatchersString.Valid {
			err = json.Unmarshal([]byte(dispatchersString.String), &clientState.Dispatchers)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse dispatchers from database.", err, clientState.Account, clientState.OrgID, clientState.ClientID)
			}
		}

		if canonicalFactsString.Valid {
			err = json.Unmarshal([]byte(canonicalFactsString.String), &clientState.CanonicalFacts)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts from database.", err, clientState.Account, clientState.OrgID, clientState.ClientID)
			}
		}

		if tagsString.Valid {
			err = json.Unmarshal([]byte(tagsString.String), &clientState.Tags)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to parse tags from database.", err, clientState.Account, clientState.OrgID, clientState.ClientID)
			}
		}

		return clientState, nil
	}, nil
}

func NewSqlGetConnectionsByOrgID(cfg *config.Config, database *sql.DB) (GetConnectionsByOrgID, error) {

	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, offset int, limit int) (map[domain.ClientID]domain.ConnectorClientState, int, error) {

		var totalConnections int

		callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionsByAccountDuration)
		defer callDurationTimer.ObserveDuration()

		ctx, cancel := context.WithTimeout(ctx, cfg.ConnectionDatabaseQueryTimeout)
		defer cancel()

		connectionsPerAccount := make(map[domain.ClientID]domain.ConnectorClientState)

		fmt.Println("orgId: ", orgId)
		fmt.Println("limit: ", limit)
		fmt.Println("offset: ", offset)

		statement, err := database.Prepare(
			`SELECT client_id, org_id, account, dispatchers, canonical_facts, tags, COUNT(*) OVER() FROM connections
                WHERE org_id = $1
                ORDER BY client_id
                OFFSET $2
                LIMIT $3`)
		if err != nil {
			logger.LogWithError(log, "SQL Prepare failed", err)
			return nil, totalConnections, err
		}
		defer statement.Close()

		rows, err := statement.QueryContext(ctx, orgId, offset, limit)
		if err != nil {
			logger.LogWithError(log, "SQL query failed", err)
			return nil, totalConnections, err
		}
		defer rows.Close()

		for rows.Next() {
			var clientId domain.ClientID
			var orgId string
			var accountString sql.NullString
			var dispatchersString sql.NullString
			var canonicalFactsString sql.NullString
			var tagsString sql.NullString

			if err := rows.Scan(&clientId, &orgId, &accountString, &dispatchersString, &canonicalFactsString, &tagsString, &totalConnections); err != nil {
				logger.LogWithError(log, "SQL scan failed.  Skipping row.", err)
				continue
			}

			clientState := domain.ConnectorClientState{
				OrgID:    domain.OrgID(orgId),
				ClientID: clientId,
			}

			if accountString.Valid {
				clientState.Account = domain.AccountID(accountString.String)
			}

			if dispatchersString.Valid {
				err = json.Unmarshal([]byte(dispatchersString.String), &clientState.Dispatchers)
				if err != nil {
					logger.LogErrorWithAccountAndClientId("Unable to parse dispatchers from database.", err, clientState.Account, clientState.OrgID, clientState.ClientID)
				}
			}

			if canonicalFactsString.Valid {
				err = json.Unmarshal([]byte(canonicalFactsString.String), &clientState.CanonicalFacts)
				if err != nil {
					logger.LogErrorWithAccountAndClientId("Unable to parse canonical facts from database.", err, clientState.Account, clientState.OrgID, clientState.ClientID)
				}
			}

			if tagsString.Valid {
				err = json.Unmarshal([]byte(tagsString.String), &clientState.Tags)
				if err != nil {
					logger.LogErrorWithAccountAndClientId("Unable to parse tags from database.", err, clientState.Account, clientState.OrgID, clientState.ClientID)
				}
			}

			connectionsPerAccount[clientId] = clientState
		}

		return connectionsPerAccount, totalConnections, nil
	}, nil
}
