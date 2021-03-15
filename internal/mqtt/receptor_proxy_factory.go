package mqtt

import (
	"context"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type ReceptorMQTTProxyFactory struct {
	mqttClient   MQTT.Client
	topicBuilder *TopicBuilder
}

func NewReceptorMQTTProxyFactory(cfg *config.Config, mqttClient MQTT.Client, topicBuilder *TopicBuilder) (controller.ReceptorProxyFactory, error) {
	proxyFactory := ReceptorMQTTProxyFactory{mqttClient: mqttClient, topicBuilder: topicBuilder}
	return &proxyFactory, nil
}

func (rhp *ReceptorMQTTProxyFactory) CreateProxy(ctx context.Context, account domain.AccountID, client_id domain.ClientID) (controller.Receptor, error) {

	proxy := ReceptorMQTTProxy{
		AccountID:    account,
		ClientID:     client_id,
		Client:       rhp.mqttClient,
		TopicBuilder: rhp.topicBuilder,
	}

	return &proxy, nil
}
