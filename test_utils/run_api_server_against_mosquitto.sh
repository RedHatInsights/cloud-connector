#CLOUD_CONNECTOR_TENANT_TRANSLATOR_MOCK_MAPPING="{\"10001\": \"010101\", \"10002\": \"010102\", \"10000\": \"000000\", \"0002\": \"111000\", \"0001\": \"\"}" \

CLOUD_CONNECTOR_API_SERVER_CONNECTION_LOOKUP_IMPL="relaxed" \
CLOUD_CONNECTOR_TENANT_TRANSLATOR_IMPL="mock" \
CLOUD_CONNECTOR_SERVICE_TO_SERVICE_CREDENTIALS='{"fred": "fredskey"}' \
CLOUD_CONNECTOR_MQTT_CLIENT_ID="api-service" \
CLOUD_CONNECTOR_MQTT_BROKER_TLS_SKIP_VERIFY=true \
./cloud-connector api_server -l localhost:8081
