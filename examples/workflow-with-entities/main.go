// Package main provides a comprehensive example of using the Midaz Go SDK.
//
// # Complete Workflow Example
//
// This example demonstrates a complete workflow using the Midaz Go SDK, including:
// - Creating organizations
// - Creating ledgers
// - Creating assets
// - Creating accounts
// - Performing transactions
// - Creating segments and portfolios
// - Listing accounts
// - Updating and retrieving organizations
//
// # Server Requirements
//
// IMPORTANT: This example requires a running Midaz server and does NOT run in mock mode.
// To start the server locally, run the following command from the project root:
//
//	make up
//
// This will start all necessary services using Docker Compose. The server does not
// include an OAuth2 layer in local development mode, so you can use any string as
// the authentication token.
//
// # Environment Configuration
//
// The example uses the SDK's config package to load configuration from environment variables.
// You can set these variables in a .env file:
//
//	MIDAZ_AUTH_TOKEN=example-auth-token
//	MIDAZ_ENVIRONMENT=local  # Can be local, development, or production
//	MIDAZ_ONBOARDING_URL=http://localhost:3000/v1 # Optional override
//	MIDAZ_TRANSACTION_URL=http://localhost:3001/v1 # Optional override
//	MIDAZ_DEBUG=true # Optional, enables detailed API logging
//
// # Workflow Steps
//
// The example follows these steps:
// 1. Organization Creation - Creates a new organization
// 2. Ledger Creation - Creates a ledger within the organization
// 3. Asset Creation - Creates a USD asset
// 4. Account Creation - Creates customer and merchant accounts
// 5. Transaction Execution - Transfers funds between accounts using DSL
// 6. Segment Creation - Creates a segment for account categorization
// 7. Portfolio Creation - Creates a portfolio for account grouping
// 8. Account Listing - Lists all accounts in the ledger
// 9. Organization Update - Updates the organization details
// 10. Organization Retrieval - Retrieves the updated organization
//
// # Error Handling
//
// The example includes comprehensive error handling to demonstrate best practices
// for working with the Midaz API. Each step checks for errors and provides
// meaningful error messages to help with troubleshooting.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/examples/workflow-with-entities/pkg/workflows"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/joho/godotenv"
)

// main is the entry point for the example.
//
// This function performs the following steps:
// 1. Loads environment variables from a .env file (if present)
// 2. Validates required environment variables
// 3. Sets up default values for optional environment variables
// 4. Configures debug mode based on environment settings
// 5. Initializes the SDK client with appropriate configuration
// 6. Runs the complete workflow demonstration
//
// The function uses the godotenv package to load environment variables from a .env file,
// which makes it easier to configure the example without modifying the code.
func main() {
	loadEnvFile()

	if err := validateEnvironment(); err != nil {
		log.Fatalf("Environment validation failed: %v", err)
	}

	shutdownObservability := setupObservability()
	defer shutdownObservability()

	ctx, cancel := createWorkflowContext()
	defer cancel()

	cfg := createConfiguration()
	c := createSDKClient(cfg)

	concurrentCustomerToMerchantTxs, concurrentMerchantToCustomerTxs := loadConcurrencySettings()

	executeWorkflow(ctx, c, concurrentCustomerToMerchantTxs, concurrentMerchantToCustomerTxs)
}

func loadEnvFile() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: Failed to load .env file, using environment variables")
	}
}

type contextKey string

func createWorkflowContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	const traceIDKey contextKey = "trace_id"

	ctx = context.WithValue(ctx, traceIDKey, "workflow-example")

	return ctx, cancel
}

func createConfiguration() *config.Config {
	fmt.Println("Loading configuration from environment...")

	options := []config.Option{
		config.FromEnvironment(),
		config.WithEnvironment(config.EnvironmentLocal),
	}

	cfg, err := config.NewConfig(options...)
	if err != nil {
		log.Fatalf("Failed to create configuration: %v", err)
	}

	retryOpts := setupRetryOptions()
	cfg.MaxRetries = retryOpts.MaxRetries
	cfg.RetryWaitMin = retryOpts.InitialDelay
	cfg.RetryWaitMax = retryOpts.MaxDelay

	printConnectionInfo(cfg)

	return cfg
}

