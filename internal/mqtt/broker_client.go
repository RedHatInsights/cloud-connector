package mqtt

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	Topic      string
	EntryPoint MQTT.MessageHandler
	Qos        byte
}

// dnsLookupTimeout caps the time spent resolving broker hostname and reverse DNS.
// Two seconds is sufficient for a healthy resolver while keeping reconnect latency low.
// Adjust if DNS infrastructure changes.
const dnsLookupTimeout = 2 * time.Second

// logBrokerNode resolves the hostname in brokerUrl to an IP and reverse-DNS
// hostname, then emits a structured log entry. This makes the actual physical
// broker node visible in Kibana after each connect or reconnect, which is
// otherwise obscured by the load-balanced VIP address.
func logBrokerNode(brokerUrl string) {
	fields := logrus.Fields{"broker_url": brokerUrl}
	// The anonymous function wrapper ensures WithFields is called at function exit,
	// not at defer declaration time. This allows the lookups below to populate
	// fields before the log entry is emitted.
	defer func() {
		logger.Log.WithFields(fields).Info("Connected to MQTT broker")
	}()

	u, err := url.Parse(brokerUrl)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err, "broker_url": brokerUrl}).Warn("Failed to parse broker URL for node resolution")
		return
	}

	fields["broker_hostname"] = u.Hostname()

	// PreferGo forces the pure Go DNS resolver, which honors context cancellation.
	// The cgo-based resolver ignores context deadlines and can hang indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), dnsLookupTimeout)
	defer cancel()

	resolver := net.Resolver{PreferGo: true}

	ips, err := resolver.LookupHost(ctx, u.Hostname())
	if err != nil || len(ips) == 0 {
		logger.Log.WithFields(logrus.Fields{"error": err, "broker_url": brokerUrl}).Warn("Failed to resolve broker hostname")
		return
	}

	fields["broker_resolved_ip"] = ips[0]

	hostnames, err := resolver.LookupAddr(ctx, ips[0])
	if err != nil || len(hostnames) == 0 {
		logger.Log.WithFields(logrus.Fields{"error": err, "broker_url": brokerUrl}).Warn("Failed to resolve broker node address")
		return
	}
	fields["broker_node"] = strings.TrimSuffix(hostnames[0], ".")
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
