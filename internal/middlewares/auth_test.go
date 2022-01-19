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
	TOKEN_HEADER_CLIENT_NAME              = middlewares.PSKClientIdHeader
	TOKEN_HEADER_ORG_NAME                 = middlewares.PSKOrgIdHeader
	TOKEN_HEADER_ACCOUNT_NAME             = middlewares.PSKAccountHeader
	TOKEN_HEADER_PSK_NAME                 = middlewares.PSKHeader
	authFailure                           = "Authentication failed"
	IDENTITY_HEADER_NAME                  = "x-rh-identity"
	EXPECTED_ACCOUNT_FROM_TOKEN           = "0000001"
	EXPECTED_ORG_FROM_TOKEN               = "000001"
	VALID_IDENTITY_HEADER                 = "eyJpZGVudGl0eSI6IHsiYWNjb3VudF9udW1iZXIiOiAiMDAwMDAwMiIsICJpbnRlcm5hbCI6IHsib3JnX2lkIjogIjAwMDAwMSJ9LCAidHlwZSI6ICJiYXNpYyJ9fQ=="
	EXPECTED_ACCOUNT_FROM_IDENTITY_HEADER = "0000002"
	EXPECTED_ORG_ID_FROM_IDENTITY_HEADER  = "000001"
)

func GetTestHandler(expectedAccountNumber string, expectedOrgID string) http.HandlerFunc {
	fn := func(rw http.ResponseWriter, req *http.Request) {
		principal, ok := middlewares.GetPrincipal(req.Context())
		Expect(ok).To(Equal(true))
		Expect(principal.GetAccount()).To(Equal(expectedAccountNumber))
		Expect(principal.GetOrgID()).To(Equal(expectedOrgID))
	}

	return http.HandlerFunc(fn)
}

func boiler(req *http.Request, expectedStatusCode int, expectedBody string, expectedAccountNumber string, expectedOrgID string, amw *middlewares.AuthMiddleware) {
	rr := httptest.NewRecorder()
	handler := amw.Authenticate(GetTestHandler(expectedAccountNumber, expectedOrgID))
	handler.ServeHTTP(rr, req)

	Expect(rr.Code).To(Equal(expectedStatusCode))
	Expect(rr.Body.String()).To(Equal(expectedBody))
}

var _ = Describe("Auth", func() {
	var (
		req *http.Request
		amw *middlewares.AuthMiddleware
	)

	BeforeEach(func() {
		knownSecrets := make(map[string]interface{})
		knownSecrets["test_client_1"] = "12345"
		amw = &middlewares.AuthMiddleware{Secrets: knownSecrets, IdentityAuth: identity.EnforceIdentity}

		r, err := http.NewRequest("GET", "/api/cloud-connector/v1/job", nil)
		if err != nil {
			panic("Test error unable to get new request")
		}
		req = r
	})

	Describe("Using token authentication", func() {
		Context("With no missing token auth headers", func() {
			It("Should return 200 when the key is correct", func() {
				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, EXPECTED_ACCOUNT_FROM_TOKEN)
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				boiler(req, 200, "", EXPECTED_ACCOUNT_FROM_TOKEN, "", amw)
			})

			It("Should return 200 when passing an orgID", func() {
				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, EXPECTED_ACCOUNT_FROM_TOKEN)
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")
				req.Header.Add(TOKEN_HEADER_ORG_NAME, EXPECTED_ORG_FROM_TOKEN)

				boiler(req, 200, "", EXPECTED_ACCOUNT_FROM_TOKEN, EXPECTED_ORG_FROM_TOKEN, amw)
			})

			It("Should return a 401 when the key is incorrect", func() {
				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, EXPECTED_ACCOUNT_FROM_TOKEN)
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "678910")

				boiler(req, 401, authFailure+"\n", EXPECTED_ACCOUNT_FROM_TOKEN, "", amw)
			})

			It("Should return a 401 when the client id is unknown", func() {
				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_nil")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, EXPECTED_ACCOUNT_FROM_TOKEN)
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				boiler(req, 401, authFailure+"\n", EXPECTED_ACCOUNT_FROM_TOKEN, "", amw)
			})
		})

		Context("With missing token auth headers", func() {
			It("Should return 401 when the client id header is missing", func() {
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				boiler(req, 401, authFailure+"\n", "dont care", "", amw)
			})

			It("Should return 401 when the account header is missing", func() {
				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				boiler(req, 401, authFailure+"\n", "dont care", "", amw)
			})

			It("Should return 401 when the psk header is missing", func() {
				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")

				boiler(req, 401, authFailure+"\n", "dont care", "", amw)
			})
		})
	})

	Describe("Use identity header authentication", func() {
		Context("With valid identity header and no psk headers", func() {
			It("Should return 200 when the key is correct", func() {
				req.Header.Add(IDENTITY_HEADER_NAME, VALID_IDENTITY_HEADER)

				boiler(req, 200, "", EXPECTED_ACCOUNT_FROM_IDENTITY_HEADER, EXPECTED_ORG_ID_FROM_IDENTITY_HEADER, amw)
			})

		})

	})

})
