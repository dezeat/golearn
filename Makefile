.PHONY: build build-core test vet lint fmt check eval clean run db-reset hooks agents workspace tidy

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

# Every module-scoped target runs with GOWORK=off, so the gate sees exactly
# what a clean checkout and CI see.
#
# A local go.work is a convenience for editors, but under a workspace module
# resolution is workspace-wide: the addon resolves the core's transitive
# dependencies through the workspace and never needs its own go.sum entries.
# A commit that widens the addon's import graph then leaves addons/forge/go.sum
# stale, and the developer's own gate cannot see it — green locally, broken for
# everyone else. Measured in docs/design/FORGE-EXPERIMENTS.md A-12.
GOWORK_OFF := GOWORK=off
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
	$(GOWORK_OFF) go build -o $(BIN_DIR)/$(BINARY) $(SRC)
	cd $(FORGE_DIR) && $(GOWORK_OFF) go build -o ../../$(BIN_DIR)/$(FORGE_BINARY) $(FORGE_SRC)

## tidy: refresh each module's go.sum the way a clean checkout resolves it
## Run this after any change that widens a module's import graph.
tidy:
	@for m in $(MODULES); do \
		echo "--> go mod tidy in $$m"; \
		(cd $$m && $(GOWORK_OFF) go mod tidy) || exit 1; \
	done

## build-core: compile only the offline core, exactly as a consumer would
build-core:
	@mkdir -p $(BIN_DIR)
	GOWORK=off go build -o $(BIN_DIR)/$(BINARY) $(SRC)

## test: run all tests in every module, as a clean checkout would
test:
	@for m in $(MODULES); do \
		echo "--> go test in $$m"; \
		(cd $$m && $(GOWORK_OFF) go test ./... -v -count=1) || exit 1; \
	done

## vet: run go vet in every module, as a clean checkout would
vet:
	@for m in $(MODULES); do \
		echo "--> go vet in $$m"; \
		(cd $$m && $(GOWORK_OFF) go vet ./...) || exit 1; \
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
			(cd $$m && $(GOWORK_OFF) $(GOLANGCI_LINT) run --config=$(CURDIR)/.golangci.yml ./...) || exit 1; \
		done; \
	else \
		echo "golangci-lint not installed — skipping"; \
	fi

## check: full CI gate (fmt + vet + lint + test, all modules)
check: fmt vet lint test

## eval: run the deterministic, network-free Forge evaluation contract gate
eval:
	cd $(FORGE_DIR) && $(GOWORK_OFF) go test ./evaluation -count=1 -v

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
