# Makefile for the workflow-with-entities example
#
# This Makefile provides commands to easily run and test the workflow-with-entities
# example with proper environment configuration.

# Variables
EXAMPLE_NAME := workflow-with-entities
EXAMPLE_DIR := $(shell pwd)
SDK_ROOT_DIR := $(shell cd ../.. && pwd)
ENV_FILE := $(SDK_ROOT_DIR)/.env
ENV_EXAMPLE := $(SDK_ROOT_DIR)/.env.example

# No colors used

# Default values for concurrent transactions (if not specified in .env)
DEFAULT_CUSTOMER_TO_MERCHANT_TXS := 10
DEFAULT_MERCHANT_TO_CUSTOMER_TXS := 10

# Load environment variables if .env exists
ifneq (,$(wildcard $(ENV_FILE)))
    include $(ENV_FILE)
    export
    ENV_SOURCE := $(ENV_FILE)
else
    ENV_SOURCE := "none"
endif

# Help command
.PHONY: help
help:
	@echo ""
	@echo "Midaz Go SDK - Workflow Example Commands"
	@echo ""
	@echo "Basic Commands:"
	@echo "  make help          - Display this help message"
	@echo "  make setup-env     - Create .env file in SDK root directory from .env.example if it doesn't exist"
	@echo "  make run           - Run the workflow example"
	@echo "  make run-debug     - Run with verbose debug output"
	@echo "  make run-small     - Run with fewer concurrent transactions (faster)"
	@echo "  make run-large     - Run with many concurrent transactions (stress test)"
	@echo "  make clean         - Clean build artifacts"
	@echo ""
	@echo "Server Commands:"
	@echo "  make check-server  - Check if Midaz API servers are running"
	@echo ""
	@echo "Environment Variables (can be set in SDK root .env):"
	@echo "  MIDAZ_AUTH_TOKEN                  - Authentication token for the Midaz API"
	@echo "  MIDAZ_ENVIRONMENT                 - Environment to use (local, development, production)"
	@echo "  MIDAZ_ONBOARDING_URL              - URL for the Onboarding API"
	@echo "  MIDAZ_TRANSACTION_URL             - URL for the Transaction API"
	@echo "  MIDAZ_DEBUG                       - Enable debug output (true/false)"
	@echo "  CONCURRENT_CUSTOMER_TO_MERCHANT_TXS - Number of customer-to-merchant transactions to run"
	@echo "  CONCURRENT_MERCHANT_TO_CUSTOMER_TXS - Number of merchant-to-customer transactions to run"
	@echo ""
	@if [ "$(ENV_SOURCE)" = "none" ]; then \
		echo "WARNING: No .env file found. Run 'make setup-env' to create one."; \
	else \
		echo "INFO: Using environment from: $(ENV_SOURCE)"; \
	fi
	@echo ""

# Setup environment file in SDK root if it doesn't exist
.PHONY: setup-env
setup-env:
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "Creating $(ENV_FILE) from $(ENV_EXAMPLE)..."; \
		cp $(ENV_EXAMPLE) $(ENV_FILE); \
		echo "Created .env file in SDK root directory."; \
		echo "Adding example-specific environment variables..."; \
		echo "" >> $(ENV_FILE); \
		echo "# Example-specific configuration" >> $(ENV_FILE); \
		echo "CONCURRENT_CUSTOMER_TO_MERCHANT_TXS=100" >> $(ENV_FILE); \
		echo "CONCURRENT_MERCHANT_TO_CUSTOMER_TXS=200" >> $(ENV_FILE); \
		echo "Added example-specific variables. You may want to edit it to customize settings."; \
	else \
		echo "SDK root .env file already exists."; \
		if ! grep -q "CONCURRENT_CUSTOMER_TO_MERCHANT_TXS" $(ENV_FILE); then \
			echo "Adding example-specific environment variables..."; \
			echo "" >> $(ENV_FILE); \
			echo "# Example-specific configuration" >> $(ENV_FILE); \
			echo "CONCURRENT_CUSTOMER_TO_MERCHANT_TXS=100" >> $(ENV_FILE); \
			echo "CONCURRENT_MERCHANT_TO_CUSTOMER_TXS=200" >> $(ENV_FILE); \
			echo "Added example-specific variables."; \
		fi \
	fi

# Check if required servers are running
.PHONY: check-server
check-server:
	@echo "Checking if Midaz API servers are running..."
	@curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/v1/healthz > /dev/null 2>&1 || \
		{ echo "ERROR: Onboarding API server is not running at http://localhost:3000/v1"; \
		  echo "Run 'make up' from the project root to start the servers."; exit 1; }
	@curl -s -o /dev/null -w "%{http_code}" http://localhost:3001/v1/healthz > /dev/null 2>&1 || \
		{ echo "ERROR: Transaction API server is not running at http://localhost:3001/v1"; \
		  echo "Run 'make up' from the project root to start the servers."; exit 1; }
	@echo "Midaz API servers are running."

# Build the example
.PHONY: build
build:
	@echo "Building workflow example..."
	@cd $(EXAMPLE_DIR) && go build -o $(EXAMPLE_NAME)
	@echo "Build complete."

# Clean artifacts
.PHONY: clean
clean:
	@echo "Cleaning artifacts..."
	@rm -f $(EXAMPLE_DIR)/$(EXAMPLE_NAME)
	@echo "Clean complete."

# Run the example with default settings
.PHONY: run
run: setup-env check-server
	@echo "Running workflow example..."
	@cd $(EXAMPLE_DIR) && go run main.go
	@echo "Workflow example complete."

# Run with debug output
.PHONY: run-debug
run-debug: setup-env check-server
	@echo "Running workflow example with debug output..."
	@cd $(EXAMPLE_DIR) && MIDAZ_DEBUG=true go run main.go
	@echo "Workflow example complete."

# Run with smaller transaction counts (faster)
.PHONY: run-small
run-small: setup-env check-server
	@echo "Running workflow example with small transaction counts..."
	@cd $(EXAMPLE_DIR) && CONCURRENT_CUSTOMER_TO_MERCHANT_TXS=5 CONCURRENT_MERCHANT_TO_CUSTOMER_TXS=5 go run main.go
	@echo "Workflow example complete."

# Run with larger transaction counts (stress test)
.PHONY: run-large
run-large: setup-env check-server
	@echo "Running workflow example with large transaction counts..."
	@cd $(EXAMPLE_DIR) && CONCURRENT_CUSTOMER_TO_MERCHANT_TXS=200 CONCURRENT_MERCHANT_TO_CUSTOMER_TXS=300 go run main.go
	@echo "Workflow example complete."

# Default target
.DEFAULT_GOAL := help