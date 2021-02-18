package mqtt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type SqlConnectionRegistrar struct {
	database *sql.DB
}

func NewSqlConnectionRegistrar(cfg *config.Config) (*SqlConnectionRegistrar, error) {

	database, err := initializeDatabaseConnection(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionRegistrar{
		database: database,
	}, nil
}

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, account domain.AccountID, client_id domain.ClientID, client controller.Receptor) error {

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

	statement, err := scm.database.Prepare("INSERT INTO connections (account, client_id) VALUES ($1, $2)")
	if err != nil {
		logger.Fatal(err)
	}
	defer statement.Close()

	_, err = statement.Exec(account, client_id)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debug("Registered a connection")
	return nil
}

func (scm *SqlConnectionRegistrar) Unregister(ctx context.Context, client_id domain.ClientID) {
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

func initializePostgresConnection(cfg *config.Config) (*sql.DB, error) {
	psqlConnectionInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.ConnectionDatabaseHost,
		cfg.ConnectionDatabasePort,
		cfg.ConnectionDatabaseUser,
		cfg.ConnectionDatabasePassword,
		cfg.ConnectionDatabaseName)

	return sql.Open("postgres", psqlConnectionInfo)
}

func initializeSqliteConnection(cfg *config.Config) (*sql.DB, error) {
	return sql.Open("sqlite3", cfg.ConnectionDatabaseSqliteFile)
}

func initializeDatabaseConnection(cfg *config.Config) (*sql.DB, error) {

	var database *sql.DB
	var err error

	if cfg.ConnectionDatabaseImpl == "postgres" {
		database, err = initializePostgresConnection(cfg)
	} else if cfg.ConnectionDatabaseImpl == "sqlite3" {
		database, err = initializeSqliteConnection(cfg)
	} else {
		return nil, errors.New("Invalid SQL database impl requested")
	}

	if err != nil {
		return nil, err
	}

	statement, err := database.Prepare(
		"CREATE TABLE IF NOT EXISTS connections (id SERIAL PRIMARY KEY, account VARCHAR(10), client_id VARCHAR(100))")
	if err != nil {
		return nil, err
	}
	defer statement.Close()

	_, err = statement.Exec()
	if err != nil {
		return nil, err
	}

	return database, nil
}
