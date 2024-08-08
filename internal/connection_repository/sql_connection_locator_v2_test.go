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

	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

func TestSqlConnectionLocatorV2PermittedTenant(t *testing.T) {

	cfg := config.GetConfig()

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	getConnectionByClientID, err := NewPermittedTenantSqlGetConnectionByClientID(cfg, database)

	testCases := []struct {
		testName       string
		orgID          domain.OrgID
		account        domain.AccountID
		clientID       domain.ClientID
		dispatchers    string
		requestedOrgID domain.OrgID
		expectedError  error
	}{
		{"with no dispatchers", "999991", "999999", "sql-locator-v2-test-client-1", "{}", "999991", nil},
		{"with satellite dispatchers", "888881", "888888", "sql-locator-v2-test-client-2", "{\"satellite\": {\"version\": \"0.2\"}}", "888881", nil},
		{"with satellite dispatchers", "888881", "888888", "sql-locator-v2-test-client-3", "{\"satellite\": {\"version\": \"0.2\"}}", "different_should_miss", NotFoundError},
		{"with " + satelliteWorker + " dispatchers", "888881", "888888", "sql-locator-v2-test-client-4", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "different_should_match", nil},
		{"with no account", "", "999992", "sql-locator-v2-test-client-5", "{}", "9999", NotFoundError},     // anemic tenant
		{"with no account or org-id", "", "", "sql-locator-v2-test-client-6", "{}", "9999", NotFoundError}, // ghost connection
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			log := logger.Log.WithFields(logrus.Fields{"test_name": tc.testName})

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

			actualClientState, err := getConnectionByClientID(context.TODO(), log, tc.requestedOrgID, tc.clientID)
			if err != tc.expectedError {
				t.Fatal("unexpected error while looking up a connection", err)
			}

			if err != NotFoundError {
				verifyConnectorClientState(t, connectorClientState, actualClientState)
			}

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
