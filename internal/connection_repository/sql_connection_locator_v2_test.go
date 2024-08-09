//go:build sql
// +build sql

package connection_repository

import (
	"context"
	"database/sql"
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

type connectionLocatorV2TestCaseData struct {
	testName       string
	orgID          domain.OrgID
	account        domain.AccountID
	clientID       domain.ClientID
	dispatchers    string
	requestedOrgID domain.OrgID
	expectedError  error
}

func TestSqlConnectionLocatorV2StrictImplementation(t *testing.T) {

	cfg := config.GetConfig()

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	getConnectionByClientID, err := NewSqlGetConnectionByClientID(cfg, database)

	testCases := []connectionLocatorV2TestCaseData{
		connectionLocatorV2TestCaseData{"with no dispatchers", "999991", "999999", "sql-locator-v2-strict-test-client-1", "{}", "999991", nil},
		connectionLocatorV2TestCaseData{"with satellite dispatchers", "888881", "888888", "sql-locator-v2-strict-test-client-2", "{\"satellite\": {\"version\": \"0.2\"}}", "888881", nil},
		connectionLocatorV2TestCaseData{"with satellite dispatchers", "888881", "888888", "sql-locator-v2-strict-test-client-3", "{\"satellite\": {\"version\": \"0.2\"}}", "different_should_miss", NotFoundError},
		connectionLocatorV2TestCaseData{"with " + satelliteWorker + " dispatchers", "888881", "888888", "sql-locator-v2-strict-test-client-4", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "different_should_miss_too", NotFoundError},
		connectionLocatorV2TestCaseData{"with no account", "", "999992", "sql-locator-v2-strict-test-client-5", "{}", "9999", NotFoundError},                                                                                             // anemic tenant
		connectionLocatorV2TestCaseData{"with no account or org-id", "", "", "sql-locator-v2-strict-test-client-6", "{}", "9999", NotFoundError},                                                                                         // ghost connection
		connectionLocatorV2TestCaseData{"with no account with " + satelliteWorker + " dispatchers", "", "999992", "sql-locator-v2-strict-test-client-5", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "9999", NotFoundError}, // anemic tenant
		connectionLocatorV2TestCaseData{"with no account or org-id " + satelliteWorker + " dispatchers", "", "", "sql-locator-v2-strict-test-client-6", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "9999", NotFoundError},  // ghost connection
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			runSqlConnectionLocatorV2Test(t, tc, cfg, database, getConnectionByClientID)
		})
	}
}

func TestSqlConnectionLocatorV2RelaxedImplementation(t *testing.T) {

	cfg := config.GetConfig()

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	getConnectionByClientID, err := NewPermittedTenantSqlGetConnectionByClientID(cfg, database)

	testCases := []connectionLocatorV2TestCaseData{
		connectionLocatorV2TestCaseData{"with no dispatchers", "999991", "999999", "sql-locator-v2-test-client-1", "{}", "999991", nil},
		connectionLocatorV2TestCaseData{"with satellite dispatchers", "888881", "888888", "sql-locator-v2-test-client-2", "{\"satellite\": {\"version\": \"0.2\"}}", "888881", nil},
		connectionLocatorV2TestCaseData{"with satellite dispatchers", "888881", "888888", "sql-locator-v2-test-client-3", "{\"satellite\": {\"version\": \"0.2\"}}", "different_should_miss", NotFoundError},
		connectionLocatorV2TestCaseData{"with " + satelliteWorker + " dispatchers", "888881", "888888", "sql-locator-v2-test-client-4", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "different_should_match", nil},
		connectionLocatorV2TestCaseData{"with no account", "", "999992", "sql-locator-v2-test-client-5", "{}", "9999", NotFoundError},                                                                                             // anemic tenant
		connectionLocatorV2TestCaseData{"with no account or org-id", "", "", "sql-locator-v2-test-client-6", "{}", "9999", NotFoundError},                                                                                         // ghost connection
		connectionLocatorV2TestCaseData{"with no account with " + satelliteWorker + " dispatchers", "", "999992", "sql-locator-v2-test-client-5", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "9999", NotFoundError}, // anemic tenant
		connectionLocatorV2TestCaseData{"with no account or org-id " + satelliteWorker + " dispatchers", "", "", "sql-locator-v2-test-client-6", "{\"" + satelliteWorker + "\": {\"version\": \"0.2\"}}", "9999", NotFoundError},  // ghost connection
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			runSqlConnectionLocatorV2Test(t, tc, cfg, database, getConnectionByClientID)
		})
	}
}

func runSqlConnectionLocatorV2Test(t *testing.T, tc connectionLocatorV2TestCaseData, cfg *config.Config, database *sql.DB, getConnectionByClientID GetConnectionByClientID) {

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
}
