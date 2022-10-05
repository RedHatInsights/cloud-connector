package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"

	"github.com/gorilla/mux"
)

const (
	CONNECTION_LIST_ENDPOINT       = URL_BASE_PATH + "/v1/connection"
	CONNECTION_STATUS_ENDPOINT     = URL_BASE_PATH + "/v1/connection/status"
	CONNECTION_RECONNECT_ENDPOINT  = URL_BASE_PATH + "/v1/connection/reconnect"
	CONNECTION_DISCONNECT_ENDPOINT = URL_BASE_PATH + "/v1/connection/disconnect"
	CONNECTION_PING_ENDPOINT       = URL_BASE_PATH + "/v1/connection/ping"

	CONNECTED_ACCOUNT_NUMBER = "1234"
	CONNECTED_NODE_ID        = "345"
)

func init() {
	logger.InitLogger()
}

func createConnectionStatusPostBody(account_number string, node_id string) io.Reader {
	jsonString := fmt.Sprintf("{\"account\": \"%s\", \"node_id\": \"%s\"}", account_number, node_id)
	return strings.NewReader(jsonString)
}

func createConnectionReconnectPostBody(account_number string, node_id string, delay int, message string) io.Reader {
	jsonString := fmt.Sprintf("{\"account\": \"%s\", \"node_id\": \"%s\", \"delay\": %d, \"message\": \"%s\"}", account_number, node_id, delay, message)
	return strings.NewReader(jsonString)
}

