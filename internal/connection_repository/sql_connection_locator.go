package connection_repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type SqlConnectionLocator struct {
	database     *sql.DB
	queryTimeout time.Duration
	proxyFactory controller.ConnectorClientProxyFactory
}

func NewSqlConnectionLocator(cfg *config.Config, database *sql.DB, proxyFactory controller.ConnectorClientProxyFactory) (*SqlConnectionLocator, error) {
	return &SqlConnectionLocator{
		database:     database,
		queryTimeout: cfg.ConnectionDatabaseQueryTimeout,
		proxyFactory: proxyFactory,
	}, nil
}

func (scm *SqlConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, orgID domain.OrgID, client_id domain.ClientID) controller.ConnectorClient {
	var conn controller.ConnectorClient
	var err error

	log := logger.Log.WithFields(logrus.Fields{"client_id": client_id,
		"account": account,
		"org_id":  orgID})

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionByAccountAndClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	statement, err := scm.database.Prepare("SELECT client_id, canonical_facts, dispatchers, tags FROM connections WHERE account = $1 AND client_id = $2")
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	var name string
	var serializedCanonicalFacts sql.NullString
	var serializedDispatchers sql.NullString
	var serializedTags sql.NullString

	err = statement.QueryRowContext(ctx, account, client_id).Scan(&name, &serializedCanonicalFacts, &serializedDispatchers, &serializedTags)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.LogError("SQL query failed:", err)
		}
		return nil
	}

	canonicalFacts := deserializeCanonicalFacts(log, serializedCanonicalFacts)
	dispatchers := deserializeDispatchers(log, serializedDispatchers)
	tags := deserializeTags(log, serializedTags)

	conn, err = scm.proxyFactory.CreateProxy(ctx, orgID, domain.AccountID(account), domain.ClientID(client_id), canonicalFacts, dispatchers, tags)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to create the proxy", err, account, orgID, client_id)
		return nil
	}

	return conn
}

func (scm *SqlConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {

	log := logger.Log.WithFields(logrus.Fields{"account": account})

	var totalConnections int

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionsByAccountDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	connectionsPerAccount := make(map[domain.ClientID]controller.ConnectorClient)

	statement, err := scm.database.Prepare(
		`SELECT client_id, org_id, canonical_facts, dispatchers, tags, COUNT(*) OVER() FROM connections
            WHERE account = $1
            ORDER BY client_id
            OFFSET $2
            LIMIT $3`)
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil, totalConnections, err
	}
	defer statement.Close()

	rows, err := statement.QueryContext(ctx, account, offset, limit)
	if err != nil {
		logger.LogError("SQL query failed", err)
		return nil, totalConnections, err
	}
	defer rows.Close()

	for rows.Next() {
		var client_id domain.ClientID
		var orgIdString sql.NullString
		var serializedCanonicalFacts sql.NullString
		var serializedDispatchers sql.NullString
		var serializedTags sql.NullString

		if err := rows.Scan(&client_id, &orgIdString, &serializedCanonicalFacts, &serializedDispatchers, &serializedTags, &totalConnections); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		org_id := domain.OrgID(orgIdString.String)

		log := log.WithFields(logrus.Fields{"org_id": org_id, "client_id": client_id})

		canonicalFacts := deserializeCanonicalFacts(log, serializedCanonicalFacts)
		dispatchers := deserializeDispatchers(log, serializedDispatchers)
		tags := deserializeTags(log, serializedTags)

		proxy, err := scm.proxyFactory.CreateProxy(ctx, org_id, domain.AccountID(account), domain.ClientID(client_id), canonicalFacts, dispatchers, tags)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to create the proxy.  Skipping row.", err, account, org_id, client_id)
			continue
		}

		connectionsPerAccount[client_id] = proxy
	}

	return connectionsPerAccount, totalConnections, nil
}

func (scm *SqlConnectionLocator) GetAllConnections(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {

	var totalConnections int

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupAllConnectionsDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	connectionMap := make(map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient)

	statement, err := scm.database.Prepare(
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

		proxy, err := scm.proxyFactory.CreateProxy(ctx, orgId, domain.AccountID(account), domain.ClientID(clientId), canonicalFacts, dispatchers, tags)
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
}
