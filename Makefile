.PHONY: build test vet lint fmt check clean run db-reset hooks

BINARY  := golearn
BIN_DIR := ./bin
SRC     := ./cmd/golearn
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)

ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
endif

## run: run without building
run:
	go run $(SRC)/main.go

## build: compile the golearn binary
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) $(SRC)

## test: run all tests
test:
	go test ./... -v -count=1

## vet: run go vet
vet:
	go vet ./...

## fmt: check formatting (fails if files need formatting)
fmt:
	@test -z "$$(gofmt -l .)" || { echo "Files need formatting:"; gofmt -l .; exit 1; }

## lint: run golangci-lint (install separately)
lint:
	@if [ -x "$(GOLANGCI_LINT)" ]; then \
		echo "Validating configuration..."; \
		$(GOLANGCI_LINT) config verify --config=.golangci.yml; \
		echo "Running golangci-lint..."; \
		$(GOLANGCI_LINT) run --config=.golangci.yml ./...; \
	else \
		echo "golangci-lint not installed — skipping"; \
	fi

## check: full CI gate (fmt + vet + lint + test)
check: fmt vet lint test

## hooks: install git hooks (pre-commit + pre-push) via core.hooksPath
hooks:
	git config core.hooksPath .githooks
	@echo "git hooks installed (core.hooksPath -> .githooks)"

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)

## db-reset: delete the local database and start fresh
db-reset: build
	$(BIN_DIR)/$(BINARY) db reset --yes

## help: show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'
