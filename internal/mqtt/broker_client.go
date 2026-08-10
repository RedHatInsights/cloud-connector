package mqtt

import (
	"net"
	"net/url"
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	Topic      string
	EntryPoint MQTT.MessageHandler
	Qos        byte
}

// logBrokerNode resolves the hostname in brokerUrl to an IP and reverse-DNS
// hostname, then emits a structured log entry. This makes the actual physical
// broker node visible in Kibana after each connect or reconnect, which is
// otherwise obscured by the load-balanced VIP address.
func logBrokerNode(brokerUrl string) {
	fields := logrus.Fields{"broker_url": brokerUrl}
	defer logger.Log.WithFields(fields).Info("Connected to MQTT broker")

	u, err := url.Parse(brokerUrl)
	if err != nil {
		logger.Log.WithFields(fields).Warn("Failed to parse broker URL for node resolution")
		return
	}

	ips, err := net.LookupHost(u.Hostname())
	if err != nil || len(ips) == 0 {
		logger.Log.WithFields(logrus.Fields{"error": err, "broker_url": brokerUrl}).Warn("Failed to resolve broker hostname")
		return
	}

	fields["broker_resolved_ip"] = ips[0]

	if hostnames, err := net.LookupAddr(ips[0]); err == nil && len(hostnames) > 0 {
		fields["broker_node"] = strings.TrimSuffix(hostnames[0], ".")
	}
}

func CreateBrokerConnection(brokerUrl string, brokerConfigFuncs ...MqttClientOptionsFunc) (MQTT.Client, error) {

	connOpts, err := NewBrokerOptions(brokerUrl, brokerConfigFuncs...)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to build MQTT ClientOptions")
		return nil, err
	}

	mqttClient := MQTT.NewClient(connOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		logger.Log.WithFields(logrus.Fields{"error": token.Error()}).Error("Unable to connect to MQTT broker")
		return nil, token.Error()
	}

	return mqttClient, nil
}
