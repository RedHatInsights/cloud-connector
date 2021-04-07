package mqtt

import (
	"context"
	"crypto/tls"
	"net/http"

	MQTT "github.com/eclipse/paho.mqtt.golang"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/jwt_utils"

	"github.com/sirupsen/logrus"
)

const akamaiTokenHeader = "X-Akamai-DCP-Token"

type MqttClientOptionsFunc func(*MQTT.ClientOptions) error

func WithJwtAsHttpHeader(tokenGenerator jwt_utils.JwtGenerator) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		headers := http.Header{}
		jwtToken, err := tokenGenerator(context.Background())

		if err != nil {
			logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to retrieve the JWT Token for the MQTT broker connection")
			return err
		}

		headers.Add(akamaiTokenHeader, jwtToken)

		logger.Log.Trace("Setting the MQTT JWT HTTP header")

		opts.SetHTTPHeaders(headers)

		return nil
	}
}

func WithJwtReconnectingHandler(tokenGenerator jwt_utils.JwtGenerator) MqttClientOptionsFunc {
	//'Reconnecting' handler is called prior to a reconnection attempt , before 'Reconnect' handlers
	return func(opts *MQTT.ClientOptions) error {
		tokenRefresher := func(c MQTT.Client, opts *MQTT.ClientOptions) {
			logger.Log.Info("Attempting JWT token refresh")
			jwtToken, err := tokenGenerator(context.Background())
			if err != nil {
				logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to refresh the JWT Token for the MQTT broker connection")
			} else {
				opts.HTTPHeaders.Set(akamaiTokenHeader, jwtToken)
			}
		}

		logger.Log.Trace("Setting MQTT JWT reconnecting handler")

		opts.SetReconnectingHandler(tokenRefresher)

		return nil
	}
}

func WithTlsConfig(tlsConfig *tls.Config) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		logger.Log.Trace("Setting the TLS config")
		opts.SetTLSConfig(tlsConfig)
		return nil
	}
}

func WithClientID(clientID string) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		logger.Log.Trace("Setting the MQTT client-id: ", clientID)
		opts.SetClientID(clientID)
		return nil
	}
}

func WithCleanSession(cleanSession bool) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		logger.Log.Tracef("Setting the clean session: %v\n", cleanSession)
		opts.SetCleanSession(cleanSession)
		return nil
	}
}

func WithResumeSubs(resumeSubs bool) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		logger.Log.Tracef("Setting the resume subs: %v\n", resumeSubs)
		opts.SetResumeSubs(resumeSubs)
		return nil
	}
}

func WithDefaultPublishHandler(msgHdlr MQTT.MessageHandler) MqttClientOptionsFunc {
	return func(opts *MQTT.ClientOptions) error {
		logger.Log.Trace("Setting the default publish handler")
		opts.SetDefaultPublishHandler(msgHdlr)
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
