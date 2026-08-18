# Midaz Go SDK Makefile

# Color definitions - empty to disable colors
YELLOW := 
GREEN := 
CYAN := 
RED := 
NC := 
BOLD := 

# Component-specific variables
SERVICE_NAME := Midaz Go SDK
BIN_DIR := ./bin
ARTIFACTS_DIR := ./artifacts
DOCS_DIR := ./docs/godoc
VERSION := 1.0.0

# Ensure directories exist
$(shell mkdir -p $(ARTIFACTS_DIR))
$(shell mkdir -p $(DOCS_DIR))

# Define a simple function for section headers
define print_header
	@echo ""
	@echo "==== $(1) ===="
	@echo ""
endef

# Go commands
GO := go
GOFMT := gofmt
GOLINT := golangci-lint
GOLANGCI_LINT_VERSION := v2.12.1
GOLANGCI_LINT_VERSION_NUMBER := $(patsubst v%,%,$(GOLANGCI_LINT_VERSION))
GOLANGCI_LINT_MODULE := github.com/golangci/golangci-lint/v2/cmd/golangci-lint
GOSEC_VERSION := v2.25.0
GOSEC := $(GO) run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
GOMOD := $(GO) mod
GOBUILD := $(GO) build
GOTEST := $(GO) test
GOTOOL := $(GO) tool
GOCLEAN := $(GO) clean

# Project variables
PROJECT_ROOT := $(shell pwd)
PROJECT_NAME := midaz-go-sdk
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
MODULE := $(shell $(GO) list -m)

# Environment variables
ENV_FILE := $(PROJECT_ROOT)/.env
ENV_TEMPLATE ?= .env.local.example
COVERAGE_THRESHOLD ?= 80.0
DEMO_AUTH_MODE ?= anonymous-local

# Load environment variables if .env exists
ifneq (,$(wildcard .env))
    include .env
endif

#-------------------------------------------------------
# Core Commands
#-------------------------------------------------------

.PHONY: help ci

ci:
	$(call print_header,"Running SDK CI pipeline")
	@$(MAKE) tidy fmt lint gosec test test-contract coverage verify-sdk
	@echo "$(GREEN)[ok]$(NC) SDK CI pipeline completed successfully$(GREEN) ✔️$(NC)"

help:
	@echo ""
	@echo "$(SERVICE_NAME) Commands"
	@echo ""
	@echo "Core Commands:"
	@echo "  make help                        - Display this help message"
	@echo "  make ci                          - Run the SDK CI pipeline locally"
	@echo "  make set-env                     - Create .env from ENV_TEMPLATE (default: .env.local.example)"
	@echo "  make set-env FORCE=1             - Backup and overwrite an existing .env"
	@echo "  make test                        - Run all tests"
	@echo "  make test-fast                   - Run tests with -short flag"
	@echo "  make clean                       - Clean build artifacts"
	@echo "  make coverage                    - Generate test coverage report"
	@echo ""
	@echo "Code Quality Commands:"
	@echo "  make lint                        - Run linting tools"
	@echo "  make fmt                         - Format code"
	@echo "  make tidy                        - Clean dependencies"
	@echo "  make verify-sdk                  - Run SDK quality checks"
	@echo "  make hooks                       - Install git hooks"
	@echo "  make gosec                       - Run security checks with gosec"
	@echo "  make gosec-audit                 - Run deeper scheduled gosec audit checks"
	@echo ""
	@echo "Example Commands:"
	@echo "  make example                     - Run complete workflow example"
	@echo "  make demo-data                   - Run mass demo data generator (non-interactive)"
	@echo "  make demo-data-interactive       - Run mass demo data generator with prompts"
	@echo "  make examples-test               - Build every example and run their test suites"
	@echo ""
	@echo "Documentation Commands:"
	@echo "  make godoc                       - Start a godoc server for interactive documentation"
	@echo "  make godoc-static                - Generate static documentation files"
	@echo "  make docs                        - Generate comprehensive documentation (includes godoc-static)"
	@echo ""

#-------------------------------------------------------
# Environment Setup
#-------------------------------------------------------

.PHONY: set-env

