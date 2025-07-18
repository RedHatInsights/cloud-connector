package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

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
	var connections []domain.ConnectorClientState
	for i := 1; i <= connectionCount; i++ {
		connections = append(connections, domain.ConnectorClientState{})
	}

	return func(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]domain.ConnectorClientState, int, error) {
		ret := make(map[domain.AccountID]map[domain.ClientID]domain.ConnectorClientState)

		i := offset
		ret["540155"] = make(map[domain.ClientID]domain.ConnectorClientState)
		for i < len(connections) && len(ret["540155"]) < limit {
			ret["540155"][domain.ClientID(strconv.Itoa(i))] = connections[i]
			// ret["540155"][domain.ClientID](strconv.Itoa(i)) = domain.ConnectorClientState{
			// 	Account: connections[i]
			// }
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

	accountNumber := "1234"

	connectorClient := domain.ConnectorClientState{
		Account:  domain.AccountID(accountNumber),
		OrgID:    domain.OrgID("1979710"),
		ClientID: domain.ClientID("345"),
	}

	getConnByClientID := mockedGetConnectionByClientID(connectorClient)
	getConnByOrgID := mockedPaginatedGetConnectionsByAccount(connectionCount, connectorClient.OrgID, connectorClient.Account, connectorClient.ClientID)
	getAllConnections := mockedPaginatedGetAllConnections(connectionCount, connectorClient.Account, connectorClient.ClientID)
	proxyFactory := &MockClientProxyFactory{}

	mapping := map[string]*string{
		string(connectorClient.OrgID): &accountNumber,
	}

	tenantTranslator := tenantid.NewTranslatorMockWithMapping(mapping)

	managementServer := NewManagementServer(getConnByClientID, getConnByOrgID, getAllConnections, tenantTranslator, proxyFactory, apiMux, URL_BASE_PATH, cfg)
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
