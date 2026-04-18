.PHONY: run build test integration-test integration-tests lint tidy generate stop vuln check-coverage check-routes guard hooks

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
	go test -race -coverprofile=coverage.out ./internal/...

integration-test:
	go test -count=1 ./tests/integration/...

integration-tests: integration-test

tidy:
	go mod tidy

generate:
	go generate ./...

vuln:
	$(HOME)/go/bin/govulncheck ./...

check-coverage:
	$(HOME)/go/bin/go-test-coverage --config=.testcoverage.yml

check-routes:
	scripts/check-routes.sh

guard: lint vuln test check-coverage check-routes integration-test

hooks:
	cp scripts/pre-commit .git/hooks/pre-commit
	chmod +x .git/hooks/pre-commit
