package mqtt

import (
	"context"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

type ConnectorClientMQTTProxyFactory struct {
	mqttClient   MQTT.Client
	topicBuilder *TopicBuilder
	config       *config.Config
}

func NewConnectorClientMQTTProxyFactory(cfg *config.Config, mqttClient MQTT.Client, topicBuilder *TopicBuilder) (controller.ConnectorClientProxyFactory, error) {
	proxyFactory := ConnectorClientMQTTProxyFactory{mqttClient: mqttClient, topicBuilder: topicBuilder, config: cfg}
	return &proxyFactory, nil
}

func (ccpf *ConnectorClientMQTTProxyFactory) CreateProxy(ctx context.Context, orgID domain.OrgID, account domain.AccountID, client_id domain.ClientID, canonicalFacts domain.CanonicalFacts, dispatchers domain.Dispatchers, tags domain.Tags) (controller.ConnectorClient, error) {

	logger := logger.Log.WithFields(logrus.Fields{"org_id": orgID, "account": account, "client_id": client_id})

	proxy := ConnectorClientMQTTProxy{
		Logger:         logger,
		Config:         ccpf.config,
		OrgID:          orgID,
		AccountID:      account,
		ClientID:       client_id,
		Client:         ccpf.mqttClient,
		TopicBuilder:   ccpf.topicBuilder,
		CanonicalFacts: canonicalFacts,
		Dispatchers:    dispatchers,
		Tags:           tags,
	}

	return &proxy, nil
}
