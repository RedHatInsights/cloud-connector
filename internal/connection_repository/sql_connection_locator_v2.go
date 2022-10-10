package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const (
	satelliteWorker              = "foreman_rh_cloud"
	connectionQueryPrefix        = "SELECT  account, org_id, dispatchers, canonical_facts, tags FROM connections "
	strictConnectionLookupQuery  = connectionQueryPrefix + "WHERE org_id = $1 AND client_id = $2"
	relaxedConnectionLookupQuery = connectionQueryPrefix + "WHERE (org_id = $1 OR dispatchers ? '" + satelliteWorker + "') AND client_id = $2"
)

func NewSqlGetConnectionByClientID(cfg *config.Config, database *sql.DB) (GetConnectionByClientID, error) {

	return createGetConnectionByClientIDImpl(cfg, database, strictConnectionLookupQuery)
}

func NewPermittedTenantSqlGetConnectionByClientID(cfg *config.Config, database *sql.DB) (GetConnectionByClientID, error) {

	// The "relaxed" / "permitted" tenant logic is basically contained here.
	// This allows us to reuse a big chunk of the logic required to read the
	// connection state from the database.

	lookupFunc, err := createGetConnectionByClientIDImpl(cfg, database, relaxedConnectionLookupQuery)
	if err != nil {
		return lookupFunc, err
	}

	// Wrap the real connection lookup method with a method that logs that the access has been "relaxed"
	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, clientId domain.ClientID) (domain.ConnectorClientState, error) {

		clientState, err := lookupFunc(ctx, log, orgId, clientId)
		if err != nil {
			return clientState, err
		}

		if orgId != clientState.OrgID {
			log = log.WithFields(logrus.Fields{
				"connection_owner_account": clientState.Account,
				"connection_owner_org_id":  clientState.OrgID})

			log.Info("Allowing relaxed access to connection")
		}

		return clientState, err
	}, nil
}

func createGetConnectionByClientIDImpl(cfg *config.Config, database *sql.DB, sqlQuery string) (GetConnectionByClientID, error) {

	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, clientId domain.ClientID) (domain.ConnectorClientState, error) {
		var clientState domain.ConnectorClientState
		var err error

		err = verifyOrgId(orgId)
		if err != nil {
			return clientState, err
		}

		err = verifyClientId(clientId)
		if err != nil {
			return clientState, err
		}

		// We should probably use different timer for different look ups.
		callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionByAccountAndClientIDDuration)
		defer callDurationTimer.ObserveDuration()

		ctx, cancel := context.WithTimeout(ctx, cfg.ConnectionDatabaseQueryTimeout)
		defer cancel()

		statement, err := database.Prepare(sqlQuery)
		if err != nil {
			logger.LogWithError(log, "SQL Prepare failed", err)
			return clientState, nil
		}
		defer statement.Close()

		var accountString sql.NullString
		var orgID string
		var dispatchersString sql.NullString
		var canonicalFactsString sql.NullString
		var tagsString sql.NullString

		err = statement.QueryRowContext(ctx, orgId, clientId).Scan(&accountString, &orgID, &dispatchersString, &canonicalFactsString, &tagsString)

		if err != nil {
			if err == sql.ErrNoRows {
				return clientState, NotFoundError
			}

			logger.LogWithError(log, "SQL query failed:", err)
			return clientState, err
		}

		clientState.OrgID = domain.OrgID(orgID)
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

func NewGetAllConnections(cfg *config.Config, database *sql.DB, proxyFactory controller.ConnectorClientProxyFactory) (GetAllConnections, error) {
	return func(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {
		var totalConnections int

		callDurationTimer := prometheus.NewTimer(metrics.sqlLookupAllConnectionsDuration)
		defer callDurationTimer.ObserveDuration()
	
		ctx, cancel := context.WithTimeout(ctx, cfg.ConnectionDatabaseQueryTimeout)
		defer cancel()
	
		connectionMap := make(map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient)
	
		statement, err := database.Prepare(
			`SELECT account, org_id, client_id, canonical_facts, dispatchers, tags, COUNT(*) OVER() FROM connections
				ORDER BY account, client_id
				OFFSET $1
				LIMIT $2`)
		if err != nil {
			logger.LogError("SQL Prepare failed", err)
			return nil, totalConnections, err
		}
		defer statement.Close()
	
		rows, err := statement.QueryContext(ctx, offset, limit)
		if err != nil {
			logger.LogError("SQL query failed", err)
			return nil, totalConnections, err
		}
		defer rows.Close()
	
		for rows.Next() {
			var account domain.AccountID
			var orgIdString sql.NullString
			var clientId domain.ClientID
			var serializedCanonicalFacts sql.NullString
			var serializedDispatchers sql.NullString
			var serializedTags sql.NullString
	
			if err := rows.Scan(&account, &orgIdString, &clientId, &serializedCanonicalFacts, &serializedDispatchers, &serializedTags, &totalConnections); err != nil {
				logger.LogError("SQL scan failed.  Skipping row.", err)
				continue
			}
	
			orgId := domain.OrgID(orgIdString.String)
	
			log := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": orgId, "client_id": clientId})
	
			canonicalFacts := deserializeCanonicalFacts(log, serializedCanonicalFacts)
			dispatchers := deserializeDispatchers(log, serializedDispatchers)
			tags := deserializeTags(log, serializedTags)
	
			proxy, err := proxyFactory.CreateProxy(ctx, orgId, domain.AccountID(account), domain.ClientID(clientId), canonicalFacts, dispatchers, tags)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to create the proxy.  Skipping row.", err, account, orgId, clientId)
				continue
			}
	
			if _, exists := connectionMap[account]; !exists {
				connectionMap[account] = make(map[domain.ClientID]controller.ConnectorClient)
			}
	
			connectionMap[account][clientId] = proxy
		}
	
		return connectionMap, totalConnections, err
	}, nil
}
