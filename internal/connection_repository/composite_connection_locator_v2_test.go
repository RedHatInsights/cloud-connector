package connection_repository

import (
	"context"
	"fmt"
	//"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	//"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

// TEST TODO
// verify psk, request id are passed

// found in first server
//  verify second is not called
// found in second server
//  verify first server is called
// not found in either
//  verify both servers are called

func TestCompositeConnectionLocatorNoConnectionFound(t *testing.T) {

	var targetClientId domain.ClientID = "clientId"

	testHttpHandler := testServerHandler{}

	server := httptest.NewServer(testHttpHandler.buildRequestHandler(t, targetClientId, http.StatusOK, []byte(`{"status":"disconnected"}`)))

	defer server.Close()

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	cfg := config.GetConfig()

	urls := []string{server.URL}

	getConnectionByClientID, err := NewCompositeGetConnectionByClientID(cfg, urls, cache)
	if err != nil {
		t.Fatalf("Got an error: %s", err)
	}

	log := logger.Log.WithFields(logrus.Fields{"client_id": "clientId"})

	connectionState, err := getConnectionByClientID(context.TODO(), log, "orgId", "clientId")
	if err != NotFoundError {
		t.Fatalf("Expected an error but did not get one!")
	}

	cachedUrl, ok := cache.Get("clientId")
	if ok {
		t.Fatalf("Expected a cache miss, but found a cached url!")
	}

	if cachedUrl != "" {
		t.Fatalf("Expected an empty string for the url, but found %s!", cachedUrl)
	}

	fmt.Println("connectionState:", connectionState)
}

func TestCompositeConnectionLocatorConnectionFound(t *testing.T) {

	var targetClientId domain.ClientID = "clientId"

	testHttpHandler := testServerHandler{}

	server := httptest.NewServer(testHttpHandler.buildRequestHandler(t, targetClientId, http.StatusOK, []byte(`{"status":"connected", "account": "1234", "org_id": "4321", "client_id": "clientId"}`)))

	defer server.Close()

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	cfg := config.GetConfig()

	urls := []string{server.URL}

	getConnectionByClientID, err := NewCompositeGetConnectionByClientID(cfg, urls, cache)
	if err != nil {
		t.Fatalf("Got an error: %s", err)
	}

	log := logger.Log.WithFields(logrus.Fields{"client_id": targetClientId})

	connectionState, err := getConnectionByClientID(context.TODO(), log, "orgId", targetClientId)
	if err != nil {
		t.Fatalf("Received unexpected error: %s", err)
	}

	cachedUrl, ok := cache.Get(targetClientId)
	if !ok {
		t.Fatalf("Expected a cached url, but did not find one!")
	}

	if cachedUrl != server.URL {
		t.Fatalf("Expected an the cached url (%s) to match test server url (%s)!", cachedUrl, server.URL)
	}

	if testHttpHandler.wasCalled != true {
		t.Fatalf("Expected the mocked http server to have been called")
	}

	fmt.Println("connectionState:", connectionState)
}

type testServerHandler struct {
	wasCalled bool
}

func (this *testServerHandler) buildRequestHandler(t *testing.T, expectedClientId domain.ClientID, statusCode int, response []byte) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		this.wasCalled = true

		expectedUrl := fmt.Sprintf("/api/cloud-connector/v2/connections/%s/status", expectedClientId)

		if r.URL.Path != expectedUrl {
			t.Errorf("Expected to request '%s', got: %s", expectedUrl, r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(statusCode)
		w.Write(response)
	}
}
