package main

import (
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/mqtt"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

func init() {
	logger.InitLogger()
}

// fakeToken is a minimal MQTT.Token used to simulate either a subscribe ack
// that arrives in time, or one that never arrives within the configured
// timeout (as happens when a broker is busy replaying a resumed session's
// backlog before acking a new subscription).
type fakeToken struct {
	arrivesInTime bool
	err           error
	done          chan struct{}
}

func newFakeToken(arrivesInTime bool, err error) *fakeToken {
	done := make(chan struct{})
	if arrivesInTime {
		close(done)
	}
	return &fakeToken{arrivesInTime: arrivesInTime, err: err, done: done}
}

func (t *fakeToken) Wait() bool                       { return t.arrivesInTime }
func (t *fakeToken) WaitTimeout(_ time.Duration) bool { return t.arrivesInTime }
func (t *fakeToken) Done() <-chan struct{}            { return t.done }
func (t *fakeToken) Error() error                     { return t.err }

// fakeClient is a partial MQTT.Client mock. It embeds the (nil) interface so
// it satisfies MQTT.Client without implementing every method - only
// Subscribe and Disconnect are exercised by subscribeOnMqttConnectHandler.
type fakeClient struct {
	MQTT.Client

	subscribeToken  MQTT.Token
	subscribeCalls  []string
	disconnectCalls int
}

func (c *fakeClient) Subscribe(topic string, _ byte, _ MQTT.MessageHandler) MQTT.Token {
	c.subscribeCalls = append(c.subscribeCalls, topic)
	return c.subscribeToken
}

func (c *fakeClient) Disconnect(_ uint) {
	c.disconnectCalls++
}

func TestSubscribeOnMqttConnectHandler_TimeoutDisconnectsInsteadOfCrashing(t *testing.T) {
	subscribers := []mqtt.Subscriber{
		{Topic: "redhat/insights/+/control/out", Qos: 1},
		{Topic: "redhat/insights/+/data/out", Qos: 1},
	}

	client := &fakeClient{
		// Simulate a SUBACK that never arrives within mqttSubscribeTimeout.
		subscribeToken: newFakeToken(false, nil),
	}

	handler := subscribeOnMqttConnectHandler(subscribers)

	// If this regresses to calling logger.Log.Fatalf, the test binary itself
	// would os.Exit here instead of returning - a crash is impossible to miss.
	handler(client)

	if client.disconnectCalls != 1 {
		t.Errorf("expected Disconnect to be called exactly once on subscribe timeout, got %d", client.disconnectCalls)
	}

	if len(client.subscribeCalls) != 1 {
		t.Errorf("expected only the first subscriber to be attempted before bailing out on timeout, got %d calls: %v",
			len(client.subscribeCalls), client.subscribeCalls)
	}
}

func TestSubscribeOnMqttConnectHandler_SuccessSubscribesAllTopics(t *testing.T) {
	subscribers := []mqtt.Subscriber{
		{Topic: "redhat/insights/+/control/out", Qos: 1},
		{Topic: "redhat/insights/+/data/out", Qos: 1},
	}

	client := &fakeClient{
		// Simulate a SUBACK that arrives in time with no error.
		subscribeToken: newFakeToken(true, nil),
	}

	handler := subscribeOnMqttConnectHandler(subscribers)
	handler(client)

	if client.disconnectCalls != 0 {
		t.Errorf("expected Disconnect not to be called on successful subscribe, got %d", client.disconnectCalls)
	}

	if len(client.subscribeCalls) != len(subscribers) {
		t.Errorf("expected all subscribers to be attempted, got %d calls: %v",
			len(client.subscribeCalls), client.subscribeCalls)
	}
}
