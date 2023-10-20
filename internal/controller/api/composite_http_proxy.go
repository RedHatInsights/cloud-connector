package api

import (
	"context"
	"errors"
    "fmt"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	"github.com/google/uuid"
	//"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	errUnableToSendMessage = errors.New("unable to send message")
)

type ConnectorClientHTTPProxy struct {
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

//	logger := this.Logger.WithFields(logrus.Fields{"message_id": messageID})

// FIXME: get the request id from the context

	this.Logger.Debug("Sending data message to connected client")

//	err = sendMessage(cc.Client, logger, cc.ClientID, messageID, topic, cc.Config.MqttDataPublishQoS, cc.Config.MqttPublishTimeout, message)

	return nil, fmt.Errorf("Not implemented!!")
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
