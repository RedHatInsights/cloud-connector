package mqtt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
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

func (scm *SqlConnectionRegistrar) Register(ctx context.Context, account domain.AccountID, client_id domain.ClientID, client controller.Receptor) (controller.RegistrationResults, error) {

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": client_id})

	update := "UPDATE connections SET dispatchers=$1, updated_at = NOW() WHERE account=$2 AND client_id=$3"
	insert := "INSERT INTO connections (account, client_id, dispatchers, canonical_facts) SELECT $4, $5, $6, $7"
	insertOrUpdate := fmt.Sprintf("WITH upsert AS (%s RETURNING *) %s WHERE NOT EXISTS (SELECT * FROM upsert)", update, insert)

	statement, err := scm.database.Prepare(insertOrUpdate)
	if err != nil {
		logger.Fatal(err)
	}
	defer statement.Close()

	dispatchers, err := client.GetDispatchers(ctx)
	if err != nil {
		logger.Fatal(err)
	}

	dispatchersString, err := json.Marshal(dispatchers)
	if err != nil {
		logger.Fatal(err)
	}

	canonicalFactsString := "{\"fqdn\": \"fred.flintstone.com\"}"
	//canonicalFactsString = "{}"

	results, err := statement.Exec(dispatchersString, account, client_id, account, client_id, dispatchersString, canonicalFactsString)
	if err != nil {
		logger.Fatal(err)
	}

	rowsAffected, err := results.RowsAffected()
	if err != nil {
		logger.Fatal(err)
	}

	var registrationResults controller.RegistrationResults
	if rowsAffected == 0 {
		registrationResults = controller.ExistingConnection
	} else if rowsAffected == 1 {
		registrationResults = controller.NewConnection
	} else {
		logger.Warn("Unable to determine registration results: rowsAffected:", rowsAffected)
		return controller.NewConnection, errors.New("Unable to determine registration results")
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
