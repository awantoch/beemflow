# ────────────────────────────────────────────────────────────────────────────
# Use bash with strict modes (-e: exit on error, -u: unset vars, -o pipefail)
SHELL   := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
# ────────────────────────────────────────────────────────────────────────────
BINARY   := flow
CMD_PATH := ./cmd/flow
# ────────────────────────────────────────────────────────────────────────────
.PHONY: all clean build install test fmt vet lint deps coverage run e2e serve check fix test-integration test-comprehensive test-edge-cases test-performance

all: clean test build install 

# remove all untracked & ignored files except .env
clean:
	git clean -dfx -e .env

# compile CLI binary
build:
	go build -o $(BINARY) $(CMD_PATH)

build-static:
	CGO_ENABLED=0 \
	GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags="-s -w" -o $(BINARY) $(CMD_PATH)

# start the HTTP server
serve:
	go run $(CMD_PATH) serve

# install to $(GOBIN) or $GOPATH/bin
install: build
	go install $(CMD_PATH)

# run all tests with verbose output
test:
	go test -v ./...

# run tests with race detection
test-race:
	go test -race -v ./...

# generate a coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

# run common e2e flows
e2e:
	go run $(CMD_PATH) run flows/e2e/fetch_and_summarize.flow.yaml
	go run $(CMD_PATH) run flows/e2e/parallel_openai.flow.yaml
	go run $(CMD_PATH) run flows/e2e/airtable_integration.flow.yaml

# run comprehensive integration tests
test-integration:
	@echo "Running integration tests..."
	go run ./cmd/flow run flows/integration/engine_comprehensive.flow.yaml
	go run ./cmd/flow run flows/integration/parallel_execution.flow.yaml
	go run ./cmd/flow run flows/integration/templating_system.flow.yaml
	go run ./cmd/flow run flows/integration/nested_parallel.flow.yaml
	go run ./cmd/flow run flows/integration/http_patterns.flow.yaml

# run edge case tests
test-edge-cases:
	@echo "Running edge case tests..."
	go run ./cmd/flow run flows/integration/edge_cases.flow.yaml

# run performance tests
test-performance:
	@echo "Running performance tests..."
	time go run ./cmd/flow run flows/integration/performance.flow.yaml

# run comprehensive test suite including unit, integration, and e2e tests
test-comprehensive: test test-race test-integration test-edge-cases test-performance
	@echo "All tests completed successfully!"

# master check target that runs all code quality checks
check: fmt vet lint tidy

# format all Go files
fmt:
	go fmt ./...

# vet for static checks
vet:
	go vet ./...

# run linter with our custom configuration
lint:
	golangci-lint run -c .golangci.yml ./...

# tidy & verify modules
tidy:
	go mod tidy
	go mod verify

# auto-fix issues where possible
fix:
	@golangci-lint run --fix -c .golangci.yml ./...
	@go fmt ./...
