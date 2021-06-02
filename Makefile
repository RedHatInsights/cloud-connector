CONNECTOR_SERVICE_BINARY=cloud-connector
CONNECTED_CLIENT_BINARY=test_client
MIGRATE_DB_BINARY=migrate_db

DOCKER_COMPOSE_CFG=dev.yml

COVERAGE_OUTPUT=coverage.out
COVERAGE_HTML=coverage.html
LOCAL_BROKER="ssl://localhost:8883"

.PHONY: test clean deps coverage 

build:
	go build -o $(CONNECTOR_SERVICE_BINARY) cmd/$(CONNECTOR_SERVICE_BINARY)/*.go
	go build -o $(CONNECTED_CLIENT_BINARY) cmd/$(CONNECTED_CLIENT_BINARY)/*.go
	go build -o $(MIGRATE_DB_BINARY) cmd/$(MIGRATE_DB_BINARY)/main.go

deps:
	go get -u golang.org/x/lint/golint

test:
	# Use the following command to run specific tests (not the entire suite)
	# TEST_ARGS="-run TestReadMessage -v" make test
	go test $(TEST_ARGS) ./...

migrate: $(MIGRATE_DB_BINARY)
	./$(MIGRATE_DB_BINARY) upgrade

coverage:
	go test -v -coverprofile=$(COVERAGE_OUTPUT) ./...
	go tool cover -html=$(COVERAGE_OUTPUT) -o $(COVERAGE_HTML)
	@echo "file://$(PWD)/$(COVERAGE_HTML)"

start-test-env:
	podman-compose -f $(DOCKER_COMPOSE_CFG) up

stop-test-env:
	podman-compose -f $(DOCKER_COMPOSE_CFG) down

fmt:
	go fmt ./...

lint:
	$(GOPATH)/bin/golint ./...

clean:
	go clean
	rm -f $(CONNECTOR_SERVICE_BINARY) $(CONNECTED_CLIENT_BINARY) $(MIGRATE_DB_BINARY)
	rm -f $(COVERAGE_OUTPUT) $(COVERAGE_HTML)

test-client:
	./test_client -broker $(LOCAL_BROKER) -connection_count 1 -cert ./dev/test_client/client-0-cert.pem -key ./dev/test_client/client-0-key.pem

test-server: migrate
	CLOUD_CONNECTOR_MQTT_CLIENT_ID="connector-service" CLOUD_CONNECTOR_SOURCES_RECORDER_IMPL="fake" CLOUD_CONNECTOR_MQTT_BROKER_TLS_SKIP_VERIFY="true" CLOUD_CONNECTOR_CONNECTED_CLIENT_RECORDER_IMPL="fake" CLOUD_CONNECTOR_MQTT_BROKER_TLS_CERT_FILE="./dev/test_client/connector-service_crt.pem"  CLOUD_CONNECTOR_MQTT_BROKER_TLS_KEY_FILE="./dev/test_client/connector-service_key.pem" CLOUD_CONNECTOR_MQTT_BROKER_ADDRESS="tls://localhost:8883" CLOUD_CONNECTOR_MQTT_TOPIC_PREFIX="redhat" CLOUD_CONNECTOR_LOG_LEVEL="debug" CLOUD_CONNECTOR_ENABLE_PROFILE="true" ./cloud-connector mqtt_connection_handler -l localhost:8081