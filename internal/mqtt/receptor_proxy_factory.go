package mqtt

import (
	"context"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/controller"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type ReceptorMQTTProxyFactory struct {
	mqttClient MQTT.Client
}

func NewReceptorMQTTProxyFactory(cfg *config.Config, mqttClient MQTT.Client) (controller.ReceptorProxyFactory, error) {
	proxyFactory := ReceptorMQTTProxyFactory{mqttClient: mqttClient}
	return &proxyFactory, nil
}

func (rhp *ReceptorMQTTProxyFactory) CreateProxy(ctx context.Context, account domain.AccountID, client_id domain.ClientID) (controller.Receptor, error) {

	proxy := ReceptorMQTTProxy{
		AccountID: account,
		ClientID:  client_id,
		Client:    rhp.mqttClient,
	}

	return &proxy, nil
}
