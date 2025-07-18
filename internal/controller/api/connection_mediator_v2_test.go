package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func mockedGetConnectionByClientID(expectedClientState domain.ConnectorClientState) connection_repository.GetConnectionByClientID {
	return func(ctx context.Context, log *logrus.Entry, actualOrgId domain.OrgID, actualClientId domain.ClientID) (domain.ConnectorClientState, error) {
		if actualOrgId != expectedClientState.OrgID {
			return domain.ConnectorClientState{}, fmt.Errorf("Actual org id does not match expected org id")
		}

		if actualClientId != expectedClientState.ClientID {
			return domain.ConnectorClientState{}, fmt.Errorf("Actual client id does not match expected client id")
		}

		return expectedClientState, nil
	}
}

func mockedGetConnectionsByOrgID(expectedClientState domain.ConnectorClientState) connection_repository.GetConnectionsByOrgID {
	return func(ctx context.Context, log *logrus.Entry, actualOrgId domain.OrgID, offset int, limit int) (map[domain.ClientID]domain.ConnectorClientState, int, error) {
		if actualOrgId != expectedClientState.OrgID {
			return map[domain.ClientID]domain.ConnectorClientState{}, 0, fmt.Errorf("Actual org id does not match expected org id")
		}

		return map[domain.ClientID]domain.ConnectorClientState{expectedClientState.ClientID: expectedClientState}, 1, nil
	}
}

func mockedGetAllConnections(expectedAccount domain.AccountID, expectedClientId domain.ClientID) connection_repository.GetAllConnections {
	return func(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]domain.ConnectorClientState, int, error) {
		allConnections := map[domain.AccountID]map[domain.ClientID]domain.ConnectorClientState{expectedAccount: {expectedClientId: {Account: expectedAccount, ClientID: expectedClientId}}}
		return allConnections, len(allConnections), nil
	}
}

var _ = Describe("ConnectionMediatorV2", func() {

	var (
		cm                  *ConnectionMediatorV2
		messageEndpointV2   string
		validIdentityHeader string
	)

	BeforeEach(func() {
		apiMux := mux.NewRouter()
		cfg := config.GetConfig()

		proxyFactory := &MockClientProxyFactory{}

		messageEndpointV2 = URL_BASE_PATH + "/v2/connections/345/message"
		accountNumber := domain.AccountID("1234")
		validIdentityHeader = buildIdentityHeader(accountNumber, "Associate")

		connectorClient := domain.ConnectorClientState{
			Account:  accountNumber,
			OrgID:    domain.OrgID("1979710"),
			ClientID: domain.ClientID("345"),
		}

		getConnByClientID := mockedGetConnectionByClientID(connectorClient)
		getConnByOrgID := mockedGetConnectionsByOrgID(connectorClient)

		cm = NewConnectionMediatorV2(getConnByClientID, getConnByOrgID, proxyFactory, apiMux, URL_BASE_PATH, cfg)
		cm.Routes()

	})

	Describe("Connecting to the v2 message endpoint", func() {
		Context("With valid identity header", func() {
			It("Should be able to send a job to a client", func() {
				postBody := "{\"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", messageEndpointV2, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				cm.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusCreated))

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("id"))
			})

			It("Should be able to send a job to a client without the payload field", func() {
				postBody := "{\"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", messageEndpointV2, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				cm.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusCreated))

				var m map[string]string
				json.Unmarshal(rr.Body.Bytes(), &m)
				Expect(m).Should(HaveKey("id"))
			})

			It("Should not be able to send a job to a client with empty post body", func() {
				postBody := "{}"

				req, err := http.NewRequest("POST", messageEndpointV2, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				cm.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})
})
