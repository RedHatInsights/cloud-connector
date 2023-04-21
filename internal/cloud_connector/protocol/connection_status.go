package protocol

import ()

func GetClientNameFromConnectionStatusContent(payload map[string]interface{}) (string, bool) {
	return getStringFromContentPayload("client_name", payload)
}

func GetClientVersionFromConnectionStatusContent(payload map[string]interface{}) (string, bool) {
	return getStringFromContentPayload("client_version", payload)
}

func getStringFromContentPayload(fieldName string, payload map[string]interface{}) (string, bool) {
	data, found := payload[fieldName]
	if found {
		return data.(string), found
	}

	return "", found
}
