package mqtt

import (
	"context"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	MQTT "github.com/eclipse/paho.mqtt.golang"
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

func (rhp *ConnectorClientMQTTProxyFactory) CreateProxy(ctx context.Context, account domain.AccountID, client_id domain.ClientID, dispatchers domain.Dispatchers) (controller.ConnectorClient, error) {

	proxy := ConnectorClientMQTTProxy{
		Config:       rhp.config,
		AccountID:    account,
		ClientID:     client_id,
		Client:       rhp.mqttClient,
		TopicBuilder: rhp.topicBuilder,
		Dispatchers:  dispatchers,
	}

	return &proxy, nil
}
