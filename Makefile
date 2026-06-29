BIN       := incidentflow
BUILD_DIR := ./bin
MODULE    := github.com/incidentflow/incidentflow-cli
CMD       := ./cmd/incidentflow

.PHONY: build install test lint clean

build:
	GOWORK=off go build -o $(BUILD_DIR)/$(BIN) $(CMD)

install:
	GOWORK=off go install $(CMD)

test:
	GOWORK=off go test ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