func printConnectionInfo(cfg *config.Config) {
	fmt.Printf("Connecting to Midaz APIs:\n")
	fmt.Printf("   - Onboarding API: %s\n", cfg.ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("   - Transaction API: %s\n", cfg.ServiceURLs[config.ServiceTransaction])
	fmt.Printf("   - Environment: %s\n", cfg.Environment)
	fmt.Printf("   - Debug mode: %t\n", cfg.Debug)
}

func createSDKClient(cfg *config.Config) *client.Client {
	fmt.Println("\nInitializing SDK client...")

	c, err := client.New(
		client.WithConfig(cfg),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("SDK client initialized successfully")

	return c
}

func loadConcurrencySettings() (int, int) {
	concurrentCustomerToMerchantTxs, err := getEnvInt("CONCURRENT_CUSTOMER_TO_MERCHANT_TXS", 10)
	if err != nil {
		log.Printf("Warning: Failed to parse CONCURRENT_CUSTOMER_TO_MERCHANT_TXS, using default: %v", err)
		concurrentCustomerToMerchantTxs = 10
	}

	concurrentMerchantToCustomerTxs, err := getEnvInt("CONCURRENT_MERCHANT_TO_CUSTOMER_TXS", 10)
	if err != nil {
		log.Printf("Warning: Failed to parse CONCURRENT_MERCHANT_TO_CUSTOMER_TXS, using default: %v", err)
		concurrentMerchantToCustomerTxs = 10
	}

	return concurrentCustomerToMerchantTxs, concurrentMerchantToCustomerTxs
}

func executeWorkflow(ctx context.Context, c *client.Client, customerToMerchant, merchantToCustomer int) {
	fmt.Println("\nStarting complete workflow...")

	if err := workflows.RunCompleteWorkflow(ctx, c.Entity, customerToMerchant, merchantToCustomer); err != nil {
		log.Fatalf("Workflow failed: %s", err.Error())
	}

	fmt.Println("\nWorkflow completed successfully!")
}

// getEnvInt gets an integer environment variable with a default value.
//
// Parameters:
//   - envVar: The name of the environment variable
//   - defaultValue: The default value to use if the environment variable is not set or invalid
//
// Returns:
//   - int: The parsed integer value
//   - error: Any error encountered during parsing
func getEnvInt(envVar string, defaultValue int) (int, error) {
	envValue := os.Getenv(envVar)
	if envValue == "" {
		return defaultValue, nil
	}

	intValue, err := strconv.Atoi(envValue)
	if err != nil {
		return defaultValue, fmt.Errorf("invalid value for %s: %w", envVar, err)
	}

	return intValue, nil
}

// validateEnvironment validates required environment variables
func validateEnvironment() error {
	requiredVars := []string{
		"MIDAZ_AUTH_TOKEN",
	}

	var missingVars []string

	for _, varName := range requiredVars {
		if os.Getenv(varName) == "" {
			missingVars = append(missingVars, varName)
		}
	}

	if len(missingVars) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missingVars)
	}

	// Use validation package to validate auth token format
	token := os.Getenv("MIDAZ_AUTH_TOKEN")
	if !isValidAuthToken(token) {
		return errors.New("invalid auth token format")
	}

	return nil
}

// setupObservability initializes the observability module
func setupObservability() func() {
	// Create a simple provider for observability with functional options
	obsProvider, err := observability.New(context.Background(),
		observability.WithServiceName("midaz-workflow-example"),
		observability.WithServiceVersion("1.0.0"),
		observability.WithEnvironment("local"),
		observability.WithComponentEnabled(true, true, true), // Enable tracing, metrics, and logging
	)
	if err != nil {
		log.Printf("Warning: Failed to create observability provider: %v", err)
		return func() {} // Return no-op shutdown function
	}

	// Return function to shut down observability when done
	return func() {
		if obsProvider != nil {
			if err := obsProvider.Shutdown(context.Background()); err != nil {
				log.Printf("Warning: Failed to shut down observability provider: %v", err)
			}
		}
	}
}

// setupRetryOptions configures retry behavior for API requests using functional options
func setupRetryOptions() *retry.Options {
	maxRetries, err := getEnvInt("MIDAZ_MAX_RETRIES", 3)
	if err != nil {
		log.Printf("Warning: Failed to parse MIDAZ_MAX_RETRIES, using default: %v", err)
	}

	// Create options with defaults
	options := retry.DefaultOptions()

	// Apply specific options
	if err := retry.WithMaxRetries(maxRetries)(options); err != nil {
		log.Printf("Warning: Failed to set max retries: %v", err)
	}

	if err := retry.WithInitialDelay(100 * time.Millisecond)(options); err != nil {
		log.Printf("Warning: Failed to set initial delay: %v", err)
	}

	if err := retry.WithMaxDelay(2 * time.Second)(options); err != nil {
		log.Printf("Warning: Failed to set max delay: %v", err)
	}

	if err := retry.WithBackoffFactor(2.0)(options); err != nil {
		log.Printf("Warning: Failed to set backoff factor: %v", err)
	}

	if err := retry.WithRetryableErrors(retry.DefaultRetryableErrors)(options); err != nil {
		log.Printf("Warning: Failed to set retryable errors: %v", err)
	}

	return options
}

// isValidAuthToken is a simple validation function for auth tokens
func isValidAuthToken(token string) bool {
	// Simple validation - token should be non-empty
	return token != ""
}
