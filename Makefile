.PHONY: build build-core test vet lint fmt check clean run db-reset hooks agents workspace

BINARY       := golearn
FORGE_BINARY := golearn-forge
BIN_DIR      := ./bin
SRC          := ./cmd/golearn
FORGE_DIR    := ./addons/forge
FORGE_SRC    := ./cmd/golearn-forge

# The core module and the Forge addon (D-015). Every module-scoped target
# iterates this list: `go test ./...` is module-scoped, NOT workspace-scoped,
# so a single run from the root silently skips addons/forge entirely. See
# docs/design/FORGE-EXPERIMENTS.md (A-1) for the measurement.
MODULES := . $(FORGE_DIR)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)

ifeq ($(GOLANGCI_LINT),)
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
endif

## run: run without building
run:
	go run $(SRC)/main.go

## build: compile the golearn and golearn-forge binaries
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) $(SRC)
	cd $(FORGE_DIR) && go build -o ../../$(BIN_DIR)/$(FORGE_BINARY) $(FORGE_SRC)

## build-core: compile only the offline core, exactly as a consumer would
build-core:
	@mkdir -p $(BIN_DIR)
	GOWORK=off go build -o $(BIN_DIR)/$(BINARY) $(SRC)

## test: run all tests in every module
test:
	@for m in $(MODULES); do \
		echo "--> go test in $$m"; \
		(cd $$m && go test ./... -v -count=1) || exit 1; \
	done

## vet: run go vet in every module
vet:
	@for m in $(MODULES); do \
		echo "--> go vet in $$m"; \
		(cd $$m && go vet ./...) || exit 1; \
	done

## fmt: check formatting (fails if files need formatting)
fmt:
	@test -z "$$(gofmt -l .)" || { echo "Files need formatting:"; gofmt -l .; exit 1; }

## lint: run golangci-lint in every module (install separately)
lint:
	@if [ -x "$(GOLANGCI_LINT)" ]; then \
		echo "Validating configuration..."; \
		$(GOLANGCI_LINT) config verify --config=.golangci.yml; \
		for m in $(MODULES); do \
			echo "--> golangci-lint in $$m"; \
			(cd $$m && $(GOLANGCI_LINT) run --config=$(CURDIR)/.golangci.yml ./...) || exit 1; \
		done; \
	else \
		echo "golangci-lint not installed — skipping"; \
	fi

## check: full CI gate (fmt + vet + lint + test, all modules)
check: fmt vet lint test

## agents: link vendor agent config (.claude/skills) to the shared .agents/ tree
agents:
	@./scripts/agents-link.sh

## workspace: create the local, gitignored go.work spanning both modules
workspace:
	@test -f go.work || go work init $(MODULES)
	@go work sync
	@echo "go.work ready (local only — neither module requires it)"

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
