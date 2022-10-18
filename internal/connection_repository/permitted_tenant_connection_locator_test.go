//go:build sql
// +build sql

package connection_repository

import (
	"context"
	"gorm.io/gorm"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"encoding/json"
)

func init() {
	logger.InitLogger()
}

type connection struct {
	account     string
	org_id      string
	client_id   string
	dispatchers string
}

func insertTestData(database *gorm.DB, account domain.AccountID, org_id domain.OrgID, client_id domain.ClientID, dispatchers domain.Dispatchers) (func(), error) {
	delete := "DELETE FROM connections WHERE org_id = $1 AND client_id = $2"

	noOpFunc := func() {}

	dispatchersJson, err := json.Marshal(dispatchers)
	if err != nil {
		return noOpFunc, err
	}

	result := database.Table("connections").Select("account", "org_id", "client_id", "dispatchers").Create(connection{
		account:     account,
		org_id:      org_id,
		client_id:   client_id,
		dispatchers: dispatchersJson,
	})
	if result.Error != nil {
		return noOpFunc, result.Error
	}

	cleanUpTestDataFunc := func() {
		result := database.Table("connections").Where("org_id = ?", org_id).Where("client_id = ?", client_id).Delete()
		if result.Error != nil {
			return
		}
	}

	return cleanUpTestDataFunc, nil
}

type mockConnectorClientProxyFactory struct {
	callCount int
	orgID     domain.OrgID
	account   domain.AccountID
	clientID  domain.ClientID
}

func (m *mockConnectorClientProxyFactory) CreateProxy(ctx context.Context, orgID domain.OrgID, account domain.AccountID, clientID domain.ClientID, canonical_facts domain.CanonicalFacts, dispatchers domain.Dispatchers, tags domain.Tags) (controller.ConnectorClient, error) {
	m.callCount += 1
	m.orgID = orgID
	m.account = account
	m.clientID = clientID
	return nil, nil
}

func TestPermittedTenantConnectionLocator(t *testing.T) {

	cfg := config.GetConfig()

	database, err := db.InitializeGormDatabaseConnection(cfg)
	if err != nil {
		t.Fatal("Unable to connect to database: ", err)
	}

	dispatchersWithSatellite := map[string]map[string]string{
		"echo":          {},
		"fred":          {},
		satelliteWorker: {"version": "1.2.3"},
	}
	dispatchersWithoutSatellite := map[string]map[string]string{}

	testCases := []struct {
		testName            string
		account             domain.AccountID
		orgID               domain.OrgID
		clientID            domain.ClientID
		dispatchers         domain.Dispatchers
		accountToSearchFor  domain.AccountID
		orgIDToSearchFor    domain.OrgID
		clientIDToSearchFor domain.ClientID
		verifyResults       func(*testing.T, mockConnectorClientProxyFactory)
	}{
		{"org_id match", "999999", "11111", "client-1", dispatchersWithoutSatellite, "dont_match_account", "11111", "client-1", verifyProxyFactoryWasCalled("999999", "11111", "client-1")},
		{"failed org_id match, match based on satellite worker", "1000000", "11112", "client-2", dispatchersWithSatellite, "dont_match_account", "dont_match_org_id", "client-2", verifyProxyFactoryWasCalled("1000000", "11112", "client-2")},
		{"no matching org_id, no matching satellite worker", "999999", "111113", "client-3", nil, "888888", "77777", "client-3", verifyProxyFactoryWasNotCalled()},
		{"org id matches, but client-id does not", "999998", "111114", "client-4", dispatchersWithoutSatellite, "999998", "111114", "will-not-find-this-client", verifyProxyFactoryWasNotCalled()},
		{"org id matches and has satellite worker, but client-id does not", "999997", "111115", "client-5", dispatchersWithSatellite, "999997", "111115", "will-not-find-this-client", verifyProxyFactoryWasNotCalled()},
		{"search for blank org_id", "999996", "", "client-6", dispatchersWithoutSatellite, "999996", "", "client-6", verifyProxyFactoryWasNotCalled()},
		{"search for blank client_id", "999996", "1", "client-6", dispatchersWithoutSatellite, "999996", "1", "", verifyProxyFactoryWasNotCalled()},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {

			var mockProxyFactory = mockConnectorClientProxyFactory{}

			var connectionLocator ConnectionLocator
			connectionLocator, err := NewPermittedTenantConnectionLocator(cfg, database, &mockProxyFactory)
			if err != nil {
				t.Fatal("unexpected error while creating the PermittedTenantConnectionLocator", err)
			}

			cleanUpTestData, err := insertTestData(database, tc.account, tc.orgID, tc.clientID, tc.dispatchers)
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

func verifyProxyFactoryWasCalled(expectedAccount domain.AccountID, expectedOrgID domain.OrgID, expectedClientID domain.ClientID) func(t *testing.T, mockProxyFactory mockConnectorClientProxyFactory) {
	return func(t *testing.T, mockProxyFactory mockConnectorClientProxyFactory) {
		if mockProxyFactory.callCount != 1 {
			t.Fatalf("expected proxy factory call count to be 1, but got %d!", mockProxyFactory.callCount)
		}

		if mockProxyFactory.orgID != expectedOrgID {
			t.Fatalf("expected proxy factory to be called with orgID %s, but got %s!", mockProxyFactory.orgID, expectedOrgID)
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
