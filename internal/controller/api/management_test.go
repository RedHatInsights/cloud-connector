package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/gorilla/mux"
)

const (
	CONNECTION_LIST_ENDPOINT       = URL_BASE_PATH + "/connection"
	CONNECTION_STATUS_ENDPOINT     = URL_BASE_PATH + "/connection/status"
	CONNECTION_DISCONNECT_ENDPOINT = URL_BASE_PATH + "/connection/disconnect"

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

var _ = Describe("Management", func() {

	var (
		ms                  *ManagementServer
		validIdentityHeader string
	)

	BeforeEach(func() {
		apiMux := mux.NewRouter()
		cfg := config.GetConfig()
		connectionManager := NewMockConnectionManager()
		mc := MockClient{}
		connectionManager.Register(context.TODO(), CONNECTED_ACCOUNT_NUMBER, CONNECTED_NODE_ID, mc)
		ms = NewManagementServer(connectionManager, apiMux, URL_BASE_PATH, cfg)
		ms.Routes()

		identity := `{ "identity": {"account_number": "540155", "type": "User", "internal": { "org_id": "1979710" } } }`
		validIdentityHeader = base64.StdEncoding.EncodeToString([]byte(identity))
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

				// FIXME: need to verify that connection list
				var m map[string]interface{}
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("connections"))
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

				var m map[string][]string
				expected := map[string][]string{"connections": []string{"345"}}
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(Equal(expected))
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

})
