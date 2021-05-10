package controller

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

type SqlConnectionLocator struct {
	database     *sql.DB
	proxyFactory ReceptorProxyFactory
}

func NewSqlConnectionLocator(cfg *config.Config, proxyFactory ReceptorProxyFactory) (*SqlConnectionLocator, error) {

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionLocator{
		database:     database,
		proxyFactory: proxyFactory,
	}, nil
}

func (scm *SqlConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, client_id domain.ClientID) Receptor {
	var conn Receptor
	var err error

	statement, err := scm.database.Prepare("SELECT client_id, dispatchers FROM connections WHERE account = $1 AND client_id = $2")
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

func (scm *SqlConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID) map[domain.ClientID]Receptor {

	connectionsPerAccount := make(map[domain.ClientID]Receptor)

	statement, err := scm.database.Prepare("SELECT client_id, dispatchers FROM connections WHERE account = $1")
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.Query(account)
	if err != nil {
		logger.LogError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var client_id domain.ClientID
		var dispatchersString string
		if err := rows.Scan(&client_id, &dispatchersString); err != nil {
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

	return connectionsPerAccount
}

func (scm *SqlConnectionLocator) GetAllConnections(ctx context.Context) map[domain.AccountID]map[domain.ClientID]Receptor {

	connectionMap := make(map[domain.AccountID]map[domain.ClientID]Receptor)

	statement, err := scm.database.Prepare("SELECT account, client_id, dispatchers FROM connections")
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.Query()
	if err != nil {
		logger.LogError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var account domain.AccountID
		var clientId domain.ClientID
		var dispatchersString string
		if err := rows.Scan(&account, &clientId, &dispatchersString); err != nil {
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
			connectionMap[account] = make(map[domain.ClientID]Receptor)
		}

		connectionMap[account][clientId] = proxy
	}

	return connectionMap
}
