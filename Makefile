CONNECTOR_SERVICE_BINARY=connector_service
CONNECTED_CLIENT_BINARY=test_client

DOCKER_COMPOSE_CFG=docker-compose.yml

COVERAGE_OUTPUT=coverage.out
COVERAGE_HTML=coverage.html

.PHONY: test clean deps coverage 

build:
	go build -o $(CONNECTOR_SERVICE_BINARY) cmd/connector_service/main.go
	go build -o $(CONNECTED_CLIENT_BINARY) cmd/test_client/main.go

deps:
	go get -u golang.org/x/lint/golint

test:
	# Use the following command to run specific tests (not the entire suite)
	# TEST_ARGS="-run TestReadMessage -v" make test
	go test $(TEST_ARGS) ./...

coverage:
	go test -v -coverprofile=$(COVERAGE_OUTPUT) ./...
	go tool cover -html=$(COVERAGE_OUTPUT) -o $(COVERAGE_HTML)
	@echo "file://$(PWD)/$(COVERAGE_HTML)"

start-test-env:
	docker-compose -f $(DOCKER_COMPOSE_CFG) up

stop-test-env:
	docker-compose -f $(DOCKER_COMPOSE_CFG) down

fmt:
	go fmt ./...

lint:
	$(GOPATH)/bin/golint ./...

clean:
	go clean
	rm -f $(CONNECTOR_SERVICE_BINARY) $(CONNECTED_CLIENT_BINARY)
	rm -f $(COVERAGE_OUTPUT) $(COVERAGE_HTML)
