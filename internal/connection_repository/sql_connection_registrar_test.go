//go:build sql
// +build sql

package connection_repository

import (
	"context"
	"encoding/json"
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
	t.Fatal("Implement this test!!", nil)
}

func verifyConnectorClientState(t *testing.T, expectedClientState, actualClientState domain.ConnectorClientState) {
	if expectedClientState.Account != actualClientState.Account ||
		expectedClientState.OrgID != actualClientState.OrgID ||
		expectedClientState.ClientID != actualClientState.ClientID {
		t.Fatal("actual client state does not match expected client state", actualClientState, expectedClientState)
	}
}
