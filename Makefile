# Midaz Go SDK Makefile

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
GOMOD := $(GO) mod
GOBUILD := $(GO) build
GOTEST := $(GO) test
GOTOOL := $(GO) tool
GOCLEAN := $(GO) clean

# Project variables
PROJECT_ROOT := $(shell pwd)
PROJECT_NAME := midaz-go-sdk
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

# Environment variables
ENV_FILE := $(PROJECT_ROOT)/.env
ENV_EXAMPLE_FILE := $(PROJECT_ROOT)/.env.example

# Load environment variables if .env exists
ifneq (,$(wildcard .env))
    include .env
endif

#-------------------------------------------------------
# Core Commands
#-------------------------------------------------------

.PHONY: help
help:
	@echo ""
	@echo "$(SERVICE_NAME) Commands"
	@echo ""
	@echo "Core Commands:"
	@echo "  make help                        - Display this help message"
	@echo "  make setup-env                   - Create .env file from .env.example if it doesn't exist"
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
	@echo ""
	@echo "Example Commands:"
	@echo "  make example                     - Run complete workflow example"
	@echo ""
	@echo "Documentation Commands:"
	@echo "  make godoc                       - Start a godoc server for interactive documentation"
	@echo "  make godoc-static                - Generate static documentation files"
	@echo "  make docs                        - Generate comprehensive documentation (includes godoc-static)"
	@echo ""

#-------------------------------------------------------
# Environment Setup
#-------------------------------------------------------

.PHONY: setup-env

setup-env:
	$(call print_header,"Setting up environment")
	@if [ ! -f "$(ENV_FILE)" ] && [ -f "$(ENV_EXAMPLE_FILE)" ]; then \
		echo "No .env file found. Creating from .env.example..."; \
		cp $(ENV_EXAMPLE_FILE) $(ENV_FILE); \
		echo "[ok] Created .env file from .env.example "; \
	elif [ ! -f "$(ENV_FILE)" ] && [ ! -f "$(ENV_EXAMPLE_FILE)" ]; then \
		echo "[error] Neither .env nor .env.example files found"; \
		exit 1; \
	elif [ -f "$(ENV_FILE)" ]; then \
		read -t 10 -p ".env file already exists. Overwrite with .env.example? [Y/n] (auto-yes in 10s) " answer || answer="Y"; \
		answer=$${answer:-Y}; \
		if [[ $$answer =~ ^[Yy] ]]; then \
			cp $(ENV_EXAMPLE_FILE) $(ENV_FILE); \
			echo "[ok] Overwrote .env file with .env.example "; \
		else \
			echo "[skipped] Kept existing .env file"; \
		fi; \
	fi

#-------------------------------------------------------
# SDK Quality Check Targets
#-------------------------------------------------------

.PHONY: check-references check-api-compatibility verify-sdk hooks

# Check that no lib-commons references appear in public packages
check-references:
	@echo "Checking for lib-commons references in public API..."
	@! grep -r "lib-commons" --include="*.go" ./models ./entities | grep -v "//.*lib-commons" || (echo " Found lib-commons references in public API!" && exit 1)
	@echo " No lib-commons references found in public API"

# Verify that our refactoring doesn't break API compatibility
check-api-compatibility:
	@echo "Checking API compatibility..."
	@go build ./models ./entities ./internal/...
	@echo " API builds successfully"

# Verify our implementation
verify-sdk: check-references check-api-compatibility
	@echo " All SDK quality checks passed!"

# Install git hooks
hooks:
	$(call print_header,"Installing Git Hooks")
	@chmod +x scripts/install-hooks.sh
	@./scripts/install-hooks.sh

#-------------------------------------------------------
# Test Commands
#-------------------------------------------------------

.PHONY: test test-fast coverage

test:
	$(call print_header,"Running tests")
	@./scripts/run_tests.sh

test-fast:
	$(call print_header,"Running fast tests")
	@GOTEST_SHORT=1 ./scripts/run_tests.sh

coverage:
	$(call print_header,"Generating test coverage")
	@$(GOTEST) -coverprofile=$(ARTIFACTS_DIR)/coverage.out ./...
	@$(GOTOOL) cover -html=$(ARTIFACTS_DIR)/coverage.out -o $(ARTIFACTS_DIR)/coverage.html
	@echo "Coverage report generated at $(ARTIFACTS_DIR)/coverage.html"
	@echo "[ok] Coverage report generated successfully"

