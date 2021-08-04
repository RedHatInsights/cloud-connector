package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/gorilla/mux"
)

type PaginatedMockConnectionManager struct {
	connections []controller.Receptor
}

func NewPaginatedMockConnectionManager() *PaginatedMockConnectionManager {
	mcm := PaginatedMockConnectionManager{connections: make([]controller.Receptor, 0)}
	return &mcm
}

func (m *PaginatedMockConnectionManager) Register(ctx context.Context, account domain.AccountID, clientID domain.ClientID, receptor controller.Receptor) error {
	m.connections = append(m.connections, receptor)
	return nil
}

func (m *PaginatedMockConnectionManager) Unregister(ctx context.Context, clientID domain.ClientID) {
	return
}

func (m *PaginatedMockConnectionManager) GetConnection(ctx context.Context, account domain.AccountID, clientID domain.ClientID) controller.Receptor {
	return nil
}

func (m *PaginatedMockConnectionManager) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.Receptor, int, error) {

	ret := make(map[domain.ClientID]controller.Receptor)

	i := offset
	for i < len(m.connections) && len(ret) < limit {
		ret[domain.ClientID(strconv.Itoa(i))] = m.connections[i]
		i++
	}

	return ret, len(m.connections), nil
}

func (m *PaginatedMockConnectionManager) GetAllConnections(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.Receptor, int, error) {
	ret := make(map[domain.AccountID]map[domain.ClientID]controller.Receptor)

	fmt.Println("**** offset:", offset)
	fmt.Println("**** limit:", limit)

	i := offset
	ret["540155"] = make(map[domain.ClientID]controller.Receptor)
	for i < len(m.connections) && len(ret["540155"]) < limit {
		fmt.Println("**** i:", i)
		ret["540155"][domain.ClientID(strconv.Itoa(i))] = m.connections[i]
		i++
	}

	return ret, len(m.connections), nil
}

var _ = Describe("Managment API Pagination - All Connections", func() {

	var (
		ms                  *ManagementServer
		validIdentityHeader string
	)

	BeforeEach(func() {
		ms, validIdentityHeader = testSetup(11)
	})

	Describe("Connecting to the connection list endpoint", func() {
		It("Should be able to get a list of open connections", func() {

			var expectedResponse = paginatedResponse{
				Meta: meta{Count: 11},
				Links: navigationLinks{
					First: "/api/cloud-connector/api/v1/connection?limit=5&offset=0",
					Last:  "/api/cloud-connector/api/v1/connection?limit=5&offset=10",
					Next:  "/api/cloud-connector/api/v1/connection?limit=5&offset=5",
					Prev:  "",
				},
				Data: []interface{}{},
			}

			runTest(CONNECTION_LIST_ENDPOINT+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)

			//            runTest(CONNECTION_LIST_ENDPOINT+"/540155"+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)

		})

		It("Should be able to get a list of open connections #2", func() {
			var expectedResponse = paginatedResponse{
				Meta: meta{Count: 11},
				Links: navigationLinks{
					First: "/api/cloud-connector/api/v1/connection?limit=5&offset=0",
					Last:  "/api/cloud-connector/api/v1/connection?limit=5&offset=10",
					Next:  "/api/cloud-connector/api/v1/connection?limit=5&offset=7",
					Prev:  "/api/cloud-connector/api/v1/connection?limit=5&offset=0",
				},
				Data: []interface{}{},
			}

			runTest(CONNECTION_LIST_ENDPOINT+"?offset=2&limit=5", ms, validIdentityHeader, expectedResponse)

		})

		It("Should be able to get a list of open connections #3", func() {
			var expectedResponse = paginatedResponse{
				Meta: meta{Count: 11},
				Links: navigationLinks{
					First: "/api/cloud-connector/api/v1/connection?limit=5&offset=0",
					Last:  "/api/cloud-connector/api/v1/connection?limit=5&offset=10",
					Next:  "",
					Prev:  "/api/cloud-connector/api/v1/connection?limit=5&offset=5",
				},
				Data: []interface{}{},
			}

			runTest(CONNECTION_LIST_ENDPOINT+"?offset=10&limit=5", ms, validIdentityHeader, expectedResponse)
		})
	})
})

var _ = Describe("Managment API Pagination - All Connections - no results", func() {

	var (
		ms                  *ManagementServer
		validIdentityHeader string
	)

	BeforeEach(func() {
		ms, validIdentityHeader = testSetup(0)
	})

	Describe("Connecting to the connection list endpoint - returning no results", func() {
		It("Meta count should be 0, links should be empty", func() {

			var expectedResponse = paginatedResponse{
				Meta: meta{Count: 0},
				Links: navigationLinks{
					First: "",
				},
				Data: []interface{}{},
			}

			runTest(CONNECTION_LIST_ENDPOINT+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)
		})
	})

	// Add a test for the case where there are connections to return

})

func testSetup(connectionCount int) (*ManagementServer, string) {
	apiMux := mux.NewRouter()
	cfg := config.GetConfig()
	connectionManager := NewPaginatedMockConnectionManager()

	i := 0
	for i < connectionCount {
		mc := MockClient{}
		client_id := strconv.Itoa(i)
		connectionManager.Register(context.TODO(), CONNECTED_ACCOUNT_NUMBER, domain.ClientID(client_id), mc)
		i++
	}

	managementServer := NewManagementServer(connectionManager, apiMux, URL_BASE_PATH, cfg)
	managementServer.Routes()

	identity := `{ "identity": {"account_number": "540155", "type": "User", "internal": { "org_id": "1979710" } } }`
	identityHeader := base64.StdEncoding.EncodeToString([]byte(identity))

	return managementServer, identityHeader
}

func runTest(endpoint string, managementServer *ManagementServer, identityHeader string, expectedResponse paginatedResponse) {
	req, err := http.NewRequest("GET", endpoint, nil)
	Expect(err).NotTo(HaveOccurred())

	req.Header.Add(IDENTITY_HEADER_NAME, identityHeader)

	rr := httptest.NewRecorder()

	managementServer.router.ServeHTTP(rr, req)

	Expect(rr.Code).To(Equal(http.StatusOK))

	var actualResponse paginatedResponse
	json.Unmarshal(rr.Body.Bytes(), &actualResponse)

	Expect(actualResponse.Meta).Should(Equal(expectedResponse.Meta))
	Expect(actualResponse.Links).Should(Equal(expectedResponse.Links))
}