var _ = Describe("Management", func() {

	var (
		ms                  *ManagementServer
		validIdentityHeader string
		accountNumber       domain.AccountID
	)

	BeforeEach(func() {
		apiMux := mux.NewRouter()
		cfg := config.GetConfig()
		cfg.ServiceToServiceCredentials["test_client_1"] = "12345"
		connectionManager := NewMockConnectionManager()
		connectorClient := domain.ConnectorClientState{Account: CONNECTED_ACCOUNT_NUMBER, ClientID: CONNECTED_NODE_ID}
		connectionManager.Register(context.TODO(), connectorClient)

		accountNumber = "1234"
		orgID := domain.OrgID("1979710")
		clientID := domain.ClientID("345")

		getConnByClientID := mockedGetConnectionByClientID(orgID, accountNumber, clientID)
		proxyFactory := &MockClientProxyFactory{}

		accountNumberStr := CONNECTED_ACCOUNT_NUMBER

		mapping := map[string]*string{
			string(orgID): &accountNumberStr,
		}

		tenantTranslator := tenantid.NewTranslatorMockWithMapping(mapping)

		ms = NewManagementServer(connectionManager, getConnByClientID, tenantTranslator, proxyFactory, apiMux, URL_BASE_PATH, cfg)
		ms.Routes()

		validIdentityHeader = buildIdentityHeader("540155", "Associate")
	})

	Describe("Connecting to the connection/status endpoint", func() {
		Context("With a valid identity header", func() {
			It("Should be able get the status of a connected customer", func() {

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_STATUS_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKeyWithValue("status", CONNECTED_STATUS))

				Expect(rr.Code).To(Equal(http.StatusOK))
			})

			It("Should be able to get the status of a disconnected customer", func() {

				postBody := createConnectionStatusPostBody("1234-not-here", CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_STATUS_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKeyWithValue("status", DISCONNECTED_STATUS))

				Expect(rr.Code).To(Equal(http.StatusOK))
			})

			It("Should not be able get the status of a connected customer without providing account number", func() {

				postBody := createConnectionStatusPostBody("", CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_STATUS_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should not be able get the status of a connected customer without providing the node id", func() {

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, "")

				req, err := http.NewRequest("POST", CONNECTION_STATUS_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

		Context("With valid service to service credentials", func() {
			It("Should be able to get the status of a connected customer", func() {
				ms.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_STATUS_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKeyWithValue("status", CONNECTED_STATUS))

				Expect(rr.Code).To(Equal(http.StatusOK))
			})
		})

		Context("Without an identity header or service to service credentials", func() {
			It("Should fail to send a job to a connected customer", func() {

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_STATUS_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})

		})

	})

	Describe("Connecting to the connection/disconnect endpoint", func() {
		Context("With a valid identity header", func() {
			It("Should be able to disconnect a connected customer", func() {

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))

				// FIXME: need to verify that diconnect is called on the client connection
			})

			It("Should not be able to disconnect a disconnected customer", func() {

				postBody := createConnectionStatusPostBody("1234-not-here", CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				fmt.Println("rr.body...", rr.Body)
				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should not be able to disconnect a connected customer without providing account number", func() {

				postBody := createConnectionStatusPostBody("", CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

			It("Should not be able get the status of a connected customer without providing the node id", func() {

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, "")

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

		Context("With valid service to service credentials", func() {
			It("Should be able to disconnect a connected customer", func() {
				ms.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))

				// FIXME: need to verify that diconnect is called on the client connection
			})

			It("Should not be able to disconnect a disconnected customer", func() {
				ms.config.ServiceToServiceCredentials["test_client_1"] = "12345"

				postBody := createConnectionStatusPostBody("1234-not-here", CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(TOKEN_HEADER_CLIENT_NAME, "test_client_1")
				req.Header.Add(TOKEN_HEADER_ACCOUNT_NAME, "0000001")
				req.Header.Add(TOKEN_HEADER_PSK_NAME, "12345")

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

		Context("Without an identity header or service to service credentials", func() {
			It("Should fail to send a job to a connected customer", func() {

				postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

				req, err := http.NewRequest("POST", CONNECTION_DISCONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})

		})

	})

	Describe("Connecting to the connection list endpoint", func() {
		Context("With a valid identity header", func() {
			It("Should be able to get a list of open connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT, nil)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))

				var actualResponse paginatedResponse

				var expectedResponse = paginatedResponse{
					Meta: meta{Count: 1},
					Links: navigationLinks{
						First: "/api/cloud-connector/v1/connection?limit=1000&offset=0",
						Last:  "/api/cloud-connector/v1/connection?limit=1000&offset=0",
						Next:  "",
						Prev:  "",
					},
					Data: []interface{}{
						map[string]interface{}{
							"account":     "1234",
							"connections": []interface{}{string("345")},
						},
					},
				}

				json.Unmarshal(rr.Body.Bytes(), &actualResponse)

				Expect(actualResponse).Should(Equal(expectedResponse))
			})

		})

		Context("Without an identity header", func() {
			It("Should fail to get a list of connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT, nil)
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})

		})

		Context("With an invalid limit", func() {
			It("Should fail to get a list of connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT+"?limit=fred", nil)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

		Context("With an invalid offset", func() {
			It("Should fail to get a list of connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT+"?offset=barney", nil)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

	})

	Describe("Connecting to the connection list endpoint with account identifier", func() {
		Context("With a valid identity header", func() {
			It("Should be able to get a list of open connections for provided account", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT+"/"+CONNECTED_ACCOUNT_NUMBER, nil)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))

				var actualResponse paginatedResponse
				var expectedResponse = paginatedResponse{
					Meta: meta{Count: 1},
					Links: navigationLinks{
						First: "/api/cloud-connector/v1/connection/1234?limit=1000&offset=0",
						Last:  "/api/cloud-connector/v1/connection/1234?limit=1000&offset=0",
						Next:  "",
						Prev:  "",
					},
					Data: []interface{}{string("345")},
				}

				json.Unmarshal(rr.Body.Bytes(), &actualResponse)

				Expect(actualResponse).Should(Equal(expectedResponse))
			})

		})

		Context("Without an identity header", func() {
			It("Should fail to get a list of connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT+"/"+CONNECTED_ACCOUNT_NUMBER, nil)
				Expect(err).NotTo(HaveOccurred())

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusUnauthorized))
			})

		})

		Context("With an invalid limit", func() {
			It("Should fail to get a list of connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT+"/"+CONNECTED_ACCOUNT_NUMBER+"?limit=fred", nil)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

		Context("With an invalid offset", func() {
			It("Should fail to get a list of connections", func() {

				req, err := http.NewRequest("GET", CONNECTION_LIST_ENDPOINT+"/"+CONNECTED_ACCOUNT_NUMBER+"?offset=barney", nil)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})

		})

	})

	Describe("Connecting to the reconnect endpoint", func() {
		Context("With a negative delay", func() {
			It("Should fail to reconnect", func() {

				postBody := createConnectionReconnectPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID, -1, "requesting to reconnect")

				req, err := http.NewRequest("POST", CONNECTION_RECONNECT_ENDPOINT, postBody)
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				ms.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))

				var actualResponse errorResponse
				var expectedResponse = errorResponse{
					Title:  NEGATIVE_DELAY_ERROR,
					Status: http.StatusBadRequest,
					Detail: NEGATIVE_DELAY_ERROR,
				}

				json.Unmarshal(rr.Body.Bytes(), &actualResponse)

				Expect(actualResponse).Should(Equal(expectedResponse))
			})
		})
	})

	DescribeTable("Connecting to the connection/ping endpoint",
		func(expectedStatusCode int, headers map[string]string) {

			postBody := createConnectionStatusPostBody(CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID)

			req, err := http.NewRequest("POST", CONNECTION_PING_ENDPOINT, postBody)
			Expect(err).NotTo(HaveOccurred())

			for k, v := range headers {
				req.Header.Add(k, v)
			}

			rr := httptest.NewRecorder()

			ms.router.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(expectedStatusCode))
		},

		Entry("authenticated internal user",
			http.StatusOK,
			map[string]string{IDENTITY_HEADER_NAME: buildIdentityHeader("540155", "Associate")}),
		Entry("authenticated user",
			http.StatusUnauthorized,
			map[string]string{IDENTITY_HEADER_NAME: buildIdentityHeader("540155", "User")}),
		Entry("valid psk",
			http.StatusOK,
			map[string]string{TOKEN_HEADER_CLIENT_NAME: "test_client_1",
				TOKEN_HEADER_ACCOUNT_NAME: "0000001",
				TOKEN_HEADER_PSK_NAME:     "12345"}),
		Entry("invalid psk",
			http.StatusUnauthorized,
			map[string]string{TOKEN_HEADER_CLIENT_NAME: "test_client_1",
				TOKEN_HEADER_ACCOUNT_NAME: "0000001",
				TOKEN_HEADER_PSK_NAME:     "wrong"}),
	)

})
