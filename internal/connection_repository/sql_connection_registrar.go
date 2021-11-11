package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

type SqlConnectionRegistrar struct {
	database *sql.DB
	metrics  *sqlConnectionRegistrarMetrics
}

type sqlConnectionRegistrarMetrics struct {
	sqlConnectionRegistrationDuration     prometheus.Histogram
	sqlConnectionUnregistrationDuration   prometheus.Histogram
	sqlConnectionLookupByClientIDDuration prometheus.Histogram
}

func initializeSqlConnectionRegistrationMetrics() *sqlConnectionRegistrarMetrics {
	metrics := new(sqlConnectionRegistrarMetrics)

	metrics.sqlConnectionRegistrationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_register_connection_duration",
		Help: "The amount of time the it took to register a connection in the db",
	})

	metrics.sqlConnectionUnregistrationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_unregister_connection_duration",
		Help: "The amount of time the it took to unregister a connection in the db",
	})

	metrics.sqlConnectionLookupByClientIDDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "cloud_connector_sql_lookup_connection_by_client_id_duration",
		Help: "The amount of time the it took to register a connection in the db",
	})

	return metrics
}

func NewSqlConnectionRegistrar(cfg *config.Config) (*SqlConnectionRegistrar, error) {

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionRegistrar{
		database: database,
		metrics:  initializeSqlConnectionRegistrationMetrics(),
	}, nil
}

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, rhcClient domain.ConnectorClientState) error {

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionRegistrationDuration)
	defer callDurationTimer.ObserveDuration()

	account := rhcClient.Account
	client_id := rhcClient.ClientID

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

	update := "UPDATE connections SET dispatchers=$1, tags = $2, updated_at = NOW(), message_id = $3, message_sent = $4 WHERE account=$5 AND client_id=$6"
	insert := "INSERT INTO connections (account, client_id, dispatchers, canonical_facts, tags, message_id, message_sent) SELECT $7, $8, $9, $10, $11, $12, $13"
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

	_, err = statement.Exec(dispatchersString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp, account, client_id, account, client_id, dispatchersString, canonicalFactsString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Insert/update failed")
		return FatalError{err}
	}

	logger.Debug("Registered a connection")
	return nil
}

func (scm *SqlConnectionRegistrar) Unregister(ctx context.Context, client_id domain.ClientID) error {

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionUnregistrationDuration)
	defer callDurationTimer.ObserveDuration()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": client_id})

	statement, err := scm.database.Prepare("DELETE FROM connections WHERE client_id = $1")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Prepare failed")
		return FatalError{err}
	}
	defer statement.Close()

	_, err = statement.Exec(client_id)
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

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionLookupByClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	statement, err := scm.database.Prepare("SELECT account, client_id, dispatchers, canonical_facts, tags, message_id, message_sent FROM connections WHERE client_id = $1")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("SQL prepare failed")
		return connectorClient, FatalError{err}
	}
	defer statement.Close()

	var account domain.AccountID
	var dispatchersString sql.NullString
	var canonicalFactsString sql.NullString
	var tagsString sql.NullString

	err = statement.QueryRow(client_id).Scan(&account,
		&connectorClient.ClientID,
		&dispatchersString,
		&canonicalFactsString,
		&tagsString,
		&connectorClient.MessageMetadata.LatestMessageID,
		&connectorClient.MessageMetadata.LatestTimestamp)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.WithFields(logrus.Fields{"error": err}).Error("SQL query failed")
			err = FatalError{err}
		}
		return connectorClient, err
	}

	connectorClient.Account = account

	logger := logger.Log.WithFields(logrus.Fields{"account": account})

	if dispatchersString.Valid {
		err = json.Unmarshal([]byte(dispatchersString.String), &connectorClient.Dispatchers)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal dispatchers from database")
			return connectorClient, err
		}
	}

	if canonicalFactsString.Valid {
		err = json.Unmarshal([]byte(canonicalFactsString.String), &connectorClient.CanonicalFacts)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal canonical facts from database")
			return connectorClient, err
		}
	}

	if tagsString.Valid {
		err = json.Unmarshal([]byte(tagsString.String), &connectorClient.Tags)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Unable to unmarshal tags from database")
			return connectorClient, err
		}
	}

	return connectorClient, nil
}
