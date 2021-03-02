package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

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

func InitializeDatabaseConnection(cfg *config.Config) (*sql.DB, error) {

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

	return database, nil
}
