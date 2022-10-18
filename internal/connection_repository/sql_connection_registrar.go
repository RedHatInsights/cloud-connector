package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type SqlConnectionRegistrar struct {
	database     *gorm.DB
	queryTimeout time.Duration
}

func NewSqlConnectionRegistrar(cfg *config.Config, database *gorm.DB) (*SqlConnectionRegistrar, error) {
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

	ctx, cancel := context.WithTimeout(ctx, scm.queryTimeout)
	defer cancel()

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "org_id": org_id, "client_id": client_id})

	dispatchersString, err := json.Marshal(rhcClient.Dispatchers)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "dispatchers": rhcClient.Dispatchers}).Error("Unable to marshal dispatchers")
		return err
	}

	canonicalFactsString, err := json.Marshal(rhcClient.CanonicalFacts)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "canonical_facts": rhcClient.CanonicalFacts}).Error("Unable to marshal canonical_facts")
		return err
	}

	tagsString, err := json.Marshal(rhcClient.Tags)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "tags": rhcClient.CanonicalFacts}).Error("Unable to marshal tags")
		return err
	}

	// gorm does not seem to support INSERT INTO ... SELECT ... so we need to use raw sql here
	query := scm.database.
		Exec("WITH upsert AS (UPDATE connections SET dispatchers=?, tags = ?, updated_at = NOW(), message_id = ?, message_sent = ? WHERE account=? AND client_id=? RETURNING *) "+
			"INSERT INTO connections (account, org_id, client_id, dispatchers, canonical_facts, tags, message_id, message_sent) "+
			"SELECT ?, ?, ?, ?, ?, ?, ?, ? "+
			"WHERE NOT EXISTS (SELECT * FROM upsert) RETURNING *",
			dispatchersString,
			tagsString,
			rhcClient.MessageMetadata.LatestMessageID,
			rhcClient.MessageMetadata.LatestTimestamp,
			account,
			client_id,
			account,
			org_id,
			client_id,
			dispatchersString,
			canonicalFactsString,
			tagsString,
			rhcClient.MessageMetadata.LatestMessageID,
			rhcClient.MessageMetadata.LatestTimestamp,
		)

	if query.Error != nil {
		logger.WithFields(logrus.Fields{"error": query.Error}).Error("Insert/update failed")
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

	query := scm.database.Delete(&Connection{}, "client_id = ?", client_id)
	if query.Error != nil {
		logger.WithFields(logrus.Fields{"error": query.Error}).Error("Delete failed")
		return FatalError{query.Error}
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

	row := scm.database.
		Table("connections").
		Select("account", "org_id", "client_id", "dispatchers", "canonical_facts", "tags", "message_id", "message_sent").
		Where("client_id = ?", client_id).
		Row()

	var account domain.AccountID
	var orgID sql.NullString
	var dispatchersString sql.NullString
	var canonicalFactsString sql.NullString
	var tagsString sql.NullString
	var latestMessageID sql.NullString

	err = row.Scan(&account,
		&orgID,
		&connectorClient.ClientID,
		&dispatchersString,
		&canonicalFactsString,
		&tagsString,
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

	connectorClient.Account = account

	if orgID.Valid {
		connectorClient.OrgID = domain.OrgID(orgID.String)
	}

	logger = logger.WithFields(logrus.Fields{"account": account, "org_id": connectorClient.OrgID})

	if dispatchersString.Valid {
		err = json.Unmarshal([]byte(dispatchersString.String), &connectorClient.Dispatchers)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal dispatchers from database")
		}
	}

	if canonicalFactsString.Valid {
		err = json.Unmarshal([]byte(canonicalFactsString.String), &connectorClient.CanonicalFacts)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal canonical facts from database")
		}
	}

	if tagsString.Valid {
		err = json.Unmarshal([]byte(tagsString.String), &connectorClient.Tags)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal tags from database")
		}
	}

	if latestMessageID.Valid {
		connectorClient.MessageMetadata.LatestMessageID = latestMessageID.String
	}

	return connectorClient, nil
}
