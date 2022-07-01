//go:build sql
// +build sql

package connection_repository

import (
	"context"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"database/sql"
)

func init() {
	logger.InitLogger()
}

func insertTestData(database *sql.DB, account domain.AccountID, org_id domain.OrgID, client_id domain.ClientID, permitted_tenants string) (func(), error) {
	insert := "INSERT INTO connections (account, client_id, permitted_tenants, org_id) VALUES ($1, $2, $3, $4)"
	delete := "DELETE FROM connections WHERE account = $1 AND client_id = $2"

	noOpFunc := func() {}

	statement, err := database.Prepare(insert)
	if err != nil {
		return noOpFunc, err
	}
	defer statement.Close()

	_, err = statement.Exec(account, client_id, permitted_tenants, org_id)
	if err != nil {
		return noOpFunc, err
	}

	cleanUpTestDataFunc := func() {
		statement, err := database.Prepare(delete)
		if err != nil {
			return
		}
		defer statement.Close()

		_, err = statement.Exec(account, client_id)
		if err != nil {
			return
		}
	}

	return cleanUpTestDataFunc, nil
}

type mockConnectorClientProxyFactory struct {
	callCount int
	account   domain.AccountID
	clientID  domain.ClientID
}

func (m *mockConnectorClientProxyFactory) CreateProxy(ctx context.Context, account domain.AccountID, clientID domain.ClientID, dispatchers domain.Dispatchers) (controller.ConnectorClient, error) {
	m.callCount += 1
	m.account = account
	m.clientID = clientID
	return nil, nil
}

func TestPermittedTenantConnectionLocator(t *testing.T) {

	cfg := config.GetConfig()

	database, err := db.InitializeDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	testCases := []struct {
		testName            string
		account             domain.AccountID
		orgID               domain.OrgID
		clientID            domain.ClientID
		permittedTenants    string
		accountToSearchFor  domain.AccountID
		orgIDToSearchFor    domain.OrgID
		clientIDToSearchFor domain.ClientID
		verifyResults       func(*testing.T, mockConnectorClientProxyFactory)
	}{
		{"primary account match", "999999", "11111", "client-1", "[]", "999999", "dont_match_org_id", "client-1", verifyProxyFactoryWasCalled("999999", "client-1")},
		{"primary org_id match", "888888", "11112", "client-2", "[\"0001\", \"0002\"]", "dont_match_account", "11112", "client-2", verifyProxyFactoryWasCalled("888888", "client-2")},
		{"primary account match and org_id match", "2222", "3333", "client-2", "[\"0001\", \"0002\"]", "2222", "3333", "client-2", verifyProxyFactoryWasCalled("2222", "client-2")},
		{"permitted tenant match", "888888", "11112", "client-2", "[\"0001\", \"0002\"]", "dont_match_account", "0002", "client-2", verifyProxyFactoryWasCalled("888888", "client-2")},
		{"no matching account, no matching primary org_id, no matching permitted tenants (empty array)", "999999", "111113", "client-3", "[]", "888888", "77777", "client-3", verifyProxyFactoryWasNotCalled()},
		{"no matching account, no matching primary org_id, no matching permitted tenants", "999999", "111113", "client-3", "[\"nomatch\",\"nope\"]", "888888", "77777", "client-3", verifyProxyFactoryWasNotCalled()},
		{"search for blank org_id", "999999", "", "client-3", "[]", "888888", "", "client-3", verifyProxyFactoryWasNotCalled()},
		{"account matches, but client-id does not", "999999", "111114", "client-4", "[]", "999999", "1111", "will-not-find-this-client", verifyProxyFactoryWasNotCalled()},
		{"account matches, primary org_id matches, but client-id does not", "999999", "111114", "client-4", "[111114]", "999999", "111114", "will-not-find-this-client", verifyProxyFactoryWasNotCalled()},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			var mockProxyFactory = mockConnectorClientProxyFactory{}

			var connectionLocator ConnectionLocator
			connectionLocator, err := NewPermittedTenantConnectionLocator(cfg, database, &mockProxyFactory)
			if err != nil {
				t.Fatal("unexpected error while creating the PermittedTenantConnectionLocator", err)
			}

			cleanUpTestData, err := insertTestData(database, tc.account, tc.orgID, tc.clientID, tc.permittedTenants)
			if err != nil {
				t.Fatal("unexpected error while inserting test data into the database", err)
			}

			defer cleanUpTestData()

			connectionLocator.GetConnection(context.TODO(), tc.accountToSearchFor, tc.orgIDToSearchFor, tc.clientIDToSearchFor)

			// Use the calls to the mock proxy factory to verify the behavior is correct
			tc.verifyResults(t, mockProxyFactory)
		})
	}
}

func verifyProxyFactoryWasCalled(expectedAccount domain.AccountID, expectedClientID domain.ClientID) func(t *testing.T, mockProxyFactory mockConnectorClientProxyFactory) {
	return func(t *testing.T, mockProxyFactory mockConnectorClientProxyFactory) {
		if mockProxyFactory.callCount != 1 {
			t.Fatalf("expected proxy factory call count to be 1, but got %d!", mockProxyFactory.callCount)
		}

		if mockProxyFactory.account != expectedAccount {
			t.Fatalf("expected proxy factory to be called with account %s, but got %s!", mockProxyFactory.account, expectedAccount)
		}

		if mockProxyFactory.clientID != expectedClientID {
			t.Fatalf("expected proxy factory to be called with clientID %s, but got %s!", mockProxyFactory.clientID, expectedClientID)
		}
	}
}

func verifyProxyFactoryWasNotCalled() func(t *testing.T, mockProxyFactory mockConnectorClientProxyFactory) {
	return func(t *testing.T, mockProxyFactory mockConnectorClientProxyFactory) {
		if mockProxyFactory.callCount != 0 {
			t.Fatalf("expected proxy factory call count to be 0, but got %d!", mockProxyFactory.callCount)
		}
	}
}
