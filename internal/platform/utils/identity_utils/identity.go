package identity_utils

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

func AuthenticatedWithCertificate(identity domain.Identity) (bool, error) {

	identityMap, err := convertIdentityToMap(identity)
	if err != nil {
		return false, err
	}

	return identityMap["auth_type"] == "cert-auth", nil
}

func convertIdentityToMap(identity domain.Identity) (map[string]interface{}, error) {
	idRaw, err := base64.StdEncoding.DecodeString(string(identity))
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to decode identity string")
		return nil, err
	}

	var jsonDataMap map[string]interface{}
	err = json.Unmarshal(idRaw, &jsonDataMap)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Unable to parse identity string")
		return nil, err
	}

	if data, ok := jsonDataMap["identity"].(map[string]interface{}); ok {
		return data, nil
	} else {
		return nil, errors.New("Unable to parse identity string")
	}
}
