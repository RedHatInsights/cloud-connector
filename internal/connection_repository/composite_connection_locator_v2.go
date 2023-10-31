package connection_repository

import (
	"context"
	"encoding/json"
	//"errors"
	"fmt"
	//"io/ioutil"
	"net/http"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	//"github.com/prometheus/client_golang/prometheus"
	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/sirupsen/logrus"
)

func NewCompositeGetConnectionByClientID(cfg *config.Config, urls []string, connectionLocationCache *expirable.LRU[domain.ClientID, string]) (GetConnectionByClientID, error) {

	return createGetConnectionByClientIDCompositeImpl(cfg, urls, connectionLocationCache)
}

func createGetConnectionByClientIDCompositeImpl(cfg *config.Config, urls []string, connectionLocationCache *expirable.LRU[domain.ClientID, string]) (GetConnectionByClientID, error) {

	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, clientId domain.ClientID) (domain.ConnectorClientState, error) {
		var err error

		err = verifyOrgId(orgId)
		if err != nil {
			return domain.ConnectorClientState{}, err
		}

		err = verifyClientId(clientId)
		if err != nil {
			return domain.ConnectorClientState{}, err
		}

		cachedConnectionLocationUrl, ok := connectionLocationCache.Get(clientId)
		if ok {
			return getConnectionState(ctx, log, orgId, clientId, cachedConnectionLocationUrl)
		}

		connectionState, err := lookupConnectionState(ctx, log, orgId, clientId, urls, connectionLocationCache)
		if err == nil {
			return connectionState, err
		}

		return domain.ConnectorClientState{}, NotFoundError
	}, nil
}

func lookupConnectionState(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, clientId domain.ClientID, urls []string, cache *expirable.LRU[domain.ClientID, string]) (domain.ConnectorClientState, error) {
	for i := 0; i < len(urls); i++ {
		clientState, err := getConnectionState(ctx, log, orgId, clientId, urls[i])
		if err == nil {
			fmt.Println("url: ", urls[i])
			cache.Add(clientId, urls[i])
			return clientState, nil
		}
	}

	return domain.ConnectorClientState{}, NotFoundError
}

func getConnectionState(ctx context.Context, log *logrus.Entry, orgID domain.OrgID, clientID domain.ClientID, url string) (domain.ConnectorClientState, error) {

	var clientState domain.ConnectorClientState
	var err error

	requestID := getRequestId(ctx)

	logger := log.WithFields(logrus.Fields{"client_id": clientID, "request_id": requestID, "url": url})

	logger.Infof("Searching for connection - org id: %s, client id: %s", orgID, clientID)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	u := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/status", url, clientID)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return clientState, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-insights-request-id", requestID)
	req.Header.Add("x-rh-cloud-connector-org-id", string(orgID))
	req.Header.Add("x-rh-cloud-connector-client-id", "cloud-connector-composite")
	req.Header.Add("x-rh-cloud-connector-psk", "secret_used_by_composite")

	logger.Debug("About to call child Cloud-Connector")
	r, err := client.Do(req)
	logger.Debug("Returned from call to child Cloud-Connector")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Call to child Cloud-Connector failed")
		return clientState, err
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		logger.Debugf("Call to child Cloud-Connector returned http status code %d", r.StatusCode)
		return clientState, fmt.Errorf("Unable to find connection")
	}

	type cloudConnectorStatusResponse struct {
		Status         string                `json:"status"`
		Account        string                `json:"account"`
		OrgID          string                `json:"org_id"`
		ClientID       string                `json:"client_id"`
		CanonicalFacts domain.CanonicalFacts `json:"canonical_facts"`
		Dispatchers    domain.Dispatchers    `json:"dispatchers"`
		Tags           domain.Tags           `json:"tags"`
	}

	var resp cloudConnectorStatusResponse
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse response from child Cloud-Connector")
		return clientState, err
	}

	fmt.Println("\n\nresp: ", resp)

	if resp.Status != "connected" {
		return clientState, NotFoundError
	}

	clientState.Account = domain.AccountID(resp.Account)
	clientState.OrgID = domain.OrgID(resp.OrgID)
	clientState.ClientID = domain.ClientID(resp.ClientID)
	clientState.CanonicalFacts = resp.CanonicalFacts
	clientState.Dispatchers = resp.Dispatchers
	clientState.Tags = resp.Tags

	return clientState, nil
}

func NewCompositeGetConnectionsByOrgID(cfg *config.Config) (GetConnectionsByOrgID, error) {

	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, offset int, limit int) (map[domain.ClientID]domain.ConnectorClientState, int, error) {

		var totalConnections int

		connectionsPerAccount := make(map[domain.ClientID]domain.ConnectorClientState)

		return connectionsPerAccount, totalConnections, nil
	}, nil
}

func NewCompositeGetAllConnections(cfg *config.Config) (GetAllConnections, error) {
	return func(ctx context.Context, offset int, limit int) (map[domain.AccountID]map[domain.ClientID]domain.ConnectorClientState, int, error) {
		var totalConnections int

		connectionMap := make(map[domain.AccountID]map[domain.ClientID]domain.ConnectorClientState)

		return connectionMap, totalConnections, nil
	}, nil
}

func getRequestId(ctx context.Context) string {
	requestId := request_id.GetReqID(ctx)

	if requestId == "" {
		return uuid.NewString()
	}

	return requestId
}
