.PHONY: build test vet lint check clean

BINARY  := golearn
BIN_DIR := ./bin
SRC     := ./cmd/golearn

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

## lint: run golangci-lint (install separately)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed — skipping"; \
	fi

## check: full CI gate (vet + lint + test)
check: vet lint test

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR)

## help: show available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'
