package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type PermittedTenantConnectionLocator struct {
	database     *sql.DB
	proxyFactory controller.ConnectorClientProxyFactory
}

func NewPermittedTenantConnectionLocator(cfg *config.Config, database *sql.DB, proxyFactory controller.ConnectorClientProxyFactory) (*PermittedTenantConnectionLocator, error) {

	return &PermittedTenantConnectionLocator{
		database:     database,
		proxyFactory: proxyFactory,
	}, nil
}

func (pacl *PermittedTenantConnectionLocator) GetConnection(ctx context.Context, account domain.AccountID, org_id domain.OrgID, client_id domain.ClientID) controller.ConnectorClient {
	var conn controller.ConnectorClient
	var err error

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionByAccountOrPermittedTenantAndClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	log := logger.Log.WithFields(logrus.Fields{"client_id": client_id,
		"account": account,
		"org_id":  org_id})

	if account == "" || org_id == "" || client_id == "" {
		log.Warn("Missing required parameters (account, org_id, client_id)")
		return nil
	}

	log.Info("Searching for connection")

	// Match a connection if the account number matches or if the org_id is within the permitted_tenants list
	statement, err := pacl.database.Prepare(`SELECT account, org_id, client_id, dispatchers, permitted_tenants FROM connections
        WHERE (account = $1 OR (org_id = $2 OR permitted_tenants @> to_jsonb($2::text)) )
        AND client_id = $3`)
	if err != nil {
		logger.LogError("SQL Prepare failed", err)
		return nil
	}
	defer statement.Close()

	var connectionAccount string
	var orgId sql.NullString
	var clientId string
	var dispatchersString sql.NullString
	var permittedTenants sql.NullString

	err = statement.QueryRow(account, org_id, client_id).Scan(&connectionAccount, &orgId, &clientId, &dispatchersString, &permittedTenants)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.LogError("SQL query failed:", err)
		}
		return nil
	}

	var connectionOrgId string
	if orgId.Valid {
		connectionOrgId = orgId.String
	}

	log = log.WithFields(logrus.Fields{
		"connection_account": connectionAccount,
		"connection_org_id":  connectionOrgId,
		"permitted_tenants":  permittedTenants})

	log.Debug("Connection found")

	var dispatchers domain.Dispatchers
	if dispatchersString.Valid {
		err = json.Unmarshal([]byte(dispatchersString.String), &dispatchers)
		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal dispatchers from database")
		}
	}

	if connectionAccount != string(account) && connectionOrgId != string(org_id) && org_id != "" {
		log.Info("Connection located based on permitted tenant match")
	}

	conn, err = pacl.proxyFactory.CreateProxy(ctx, domain.AccountID(connectionAccount), domain.ClientID(client_id), dispatchers)
	if err != nil {
		log.WithFields(logrus.Fields{"error": err}).Error("Unable to create the proxy")
		return nil
	}

	return conn
}

func (pacl *PermittedTenantConnectionLocator) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {
	return nil, 0, errors.New("Not implemented!")
}

func (pacl *PermittedTenantConnectionLocator) GetAllConnections(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {
	return nil, 0, errors.New("Not implemented!")
}
