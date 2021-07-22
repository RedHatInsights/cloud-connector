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
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type SqlConnectionRegistrar struct {
	database *sql.DB
	metrics  *sqlConnectionRegistrarMetrics
}

type sqlConnectionRegistrarMetrics struct {
	sqlConnectionRegistrationDuration   prometheus.Histogram
	sqlConnectionUnregistrationDuration prometheus.Histogram
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

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, rhcClient domain.RhcClient) (RegistrationResults, error) {

	callDurationTimer := prometheus.NewTimer(scm.metrics.sqlConnectionRegistrationDuration)
	defer callDurationTimer.ObserveDuration()

	account := rhcClient.Account
	client_id := rhcClient.ClientID

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

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

	var connection Connection
	connection.Account = string(rhcClient.Account)
	connection.ClientID = string(rhcClient.ClientID)
	connection.Dispatchers = string(dispatchersString)
	connection.CanonicalFacts = string(canonicalFactsString)
	connection.Tags = string(tagsString)
	connection.StaleTimestamp = time.Now()

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: scm.database}),
		&gorm.Config{})

	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Gorm open failed")
		return NewConnection, nil
	}

	query := db.Table("connections")

    results := query.Where(Connection{Account: connection.Account, ClientID: connection.ClientID}).Assign(connection).FirstOrCreate(&connection)

	if results.Error != nil {
		logger.WithFields(logrus.Fields{"error": results.Error}).Error("SQL query failed")
		return NewConnection, nil
	}

	fmt.Println("*** results:", results)

	rowsAffected := results.RowsAffected
	fmt.Println("*** rowsAffected:", rowsAffected)

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

	statement, err := scm.database.Prepare("DELETE FROM connections WHERE client_id = $1")
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
