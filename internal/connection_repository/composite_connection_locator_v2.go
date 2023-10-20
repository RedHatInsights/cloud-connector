package connection_repository

import (
	"context"
	//"encoding/json"
	//"errors"
	"fmt"
	//"io/ioutil"
	"net/http"
    "time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/uuid"
	//"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

func NewCompositeGetConnectionByClientID(cfg *config.Config, urls []string, cache map[domain.ClientID]string) (GetConnectionByClientID, error) {

	return createGetConnectionByClientIDCompositeImpl(cfg, urls, cache)
}

func createGetConnectionByClientIDCompositeImpl(cfg *config.Config, urls []string, cache map[domain.ClientID]string) (GetConnectionByClientID, error) {

	return func(ctx context.Context, log *logrus.Entry, orgId domain.OrgID, clientId domain.ClientID) (domain.ConnectorClientState, error) {
		var clientState domain.ConnectorClientState
		var err error

		err = verifyOrgId(orgId)
		if err != nil {
			return clientState, err
		}

		err = verifyClientId(clientId)
		if err != nil {
			return clientState, err
		}

        // FIXME: Look in the cache
        fmt.Println("LOOK IN THE CACHE DUMB DUMB")

        for i := 0; i < len(urls); i++ {
            clientState, err := makeHttpCall(orgId, clientId, urls[i])
            if err == nil {

                // FIXME: store it in the cache along with the url
                fmt.Println("STORE IT IN THE CACHE DUMB DUMB")
                
                cache[clientId] = urls[i]

                return clientState, nil
            }
        }

		return clientState, fmt.Errorf("NOT FOUND!!  FIXME!! ISN'T THERE AN EXISting ERROR TYpe")
	}, nil
}


func makeHttpCall(orgID domain.OrgID, clientID domain.ClientID, url string) (domain.ConnectorClientState, error) {

    var clientState domain.ConnectorClientState
    var err error

    // FIXME:  get the request id from context??  Kinda gross??
	requestID := uuid.NewString()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": clientID, "request_id": requestID})

	logger.Infof("Searching for connection - org id: %s, client id: %s", orgID, clientID)

	client := &http.Client{
		Timeout: 5*time.Second,
	}

    u := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/status", url, clientID)

    //curl -v -s -X GET -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/$NODE_ID/status | jq

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return clientState, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-insights-request-id", requestID)
	req.Header.Add("x-rh-cloud-connector-org-id", string(orgID))
	req.Header.Add("x-rh-cloud-connector-client-id", "PSK-client-id")
	req.Header.Add("x-rh-cloud-connector-psk", "PSK")

	logger.Debug("About to call backend cloud-connector")
	r, err := client.Do(req)
	logger.Debug("Returned from call to backend cloud-connector")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Call to backend cloud-connector failed")
		return clientState, err 
	}
	defer r.Body.Close()

	if r.StatusCode != 200 {
		logger.Debugf("Call to Auth Gateway returned http status code %d", r.StatusCode)
        return clientState, fmt.Errorf("Unable to find connection")
	}

    /*
	var resp AuthGwResp
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse Auth Gateway response")
		return "", "", "", err
	}

	logger.WithFields(logrus.Fields{"account": jsonData.Identity.AccountNumber, "org_id": jsonData.Identity.Internal.OrgID}).Debug("Located account number and org ID for client")


	return domain.Identity(resp.Identity), domain.AccountID(jsonData.Identity.AccountNumber), domain.OrgID(jsonData.Identity.Internal.OrgID), nil
    */
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
