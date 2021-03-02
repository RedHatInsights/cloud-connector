package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	IDENTITY_HEADER_NAME      = "x-rh-identity"
	TOKEN_HEADER_CLIENT_NAME  = "x-rh-receptor-controller-client-id"
	TOKEN_HEADER_ACCOUNT_NAME = "x-rh-receptor-controller-account"
	TOKEN_HEADER_PSK_NAME     = "x-rh-receptor-controller-psk"
	URL_BASE_PATH             = "/api/cloud-connector/api/v1"
	MESSAGE_ENDPOINT          = URL_BASE_PATH + "/message"
)

type MockClientProxyFactory struct {
}

func (MockClientProxyFactory) CreateProxy(context.Context, domain.AccountID, domain.ClientID, domain.Dispatchers) (controller.Receptor, error) {
	return MockClient{}, nil
}

type MockClient struct {
	returnAnError bool
}

func (mc MockClient) SendMessage(ctx context.Context, account domain.AccountID, recipient domain.ClientID, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {
	if mc.returnAnError {
		return nil, errors.New("ImaError")
	}
	myUUID, _ := uuid.NewRandom()
	return &myUUID, nil
}

func (mc MockClient) Ping(ctx context.Context, account domain.AccountID, recipient domain.ClientID) error {
	if mc.returnAnError {
		return errors.New("ImaError")
	}
	return nil
}

func (mc MockClient) Reconnect(ctx context.Context, account domain.AccountID, recipient domain.ClientID, delay int) error {
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

func (mc MockClient) Close(context.Context) error {
	return nil
}

type MockConnectionManager struct {
	AccountIndex map[domain.AccountID]map[domain.ClientID]controller.Receptor
	ClientIndex  map[domain.ClientID]domain.AccountID
}

func NewMockConnectionManager() *MockConnectionManager {
	mcm := MockConnectionManager{AccountIndex: make(map[domain.AccountID]map[domain.ClientID]controller.Receptor),
		ClientIndex: make(map[domain.ClientID]domain.AccountID)}
	return &mcm
}

func (m *MockConnectionManager) Register(ctx context.Context, account domain.AccountID, clientID domain.ClientID, receptor controller.Receptor) error {
	_, ok := m.AccountIndex[account]
	if !ok {
		m.AccountIndex[account] = make(map[domain.ClientID]controller.Receptor)
	}
	m.AccountIndex[account][clientID] = receptor
	m.ClientIndex[clientID] = account
	return nil
}

func (m *MockConnectionManager) Unregister(ctx context.Context, clientID domain.ClientID) {
	account, ok := m.ClientIndex[clientID]
	if !ok {
		return
	}

	delete(m.ClientIndex, clientID)
	delete(m.AccountIndex[account], clientID)
}

func (m *MockConnectionManager) GetConnection(ctx context.Context, account domain.AccountID, clientID domain.ClientID) controller.Receptor {
	return m.AccountIndex[account][clientID]
}

func (m *MockConnectionManager) GetConnectionsByAccount(ctx context.Context, account domain.AccountID) map[domain.ClientID]controller.Receptor {
	return m.AccountIndex[account]
}

func (m *MockConnectionManager) GetAllConnections(ctx context.Context) map[domain.AccountID]map[domain.ClientID]controller.Receptor {
	return m.AccountIndex
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
		apiMux := mux.NewRouter()
		cfg := config.GetConfig()
		connectionManager := NewMockConnectionManager()
		mc := MockClient{}
		connectionManager.Register(context.TODO(), "1234", "345", mc)
		errorMC := MockClient{returnAnError: true}
		connectionManager.Register(context.TODO(), "1234", "error-client", errorMC)
		jr = NewMessageReceiver(connectionManager, apiMux, URL_BASE_PATH, cfg)
		jr.Routes()

		identity := `{ "identity": {"account_number": "540155", "type": "User", "internal": { "org_id": "1979710" } } }`
		validIdentityHeader = base64.StdEncoding.EncodeToString([]byte(identity))
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
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				jr.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusCreated))

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("id"))
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
