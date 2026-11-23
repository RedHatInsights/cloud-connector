package mqtt

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/domain"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

type publishBody struct {
	Directive string `json:"directive"`
}

// TestSendMessage covers the three ways a Paho publish token can resolve.
//
// sendMessage treats WaitTimeout and Error as separate results. Paho leaves
// Error unset when WaitTimeout returns false, because the caller may wait
// again. The old check, WaitTimeout(timeout) && token.Error() != nil, skips
// that token and reports success. The timeout case is that state: the wait
// does not complete, and the token error is nil.
//
// The broker-error and success cases are the other two outcomes of the same
// call. A change that reports every publish as a timeout, or every publish
// as a failure, fails one of them. The publish counters are process-global,
// so each case asserts a delta from the value before the call.
// recordingClient and scriptedToken stand in for the Paho client and token.
func TestSendMessage(t *testing.T) {
	const (
		topic          = "redhat/insights/client-1/control/in"
		qos            = byte(1)
		publishTimeout = 250 * time.Millisecond
	)

	clientID := domain.ClientID("client-1")
	messageID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	message := publishBody{Directive: "reconnect"}
	wantPayload := []byte("{\"directive\":\"reconnect\"}\n")
	brokerErr := errors.New("broker rejected publish")

	testCases := []struct {
		name         string
		completed    bool
		tokenErr     error
		wantErrText  string
		wantErr      error
		failureDelta float64
		successDelta float64
		wantErrorLog string
	}{
		{
			name:         "timeout with no token error is not success",
			completed:    false,
			wantErrText:  "mqtt publish timed out after 250ms",
			failureDelta: 1,
			wantErrorLog: "Message publish timed out",
		},
		{
			name:         "completed publish returns the broker error",
			completed:    true,
			tokenErr:     brokerErr,
			wantErr:      brokerErr,
			failureDelta: 1,
			wantErrorLog: "Error sending a message to MQTT broker",
		},
		{
			name:         "completed publish with no error succeeds",
			completed:    true,
			successDelta: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := &scriptedToken{completed: tc.completed, err: tc.tokenErr}
			client := &recordingClient{token: token}

			log := logrus.New()
			log.SetOutput(io.Discard)
			log.SetLevel(logrus.InfoLevel)
			hook := test.NewLocal(log)

			failureBefore := testutil.ToFloat64(metrics.messagePublishedFailureCounter)
			successBefore := testutil.ToFloat64(metrics.messagePublishedSuccessCounter)

			err := sendMessage(client, log.WithField("test", tc.name), clientID, &messageID, topic, qos, publishTimeout, message)

			switch {
			case tc.wantErrText != "":
				if err == nil || err.Error() != tc.wantErrText {
					t.Fatalf("error %v, want %q", err, tc.wantErrText)
				}
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error %v, want %v", err, tc.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
			}

			if got := testutil.ToFloat64(metrics.messagePublishedFailureCounter) - failureBefore; got != tc.failureDelta {
				t.Fatalf("failure counter delta %v, want %v", got, tc.failureDelta)
			}
			if got := testutil.ToFloat64(metrics.messagePublishedSuccessCounter) - successBefore; got != tc.successDelta {
				t.Fatalf("success counter delta %v, want %v", got, tc.successDelta)
			}

			if !client.published {
				t.Fatal("Publish was not called")
			}
			if client.topic != topic || client.qos != qos || client.retained {
				t.Fatalf("Publish(%q, qos=%d, retained=%v), want (%q, qos=%d, retained=false)", client.topic, client.qos, client.retained, topic, qos)
			}
			if !bytes.Equal(client.payload, wantPayload) {
				t.Fatalf("payload %q, want %q", client.payload, wantPayload)
			}
			if !token.waitCalled {
				t.Fatal("WaitTimeout was not called")
			}
			if token.waited != publishTimeout {
				t.Fatalf("WaitTimeout(%s), want %s", token.waited, publishTimeout)
			}

			assertErrorLog(t, hook, tc.wantErrorLog)
		})
	}
}

func assertErrorLog(t *testing.T, hook *test.Hook, want string) {
	t.Helper()

	var errorLogs []string
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.ErrorLevel {
			errorLogs = append(errorLogs, entry.Message)
		}
	}

	if want == "" {
		if len(errorLogs) != 0 {
			t.Fatalf("unexpected error logs %q", errorLogs)
		}
		return
	}

	if len(errorLogs) != 1 || errorLogs[0] != want {
		t.Fatalf("error logs %q, want [%q]", errorLogs, want)
	}
}

// scriptedToken is the publish acknowledgement sendMessage waits on.
// completed false and err nil is the timeout state paho documents:
// WaitTimeout returns false and Error stays unset.
type scriptedToken struct {
	completed  bool
	err        error
	waited     time.Duration
	waitCalled bool
}

func (t *scriptedToken) Wait() bool { return t.completed }

func (t *scriptedToken) WaitTimeout(d time.Duration) bool {
	t.waitCalled = true
	t.waited = d
	return t.completed
}

func (t *scriptedToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	if t.completed {
		close(ch)
	}
	return ch
}

func (t *scriptedToken) Error() error { return t.err }

type recordingClient struct {
	MQTT.Client
	token     MQTT.Token
	topic     string
	qos       byte
	retained  bool
	payload   []byte
	published bool
}

func (c *recordingClient) Publish(topic string, qos byte, retained bool, payload interface{}) MQTT.Token {
	body, ok := payload.([]byte)
	if !ok {
		panic("Publish payload is not []byte")
	}
	c.topic = topic
	c.qos = qos
	c.retained = retained
	c.payload = append([]byte(nil), body...)
	c.published = true
	return c.token
}

var (
	_ MQTT.Client = (*recordingClient)(nil)
	_ MQTT.Token  = (*scriptedToken)(nil)
)
