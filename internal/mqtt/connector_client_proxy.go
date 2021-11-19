package mqtt

import (
	"context"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/cloud_connector/protocol"
	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	errUnableToSendMessage = errors.New("unable to send message")
)

type ConnectorClientMQTTProxy struct {
	Logger       *logrus.Entry
	Config       *config.Config
	AccountID    domain.AccountID
	ClientID     domain.ClientID
	Client       MQTT.Client
	TopicBuilder *TopicBuilder
	Dispatchers  domain.Dispatchers
}

func (cc *ConnectorClientMQTTProxy) SendMessage(ctx context.Context, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	metrics.sentMessageDirectiveCounter.With(prometheus.Labels{"directive": directive}).Inc()

	messageID, message, err := protocol.BuildDataMessage(directive, metadata, payload)

	logger := cc.Logger.WithFields(logrus.Fields{"message_id": messageID})

	logger.Debug("Sending data message to connected client")

	topic := cc.TopicBuilder.BuildOutgoingDataTopic(cc.ClientID)

	err = sendMessage(cc.Client, logger, cc.ClientID, messageID, topic, cc.Config.MqttDataPublishQoS, message)

	return messageID, err
}

func (cc *ConnectorClientMQTTProxy) Ping(ctx context.Context) error {

	commandMessageContent := protocol.CommandMessageContent{Command: "ping"}

	topic := cc.TopicBuilder.BuildOutgoingControlTopic(cc.ClientID)

	qos := cc.Config.MqttControlPublishQoS

	_, err := sendControlMessage(cc.Client, cc.Logger, topic, qos, cc.ClientID, "command", &commandMessageContent)

	return err
}

func (cc *ConnectorClientMQTTProxy) Reconnect(ctx context.Context, message string, delay int) error {

	err := SendReconnectMessageToClient(cc.Client, cc.Logger, cc.TopicBuilder, cc.Config.MqttControlPublishQoS, cc.ClientID, delay)

	return err
}

func (cc *ConnectorClientMQTTProxy) GetDispatchers(ctx context.Context) (domain.Dispatchers, error) {
	return cc.Dispatchers, nil
}

func (cc *ConnectorClientMQTTProxy) Disconnect(ctx context.Context, message string) error {

	commandMessageContent := protocol.CommandMessageContent{Command: "disconnect"}

	topic := cc.TopicBuilder.BuildOutgoingControlTopic(cc.ClientID)

	qos := cc.Config.MqttControlPublishQoS

	_, err := sendControlMessage(cc.Client, cc.Logger, topic, qos, cc.ClientID, "command", &commandMessageContent)

	return err
}
