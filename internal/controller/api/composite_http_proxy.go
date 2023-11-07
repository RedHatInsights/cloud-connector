package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/redhatinsights/platform-go-middlewares/request_id"

	"github.com/google/uuid"
	//"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	errUnableToSendMessage = errors.New("unable to send message")
)

type ConnectorClientHTTPProxy struct {
	Url            string
	Logger         *logrus.Entry
	Config         *config.Config
	OrgID          domain.OrgID
	AccountID      domain.AccountID
	ClientID       domain.ClientID
	Dispatchers    domain.Dispatchers
	CanonicalFacts domain.CanonicalFacts
	Tags           domain.Tags
}

func (this *ConnectorClientHTTPProxy) SendMessage(ctx context.Context, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	//	metrics.sentMessageDirectiveCounter.With(prometheus.Labels{"directive": directive}).Inc()

	requestID := getRequestId(ctx)

	logger := logger.Log.WithFields(logrus.Fields{"client_id": this.ClientID, "request_id": requestID})

	this.Logger.Debug("Sending data message to connected client")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	request := messageRequestV2{
		Directive: directive,
		Payload:   payload,
		Metadata:  metadata,
	}

	requestJson, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/message", this.Url, this.ClientID)

	req, err := http.NewRequest("POST", u, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-insights-request-id", requestID)
	req.Header.Add("x-rh-cloud-connector-org-id", string(this.OrgID))
	req.Header.Add("x-rh-cloud-connector-client-id", "cloud-connector-composite")
	req.Header.Add("x-rh-cloud-connector-psk", "secret_used_by_composite")

	logger.Debug("About to call child Cloud-Connector")
	r, err := client.Do(req)
	logger.Debug("Returned from call to child Cloud-Connector")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Call to child Cloud-Connector failed")
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != 201 {
		logger.Debugf("Call to child Cloud-Connector returned http status code %d", r.StatusCode)
		return nil, fmt.Errorf("Unable to find connection")
	}

	var resp messageResponse
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse response from child Cloud-Connector")
		return nil, err
	}

	messageID, err := uuid.Parse(resp.JobID)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse message id returned by child Cloud-Connector")
		return nil, errUnableToSendMessage
	}

	return &messageID, nil
}

func (this *ConnectorClientHTTPProxy) Ping(ctx context.Context) error {
	return fmt.Errorf("Not implemented!!")
}

func (this *ConnectorClientHTTPProxy) Reconnect(ctx context.Context, message string, delay int) error {
	return fmt.Errorf("Not implemented!!")
}

func (this *ConnectorClientHTTPProxy) GetDispatchers(ctx context.Context) (domain.Dispatchers, error) {
	return this.Dispatchers, nil
}

func (this *ConnectorClientHTTPProxy) GetCanonicalFacts(ctx context.Context) (domain.CanonicalFacts, error) {
	return this.CanonicalFacts, nil
}

func (this *ConnectorClientHTTPProxy) GetTags(ctx context.Context) (domain.Tags, error) {
	return this.Tags, nil
}

func (this *ConnectorClientHTTPProxy) Disconnect(ctx context.Context, message string) error {
	return fmt.Errorf("Not implemented!!")
}

func getRequestId(ctx context.Context) string {
	requestId := request_id.GetReqID(ctx)

	if requestId == "" {
		return uuid.NewString()
	}

	return requestId
}
