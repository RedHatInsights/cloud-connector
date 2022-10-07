package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type PaginatedMockConnectionManager struct {
	connections []controller.ConnectorClient
}

func NewPaginatedMockConnectionManager() *PaginatedMockConnectionManager {
	mcm := PaginatedMockConnectionManager{connections: make([]controller.ConnectorClient, 0)}
	return &mcm
}

func (m *PaginatedMockConnectionManager) Register(ctx context.Context, rhcClient domain.ConnectorClientState) error {
	mockClient := MockClient{}
	m.connections = append(m.connections, mockClient)
	return nil
}

func (m *PaginatedMockConnectionManager) Unregister(ctx context.Context, clientID domain.ClientID) {
	return
}

func (m *PaginatedMockConnectionManager) FindConnectionByClientID(ctx context.Context, clientID domain.ClientID) (domain.ConnectorClientState, error) {
	return domain.ConnectorClientState{}, nil
}

func (m *PaginatedMockConnectionManager) GetConnection(ctx context.Context, account domain.AccountID, orgID domain.OrgID, clientID domain.ClientID) controller.ConnectorClient {
	return nil
}

func (m *PaginatedMockConnectionManager) GetConnectionsByAccount(ctx context.Context, account domain.AccountID, offset int, limit int) (map[domain.ClientID]controller.ConnectorClient, int, error) {

	ret := make(map[domain.ClientID]controller.ConnectorClient)

	i := offset
	for i < len(m.connections) && len(ret) < limit {
		ret[domain.ClientID(strconv.Itoa(i))] = m.connections[i]
		i++
	}

	return ret, len(m.connections), nil
}

func mockedPaginatedGetConnectionsByAccount(connectionCount int, expectedOrgId domain.OrgID, expectedAccount domain.AccountID, expectedClientId domain.ClientID) connection_repository.GetConnectionsByOrgID {

	return func(ctx context.Context, log *logrus.Entry, actualOrgId domain.OrgID, offset int, limit int) (map[domain.ClientID]domain.ConnectorClientState, int, error) {

		ret := make(map[domain.ClientID]domain.ConnectorClientState)

		i := offset
		for i < connectionCount && len(ret) < limit {
			ret[domain.ClientID(strconv.Itoa(i))] = domain.ConnectorClientState{Account: expectedAccount, OrgID: expectedOrgId, ClientID: expectedClientId}
			i++
		}

		return ret, connectionCount, nil
	}
}

func mockedPaginatedGetAllConnections(connectionCount int, expectedAccount domain.AccountID, expectedClientId domain.ClientID) connection_repository.GetAllConnections {
	var connections []MockClient
	for i := 1; i <= connectionCount; i++ {
		connections = append(connections, MockClient{})
	}

	return func(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient, int, error) {
		ret := make(map[domain.AccountID]map[domain.ClientID]controller.ConnectorClient)

		i := offset
		ret["540155"] = make(map[domain.ClientID]controller.ConnectorClient)
		for i < len(connections) && len(ret["540155"]) < limit {
			ret["540155"][domain.ClientID(strconv.Itoa(i))] = connections[i]
			i++
		}

		return ret, len(connections), nil
	}
}

