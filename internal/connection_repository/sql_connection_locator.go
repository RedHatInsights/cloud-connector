package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"gorm.io/gorm"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
)

type SqlConnectionLocator struct {
	database     *gorm.DB
	queryTimeout time.Duration
	proxyFactory controller.ConnectorClientProxyFactory
}

func NewSqlConnectionLocator(cfg *config.Config, database *gorm.DB, proxyFactory controller.ConnectorClientProxyFactory) (*SqlConnectionLocator, error) {
	return &SqlConnectionLocator{
		database:     database,
		queryTimeout: cfg.ConnectionDatabaseQueryTimeout,
		proxyFactory: proxyFactory,
	}, nil
}

func (scm *SqlConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, orgID domain.OrgID, client_id domain.ClientID) controller.ConnectorClient {
	var conn controller.ConnectorClient
	var err error

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionByAccountAndClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	rows := scm.database.Table("connections").
		Select("client_id", "dispatchers").
		Where("account = ? AND client_id = ?", account, client_id).
		Row()

	if rows.Err() != nil {
		logger.LogError("SQL Prepare failed", rows.Err())
		return nil
	}

	var name string
	var dispatchersString sql.NullString
	err = rows.Scan(&name, &dispatchersString)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.LogError("SQL query failed:", err)
		}
		return nil
	}

	var dispatchers domain.Dispatchers
	if dispatchersString.Valid {
		err = json.Unmarshal([]byte(dispatchersString.String), &dispatchers)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, orgID, client_id)
		}
	}

	conn, err = scm.proxyFactory.CreateProxy(ctx, orgID, domain.AccountID(account), domain.ClientID(client_id), dispatchers)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to create the proxy", err, account, orgID, client_id)
		return nil
	}

	return conn
}

func (scm *SqlConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {

	var totalConnections int

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionsByAccountDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	connectionsPerAccount := make(map[domain.ClientID]controller.ConnectorClient)

	rows, err := scm.database.
		Table("connections").
		Select("client_id", "org_id", "dispatchers", "COUNT(*) OVER()").
		Where("account = ?", account).
		Order("client_id").
		Offset(offset).
		Limit(limit).
		Rows()

	if err != nil {
		logger.LogError("SQL Query failed", err)
		return nil, totalConnections, err
	}
	defer rows.Close()

	for rows.Next() {
		var client_id domain.ClientID
		var orgIdString sql.NullString
		var dispatchersString string
		if err := rows.Scan(&client_id, &orgIdString, &dispatchersString, &totalConnections); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		org_id := domain.OrgID(orgIdString.String)

		var dispatchers domain.Dispatchers
		err = json.Unmarshal([]byte(dispatchersString), &dispatchers)
		if err != nil {
			logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, org_id, client_id)
		}

		proxy, err := scm.proxyFactory.CreateProxy(ctx, org_id, domain.AccountID(account), domain.ClientID(client_id), dispatchers)
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

	rows, err := scm.database.
		Table("connections").
		Select("account", "org_id", "client_id", "dispatchers", "COUNT(*) OVER()").
		Order("account, client_id").
		Offset(offset).
		Limit(limit).
		Rows()

	if err != nil {
		logger.LogError("SQL query failed", err)
		return nil, totalConnections, err
	}
	defer rows.Close()

	for rows.Next() {
		var account domain.AccountID
		var orgIdString sql.NullString
		var clientId domain.ClientID
		var dispatchersString sql.NullString

		if err := rows.Scan(&account, &orgIdString, &clientId, &dispatchersString, &totalConnections); err != nil {
			logger.LogError("SQL scan failed.  Skipping row.", err)
			continue
		}

		orgId := domain.OrgID(orgIdString.String)

		var dispatchers domain.Dispatchers
		if dispatchersString.Valid {
			err = json.Unmarshal([]byte(dispatchersString.String), &dispatchers)
			if err != nil {
				logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, orgId, clientId)
			}
		}

		proxy, err := scm.proxyFactory.CreateProxy(ctx, orgId, domain.AccountID(account), domain.ClientID(clientId), dispatchers)
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
