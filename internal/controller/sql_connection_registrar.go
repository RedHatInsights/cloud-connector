package controller

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

	"github.com/sirupsen/logrus"
)

type SqlConnectionRegistrar struct {
	database *sql.DB
}

func NewSqlConnectionRegistrar(cfg *config.Config) (*SqlConnectionRegistrar, error) {

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		return nil, err
	}

	return &SqlConnectionRegistrar{
		database: database,
	}, nil
}

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, rhcClient domain.RhcClient) (RegistrationResults, error) {

	account := rhcClient.Account
	client_id := rhcClient.ClientID

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

	update := "UPDATE connections SET dispatchers=$1, updated_at = NOW() WHERE account=$2 AND client_id=$3"
	insert := "INSERT INTO connections (account, client_id, dispatchers, canonical_facts) SELECT $4, $5, $6, $7"
	insertOrUpdate := fmt.Sprintf("WITH upsert AS (%s RETURNING *) %s WHERE NOT EXISTS (SELECT * FROM upsert)", update, insert)

	statement, err := scm.database.Prepare(insertOrUpdate)
	if err != nil {
		logger.Fatal(err)
	}
	defer statement.Close()

	dispatchersString, err := json.Marshal(rhcClient.Dispatchers)
	if err != nil {
		logger.Fatal(err) // FIXME:??
	}
	fmt.Printf("\n\n")
	fmt.Println("rhcClient.Dispatchers:", rhcClient.Dispatchers)
	fmt.Println("dispatchersString:", dispatchersString)

	canonicalFactsString, err := json.Marshal(rhcClient.CanonicalFacts)
	if err != nil {
		logger.Fatal(err) // FIXME:??
	}

	fmt.Println("rhcClient.CanonicalFacts:", rhcClient.CanonicalFacts)
	fmt.Println("canonicalFactsString:", canonicalFactsString)

	results, err := statement.Exec(dispatchersString, account, client_id, account, client_id, dispatchersString, canonicalFactsString)
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
