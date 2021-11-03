package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

func init() {
	logger.InitLogger()
}

func TestTurnpikeAuthenticator(t *testing.T) {

	testCases := []struct {
		IdentityType       string
		ExpectedStatusCode int
	}{
		{"Associate", 200},
		{"User", 401},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("subtest identity type %s", tc.IdentityType), func(t *testing.T) {

			var req http.Request
			rr := httptest.NewRecorder()

			var xrhID identity.XRHID
			xrhID.Identity.Type = tc.IdentityType

			applicationHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(200)
			})

			handler := RequireTurnpikeAuthentication(applicationHandler)

			ctx := context.WithValue(req.Context(), identity.Key, xrhID)
			handler.ServeHTTP(rr, req.WithContext(ctx))

			if rr.Code != tc.ExpectedStatusCode {
				t.Fatalf("Invalid status code - actual: %d, expected: %d", rr.Code, tc.ExpectedStatusCode)
			}
		})
	}
}
