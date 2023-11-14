package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	//"github.com/RedHatInsights/cloud-connector/internal/platform/db"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	//	"github.com/sirupsen/logrus"
)

func init() {
	logger.InitLogger()
}

// TEST TODO
// verify psk, request id are passed

// verify the post, verify url
// verify error handling (500, etc)
// verify the post body

func TestCompositeHttpProxy(t *testing.T) {

	var targetClientId domain.ClientID = "clientId"

	expectedHttpHeaders := createExpectedHttpHeaders()

	httpHandler := testServerHandler{}
	server := httptest.NewServer(httpHandler.buildRequestHandler(t, targetClientId, expectedHttpHeaders, http.StatusCreated, []byte(`{"id": "5c3231c7-07c4-481a-8687-dc07342574e9"}`)))
	defer server.Close()

	cache := expirable.NewLRU[domain.ClientID, string](10, nil, 10*time.Millisecond)

	cache.Add(targetClientId, server.URL)

	cfg := config.GetConfig()
	cfg.CompositeServiceToServiceClientId = expectedHttpHeaders["x-rh-cloud-connector-client-id"]
	cfg.CompositeServiceToServicePsk = expectedHttpHeaders["x-rh-cloud-connector-psk"]

	ctx := createContextWithRequestId(expectedHttpHeaders)

	proxyFactory, _ := NewConnectorClientHTTPProxyFactory(cfg, cache)

	proxy, _ := proxyFactory.CreateProxy(ctx, "orgId", "account", targetClientId, nil, nil, nil)

	metadata := map[string]string{
		"satellite_id": generateUUID(),
		"job_id":       generateUUID(),
	}

	payload := map[string]string{
		"job_name": generateUUID(),
		"url":      "http://insights.app.console.redhat.com/api/testservice/v1/dostuff",
	}

	msgId, err := proxy.SendMessage(ctx, "imadirective", metadata, payload)
	if err != nil {
		t.Fatalf("Received unexpected error: %s", err)
	}

	if msgId.String() != "5c3231c7-07c4-481a-8687-dc07342574e9" {
		t.Fatalf("Bad id")
	}

	if httpHandler.wasCalled != true {
		t.Fatalf("Expected the mocked http server to have been called")
	}
}

func generateUUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
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

		fmt.Println("***\n\nr.Body", r.Body)

		var sendMsgRequest messageRequestV2
		err := json.NewDecoder(r.Body).Decode(&sendMsgRequest)
		if err != nil {
			t.Fatalf("Failed to unmarshal request to child cloud-connector - error: %s", err)
		}

		w.WriteHeader(statusCode)
		w.Write(response)
	}
}

func createExpectedHttpHeaders() map[string]string {
	return map[string]string{
		"accept":                         "application/json",
		"x-rh-insights-request-id":       "requestID-xyz-1234-5678",
		"x-rh-cloud-connector-org-id":    "orgId",
		"x-rh-cloud-connector-client-id": "cloud-connector-composite",
		"x-rh-cloud-connector-psk":       "secret_used_by_composite",
	}
}

func createContextWithRequestId(expectedHttpHeaders map[string]string) context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, request_id.RequestIDKey, expectedHttpHeaders["x-rh-insights-request-id"])
}

func buildConnectedResponse() []byte {
	return []byte(`{"status":"connected", "account": "1234", "org_id": "4321", "client_id": "clientId"}`)
}

func buildDisconnectedResponse() []byte {
	return []byte(`{"status":"disconnected"}`)
}
