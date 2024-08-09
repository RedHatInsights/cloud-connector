//go:build sql
// +build sql

package connection_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

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
		orgID       domain.OrgID
		account     domain.AccountID
		clientID    domain.ClientID
		dispatchers string
	}{
		{"with no dispatchers", "999991", "999999", "registrar-test-client-1", "{}"},
		{"with satellite dispatchers", "888881", "888888", "registrar-test-client-2", "{\"satellite\": {\"version\": \"0.2\"}}"},
		{"with no account", "", "999992", "registrar-test-client-3", "{}"},     // anemic tenant
		{"with no account or org-id", "", "", "registrar-test-client-4", "{}"}, // ghost connection
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
				OrgID:       tc.orgID,
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

func TestSqlConnectionRegistrarUpdateTenantlessConnection(t *testing.T) {
	cfg := config.GetConfig()

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	orgID := domain.OrgID("")
	account := domain.AccountID("")
	clientID := domain.ClientID("update-tenantless-test-client-1")
	dispatchersString := "{\"satellite\": {\"version\": \"0.2\"}}"

	var dispatchers map[string]interface{}
	err = json.Unmarshal([]byte(dispatchersString), &dispatchers)
	if err != nil {
		t.Fatal("unexpected error while creating dispatchers json doc", err)
	}

	var connectionRegistrar ConnectionRegistrar
	connectionRegistrar, err = NewSqlConnectionRegistrar(cfg, database)
	if err != nil {
		t.Fatal("unexpected error while creating the SqlConnectionRegistrar", err)
	}

	verifyConnectionCountByClientID(t, database, clientID, 0)

	connectorClientState := domain.ConnectorClientState{
		OrgID:       orgID,
		Account:     account,
		ClientID:    clientID,
		Dispatchers: dispatchers,
	}

	err = connectionRegistrar.Register(context.TODO(), connectorClientState)
	if err != nil {
		t.Fatal("unexpected error while registering a connection", err)
	}

	verifyConnectionCountByClientID(t, database, connectorClientState.ClientID, 1)

	connectorClientState.OrgID = "1234"
	connectorClientState.Account = "4321"

	err = connectionRegistrar.Register(context.TODO(), connectorClientState)
	if err != nil {
		t.Fatal("unexpected error while registering a connection", err)
	}

	verifyConnectionCountByClientID(t, database, connectorClientState.ClientID, 1)

	err = connectionRegistrar.Unregister(context.TODO(), connectorClientState.ClientID)
	if err != nil {
		t.Fatal("unexpected error while registering a connection", err)
	}

}

func verifyConnectionCountByClientID(t *testing.T, database *sql.DB, clientID domain.ClientID, expectedConnectionCount int) {

	connectionCount, err := getConnectionCountFromDatabase(database, clientID)
	if err != nil {
		t.Fatal("unexpected error while looking for connections by client-id", err)
	}

	if connectionCount != expectedConnectionCount {
		t.Fatalf("expected connection count (%d) did not match connection count found in database (%d)", expectedConnectionCount, connectionCount)
	}
}

func getConnectionCountFromDatabase(database *sql.DB, clientId domain.ClientID) (int, error) {

	sqlQuery := "SELECT COUNT(*) FROM CONNECTIONS WHERE client_id = $1"

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	statement, err := database.Prepare(sqlQuery)
	if err != nil {
		return 0, err
	}
	defer statement.Close()

	var count int

	err = statement.QueryRowContext(ctx, clientId).Scan(&count)

	if err != nil {
		return 0, err
	}

	return count, nil
}

func verifyConnectorClientState(t *testing.T, expectedClientState, actualClientState domain.ConnectorClientState) {
	if expectedClientState.Account != actualClientState.Account ||
		expectedClientState.OrgID != actualClientState.OrgID ||
		expectedClientState.ClientID != actualClientState.ClientID {
		t.Fatal("actual client state does not match expected client state", actualClientState, expectedClientState)
	}
}
