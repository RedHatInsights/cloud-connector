package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/config"

	"github.com/go-playground/assert/v2"
	"github.com/gorilla/mux"
)

func TestMonitoringEndpoints(t *testing.T) {
	tests := []struct {
		endpoint       string
		httpMethod     string
		expectedStatus int
	}{
		{
			endpoint:       "/metrics",
			httpMethod:     "GET",
			expectedStatus: http.StatusOK,
		},
		{
			endpoint:       "/metrics",
			httpMethod:     "POST",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			endpoint:       "/liveness",
			httpMethod:     "GET",
			expectedStatus: http.StatusOK,
		},
		{
			endpoint:       "/liveness",
			httpMethod:     "POST",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			endpoint:       "/readiness",
			httpMethod:     "GET",
			expectedStatus: http.StatusOK,
		},
		{
			endpoint:       "/readiness",
			httpMethod:     "POST",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.httpMethod+" "+tc.endpoint, func(t *testing.T) {
			req, err := http.NewRequest(tc.httpMethod, tc.endpoint, nil)
			assert.Equal(t, err, nil)

			rr := httptest.NewRecorder()

			cfg := config.GetConfig()
			apiMux := mux.NewRouter()
			apiSpecServer := NewMonitoringServer(apiMux, cfg)
			apiSpecServer.Routes()

			apiSpecServer.router.ServeHTTP(rr, req)

			assert.Equal(t, rr.Code, tc.expectedStatus)
		})
	}
}
