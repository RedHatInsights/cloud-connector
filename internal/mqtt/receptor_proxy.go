package mqtt

import (
	"context"
	"errors"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	errUnableToSendMessage = errors.New("unable to send message")
)

type ReceptorMQTTProxy struct {
	Config       *config.Config
	AccountID    domain.AccountID
	ClientID     domain.ClientID
	Client       MQTT.Client
	TopicBuilder *TopicBuilder
	Dispatchers  domain.Dispatchers
}

func (rhp *ReceptorMQTTProxy) SendMessage(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID, directive string, metadata interface{}, payload interface{}) (*uuid.UUID, error) {

	messageID, message, err := buildDataMessage(directive, metadata, payload)

	logger := logger.Log.WithFields(logrus.Fields{"message_id": messageID, "account": rhp.AccountID, "client_id": rhp.ClientID})

	go func() {
		var sleepTime time.Duration = rhp.Config.SleepTimeHack
		logger.Debugf("Sleeping for %s seconds before sending data message to connected client\n", sleepTime)
		time.Sleep(sleepTime)
		logger.Debug("Sending data message to connected client")

		topic := rhp.TopicBuilder.BuildOutgoingDataTopic(rhp.ClientID)

		err = sendMessage(rhp.Client, logger, rhp.ClientID, messageID, topic, rhp.Config.MqttDataPublishQoS, message)
	}()

	return messageID, err
}

func (rhp *ReceptorMQTTProxy) Ping(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID) error {

	commandMessageContent := CommandMessageContent{Command: "ping"}

	logger := logger.Log.WithFields(logrus.Fields{"account": rhp.AccountID})

	topic := rhp.TopicBuilder.BuildOutgoingControlTopic(rhp.ClientID)

	qos := rhp.Config.MqttControlPublishQoS

	_, err := sendControlMessage(rhp.Client, logger, topic, qos, rhp.ClientID, "command", &commandMessageContent)

            // FIXME:  This will bring down the service!  This was added to work around an
            // issue we are seeing with the production mqtt broker.  We are running into an issue in prod where
            // cloud-connector cannot send or receive messages.  On the sending side, we are getting an
            // timeout error.  BUT...things never recover.  So fall over and allow openshift to restart
            // the service.  This Fatal call needs to be removed after the mqtt broker starts behaving better.
            go func() {
                logger.Warn("cloud-connector is about to fall over...FIXME later!!")
                time.Sleep(1 * time.Second)  // Give us some time send the log message...to give the humans a clue to figure out what happened here...
	            logger.Fatal("ran into an mqtt error...going down")
            }()


	return err
}

func (rhp *ReceptorMQTTProxy) Reconnect(ctx context.Context, accountNumber domain.AccountID, recipient domain.ClientID, delay int) error {

	logger := logger.Log.WithFields(logrus.Fields{"account": rhp.AccountID})

	err := sendReconnectMessageToClient(rhp.Client, logger, rhp.TopicBuilder, rhp.Config.MqttControlPublishQoS, rhp.ClientID, delay)

	return err
}

func (rhp *ReceptorMQTTProxy) GetDispatchers(ctx context.Context) (domain.Dispatchers, error) {
	return rhp.Dispatchers, nil
}

func (rhp *ReceptorMQTTProxy) Close(ctx context.Context) error {

	commandMessageContent := CommandMessageContent{Command: "disconnect"}

	logger := logger.Log.WithFields(logrus.Fields{"account": rhp.AccountID})

	topic := rhp.TopicBuilder.BuildOutgoingControlTopic(rhp.ClientID)

	qos := rhp.Config.MqttControlPublishQoS

	_, err := sendControlMessage(rhp.Client, logger, topic, qos, rhp.ClientID, "command", &commandMessageContent)

	return err
}
