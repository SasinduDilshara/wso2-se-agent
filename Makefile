BINARY_NAME=wso2-se-agent
VERSION=$(shell git describe --tags --always 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
BUILD_DIR=bin

.PHONY: build install clean test lint fmt vet

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/wso2-se-agent/

install:
	go install $(LDFLAGS) ./cmd/wso2-se-agent/

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./internal/...

lint:
	golangci-lint run

fmt:
	go fmt ./...

vet:
	go vet ./...

# Cross-compile for releases
release: clean
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/wso2-se-agent/
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/wso2-se-agent/
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/wso2-se-agent/