set-env:
	$(call print_header,"Setting up environment")
	@template="$(ENV_TEMPLATE)"; \
	case "$$template" in /*) template_path="$$template" ;; *) template_path="$(PROJECT_ROOT)/$$template" ;; esac; \
	if [ ! -f "$$template_path" ]; then \
		echo "$(RED)[error]$(NC) Environment template not found: $$template_path$(RED) ❌$(NC)"; \
		echo "Available templates: .env.local.example, .env.production.example"; \
		exit 1; \
	fi; \
	if [ -f "$(ENV_FILE)" ] && [ "$(FORCE)" != "1" ]; then \
		echo "$(YELLOW)[skipped]$(NC) .env already exists. Re-run with FORCE=1 to overwrite after backup.$(YELLOW) ⚠️$(NC)"; \
		echo "Example: make set-env ENV_TEMPLATE=$$template FORCE=1"; \
		exit 0; \
	fi; \
	if [ -f "$(ENV_FILE)" ]; then \
		backup="$(ENV_FILE).backup.$$(date +%Y%m%d%H%M%S)"; \
		cp "$(ENV_FILE)" "$$backup"; \
		echo "$(YELLOW)[backup]$(NC) Existing .env backed up to $$backup"; \
	fi; \
	cp "$$template_path" "$(ENV_FILE)"; \
	echo "$(GREEN)[ok]$(NC) Created .env from $$template$(GREEN) ✔️$(NC)"

#-------------------------------------------------------
# SDK Quality Check Targets
#-------------------------------------------------------

.PHONY: check-references check-mmodel-references check-api-compatibility check-config-parity verify-sdk hooks

# Check that no lib-commons references appear in public packages
check-references:
	@echo "$(YELLOW)Checking for lib-commons references in public API...$(NC)"
	@! grep -r "lib-commons" --include="*.go" ./models ./entities | grep -v "//.*lib-commons" || (echo "$(RED)❌ Found lib-commons references in public API!$(NC)" && exit 1)
	@echo "$(GREEN)✅ No lib-commons references found in public API$(NC)"

# Track 7E: enforce no mmodel references in public API (root, models, entities, pkg/...).
check-mmodel-references:
	@echo "$(YELLOW)Checking for mmodel references in public API...$(NC)"
	@bad=$$(grep -rl "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/mmodel" --include="*.go" --exclude="*_test.go" ./*.go ./models ./entities ./pkg 2>/dev/null | xargs -I{} grep -Hn "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/mmodel" {} | grep -v '^[^:]*:[[:space:]]*//' || true); \
		if [ -n "$$bad" ]; then echo "$(RED)❌ Found unexpected mmodel references in public API:$(NC)"; echo "$$bad"; exit 1; fi
	@echo "$(GREEN)✅ No unexpected mmodel references in public API$(NC)"

# Verify that our refactoring doesn't break API compatibility
check-api-compatibility:
	@echo "$(YELLOW)Checking API compatibility...$(NC)"
	@go build ./models ./entities ./pkg/...
	@echo "$(GREEN)✅ API builds successfully$(NC)"

# Track 6 lint rule: enforce midaz.With* / pkg/config.With* two-layer parity.
# Fails the build when a pkg/config Option lacks a midaz wrapper (with the
# documented retry-knob exception list). See scripts/check-config-parity.sh.
check-config-parity:
	@echo "$(YELLOW)Checking midaz / pkg/config two-layer Option parity...$(NC)"
	@./scripts/check-config-parity.sh

# Verify our implementation
verify-sdk: check-references check-mmodel-references check-api-compatibility check-config-parity check-codegen-drift examples-test
	@echo "$(GREEN)✅ All SDK quality checks passed!$(NC)"

# Install git hooks
hooks:
	$(call print_header,"Installing Git Hooks")
	@chmod +x scripts/install-hooks.sh
	@./scripts/install-hooks.sh

#-------------------------------------------------------
# Test Commands
#-------------------------------------------------------

.PHONY: test test-fast test-contract coverage

test:
	$(call print_header,"Running tests")
	@./scripts/run_tests.sh

test-fast:
	$(call print_header,"Running fast tests")
	@GOTEST_SHORT=1 ./scripts/run_tests.sh

# Drift guard: pins the SDK's transaction-status vocabulary and lifecycle error
# codes against the live Midaz server contract. Lives in the nested contract/
# module so the server dependency never enters the SDK's published go.mod.
test-contract:
	$(call print_header,"Running server-contract drift tests")
	@cd contract && $(GOTEST) ./...

