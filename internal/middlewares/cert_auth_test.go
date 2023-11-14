package middlewares_test

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

const (
	validCertIdentityHeader = "eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMDAwMiIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJiYXNpYyIsICJhdXRoX3R5cGUiOiAiY2VydC1hdXRoIn19"
)

func certAuthBoiler(req *http.Request, expectedStatusCode int, expectedBody string, expectedAccountNumber string, expectedOrgID string) {
	rr := httptest.NewRecorder()
	handler := identity.EnforceIdentity(middlewares.EnforceCertAuthentication(GetTestHandler(expectedAccountNumber, expectedOrgID)))
	handler.ServeHTTP(rr, req)

	Expect(rr.Code).To(Equal(expectedStatusCode))
	Expect(rr.Body.String()).To(Equal(expectedBody))
}

var _ = Describe("Cert Auth Middleware Tests", func() {
	var (
		req *http.Request
	)

	BeforeEach(func() {
		r, err := http.NewRequest("GET", "/api/cloud-connector/v1/job", nil)
		if err != nil {
			panic("Test error unable to get new request")
		}
		req = r
	})

	Describe("Require cert authentication", func() {
		Context("With valid cert-auth identity headers", func() {
			It("Should return 200", func() {
				req.Header.Add(IDENTITY_HEADER_NAME, validCertIdentityHeader)

				certAuthBoiler(req, 200, "", EXPECTED_ACCOUNT_FROM_IDENTITY_HEADER, EXPECTED_ORG_ID_FROM_IDENTITY_HEADER)
			})

		})

		Context("With valid user identity headers", func() {
			It("Should return 401", func() {
				req.Header.Add(IDENTITY_HEADER_NAME, VALID_IDENTITY_HEADER)

				certAuthBoiler(req, 401, "Authentication failed\n", EXPECTED_ACCOUNT_FROM_TOKEN, "")
			})

		})
	})
})
