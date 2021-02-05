package mqtt

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type SqlConnectionRegistrar struct {
	database *sql.DB
}

func initializeSqlDatabase(cfg *config.Config) (*sql.DB, error) {

	psqlConnectionInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.ConnectionDatabaseHost,
		cfg.ConnectionDatabasePort,
		cfg.ConnectionDatabaseUser,
		cfg.ConnectionDatabasePassword,
		cfg.ConnectionDatabaseName)

	database, err := sql.Open("postgres", psqlConnectionInfo)
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

func NewSqlConnectionRegistrar(cfg *config.Config) (*SqlConnectionRegistrar, error) {

	database, err := initializeSqlDatabase(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionRegistrar{
		database: database,
	}, nil
}

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, account string, client_id string, client controller.Receptor) error {

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

func (scm *SqlConnectionRegistrar) Unregister(ctx context.Context, account string, client_id string) {
	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

	statement, err := scm.database.Prepare("DELETE FROM connections WHERE account = $1 AND client_id = $2")
	if err != nil {
		logger.Fatal(err)
	}
	defer statement.Close()

	_, err = statement.Exec(account, client_id)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Debug("Unregistered a connection")
}
