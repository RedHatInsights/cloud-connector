package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/sirupsen/logrus"
)

var (
	// TLS failures
	ErrTLSHandshake = errors.New("mqtt tls handshake failed")

	// Network / transport failures
	ErrBrokerConnect     = errors.New("mqtt broker connection failed")
	ErrConnectionLost    = errors.New("mqtt connection lost")
	ErrConnectionRefused = errors.New("mqtt connection refused")
	ErrHostUnreachable   = errors.New("mqtt host unreachable")
	ErrTimeout           = errors.New("mqtt connection timeout")
	ErrEOF               = errors.New("mqtt connection closed unexpectedly")
	ErrBrokenPipe        = errors.New("mqtt broken pipe")

	// Protocol / CONNACK failures
	ErrProtocolVersion       = errors.New("mqtt unacceptable protocol version")
	ErrIdentifierRejected    = errors.New("mqtt client identifier rejected")
	ErrServerUnavailable     = errors.New("mqtt server unavailable")
	ErrBadUsernameOrPassword = errors.New("mqtt bad username or password")
	ErrNotAuthorized         = errors.New("mqtt not authorized")
)

const (
	// TLS
	CodeTLSHandshake = 495 // non-standard, indicates TLS handshake failure

	// Network / transport (mostly errno-derived)
	CodeConnectionLost    = 104 // ECONNRESET
	CodeConnectionRefused = 111 // ECONNREFUSED
	CodeHostUnreachable   = 113 // EHOSTUNREACH
	CodeBrokerConnect     = 520 // generic broker/connect failure
	CodeTimeout           = 408 // i/o timeout
	CodeEOF               = 499 // client closed / eof
	CodeBrokenPipe        = 532 // arbitrary for broken pipe

	// Protocol / CONNACK
	CodeProtocolVersion       = 0x01
	CodeIdentifierRejected    = 0x02
	CodeServerUnavailable     = 0x03
	CodeBadUsernameOrPassword = 0x04
	CodeNotAuthorized         = 0x05
)

const (
	CategoryTLS      = "tls"
	CategoryNetwork  = "network"
	CategoryProtocol = "protocol"
	CategoryRuntime  = "runtime"
	CategoryUnknown  = "unknown"
)

type ConnectError struct {
	Code     int
	Kind     error
	Cause    error
	Category string
}

func (e *ConnectError) Error() string {
	return fmt.Sprintf("%v: %v", e.Kind, e.Cause)
}

func (e *ConnectError) Unwrap() error {
	if e.Cause != nil {
		return e.Cause
	}
	return e.Kind
}

// Is lets errors.Is match on the sentinel Kind (e.g. ErrTLSHandshake)
func (e *ConnectError) Is(target error) bool {
	return target == e.Kind
}

func newConnectError(code int, category string, kind error, cause error) error {
	return &ConnectError{
		Code:     code,
		Kind:     kind,
		Cause:    cause,
		Category: category,
	}
}

type Subscriber struct {
	Topic      string
	EntryPoint MQTT.MessageHandler
	Qos        byte
}

func classifyConnectError(err error) error {
	var tlsHeaderErr *tls.RecordHeaderError
	var unknownAuthErr x509.UnknownAuthorityError
	var certInvalidErr *x509.CertificateInvalidError
	var hostErr x509.HostnameError
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return newConnectError(CodeTimeout, CategoryNetwork, ErrTimeout, err)
	}

	if errors.As(err, &tlsHeaderErr) || errors.As(err, &unknownAuthErr) || errors.As(err, &certInvalidErr) || errors.As(err, &hostErr) {
		return newConnectError(CodeTLSHandshake, CategoryTLS, ErrTLSHandshake, err)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		switch {
		case errors.Is(err, syscall.ECONNREFUSED):
			return newConnectError(CodeConnectionRefused, CategoryNetwork, ErrConnectionRefused, err)
		case errors.Is(err, syscall.ECONNRESET):
			return newConnectError(CodeConnectionLost, CategoryNetwork, ErrConnectionLost, err)
		case errors.Is(err, syscall.EHOSTUNREACH) || errors.Is(err, syscall.ENETUNREACH):
			return newConnectError(CodeHostUnreachable, CategoryNetwork, ErrHostUnreachable, err)
		}
		return newConnectError(CodeBrokerConnect, CategoryNetwork, ErrBrokerConnect, err)
	}

	if errors.Is(err, io.EOF) {
		return newConnectError(CodeEOF, CategoryNetwork, ErrEOF, err)
	}
	if errors.Is(err, syscall.EPIPE) {
		return newConnectError(CodeBrokenPipe, CategoryNetwork, ErrBrokenPipe, err)
	}

	// Catch textual hints when errors aren't typed
	lowerMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowerMsg, "connection lost"):
		return newConnectError(CodeConnectionLost, CategoryNetwork, ErrConnectionLost, err)
	case strings.Contains(lowerMsg, "connection refused"):
		return newConnectError(CodeConnectionRefused, CategoryNetwork, ErrConnectionRefused, err)
	case strings.Contains(lowerMsg, "host unreachable"):
		return newConnectError(CodeHostUnreachable, CategoryNetwork, ErrHostUnreachable, err)
	case strings.Contains(lowerMsg, "timeout"):
		return newConnectError(CodeTimeout, CategoryNetwork, ErrTimeout, err)
	case strings.Contains(lowerMsg, "eof"):
		return newConnectError(CodeEOF, CategoryNetwork, ErrEOF, err)
	case strings.Contains(lowerMsg, "broken pipe"):
		return newConnectError(CodeBrokenPipe, CategoryNetwork, ErrBrokenPipe, err)
	}

	return newConnectError(CodeBrokerConnect, CategoryUnknown, ErrBrokerConnect, err)
}

