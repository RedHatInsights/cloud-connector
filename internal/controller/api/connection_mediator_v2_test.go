package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	MESSAGE_ENDPOINT_V2 = URL_BASE_PATH + "/v2/connections/345/message"
	ACCOUNT_NUMBER      = domain.AccountID("1234")
)

func MockedGetConnectionByClientID(context.Context, *logrus.Entry, domain.OrgID, domain.ClientID) (domain.ConnectorClientState, error) {
	return domain.ConnectorClientState{Account: ACCOUNT_NUMBER, ClientID: "345"}, nil
}

func MockedGetConnectionsByOrgID(context.Context, *logrus.Entry, domain.OrgID, int, int) (map[domain.ClientID]domain.ConnectorClientState, int, error) {
	return map[domain.ClientID]domain.ConnectorClientState{"345": {Account: ACCOUNT_NUMBER, ClientID: "345"}}, 1, nil
}

var _ = Describe("ConnectionMediatorV2", func() {

	var (
		cm                  *ConnectionMediatorV2
		validIdentityHeader string
	)

	BeforeEach(func() {
		apiMux := mux.NewRouter()
		cfg := config.GetConfig()

		proxyFactory := &MockClientProxyFactory{}

		cm = NewConnectionMediatorV2(MockedGetConnectionByClientID, MockedGetConnectionsByOrgID, proxyFactory, apiMux, URL_BASE_PATH, cfg)
		cm.Routes()

		validIdentityHeader = buildIdentityHeader(ACCOUNT_NUMBER, "Associate")
	})

	Describe("Connecting to the v2 message endpoint", func() {
		Context("With valid identity header", func() {
			It("Should be able to send a job to a client", func() {
				postBody := "{\"payload\": [\"678\"], \"directive\": \"fred:flintstone\"}"

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT_V2, strings.NewReader(postBody))
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

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT_V2, strings.NewReader(postBody))
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

				req, err := http.NewRequest("POST", MESSAGE_ENDPOINT_V2, strings.NewReader(postBody))
				Expect(err).NotTo(HaveOccurred())

				req.Header.Add(IDENTITY_HEADER_NAME, validIdentityHeader)

				rr := httptest.NewRecorder()

				cm.router.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusBadRequest))
			})
		})
	})
})
