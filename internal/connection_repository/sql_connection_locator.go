package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type SqlConnectionLocator struct {
	database     *sql.DB
	proxyFactory controller.ConnectorClientProxyFactory
	metrics      *sqlConnectionLookupMetrics
}

type sqlConnectionLookupMetrics struct {
	sqlLookupConnectionByAccountAndClientIDDuration prometheus.Histogram
	sqlLookupConnectionsByAccountDuration           prometheus.Histogram
	sqlLookupAllConnectionsDuration                 prometheus.Histogram
}

func initializeSqlConnectionLookupMetrics() *sqlConnectionLookupMetrics {
	metrics := new(sqlConnectionLookupMetrics)

	metrics.sqlLookupConnectionByAccountAndClientIDDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connection_by_account_and_client_id_duration",
		Help: "The amount of time the it took to lookup a connection using account and client id ",
	})

	metrics.sqlLookupConnectionsByAccountDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connections_by_account",
		Help: "The amount of time the it took to lookup a connection using account",
	})

	metrics.sqlLookupAllConnectionsDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_all_connections",
		Help: "The amount of time the it took to lookup all connections",
	})

	return metrics
}

func NewSqlConnectionLocator(cfg *config.Config, proxyFactory controller.ConnectorClientProxyFactory) (*SqlConnectionLocator, error) {

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionLocator{
		database:     database,
		proxyFactory: proxyFactory,
		metrics:      initializeSqlConnectionLookupMetrics(),
	}, nil
}

func (scm *SqlConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, client_id domain.ClientID) controller.ConnectorClient {
	var conn controller.ConnectorClient
	var err error

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlLookupConnectionByAccountAndClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	statement, err := scm.database.Prepare("SELECT client_id, dispatchers FROM connections WHERE account = $1 AND client_id = $2 AND state = 1")
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	var name string
	var dispatchersString string
	err = statement.QueryRow(account, client_id).Scan(&name, &dispatchersString)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.LogError("SQL query failed:", err)
		}
		return nil
	}

	var dispatchers domain.Dispatchers
	err = json.Unmarshal([]byte(dispatchersString), &dispatchers)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, client_id)
	}

	conn, err = scm.proxyFactory.CreateProxy(ctx, domain.AccountID(account), domain.ClientID(client_id), dispatchers)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to create the proxy", err, account, client_id)
		return nil
	}

	return conn
}

func (scm *SqlConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {

	var totalConnections int

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlLookupConnectionsByAccountDuration)
	defer callDurationTimer.ObserveDuration()

	connectionsPerAccount := make(map[domain.ClientID]controller.ConnectorClient)

	statement, err := scm.database.Prepare(
		`SELECT client_id, dispatchers, COUNT(*) OVER() FROM connections
            WHERE account = $1
              AND state = 1
            ORDER BY client_id
            OFFSET $2
            LIMIT $3`)
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil, totalConnections, err
	}
	defer statement.Close()

	rows, err := statement.Query(account, offset, limit)
	if err != nil {
		logger.LogError("SQL query failed", err)
		return nil, totalConnections, err
	}
	defer rows.Close()

	for rows.Next() {
		var client_id domain.ClientID
		var dispatchersString string
		if err := rows.Scan(&client_id, &dispatchersString, &totalConnections); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		var dispatchers domain.Dispatchers
		err = json.Unmarshal([]byte(dispatchersString), &dispatchers)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, client_id)
		}

		proxy, err := scm.proxyFactory.CreateProxy(ctx, domain.AccountID(account), domain.ClientID(client_id), dispatchers)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to create the proxy.  Skipping row.", err, account, client_id)
			continue
		}

		connectionsPerAccount[client_id] = proxy
	}

	return connectionsPerAccount, totalConnections, nil
}

func (scm *SqlConnectionLocator) GetAllConnections(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {

	var totalConnections int

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlLookupAllConnectionsDuration)
	defer callDurationTimer.ObserveDuration()

	connectionMap := make(map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient)

	statement, err := scm.database.Prepare(
		`SELECT account, client_id, dispatchers, COUNT(*) OVER() FROM connections
            WHERE state = 1
            ORDER BY account, client_id
            OFFSET $1
            LIMIT $2`)
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil, totalConnections, err
	}
	defer statement.Close()

	rows, err := statement.Query(offset, limit)
	if err != nil {
		logger.LogError("SQL query failed", err)
		return nil, totalConnections, err
	}
	defer rows.Close()

	for rows.Next() {
		var account domain.AccountID
		var clientId domain.ClientID
		var dispatchersString string

		if err := rows.Scan(&account, &clientId, &dispatchersString, &totalConnections); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		var dispatchers domain.Dispatchers
		err = json.Unmarshal([]byte(dispatchersString), &dispatchers)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, clientId)
		}

		proxy, err := scm.proxyFactory.CreateProxy(ctx, domain.AccountID(account), domain.ClientID(clientId), dispatchers)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to create the proxy.  Skipping row.", err, account, clientId)
			continue
		}

		if _, exists := connectionMap[account]; !exists {
			connectionMap[account] = make(map[domain.ClientID]controller.ConnectorClient)
		}

		connectionMap[account][clientId] = proxy
	}

	return connectionMap, totalConnections, err
}
