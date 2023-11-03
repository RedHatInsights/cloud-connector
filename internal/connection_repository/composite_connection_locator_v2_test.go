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
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

// FIXME: this global is gross ... create a function to create this??
var expectedHttpHeaders map[string]string = map[string]string{
	"accept":                         "application/json",
	"x-rh-insights-request-id":       "requestID-xyz-1234-5678",
	"x-rh-cloud-connector-org-id":    "orgId",
	"x-rh-cloud-connector-client-id": "cloud-connector-composite",
	"x-rh-cloud-connector-psk":       "secret_used_by_composite",
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

	notFoundHttpHandler1 := testServerHandler{}
	notFoundServer1 := httptest.NewServer(notFoundHttpHandler1.buildRequestHandler(t, targetClientId, expectedHttpHeaders, http.StatusOK, []byte(`{"status":"disconnected"}`)))
	defer notFoundServer1.Close()

	notFoundHttpHandler2 := testServerHandler{}
	notFoundServer2 := httptest.NewServer(notFoundHttpHandler2.buildRequestHandler(t, targetClientId, expectedHttpHeaders, http.StatusOK, []byte(`{"status":"disconnected"}`)))
	defer notFoundServer2.Close()

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	cfg := config.GetConfig()

	urls := []string{notFoundServer1.URL, notFoundServer2.URL}

	getConnectionByClientID, err := NewCompositeGetConnectionByClientID(cfg, urls, cache)
	if err != nil {
		t.Fatalf("Got an error: %s", err)
	}

	log := logger.Log.WithFields(logrus.Fields{"client_id": targetClientId})

	ctx := createContextWithRequestId()

	connectionState, err := getConnectionByClientID(ctx, log, "orgId", targetClientId)
	if err != NotFoundError {
		t.Fatalf("Expected an error but did not get one!")
	}

	cachedUrl, ok := cache.Get(targetClientId)
	if ok {
		t.Fatalf("Expected a cache miss, but found a cached url!")
	}

	if cachedUrl != "" {
		t.Fatalf("Expected an empty string for the url, but found %s!", cachedUrl)
	}

	if notFoundHttpHandler1.wasCalled != true {
		t.Fatalf("Expected the mocked http server to have been called")
	}

	if notFoundHttpHandler2.wasCalled != true {
		t.Fatalf("Expected the mocked http server to have been called")
	}

	fmt.Println("connectionState:", connectionState)
}

func TestCompositeConnectionLocatorConnectionFound(t *testing.T) {

	var targetClientId domain.ClientID = "clientId"

	notFoundHttpHandler := testServerHandler{}
	notFoundServer := httptest.NewServer(notFoundHttpHandler.buildRequestHandler(t, targetClientId, expectedHttpHeaders, http.StatusOK, []byte(`{"status":"disconnected"}`)))
	defer notFoundServer.Close()

	foundHttpHandler := testServerHandler{}
	foundServer := httptest.NewServer(foundHttpHandler.buildRequestHandler(t, targetClientId, expectedHttpHeaders, http.StatusOK, []byte(`{"status":"connected", "account": "1234", "org_id": "4321", "client_id": "clientId"}`)))
	defer foundServer.Close()

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	cfg := config.GetConfig()

	urls := []string{notFoundServer.URL, foundServer.URL}

	getConnectionByClientID, err := NewCompositeGetConnectionByClientID(cfg, urls, cache)
	if err != nil {
		t.Fatalf("Got an error: %s", err)
	}

	log := logger.Log.WithFields(logrus.Fields{"client_id": targetClientId})

	ctx := createContextWithRequestId()

	connectionState, err := getConnectionByClientID(ctx, log, "orgId", targetClientId)
	if err != nil {
		t.Fatalf("Received unexpected error: %s", err)
	}

	cachedUrl, ok := cache.Get(targetClientId)
	if !ok {
		t.Fatalf("Expected a cached url, but did not find one!")
	}

	if cachedUrl != foundServer.URL {
		t.Fatalf("Expected an the cached url (%s) to match test server url (%s)!", cachedUrl, foundServer.URL)
	}

	if notFoundHttpHandler.wasCalled != true {
		t.Fatalf("Expected the mocked http server to have been called")
	}

	if foundHttpHandler.wasCalled != true {
		t.Fatalf("Expected the mocked http server to have been called")
	}
	fmt.Println("connectionState:", connectionState)
}

type testServerHandler struct {
	wasCalled bool
}

func (this *testServerHandler) buildRequestHandler(t *testing.T, expectedClientId domain.ClientID, expectedHttpHeaders map[string]string, statusCode int, response []byte) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		this.wasCalled = true

		expectedUrl := fmt.Sprintf("/api/cloud-connector/v2/connections/%s/status", expectedClientId)

		if r.URL.Path != expectedUrl {
			t.Errorf("Expected to request '%s', got: %s", expectedUrl, r.URL.Path)
		}

		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json header, got: %s", r.Header.Get("Accept"))
		}

		for k, v := range expectedHttpHeaders {
			actualValue := r.Header.Get(k)
			if actualValue == "" {
				t.Fatalf("HTTP Header %s not found", k)
			}

			if actualValue != v {
				t.Fatalf("HTTP Header %s value (%s) does not match expected value (%s) ", k, actualValue, v)
			}
		}

		w.WriteHeader(statusCode)
		w.Write(response)
	}
}

func createContextWithRequestId() context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, request_id.RequestIDKey, expectedHttpHeaders["x-rh-insights-request-id"])
}
