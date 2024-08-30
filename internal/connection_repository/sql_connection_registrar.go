package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type SqlConnectionRegistrar struct {
	database     *sql.DB
	queryTimeout time.Duration
}

func NewSqlConnectionRegistrar(cfg *config.Config, database *sql.DB) (*SqlConnectionRegistrar, error) {
	return &SqlConnectionRegistrar{
		database:     database,
		queryTimeout: cfg.ConnectionDatabaseQueryTimeout,
	}, nil
}

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, rhcClient domain.ConnectorClientState) error {

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionRegistrationDuration)
	defer callDurationTimer.ObserveDuration()

	account := rhcClient.Account
	org_id := rhcClient.OrgID
	client_id := rhcClient.ClientID

	var resetTenantLookupCountClause string
	var tenantLookupTimestamp *time.Time

	if isTenantlessConnection(rhcClient) {
		fmt.Println("Registering tenantless connection")
		timestamp := time.Now()
		tenantLookupTimestamp = &timestamp
	} else {
		// If the connection has a tenant, then reset the tenant lookup failure count
		resetTenantLookupCountClause = ", tenant_lookup_failure_count = 0 "
	}

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": org_id, "client_id": client_id})

	update := fmt.Sprintf("UPDATE connections SET dispatchers=$1, tags = $2, updated_at = NOW(), message_id = $3, message_sent = $4, org_id = $5, account = $6, tenant_lookup_timestamp = $7 %s WHERE client_id=$8", resetTenantLookupCountClause)
	insert := "INSERT INTO connections (account, org_id, client_id, dispatchers, canonical_facts, tags, message_id, message_sent, tenant_lookup_timestamp) SELECT $9, $10, $11, $12, $13, $14, $15, $16, $17"

	insertOrUpdate := fmt.Sprintf("WITH upsert AS (%s RETURNING *) %s WHERE NOT EXISTS (SELECT * FROM upsert)", update, insert)

	statement, err := scm.database.Prepare(insertOrUpdate)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Prepare failed")
		return FatalError{err}
	}
	defer statement.Close()

	dispatchersString, err := json.Marshal(rhcClient.Dispatchers)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "dispatchers": rhcClient.Dispatchers}).Error("Unable to marshal dispatchers")
		return err
	}

	canonicalFactsString, err := json.Marshal(rhcClient.CanonicalFacts)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "canonical_facts": rhcClient.CanonicalFacts}).Error("Unable to marshal canonicalfacts")
		return err
	}

	tagsString, err := json.Marshal(rhcClient.Tags)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "tags": rhcClient.CanonicalFacts}).Error("Unable to marshal tags")
		return err
	}

	_, err = statement.ExecContext(ctx, dispatchersString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp, org_id, account, tenantLookupTimestamp, client_id, account, org_id, client_id, dispatchersString, canonicalFactsString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp, tenantLookupTimestamp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Insert/update failed")
		return FatalError{err}
	}

	logger.Debug("Registered a connection")
	return nil
}

func (scm *SqlConnectionRegistrar) Unregister(ctx context.Context, client_id domain.ClientID) error {

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionUnregistrationDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": client_id})

	statement, err := scm.database.Prepare("DELETE FROM connections WHERE client_id = $1")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Prepare failed")
		return FatalError{err}
	}
	defer statement.Close()

	_, err = statement.ExecContext(ctx, client_id)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Delete failed")
		return FatalError{err}
	}

	logger.Debug("Unregistered a connection")
	return nil
}

func (scm *SqlConnectionRegistrar) FindConnectionByClientID(ctx context.Context, client_id domain.ClientID) (domain.ConnectorClientState, error) {
	var connectorClient domain.ConnectorClientState
	var err error

	logger := logger.Log.WithFields(logrus.Fields{"client_id": client_id})

	callDurationTimer := prometheus.NewTimer(metrics.sqlConnectionLookupByClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	statement, err := scm.database.Prepare("SELECT account, org_id, client_id, dispatchers, canonical_facts, tags, message_id, message_sent FROM connections WHERE client_id = $1")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("SQL prepare failed")
		return connectorClient, FatalError{err}
	}
	defer statement.Close()

	var account sql.NullString
	var orgID domain.OrgID
	var serializedCanonicalFacts sql.NullString
	var serializedDispatchers sql.NullString
	var serializedTags sql.NullString
	var latestMessageID sql.NullString

	err = statement.QueryRowContext(ctx, client_id).Scan(&account,
		&orgID,
		&connectorClient.ClientID,
		&serializedDispatchers,
		&serializedCanonicalFacts,
		&serializedTags,
		&latestMessageID,
		&connectorClient.MessageMetadata.LatestTimestamp)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Debug("No connection found!")
			return connectorClient, NotFoundError
		} else if errors.Is(err, sql.ErrNoRows) == false {
			logger.WithFields(logrus.Fields{"error": err}).Error("SQL query failed")
			err = FatalError{err}
		}

		return connectorClient, err
	}

	connectorClient.OrgID = domain.OrgID(orgID)

	if account.Valid {
		connectorClient.Account = domain.AccountID(account.String)
	}

	logger = logger.WithFields(logrus.Fields{"account": connectorClient.Account, "org_id": connectorClient.OrgID})

	connectorClient.CanonicalFacts = deserializeCanonicalFacts(logger, serializedCanonicalFacts)
	connectorClient.Dispatchers = deserializeDispatchers(logger, serializedDispatchers)
	connectorClient.Tags = deserializeTags(logger, serializedTags)

	if latestMessageID.Valid {
		connectorClient.MessageMetadata.LatestMessageID = latestMessageID.String
	}

	return connectorClient, nil
}

func isTenantlessConnection(rhcClient domain.ConnectorClientState) bool {
	return len(rhcClient.OrgID) == 0
}
