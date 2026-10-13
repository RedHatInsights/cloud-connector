package mqtt

import (
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	"github.com/sirupsen/logrus"
)

// RecordMqttConsumerCertificateExpiry reads the MQTT broker client certificate
// and updates the cloud_connector_certificate_expiry_days gauge. Failures are
// logged and do not abort startup; TLS loading remains the source of truth for
// whether the cert is usable.
func RecordMqttConsumerCertificateExpiry(certFilePath string) {
	if certFilePath == "" {
		logger.Log.Error("MQTT consumer certificate path is empty; expiry metric not recorded")
		return
	}

	days, err := tls_utils.CertificateExpiryDays(certFilePath)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err, "cert_file": certFilePath}).Error(
			"Unable to record MQTT consumer certificate expiry metric")
		return
	}

	metrics.certificateExpiryDays.Set(days)

	logger.Log.WithFields(logrus.Fields{
		"cert_file":      certFilePath,
		"days_remaining": days,
	}).Info("Recorded MQTT consumer certificate expiry metric")
}
