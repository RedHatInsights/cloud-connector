package middlewares

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

func init() {
	logger.InitLogger()
}

func TestTurnpikeAuthenticator(t *testing.T) {

	testCases := []struct {
		IdentityType       string
		IdentityHeader     string
		ExpectedStatusCode int
	}{
		{"Associate", "eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMDAwMiIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJBc3NvY2lhdGUifX0=", http.StatusOK},
		{"User", "eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMDAwMiIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJiYXNpYyJ9fQ==", http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("subtest identity type %s", tc.IdentityType), func(t *testing.T) {

			req, _ := http.NewRequest(http.MethodGet, "/doesntmatter", strings.NewReader("doesntmatter"))

			req.Header = make(http.Header)
			req.Header.Add("x-rh-identity", tc.IdentityHeader)

			rr := httptest.NewRecorder()

			applicationHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				rw.WriteHeader(200)
			})

			handler := identity.EnforceIdentity(EnforceTurnpikeAuthentication(applicationHandler))

			handler.ServeHTTP(rr, req)

			if rr.Code != tc.ExpectedStatusCode {
				t.Fatalf("Invalid status code - actual: %d, expected: %d", rr.Code, tc.ExpectedStatusCode)
			}
		})
	}
}
