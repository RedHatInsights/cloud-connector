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
	"github.com/sirupsen/logrus"
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

	log := logger.Log.WithFields(logrus.Fields{"client_id": client_id,
		"account": account,
		"org_id":  orgID})

	callDurationTimer := prometheus.NewTimer(metrics.sqlLookupConnectionByAccountAndClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	rows := scm.database.Table("connections").
		Select("client_id", "canonical_facts", "dispatchers", "tags").
		Where("account = ?", account).
		Where("client_id = ?", client_id).
		Row()

	if rows.Err() != nil {
		logger.LogError("SQL Query failed", rows.Err())
		return nil
	}

	var name string
	var serializedCanonicalFacts sql.NullString
	var serializedDispatchers sql.NullString
	var serializedTags sql.NullString

	err = rows.Scan(&name, &serializedCanonicalFacts, &serializedDispatchers, &serializedTags)

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

	rows, err := scm.database.
		Table("connections").
		Select("client_id", "org_id", "canonical_facts", "dispatchers", "tags", "COUNT(*) OVER()").
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
