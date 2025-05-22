# ────────────────────────────────────────────────────────────────────────────
# Use bash with strict modes (-e: exit on error, -u: unset vars, -o pipefail)
SHELL   := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
# ────────────────────────────────────────────────────────────────────────────
BINARY   := flow
CMD_PATH := ./cmd/flow
# ────────────────────────────────────────────────────────────────────────────
.PHONY: all clean build install test fmt vet lint deps coverage run e2e serve proto

all: clean test build install 

# remove all untracked & ignored files except .env
clean:
	git clean -dfx -e .env

# compile CLI binary
build:
	go build -o $(BINARY) $(CMD_PATH)

# start the HTTP server
serve:
	go run $(CMD_PATH) serve

# install to $(GOBIN) or $GOPATH/bin
install: build
	go install $(CMD_PATH)

# run all tests with verbose output
test:
	go test ./...

# generate a coverage report
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

# run common e2e flows
e2e:
	go run $(CMD_PATH) run flows/fetch_and_summarize.flow.yaml
	go run $(CMD_PATH) run flows/parallel_openai.flow.yaml
	go run $(CMD_PATH) run flows/list_airtable_bases.flow.yaml

# format all Go files
fmt:
	go fmt ./...

# vet for static checks
vet:
	go vet ./...

# tidy & verify modules
tidy:
	go mod tidy
	go mod verify

# generate Go protobuf code
proto:
	protoc --go_out=paths=source_relative:. spec/proto/flow.proto
