package mqtt

import (
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

const mqttConsumerCertificateLabel = "mqtt_consumer"

// RecordMqttConsumerCertificateExpiry reads the MQTT broker client certificate
// and updates the cloud_connector_certificate_expiry_days gauge. Failures are
// logged and do not abort startup; TLS loading remains the source of truth for
// whether the cert is usable.
func RecordMqttConsumerCertificateExpiry(certFilePath string) {
	if certFilePath == "" {
		return
	}

	days, err := tls_utils.CertificateExpiryDays(certFilePath)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err, "cert_file": certFilePath}).Error(
			"Unable to record MQTT consumer certificate expiry metric")
		return
	}

	metrics.certificateExpiryDays.With(prometheus.Labels{
		"certificate_label": mqttConsumerCertificateLabel,
	}).Set(days)

	logger.Log.WithFields(logrus.Fields{
		"cert_file":         certFilePath,
		"certificate_label": mqttConsumerCertificateLabel,
		"days_remaining":    days,
	}).Info("Recorded MQTT consumer certificate expiry metric")
}
