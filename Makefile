CONNECTOR_SERVICE_BINARY=cloud-connector
CONNECTED_CLIENT_BINARY=test_client
MIGRATE_DB_BINARY=migrate_db
DB_SCHEMA_DUMPER_BINARY=db_schema_dumper

DOCKER_COMPOSE_CFG=dev.yml

COVERAGE_OUTPUT=coverage.out
COVERAGE_HTML=coverage.html

.PHONY: test clean deps coverage 

build:
	go build -o $(CONNECTOR_SERVICE_BINARY) cmd/$(CONNECTOR_SERVICE_BINARY)/*.go
	go build -o $(CONNECTED_CLIENT_BINARY) cmd/$(CONNECTED_CLIENT_BINARY)/*.go
	go build -o $(MIGRATE_DB_BINARY) cmd/$(MIGRATE_DB_BINARY)/main.go
	go build -o $(DB_SCHEMA_DUMPER_BINARY) cmd/$(DB_SCHEMA_DUMPER_BINARY)/main.go
	go build -o stage_db_fixer cmd/stage_db_fixer/main.go
	go build -o auto_test_client cmd/auto_test_client/*.go

run-pendo-transmitter:
	./$(CONNECTOR_SERVICE_BINARY) connection_count_per_account_reporter -r pendo

deps:
	go get -u golang.org/x/lint/golint

test:
	# Use the following command to run specific tests (not the entire suite)
	# TEST_ARGS="-run TestReadMessage -v" make test
	go test -v $(TEST_ARGS) ./...

test-sql: migrate
	go test -v $(TEST_ARGS) ./... -tags=sql

migrate: $(MIGRATE_DB_BINARY)
	./$(MIGRATE_DB_BINARY) upgrade

$(MIGRATE_DB_BINARY): build

coverage:
	go test -v -coverprofile=$(COVERAGE_OUTPUT) ./...
	go tool cover -html=$(COVERAGE_OUTPUT) -o $(COVERAGE_HTML)
	@echo "file://$(PWD)/$(COVERAGE_HTML)"

start-test-env:
	podman-compose -f $(DOCKER_COMPOSE_CFG) up

stop-test-env:
	podman-compose -f $(DOCKER_COMPOSE_CFG) down

build_image:
	podman build . -t cloud-connector

fmt:
	go fmt ./...

lint:
	$(GOPATH)/bin/golint ./...

clean:
	go clean
	rm -f $(CONNECTOR_SERVICE_BINARY) $(CONNECTED_CLIENT_BINARY) $(MIGRATE_DB_BINARY)
	rm -f $(COVERAGE_OUTPUT) $(COVERAGE_HTML)
