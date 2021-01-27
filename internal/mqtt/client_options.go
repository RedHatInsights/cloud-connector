package mqtt

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	MQTT "github.com/eclipse/paho.mqtt.golang"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"

	"github.com/sirupsen/logrus"
)

type MqttClientOptionsFunc func(*MQTT.ClientOptions) error

func WithJwtAsHttpHeader(tokenGenerator jwt_utils.JwtGenerator) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		headers := http.Header{}
		jwtToken, err := tokenGenerator(context.Background())

		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to retrieve the JWT Token for the MQTT broker connection")
			return err
		}

		headers.Add("X-Akamai-DCP-Token", jwtToken)
		fmt.Println("SETTING THE JWT HTTP HEADER")
		opts.SetHTTPHeaders(headers)

		return nil
	}
}

func WithTlsConfig(tlsConfig *tls.Config) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		fmt.Println("SETTING THE TLS CONFIG")
		opts.SetTLSConfig(tlsConfig)
		return nil
	}
}

func NewBrokerOptions(brokerUrl string, opts ...MqttClientOptionsFunc) (*MQTT.ClientOptions, error) {
	connOpts := MQTT.NewClientOptions()

	connOpts.AddBroker(brokerUrl)

	for _, opt := range opts {
		err := opt(connOpts)
		if err != nil {
			return nil, err
		}
	}

	return connOpts, nil
}
