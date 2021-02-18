package mqtt

import (
	"context"
	"database/sql"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type SqlConnectionLocator struct {
	database     *sql.DB
	proxyFactory controller.ReceptorProxyFactory
}

func NewSqlConnectionLocator(cfg *config.Config, proxyFactory controller.ReceptorProxyFactory) (*SqlConnectionLocator, error) {

	database, err := initializeDatabaseConnection(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionLocator{
		database:     database,
		proxyFactory: proxyFactory,
	}, nil
}

func (scm *SqlConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, client_id domain.ClientID) controller.Receptor {
	var conn controller.Receptor
	var err error

	statement, err := scm.database.Prepare("SELECT client_id FROM connections WHERE account = $1 AND client_id = $2")
	if err != nil {
		logError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	var name string
	err = statement.QueryRow(account, client_id).Scan(&name)
	if err != nil {
		logError("SQL query faile:", err)
		return nil
	}

	conn, err = scm.proxyFactory.CreateProxy(ctx, domain.AccountID(account), domain.ClientID(client_id))
	if err != nil {
		logError("Proxy creation failed", err)
		return nil
	}

	return conn
}

func (scm *SqlConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID) map[string]controller.Receptor {

	connectionsPerAccount := make(map[string]controller.Receptor)

	statement, err := scm.database.Prepare("SELECT client_id FROM connections WHERE account = $1")
	if err != nil {
		logError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.Query(account)
	if err != nil {
		logError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var client_id string
		if err := rows.Scan(&client_id); err != nil {
			logError("SQL scan failed", err)
			return nil
		}

		proxy, err := scm.proxyFactory.CreateProxy(ctx, domain.AccountID(account), domain.ClientID(client_id))
		if err != nil {
			logError("Proxy creation failed", err)
			return nil
		}

		connectionsPerAccount[client_id] = proxy
	}

	return connectionsPerAccount
}

func (scm *SqlConnectionLocator) GetAllConnections(ctx context.Context) map[string]map[string]controller.Receptor {

	connectionMap := make(map[string]map[string]controller.Receptor)

	statement, err := scm.database.Prepare("SELECT account, client_id FROM connections")
	if err != nil {
		logError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	rows, err := statement.Query()
	if err != nil {
		logError("SQL query failed", err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var account string
		var client_id string
		if err := rows.Scan(&account, &client_id); err != nil {
			logError("SQL scan failed", err)
			return nil
		}

		proxy, err := scm.proxyFactory.CreateProxy(ctx, domain.AccountID(account), domain.ClientID(client_id))
		if err != nil {
			logError("Proxy creation failed", err)
			return nil
		}

		if _, exists := connectionMap[account]; !exists {
			connectionMap[account] = make(map[string]controller.Receptor)
		}

		connectionMap[account][client_id] = proxy
	}

	return connectionMap
}

func logError(msg string, err error) {
	logger.Log.WithFields(logrus.Fields{"error": err}).Error(msg)
}
