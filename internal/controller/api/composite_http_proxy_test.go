package api

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
	//	"github.com/sirupsen/logrus"
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

// verify the post, verify url
// verify error handling (500, etc)

func TestCompositeConnectionLocatorNoConnectionFound(t *testing.T) {

	var targetClientId domain.ClientID = "clientId"

	httpHandler := testServerHandler{}
	server := httptest.NewServer(httpHandler.buildRequestHandler(t, targetClientId, expectedHttpHeaders, http.StatusCreated, []byte(`{"id": "5c3231c7-07c4-481a-8687-dc07342574e9"}`)))
	defer server.Close()

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	cache.Add(targetClientId, server.URL)

	cfg := config.GetConfig()

	ctx := createContextWithRequestId()

	proxyFactory, _ := NewConnectorClientHTTPProxyFactory(cfg, cache)

	proxy, _ := proxyFactory.CreateProxy(ctx, "orgId", "account", targetClientId, nil, nil, nil)

	msgId, err := proxy.SendMessage(ctx, "directive", "imametadata", "imapayload")
	if err != nil {
		t.Fatalf("Received unexpected error: %s", err)
	}
	if msgId.String() != "5c3231c7-07c4-481a-8687-dc07342574e9" {
		t.Fatalf("Bad id")
	}

	/*
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
	*/
}

type testServerHandler struct {
	wasCalled bool
}

func (this *testServerHandler) buildRequestHandler(t *testing.T, expectedClientId domain.ClientID, expectedHttpHeaders map[string]string, statusCode int, response []byte) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		this.wasCalled = true

		expectedUrl := fmt.Sprintf("/api/cloud-connector/v2/connections/%s/message", expectedClientId)

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

func buildConnectedResponse() []byte {
	return []byte(`{"status":"connected", "account": "1234", "org_id": "4321", "client_id": "clientId"}`)
}

func buildDisconnectedResponse() []byte {
	return []byte(`{"status":"disconnected"}`)
}