func classifyProtocolReturnCode(rc byte) error {
	switch rc {
	case 0x00:
		return nil
	case CodeProtocolVersion:
		return newConnectError(int(CodeProtocolVersion), CategoryProtocol, ErrProtocolVersion, fmt.Errorf("connack=%d", rc))
	case CodeIdentifierRejected:
		return newConnectError(int(CodeIdentifierRejected), CategoryProtocol, ErrIdentifierRejected, fmt.Errorf("connack=%d", rc))
	case CodeServerUnavailable:
		return newConnectError(int(CodeServerUnavailable), CategoryProtocol, ErrServerUnavailable, fmt.Errorf("connack=%d", rc))
	case CodeBadUsernameOrPassword:
		return newConnectError(int(CodeBadUsernameOrPassword), CategoryProtocol, ErrBadUsernameOrPassword, fmt.Errorf("connack=%d", rc))
	case CodeNotAuthorized:
		return newConnectError(int(CodeNotAuthorized), CategoryProtocol, ErrNotAuthorized, fmt.Errorf("connack=%d", rc))
	default:
		return newConnectError(int(rc), CategoryProtocol, ErrBrokerConnect, fmt.Errorf("connack=%d", rc))
	}
}

func classifyConnectionLostError(err error) error {
	if err == nil {
		return newConnectError(CodeBrokerConnect, CategoryRuntime, ErrConnectionLost, errors.New("connection lost"))
	}

	if ce, ok := err.(*ConnectError); ok {
		return ce
	}

	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return newConnectError(CodeTimeout, CategoryRuntime, ErrTimeout, err)
	}
	if errors.Is(err, io.EOF) {
		return newConnectError(CodeEOF, CategoryRuntime, ErrEOF, err)
	}
	if errors.Is(err, syscall.EPIPE) {
		return newConnectError(CodeBrokenPipe, CategoryRuntime, ErrBrokenPipe, err)
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return newConnectError(CodeConnectionLost, CategoryRuntime, ErrConnectionLost, err)
	}

	lowerMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lowerMsg, "timeout"):
		return newConnectError(CodeTimeout, CategoryRuntime, ErrTimeout, err)
	case strings.Contains(lowerMsg, "keepalive"):
		return newConnectError(CodeConnectionLost, CategoryRuntime, ErrConnectionLost, err)
	case strings.Contains(lowerMsg, "ping"):
		return newConnectError(CodeConnectionLost, CategoryRuntime, ErrConnectionLost, err)
	case strings.Contains(lowerMsg, "broken pipe"):
		return newConnectError(CodeBrokenPipe, CategoryRuntime, ErrBrokenPipe, err)
	case strings.Contains(lowerMsg, "connection lost"):
		return newConnectError(CodeConnectionLost, CategoryRuntime, ErrConnectionLost, err)
	}

	return newConnectError(CodeBrokerConnect, CategoryRuntime, ErrConnectionLost, err)
}

// ClassifyConnectionLostError exposes classification for external handlers (e.g. connection lost callbacks).
func ClassifyConnectionLostError(err error) *ConnectError {
	classified := classifyConnectionLostError(err)
	if ce, ok := classified.(*ConnectError); ok {
		return ce
	}
	return &ConnectError{
		Code:     CodeBrokerConnect,
		Kind:     ErrConnectionLost,
		Cause:    err,
		Category: CategoryRuntime,
	}
}

func CreateBrokerConnection(brokerUrl string, brokerConfigFuncs ...MqttClientOptionsFunc) (MQTT.Client, error) {

	connOpts, err := NewBrokerOptions(brokerUrl, brokerConfigFuncs...)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to build MQTT ClientOptions")
		return nil, err
	}

	mqttClient := MQTT.NewClient(connOpts)
	if token := mqttClient.Connect(); token.Wait() {
		if ct, ok := token.(*MQTT.ConnectToken); ok {
			rc := ct.ReturnCode()
			if protoErr := classifyProtocolReturnCode(rc); protoErr != nil {
				logger.Log.WithFields(logrus.Fields{"error": protoErr, "connack_code": rc}).Error("MQTT CONNACK not accepted")
				return nil, protoErr
			}
		}

		if token.Error() != nil {
			connectErr := classifyConnectError(token.Error())
			logger.Log.WithFields(logrus.Fields{"error": connectErr}).Error("Unable to connect to MQTT broker")
			return nil, connectErr
		}
	}

	logger.Log.Info("Connected to MQTT broker: ", brokerUrl)

	return mqttClient, nil
}
