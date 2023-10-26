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

	// FIXME:  get the request id from context??  Kinda gross??
	requestID := uuid.NewString()

	logger := logger.Log.WithFields(logrus.Fields{"client_id": this.ClientID, "request_id": requestID})

	this.Logger.Debug("Sending data message to connected client")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	/*
	   curl -v -X POST \
	   -d "{\"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" \
	   -H "x-rh-identity: $IDENTITY_HEADER" \
	   -H "x-rh-insights-request-id: 1234" \
	   http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/$NODE_ID/message
	*/

	type cloudConnectorSendMessageRequest struct {
		Payload   interface{} `json:"payload"`
		Metadata  interface{} `json:"metadata"`
		Directive string      `json:"directive" validate:"required"`
	}

	request := cloudConnectorSendMessageRequest{
		Directive: directive,
		Payload:   payload,
		Metadata:  metadata,
	}

	requestJson, _ := json.Marshal(request)

	u := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/message", this.Url, this.ClientID)

	req, err := http.NewRequest("POST", u, bytes.NewBuffer(requestJson))
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")
	req.Header.Add("x-rh-insights-request-id", requestID)
	req.Header.Add("x-rh-cloud-connector-org-id", string(this.OrgID))
	req.Header.Add("x-rh-cloud-connector-client-id", "cloud-connector-composite")
	req.Header.Add("x-rh-cloud-connector-psk", "imaPSK")

	logger.Debug("About to call backend cloud-connector")
	r, err := client.Do(req)
	logger.Debug("Returned from call to backend cloud-connector")
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Call to backend cloud-connector failed")
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != 201 {
		logger.Debugf("Call to child cloud-connector returned http status code %d", r.StatusCode)
		return nil, fmt.Errorf("Unable to find connection")
	}

	type cloudConnectorSendMessageResponse struct {
		MessageID string `json:"id"`
	}

	var resp cloudConnectorSendMessageResponse
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Unable to parse Cloud-Connector response")
		return nil, err
	}

	fmt.Println("\n\nresp: ", resp)

	messageID, err := uuid.Parse(resp.MessageID)
	if err != nil {
		return nil, fmt.Errorf("FIXME: Wrap it!!  Unable to send")
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