#-------------------------------------------------------
# Code Quality Commands
#-------------------------------------------------------

.PHONY: lint fmt tidy

lint:
	$(call print_header,"Running linters")
	@if find . -name "*.go" -type f | grep -q .; then \
		if ! command -v $(GOLINT) > /dev/null; then \
			echo "Installing golangci-lint..."; \
			go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		fi; \
		$(GOLINT) run; \
		echo "[ok] Linting completed successfully "; \
	else \
		echo "No Go files found, skipping linting"; \
	fi

fmt:
	$(call print_header,"Formatting code")
	@$(GOFMT) -s -w .
	@echo "[ok] Formatting completed successfully "

tidy:
	$(call print_header,"Cleaning dependencies")
	@$(GOMOD) tidy
	@echo "[ok] Dependencies cleaned successfully "

#-------------------------------------------------------
# Clean Commands
#-------------------------------------------------------

.PHONY: clean

clean:
	$(call print_header,"Cleaning build artifacts")
	@echo "Cleaning build artifacts..."
	@$(GOCLEAN)
	@rm -rf $(BIN_DIR)/ $(ARTIFACTS_DIR)/coverage.out $(ARTIFACTS_DIR)/coverage.html
	@echo "[ok] Artifacts cleaned successfully "

#-------------------------------------------------------
# Example Commands
#-------------------------------------------------------

.PHONY: example

example:
	$(call print_header,"Running Complete Workflow Example")
	$(call print_header,"Make sure the Midaz Stack is running --default is localhost")
	@cp $(ENV_FILE) examples/workflow-with-entities/.env
	@cd examples/workflow-with-entities && go run main.go

#-------------------------------------------------------
# Documentation Commands
#-------------------------------------------------------

.PHONY: godoc godoc-static docs

godoc:
	$(call print_header,"Starting godoc server")
	@echo "Starting godoc server at http://localhost:6060/pkg/github.com/LerianStudio/midaz-sdk-golang/"
	@if ! command -v godoc > /dev/null; then \
		echo "Installing godoc..."; \
		go install golang.org/x/tools/cmd/godoc@latest; \
	fi
	@godoc -http=:6060

# List of packages to generate documentation for
PACKAGES := \
	github.com/LerianStudio/midaz-sdk-golang \
	github.com/LerianStudio/midaz-sdk-golang/entities \
	github.com/LerianStudio/midaz-sdk-golang/models \
	github.com/LerianStudio/midaz-sdk-golang/pkg/config \
	github.com/LerianStudio/midaz-sdk-golang/pkg/concurrent \
	github.com/LerianStudio/midaz-sdk-golang/pkg/observability \
	github.com/LerianStudio/midaz-sdk-golang/pkg/pagination \
	github.com/LerianStudio/midaz-sdk-golang/pkg/validation \
	github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core \
	github.com/LerianStudio/midaz-sdk-golang/pkg/errors \
	github.com/LerianStudio/midaz-sdk-golang/pkg/format \
	github.com/LerianStudio/midaz-sdk-golang/pkg/retry \
	github.com/LerianStudio/midaz-sdk-golang/pkg/performance

godoc-static:
	$(call print_header,"Generating static documentation")
	@echo "Generating static documentation..."
	@mkdir -p $(DOCS_DIR)
	@# Process each package
	@for pkg in $(PACKAGES) ; do \
		echo "Generating documentation for $${pkg}..." ; \
		pkg_path=$${pkg#github.com/LerianStudio/midaz-sdk-golang/} ; \
		if [ "$${pkg_path}" = "github.com/LerianStudio/midaz-sdk-golang" ]; then \
			pkg_path="." ; \
		fi ; \
		pkg_dir=$(DOCS_DIR)/$${pkg_path} ; \
		mkdir -p $${pkg_dir} ; \
		go doc $${pkg} > $${pkg_dir}/index.txt ; \
	done
	@echo "[ok] Static documentation generated successfully in $(DOCS_DIR) "

# Just run godoc-static for now, as we have manually edited README.md
docs: godoc-static
	$(call print_header,"Documentation generation complete")
	@echo "[ok] Documentation generated successfully "