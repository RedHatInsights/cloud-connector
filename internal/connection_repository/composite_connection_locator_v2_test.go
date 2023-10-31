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

func TestCompositeConnectionLocatorNoConnectionFound(t *testing.T) {

	targetClientId := "clientId"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		expectedUrl := fmt.Sprintf("/api/cloud-connector/v2/connections/%s/status", targetClientId)

		if r.URL.Path != expectedUrl {
			t.Errorf("Expected to request '%s', got: %s", expectedUrl, r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"disconnected"}`))
	}))
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

	targetClientId := "clientId"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		expectedUrl := fmt.Sprintf("/api/cloud-connector/v2/connections/%s/status", targetClientId)

		if r.URL.Path != expectedUrl {
			t.Errorf("Expected to request '%s', got: %s", expectedUrl, r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"connected", "account": "1234", "org_id": "4321", "client_id": "clientId"}`))
	}))
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
	if err != nil {
		t.Fatalf("Received unexpected error: %s", err)
	}

	cachedUrl, ok := cache.Get("clientId")
	if !ok {
		t.Fatalf("Expected a cached url, but did not find one!")
	}

	if cachedUrl != server.URL {
		t.Fatalf("Expected an the cached url (%s) to match test server url (%s)!", cachedUrl, server.URL)
	}

	fmt.Println("connectionState:", connectionState)
}