coverage:
	$(call print_header,"Generating test coverage")
	@$(GOTEST) -coverprofile=$(ARTIFACTS_DIR)/coverage.out $$(go list ./... | grep -v -E '(examples|mocks|internal/gen)')
	@coverage=$$($(GOTOOL) cover -func=$(ARTIFACTS_DIR)/coverage.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
		echo "Total coverage: $${coverage}% (threshold: $(COVERAGE_THRESHOLD)%)"; \
		awk -v coverage="$$coverage" -v threshold="$(COVERAGE_THRESHOLD)" 'BEGIN { exit !(coverage + 0 >= threshold + 0) }' || { \
			echo "$(RED)[error]$(NC) Coverage $${coverage}% is below threshold $(COVERAGE_THRESHOLD)%$(RED) ❌$(NC)"; \
			exit 1; \
		}
	@$(GOTOOL) cover -html=$(ARTIFACTS_DIR)/coverage.out -o $(ARTIFACTS_DIR)/coverage.html
	@echo "Coverage report generated at $(ARTIFACTS_DIR)/coverage.html"
	@echo "$(GREEN)[ok]$(NC) Coverage report generated successfully"

#-------------------------------------------------------
# Code Quality Commands
#-------------------------------------------------------

.PHONY: lint fmt tidy gosec gosec-audit

lint:
	$(call print_header,"Running linters")
	@set -e; \
	if find . -name "*.go" -type f | grep -q .; then \
		if ! command -v $(GOLINT) > /dev/null || ! $(GOLINT) --version | grep -q "version $(GOLANGCI_LINT_VERSION_NUMBER)"; then \
			echo "$(YELLOW)Installing golangci-lint $(GOLANGCI_LINT_VERSION)...$(NC)"; \
			go install $(GOLANGCI_LINT_MODULE)@$(GOLANGCI_LINT_VERSION); \
		fi; \
		$(GOLINT) run; \
		echo "$(GREEN)[ok]$(NC) Linting completed successfully$(GREEN) ✔️$(NC)"; \
	else \
		echo "$(YELLOW)No Go files found, skipping linting$(NC)"; \
	fi

fmt:
	$(call print_header,"Formatting code")
	@$(GOFMT) -s -w .
	@echo "$(GREEN)[ok]$(NC) Formatting completed successfully$(GREEN) ✔️$(NC)"

tidy:
	$(call print_header,"Cleaning dependencies")
	@$(GOMOD) tidy
	@echo "$(GREEN)[ok]$(NC) Dependencies cleaned successfully$(GREEN) ✔️$(NC)"

gosec:
	$(call print_header,"Running security checks")
	@echo "$(CYAN)Running gosec security scanner ($(GOSEC_VERSION))...$(NC)"
	@$(GOSEC) -exclude-generated -quiet ./...
	@echo "$(GREEN)[ok]$(NC) Security checks completed successfully$(GREEN) ✔️$(NC)"

gosec-audit:
	$(call print_header,"Running security audit checks")
	@echo "$(CYAN)Running gosec audit scanner ($(GOSEC_VERSION))...$(NC)"
	@$(GOSEC) -quiet -enable-audit -exclude=G104 ./...
	@echo "$(GREEN)[ok]$(NC) Security audit checks completed successfully$(GREEN) ✔️$(NC)"

#-------------------------------------------------------
# Clean Commands
#-------------------------------------------------------

.PHONY: clean

clean:
	$(call print_header,"Cleaning build artifacts")
	@echo "$(CYAN)Cleaning build artifacts...$(NC)"
	@$(GOCLEAN)
	@rm -rf $(BIN_DIR)/ $(ARTIFACTS_DIR)/coverage.out $(ARTIFACTS_DIR)/coverage.html
	@echo "$(GREEN)[ok]$(NC) Artifacts cleaned successfully$(GREEN) ✔️$(NC)"

#-------------------------------------------------------
# Codegen Commands
#-------------------------------------------------------

.PHONY: generate check-codegen-drift

generate:
	$(call print_header,"Regenerating OpenAPI clients")
	@./scripts/generate-clients.sh
	@$(GO) mod tidy
	@echo "$(GREEN)[ok]$(NC) OpenAPI clients regenerated successfully$(GREEN) ✔️$(NC)"

# Determinism gate: regenerate the clients and fail if the committed output
# drifts from the source specs. The analogue of the docs-pipeline drift gate.
# See scripts/check-codegen-drift.sh.
check-codegen-drift:
	$(call print_header,"Checking OpenAPI codegen drift")
	@./scripts/check-codegen-drift.sh

#-------------------------------------------------------
# Example Commands
#-------------------------------------------------------

.PHONY: example

example:
	$(call print_header,"Running Complete Workflow Example")
	$(call print_header,"Make sure the Midaz Stack is running --default is localhost")
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "$(RED)[error]$(NC) Missing $(ENV_FILE). Run 'make set-env' or export the required MIDAZ_* variables.$(RED) ❌$(NC)"; \
		exit 1; \
	fi; \
	while IFS='=' read -r key value; do \
		case "$$key" in ''|'#'*) continue ;; esac; \
		case "$$key" in *[!A-Za-z0-9_]*|[0-9]*) continue ;; esac; \
		value=$${value%\"}; value=$${value#\"}; \
		export "$$key=$$value"; \
	done < "$(ENV_FILE)"; \
	cd examples/workflow-with-entities && go run main.go

.PHONY: demo-data demo-data-interactive

DEMO_NON_INTERACTIVE ?= 1

demo-data:
	$(call print_header,Running Mass Demo Data Generator)
	$(call print_header,Ensure Midaz Ledger is on localhost:3002/v1 and CRM is on localhost:4003/v1 or set MIDAZ_* URLs)
	@DEMO_AUTH_MODE=$(DEMO_AUTH_MODE) DEMO_NON_INTERACTIVE=$(DEMO_NON_INTERACTIVE) go run ./examples/mass-demo-generator

demo-data-interactive:
	@$(MAKE) demo-data DEMO_NON_INTERACTIVE=0

.PHONY: examples-test

# examples-test builds every example program under examples/ and runs
# the test suite for examples that ship one (notably 09-testing-with-mocks).
# It is the compile-time guarantee that every example tracks the public SDK
# surface — refactors that break a documented call shape break the build here.
examples-test:
	$(call print_header,"Building all examples")
	@$(GOBUILD) ./examples/... 2>&1
	@echo "$(GREEN)[ok]$(NC) All examples build cleanly$(GREEN) ✔️$(NC)"
	$(call print_header,"Running example tests")
	@$(GOTEST) ./examples/... 2>&1
	@echo "$(GREEN)[ok]$(NC) Example tests passed$(GREEN) ✔️$(NC)"

#-------------------------------------------------------
# Documentation Commands
#-------------------------------------------------------

.PHONY: godoc godoc-static docs

godoc:
	$(call print_header,"Starting godoc server")
	@echo "$(CYAN)Starting godoc server at http://localhost:6060/pkg/$(MODULE)/$(NC)"
	@if ! command -v godoc > /dev/null; then \
		echo "$(YELLOW)Installing godoc...$(NC)"; \
		go install golang.org/x/tools/cmd/godoc@latest; \
	fi
	@godoc -http=:6060

# List of packages to generate documentation for
PACKAGES := \
	$(MODULE) \
	$(MODULE)/entities \
	$(MODULE)/models \
	$(MODULE)/models/correlation \
	$(MODULE)/models/correlation/correlationtest \
	$(MODULE)/pkg/auth \
	$(MODULE)/pkg/config \
	$(MODULE)/pkg/concurrent \
	$(MODULE)/pkg/observability \
	$(MODULE)/pkg/sdkctx \
	$(MODULE)/pkg/validation \
	$(MODULE)/pkg/validation/core \
	$(MODULE)/pkg/errors \
	$(MODULE)/pkg/format \
	$(MODULE)/pkg/generator \
	$(MODULE)/pkg/retry \
	$(MODULE)/pkg/transaction \
	$(MODULE)/pkg/performance

godoc-static:
	$(call print_header,"Generating static documentation")
	@echo "$(CYAN)Generating static documentation...$(NC)"
	@rm -rf $(DOCS_DIR)
	@mkdir -p $(DOCS_DIR)
	@# Process each package
	@for pkg in $(PACKAGES) ; do \
		echo "$(CYAN)Generating documentation for $${pkg}...$(NC)" ; \
		if [ "$$pkg" = "$(MODULE)" ]; then \
			pkg_path="." ; \
		else \
			pkg_path=$${pkg#$(MODULE)/}; \
		fi ; \
		pkg_dir=$(DOCS_DIR)/$${pkg_path} ; \
		mkdir -p $${pkg_dir} ; \
		go doc $${pkg} > $${pkg_dir}/index.txt ; \
	done
	@echo "$(GREEN)[ok]$(NC) Static documentation generated successfully in $(DOCS_DIR)$(GREEN) ✔️$(NC)"

# Just run godoc-static for now, as we have manually edited README.md
docs: godoc-static
	$(call print_header,"Documentation generation complete")
	@echo "$(GREEN)[ok]$(NC) Documentation generated successfully$(GREEN) ✔️$(NC)"
