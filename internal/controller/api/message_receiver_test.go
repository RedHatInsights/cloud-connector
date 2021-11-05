package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/middlewares"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	IDENTITY_HEADER_NAME      = "x-rh-identity"
	TOKEN_HEADER_CLIENT_NAME  = middlewares.PSKClientIdHeader
	TOKEN_HEADER_ACCOUNT_NAME = middlewares.PSKAccountHeader
	TOKEN_HEADER_PSK_NAME     = middlewares.PSKHeader
	URL_BASE_PATH             = "/api/cloud-connector/api/v1"
	MESSAGE_ENDPOINT          = URL_BASE_PATH + "/message"
)

type MockClientProxyFactory struct {
}

func (MockClientProxyFactory) CreateProxy(context.Context, domain.AccountID, domain.ClientID, domain.Dispatchers) (controller.ConnectorClient, error) {
	return MockClient{}, nil
}

type MockClient struct {
	returnAnError bool
}

func (mc MockClient) SendMessage(ctx context.Context, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {
	if mc.returnAnError {
		return nil, errors.New("ImaError")
	}
	myUUID, _ := uuid.NewRandom()
	return &myUUID, nil
}

func (mc MockClient) Ping(ctx context.Context) error {
	if mc.returnAnError {
		return errors.New("ImaError")
	}
	return nil
}

func (mc MockClient) Reconnect(ctx context.Context, message string, delay int) error {
	if mc.returnAnError {
		return errors.New("ImaError")
	}
	return nil
}

func (mc MockClient) GetDispatchers(ctx context.Context) (domain.Dispatchers, error) {
	var dispatchers domain.Dispatchers
	if mc.returnAnError {
		return dispatchers, errors.New("ImaError")
	}
	return dispatchers, nil
}

func (mc MockClient) Disconnect(context.Context, string) error {
	return nil
}

type MockConnectionManager struct {
	AccountIndex map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient
	ClientIndex  map[domain.ClientID]domain.AccountID
}

func NewMockConnectionManager() *MockConnectionManager {
	mcm := MockConnectionManager{AccountIndex: make(map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient),
		ClientIndex: make(map[domain.ClientID]domain.AccountID)}
	return &mcm
}

func (m *MockConnectionManager) Register(ctx context.Context, rhcClient domain.ConnectorClientState) (connection_repository.RegistrationResults, error) {
	_, ok := m.AccountIndex[rhcClient.Account]
	if !ok {
		m.AccountIndex[rhcClient.Account] = make(map[domain.ClientID]controller.ConnectorClient)
	}

	mockClient := MockClient{}

	if rhcClient.ClientID == "error-client" { // FIXME: this is kinda gross
		mockClient.returnAnError = true
	}

	m.AccountIndex[rhcClient.Account][rhcClient.ClientID] = mockClient
	m.ClientIndex[rhcClient.ClientID] = rhcClient.Account

	return connection_repository.NewConnection, nil
}

func (m *MockConnectionManager) Unregister(ctx context.Context, clientID domain.ClientID) {
	account, ok := m.ClientIndex[clientID]
	if !ok {
		return
	}

	delete(m.ClientIndex, clientID)
	delete(m.AccountIndex[account], clientID)
}

func (m *MockConnectionManager) FindConnectionByClientID(ctx context.Context, clientID domain.ClientID) (domain.ConnectorClientState, error) {
	return domain.ConnectorClientState{Account: m.ClientIndex[clientID], ClientID: clientID}, nil
}

func (m *MockConnectionManager) ReenableConnection(context.Context, domain.AccountID, domain.ClientID) error {
	return nil
}

func (m *MockConnectionManager) GetConnection(ctx context.Context, account domain.AccountID, clientID domain.ClientID) controller.ConnectorClient {
	return m.AccountIndex[account][clientID]
}

func (m *MockConnectionManager) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {
	return m.AccountIndex[account], len(m.AccountIndex[account]), nil
}

func (m *MockConnectionManager) GetAllConnections(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {
	return m.AccountIndex, len(m.AccountIndex), nil
}

func init() {
	logger.InitLogger()
}

var _ = Describe("MessageReceiver", func() {

	var (
		jr                  *MessageReceiver
		validIdentityHeader string
		sqliteDbFileName    string
	)

	BeforeEach(func() {
		var account domain.AccountID = "1234"
		apiMux := mux.NewRouter()
		cfg := config.GetConfig()
		connectionManager := NewMockConnectionManager()
		connectorClient := domain.ConnectorClientState{Account: account, ClientID: "345"}
		connectionManager.Register(context.TODO(), connectorClient)
		errorConnectorClient := domain.ConnectorClientState{Account: account, ClientID: "error-client"}
		connectionManager.Register(context.TODO(), errorConnectorClient)
		jr = NewMessageReceiver(connectionManager, apiMux, URL_BASE_PATH, cfg)
		jr.Routes()

		validIdentityHeader = buildIdentityHeader(account)
	})

	AfterEach(func() {
		os.Remove(sqliteDbFileName)
	})

	Describe("Connecting to the job receiver", func() {
		Context("With a valid identity header", func() {
			It("Should be able to send a job to a connected customer", func() {

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusCreated))

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("id"))
			})

			It("Should be able to send a job to a connected customer but get an error", func() {

				Skip("This test is supposed simulate a situation where the client throws and error")

				postBody := "{\"account\": \"1234\", \"recipient\": \"error-client\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusInternalServerError))

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("status"))
				Expect(m).Should(HaveKey("title"))
				Expect(m).Should(HaveKey("detail"))
			})

			It("Should not allow sending a job to a disconnected customer", func() {

				postBody := "{\"account\": \"1234-not-here\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				validIdentityHeader = buildIdentityHeader("1234-not-here")

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusNotFound))
			})

			It("Should not allow sending a job with an empty account", func() {

				postBody := "{\"account\": \"\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should not allow sending a job with malformed json", func() {

				postBody := "{\"account\" = \"1234-bad-json\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should not allow sending a job with a string instead of json", func() {

				postBody := "account: 1234-string-value"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should not allow sending a job with missing required fields", func() {

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"]}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should allow sending a job with unknown fields", func() {

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\", \"extra\": \"field\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusCreated))
			})

			It("Should not allow sending a job to the wrong customer", func() {

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				validIdentityHeader = buildIdentityHeader("4321")

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusForbidden))

				verifyErrorResponse(rr.Body, accountMismatchErrorMsg)
			})

			It("Should not allow sending a job with empty directive field", func() {

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"   \"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
				verifyErrorResponse(rr.Body, emptyDirectictiveErrorMsg)
			})

		})

		Context("Without an identity header or pre shared key", func() {
			It("Should fail to send a job to a connected customer", func() {

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})

		})

		Context("With a valid token", func() {
			It("Should be able to send a job to a connected customer", func() {
				jr.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "1234")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusCreated))

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("id"))
			})
		})

		Context("With a valid token", func() {
			It("Should NOT be able to send a job to the wrong account", func() {
				jr.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "4321") // This account number should be different than what is in the post body
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusForbidden))

				verifyErrorResponse(rr.Body, accountMismatchErrorMsg)
			})
		})

		Context("With an invalid token", func() {
			It("Should not be able to send a job to a connected customer", func() {
				jr.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "6789")

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("With an unknown client during token auth", func() {
			It("Should not be able to send a job to a connected customer", func() {
				jr.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := "{\"account\": \"1234\", \"recipient\": \"345\", \"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_nil")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})
		})

	})
})

func buildIdentityHeader(account domain.AccountID) string {
	identityJson := fmt.Sprintf(
		"{ \"identity\": {\"account_number\": \"%s\", \"type\": \"User\", \"internal\": { \"org_id\": \"1979710\" } } }",
		account)
	return base64.StdEncoding.EncodeToString([]byte(identityJson))
}

func verifyErrorResponse(body *bytes.Buffer, expectedDetail string) {
	var errorResponse errorResponse
	err := json.Unmarshal(body.Bytes(), &errorResponse)
	Expect(err).NotTo(HaveOccurred())

	Expect(errorResponse.Detail).To(Equal(expectedDetail))
}
