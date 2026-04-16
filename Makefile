.PHONY: run build test integration-test integration-tests lint tidy generate stop

BINARY  := ./alfredo
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

run: build
	@if [ -f .env ]; then set -a && . ./.env && set +a; fi; \
	$(BINARY)

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/server

lint:
	golangci-lint run ./...

test:
	go test ./internal/...

integration-test:
	go test -count=1 ./tests/integration/...

integration-tests: integration-test

tidy:
	go mod tidy

generate:
	go generate ./...
