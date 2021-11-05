package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, rhcClient domain.ConnectorClientState) (RegistrationResults, error) {

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionRegistrationDuration)
	defer callDurationTimer.ObserveDuration()

	account := rhcClient.Account
	client_id := rhcClient.ClientID

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

	update := "UPDATE connections SET dispatchers=$1, tags = $2, updated_at = NOW(), message_id = $3, message_sent = $4, state=1 WHERE account=$5 AND client_id=$6"
	insert := "INSERT INTO connections (account, client_id, dispatchers, canonical_facts, tags, message_id, message_sent, state) SELECT $7, $8, $9, $10, $11, $12, $13, 1"
	insertOrUpdate := fmt.Sprintf("WITH upsert AS (%s RETURNING *) %s WHERE NOT EXISTS (SELECT * FROM upsert)", update, insert)

	statement, err := scm.database.Prepare(insertOrUpdate)
	if err != nil {
		logger.Fatal(err)
	}
	defer statement.Close()

	dispatchersString, err := json.Marshal(rhcClient.Dispatchers)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "dispatchers": rhcClient.Dispatchers}).Error("Unable to marshal dispatchers")
		return NewConnection, err
	}

	canonicalFactsString, err := json.Marshal(rhcClient.CanonicalFacts)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "canonical_facts": rhcClient.CanonicalFacts}).Error("Unable to marshal canonicalfacts")
		return NewConnection, err
	}

	tagsString, err := json.Marshal(rhcClient.Tags)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err, "tags": rhcClient.CanonicalFacts}).Error("Unable to marshal tags")
		return NewConnection, err
	}

	results, err := statement.Exec(dispatchersString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp, account, client_id, account, client_id, dispatchersString, canonicalFactsString, tagsString, rhcClient.MessageMetadata.LatestMessageID, rhcClient.MessageMetadata.LatestTimestamp)
	if err != nil {
		logger.Fatal(err)
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		logger.Fatal(err)
	}

	var registrationResults RegistrationResults
	if rowsAffected == 0 {
		registrationResults = ExistingConnection
	} else if rowsAffected == 1 {
		registrationResults = NewConnection
	} else {
		logger.Warn("Unable to determine registration results: rowsAffected:", rowsAffected)
		return NewConnection, errors.New("Unable to determine registration results")
	}

	logger.Debug("Registered a connection")
	return registrationResults, nil
}

func (scm *SqlConnectionRegistrar) Unregister(ctx context.Context, client_id domain.ClientID) {

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionUnregistrationDuration)
	defer callDurationTimer.ObserveDuration()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": client_id})

	statement, err := scm.database.Prepare("UPDATE connections SET state=0 WHERE client_id = $1")
	if err != nil {
		logger.Fatal(err)
	}
	defer statement.Close()

	_, err = statement.Exec(client_id)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debug("Unregistered a connection")
}

func (scm *SqlConnectionRegistrar) FindConnectionByClientID(ctx context.Context, client_id domain.ClientID) (domain.ConnectorClientState, error) {
	var connectorClient domain.ConnectorClientState
	var err error

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionLookupByClientIDDuration)
	defer callDurationTimer.ObserveDuration()

	statement, err := scm.database.Prepare("SELECT account, client_id, dispatchers, canonical_facts, tags, message_id, message_sent FROM connections WHERE client_id = $1")
	if err != nil {
		logger.LogFatalError("SQL Prepare failed", err)
		return connectorClient, err
	}
	defer statement.Close()

	var account domain.AccountID
	var dispatchersString string
	var canonicalFactsString string
	var tagsString string

	err = statement.QueryRow(client_id).Scan(&account,
		&connectorClient.ClientID,
		&dispatchersString,
		&canonicalFactsString,
		&tagsString,
		&connectorClient.MessageMetadata.LatestMessageID,
		&connectorClient.MessageMetadata.LatestTimestamp)

	if err != nil {
		if err != sql.ErrNoRows {
			logger.LogFatalError("SQL query failed:", err)
		}
		return connectorClient, err
	}

	connectorClient.Account = account

	err = json.Unmarshal([]byte(dispatchersString), &connectorClient.Dispatchers)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to unmarshal dispatchers from database", err, account, client_id)
		return connectorClient, err
	}

	err = json.Unmarshal([]byte(canonicalFactsString), &connectorClient.CanonicalFacts)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to unmarshal canonical facts from database", err, account, client_id)
		return connectorClient, err
	}

	err = json.Unmarshal([]byte(tagsString), &connectorClient.Tags)
	if err != nil {
		logger.LogErrorWithAccountAndClientId("Unable to unmarshal tags from database", err, account, client_id)
		return connectorClient, err
	}

	return connectorClient, nil
}

func (scm *SqlConnectionRegistrar) ReenableConnection(ctx context.Context, account domain.AccountID, client_id domain.ClientID) error {
	var err error

	/*
		callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionLookupByClientIDDuration)
		defer callDurationTimer.ObserveDuration()
	*/

	// FIXME:  What about message_id / message_sent??
	updateStatement := "UPDATE connections SET state=1, updated_at = NOW() WHERE account=$1 AND client_id=$2 AND state=0"

	statement, err := scm.database.Prepare(updateStatement)
	if err != nil {
		logger.Log.Fatal(err) // FIXME:??
		return err
	}
	defer statement.Close()

	results, err := statement.Exec(account, client_id)
	if err != nil {
		logger.Log.Fatal(err)
		return err
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		logger.Log.Fatal(err)
		return err
	}

	fmt.Println("rowsAffected: ", rowsAffected)

	return nil
}
