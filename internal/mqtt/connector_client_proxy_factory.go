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

func (ccpf *ConnectorClientMQTTProxyFactory) CreateProxy(ctx context.Context, clientState domain.ConnectorClientState) (controller.ConnectorClient, error) {

	logger := logger.Log.WithFields(logrus.Fields{"org_id": clientState.OrgID, "account": clientState.Account, "client_id": clientState.ClientID})

	proxy := ConnectorClientMQTTProxy{
		Logger:         logger,
		Config:         ccpf.config,
		OrgID:          clientState.OrgID,
		AccountID:      clientState.Account,
		ClientID:       clientState.ClientID,
		Client:         ccpf.mqttClient,
		TopicBuilder:   ccpf.topicBuilder,
		CanonicalFacts: clientState.CanonicalFacts,
		Dispatchers:    clientState.Dispatchers,
		Tags:           clientState.Tags,
	}

	return &proxy, nil
}
