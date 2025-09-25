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
	@echo "  make set-env                     - Create .env file from .env.example if it doesn't exist"
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
	@echo ""
	@echo "Example Commands:"
	@echo "  make example                     - Run complete workflow example"
	@echo "  make demo-data                   - Run mass demo data generator (interactive)"
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
	@if [ ! -f "$(ENV_FILE)" ] && [ -f "$(ENV_EXAMPLE_FILE)" ]; then \
		echo "$(YELLOW)No .env file found. Creating from .env.example...$(NC)"; \
		cp $(ENV_EXAMPLE_FILE) $(ENV_FILE); \
		echo "$(GREEN)[ok]$(NC) Created .env file from .env.example$(GREEN) ✔️$(NC)"; \
	elif [ ! -f "$(ENV_FILE)" ] && [ ! -f "$(ENV_EXAMPLE_FILE)" ]; then \
		echo "$(RED)[error]$(NC) Neither .env nor .env.example files found$(RED) ❌$(NC)"; \
		exit 1; \
	elif [ -f "$(ENV_FILE)" ]; then \
		read -t 10 -p "$(YELLOW).env file already exists. Overwrite with .env.example? [Y/n] (auto-yes in 10s)$(NC) " answer || answer="Y"; \
		answer=$${answer:-Y}; \
		if [[ $$answer =~ ^[Yy] ]]; then \
			cp $(ENV_EXAMPLE_FILE) $(ENV_FILE); \
			echo "$(GREEN)[ok]$(NC) Overwrote .env file with .env.example$(GREEN) ✔️$(NC)"; \
		else \
			echo "$(YELLOW)[skipped]$(NC) Kept existing .env file$(YELLOW) ⚠️$(NC)"; \
		fi; \
	fi

#-------------------------------------------------------
# SDK Quality Check Targets
#-------------------------------------------------------

.PHONY: check-references check-api-compatibility verify-sdk hooks

# Check that no lib-commons references appear in public packages
check-references:
	@echo "$(YELLOW)Checking for lib-commons references in public API...$(NC)"
	@! grep -r "lib-commons" --include="*.go" ./models ./entities | grep -v "//.*lib-commons" || (echo "$(RED)❌ Found lib-commons references in public API!$(NC)" && exit 1)
	@echo "$(GREEN)✅ No lib-commons references found in public API$(NC)"

# Verify that our refactoring doesn't break API compatibility
check-api-compatibility:
	@echo "$(YELLOW)Checking API compatibility...$(NC)"
	@go build ./models ./entities ./pkg/...
	@echo "$(GREEN)✅ API builds successfully$(NC)"

# Verify our implementation
verify-sdk: check-references check-api-compatibility
	@echo "$(GREEN)✅ All SDK quality checks passed!$(NC)"

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
	@echo "$(GREEN)[ok]$(NC) Coverage report generated successfully"

#-------------------------------------------------------
# Code Quality Commands
#-------------------------------------------------------

.PHONY: lint fmt tidy gosec

lint:
	$(call print_header,"Running linters")
	@if find . -name "*.go" -type f | grep -q .; then \
		if ! command -v $(GOLINT) > /dev/null; then \
			echo "$(YELLOW)Installing golangci-lint...$(NC)"; \
			go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
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
	@if ! command -v gosec > /dev/null; then \
		echo "$(YELLOW)Installing gosec...$(NC)"; \
		go install github.com/securego/gosec/v2/cmd/gosec@latest; \
	fi
	@echo "$(CYAN)Running gosec security scanner...$(NC)"
	@gosec -quiet ./...
	@echo "$(GREEN)[ok]$(NC) Security checks completed successfully$(GREEN) ✔️$(NC)"

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
# Example Commands
#-------------------------------------------------------

.PHONY: example

example:
	$(call print_header,"Running Complete Workflow Example")
	$(call print_header,"Make sure the Midaz Stack is running --default is localhost")
	@cp $(ENV_FILE) examples/workflow-with-entities/.env
	@cd examples/workflow-with-entities && go run main.go

.PHONY: demo-data

demo-data:
	$(call print_header,Running Mass Demo Data Generator)
	$(call print_header,Ensure Midaz services are running on localhost:3000 (onboarding) and :3001 (transaction))
	@if [ -f "$(ENV_FILE)" ]; then \
		cp $(ENV_FILE) examples/mass-demo-generator/.env; \
	fi
    @cd examples/mass-demo-generator && MIDAZ_DEBUG=true DEMO_NON_INTERACTIVE=1 go run main.go

#-------------------------------------------------------
# Documentation Commands
#-------------------------------------------------------

.PHONY: godoc godoc-static docs

godoc:
	$(call print_header,"Starting godoc server")
	@echo "$(CYAN)Starting godoc server at http://localhost:6060/pkg/github.com/LerianStudio/midaz-sdk-golang/$(NC)"
	@if ! command -v godoc > /dev/null; then \
		echo "$(YELLOW)Installing godoc...$(NC)"; \
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
	@echo "$(CYAN)Generating static documentation...$(NC)"
	@mkdir -p $(DOCS_DIR)
	@# Process each package
	@for pkg in $(PACKAGES) ; do \
		echo "$(CYAN)Generating documentation for $${pkg}...$(NC)" ; \
		pkg_path=$${pkg#github.com/LerianStudio/midaz-sdk-golang/} ; \
		if [ "$${pkg_path}" = "github.com/LerianStudio/midaz-sdk-golang" ]; then \
			pkg_path="." ; \
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
