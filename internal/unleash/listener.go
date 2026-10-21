package unleash

import (
	unleash "github.com/Unleash/unleash-go-sdk/v6"
	"github.com/sirupsen/logrus"
)

// Listener implements the Unleash event listener interface for logging
type Listener struct {
	log *logrus.Entry
}

// NewListener creates a new Unleash event listener
func NewListener(log *logrus.Entry) *Listener {
	return &Listener{log: log}
}

// OnError is called when an error occurs
func (l *Listener) OnError(err error) {
	l.log.WithError(err).Error("Unleash error")
}

// OnWarning is called when a warning occurs
func (l *Listener) OnWarning(warning error) {
	l.log.WithError(warning).Warn("Unleash warning")
}

// OnReady is called when the SDK is ready
func (l *Listener) OnReady() {
	l.log.Debug("Unleash SDK ready")
}

// OnCount is called when a feature is toggled
func (l *Listener) OnCount(name string, enabled bool) {
	l.log.WithFields(logrus.Fields{
		"feature": name,
		"enabled": enabled,
	}).Trace("Unleash feature toggled")
}

// OnSent is called when metrics are sent
func (l *Listener) OnSent(payload unleash.MetricsData) {
	l.log.Trace("Unleash metrics sent")
}

// OnRegistered is called when the client is registered
func (l *Listener) OnRegistered(payload unleash.ClientData) {
	l.log.WithFields(logrus.Fields{
		"app_name":    payload.AppName,
		"instance_id": payload.InstanceID,
	}).Debug("Unleash client registered")
}