var _ = Describe("Managment API Pagination - 11 connections total", func() {

	var (
		ms                  *ManagementServer
		validIdentityHeader string
	)

	BeforeEach(func() {
		ms, validIdentityHeader = testSetup(11)
	})

	Describe("All connections endpoint - returning 5 results", func() {
		It("Meta count should be 11, links should be populated", func() {

			baseEndpointUrl := CONNECTION_LIST_ENDPOINT

			var expectedResponse = paginatedResponse{
				Meta: meta{Count: 11},
				Links: navigationLinks{
					First: baseEndpointUrl + "?limit=5&offset=0",
					Last:  baseEndpointUrl + "?limit=5&offset=10",
					Next:  baseEndpointUrl + "?limit=5&offset=5",
					Prev:  "",
				},
				Data: []interface{}{},
			}

			runTest(baseEndpointUrl+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)

			expectedResponse.Links.Prev = baseEndpointUrl + "?limit=5&offset=0"
			expectedResponse.Links.Next = baseEndpointUrl + "?limit=5&offset=7"

			runTest(baseEndpointUrl+"?offset=2&limit=5", ms, validIdentityHeader, expectedResponse)

			expectedResponse.Links.Prev = baseEndpointUrl + "?limit=5&offset=5"
			expectedResponse.Links.Next = ""

			runTest(baseEndpointUrl+"?offset=10&limit=5", ms, validIdentityHeader, expectedResponse)
		})
	})

	Describe("Connections per account endpoint - returning 5 results", func() {
		It("Meta count should be 11, links should be populated", func() {

			baseEndpointUrl := CONNECTION_LIST_ENDPOINT + "/1234"

			var expectedResponse = paginatedResponse{
				Meta: meta{Count: 11},
				Links: navigationLinks{
					First: baseEndpointUrl + "?limit=5&offset=0",
					Last:  baseEndpointUrl + "?limit=5&offset=10",
					Next:  baseEndpointUrl + "?limit=5&offset=5",
					Prev:  "",
				},
				Data: []interface{}{},
			}

			runTest(baseEndpointUrl+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)

			expectedResponse.Links.Prev = baseEndpointUrl + "?limit=5&offset=0"
			expectedResponse.Links.Next = baseEndpointUrl + "?limit=5&offset=7"

			runTest(baseEndpointUrl+"?offset=2&limit=5", ms, validIdentityHeader, expectedResponse)

			expectedResponse.Links.Prev = baseEndpointUrl + "?limit=5&offset=5"
			expectedResponse.Links.Next = ""

			runTest(baseEndpointUrl+"?offset=10&limit=5", ms, validIdentityHeader, expectedResponse)
		})
	})

})

var _ = Describe("Managment API Pagination - 0 connections total", func() {

	var (
		ms                  *ManagementServer
		validIdentityHeader string
	)

	BeforeEach(func() {
		ms, validIdentityHeader = testSetup(0)
	})

	Describe("All connections endpoint - returning no results", func() {
		It("Meta count should be 0, links should be empty", func() {

			var expectedResponse = paginatedResponse{
				Meta:  meta{Count: 0},
				Links: navigationLinks{},
				Data:  []interface{}{},
			}

			runTest(CONNECTION_LIST_ENDPOINT+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)
		})
	})

	Describe("Connections per account endpoint - returning no results", func() {
		It("Meta count should be 0, links should be empty", func() {

			var expectedResponse = paginatedResponse{
				Meta:  meta{Count: 0},
				Links: navigationLinks{},
				Data:  []interface{}{},
			}

			runTest(CONNECTION_LIST_ENDPOINT+"/1234"+"?offset=0&limit=5", ms, validIdentityHeader, expectedResponse)
		})
	})

})

func testSetup(connectionCount int) (*ManagementServer, string) {
	apiMux := mux.NewRouter()
	cfg := config.GetConfig()
	connectionManager := NewPaginatedMockConnectionManager()

	accountNumber := domain.AccountID("1234")
	orgID := domain.OrgID("1978710")
	clientID := domain.ClientID("345")

	getConnByClientID := mockedGetConnectionByClientID(orgID, accountNumber, clientID)
	getConnByOrgID := mockedPaginatedGetConnectionsByAccount(connectionCount, orgID, accountNumber, clientID)
	getAllConnections := mockedPaginatedGetAllConnections(connectionCount, accountNumber, clientID)
	proxyFactory := &MockClientProxyFactory{}

	accountNumberStr := string(accountNumber)

	mapping := map[string]*string{
		string(orgID): &accountNumberStr,
	}

	tenantTranslator := tenantid.NewTranslatorMockWithMapping(mapping)

	managementServer := NewManagementServer(connectionManager, getConnByClientID, getConnByOrgID, getAllConnections, tenantTranslator, proxyFactory, apiMux, URL_BASE_PATH, cfg)
	managementServer.Routes()

	return managementServer, buildIdentityHeader("540155", "Associate")
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
