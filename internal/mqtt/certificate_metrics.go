package mqtt

import (
	"sync/atomic"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/tls_utils"

	"github.com/sirupsen/logrus"
)

// mqttConsumerCertFile holds the MQTT consumer certificate path used to compute
// cloud_connector_certificate_expiry_days on each Prometheus scrape.
var mqttConsumerCertFile atomic.Value // string

// RecordMqttConsumerCertificateExpiry validates the MQTT broker client
// certificate and registers its path for the expiry gauge. The gauge value is
// recomputed on scrape from the live certificate NotAfter, so it does not go
// stale on long-running pods. Failures are logged and do not abort startup;
// TLS loading remains the source of truth for whether the cert is usable.
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

	mqttConsumerCertFile.Store(certFilePath)

	logger.Log.WithFields(logrus.Fields{
		"cert_file":      certFilePath,
		"days_remaining": days,
	}).Info("Recording MQTT consumer certificate expiry metric")
}

func mqttConsumerCertificateExpiryDays() float64 {
	path, ok := mqttConsumerCertFile.Load().(string)
	if !ok || path == "" {
		return 0
	}

	days, err := tls_utils.CertificateExpiryDays(path)
	if err != nil {
		return 0
	}

	return days
}
