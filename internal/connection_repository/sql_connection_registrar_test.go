//go:build sql
// +build sql

package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
)

func init() {
	logger.InitLogger()
}

func TestSqlConnectionRegistrar(t *testing.T) {

	cfg := config.GetConfig()

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	testCases := []struct {
		testName    string
		account     domain.AccountID
		clientID    domain.ClientID
		dispatchers string
	}{
		{"with no dispatchers", "999999", "registrar-test-client-1", "{}"},
		{"with satellite dispatchers", "888888", "registrar-test-client-2", "{\"satellite\": {\"version\": \"0.2\"}}"},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			var dispatchers map[string]interface{}
			err := json.Unmarshal([]byte(tc.dispatchers), &dispatchers)
			if err != nil {
				t.Fatal("unexpected error while creating dispatchers json doc", err)
			}

			var connectionRegistrar ConnectionRegistrar
			connectionRegistrar, err = NewSqlConnectionRegistrar(cfg, database)
			if err != nil {
				t.Fatal("unexpected error while creating the SqlConnectionRegistrar", err)
			}

			connectorClientState := domain.ConnectorClientState{
				Account:     tc.account,
				ClientID:    tc.clientID,
				Dispatchers: dispatchers,
			}

			err = connectionRegistrar.Register(context.TODO(), connectorClientState)
			if err != nil {
				t.Fatal("unexpected error while registering a connection", err)
			}

			actualClientState, err := connectionRegistrar.FindConnectionByClientID(context.TODO(), tc.clientID)
			if err != nil {
				t.Fatal("unexpected error while looking up a connection", err)
			}

			verifyConnectorClientState(t, connectorClientState, actualClientState)

			verifyStoredClientState(t, database, tc.account, tc.clientID, []string{})

			err = connectionRegistrar.Unregister(context.TODO(), tc.clientID)
			if err != nil {
				t.Fatal("unexpected error while registering a connection", err)
			}

			_, err = connectionRegistrar.FindConnectionByClientID(context.TODO(), tc.clientID)
			if err != NotFoundError {
				t.Fatal("found a connection when the connection was not supposed to exist", err)
			}

		})
	}
}

func verifyConnectorClientState(t *testing.T, expectedClientState, actualClientState domain.ConnectorClientState) {
	if expectedClientState.Account != actualClientState.Account ||
		expectedClientState.ClientID != actualClientState.ClientID {
		t.Fatal("actual client state does not match expected client state", actualClientState, expectedClientState)
	}
}

func verifyStoredClientState(t *testing.T, database *sql.DB, account domain.AccountID, clientID domain.ClientID, permittedTenants []string) {

	statement, err := database.Prepare(`SELECT permitted_tenants FROM connections
    WHERE account = $1 AND
    client_id = $2`)
	if err != nil {
		t.Fatal("sql prepare failed", err)
		return
	}
	defer statement.Close()

	var permittedTenantsFromDatabase string
	err = statement.QueryRow(account, clientID).Scan(&permittedTenantsFromDatabase)
	if err != nil {
		t.Fatal("sql prepare failed", err)
		return
	}

	var actualPermittedTenents []string
	json.Unmarshal([]byte(permittedTenantsFromDatabase), &actualPermittedTenents)

	if reflect.DeepEqual(actualPermittedTenents, permittedTenants) == false {
		t.Fatalf("expected permitted tenants to be %s, but got %s", permittedTenants, actualPermittedTenents)
		return
	}

	return
}
