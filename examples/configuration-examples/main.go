// Package main provides examples of different configuration approaches for the Midaz Go SDK.
//
// This example demonstrates various ways to configure the Midaz SDK client, including:
// - Basic configuration with functional options
// - Environment-based configuration for different deployment environments
// - Configuration using environment variables
// - Advanced configuration with custom HTTP settings
// - Comprehensive configuration using the Config package directly
//
// The examples are designed to be educational and show the flexibility of the SDK's
// configuration system. They demonstrate how to handle different scenarios and
// deployment environments effectively.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

func main() {
	// For demonstration purposes only - in a real application, you would
	// use just one of these configuration approaches

	// Example 1: Basic configuration with functional options
	basicConfiguration()

	// Example 2: Environment-based configuration
	environmentBasedConfiguration()

	// Example 3: Configuration using environment variables
	configurationFromEnvironment()

	// Example 4: Advanced configuration with custom HTTP settings
	advancedHttpConfiguration()

	// Example 5: Comprehensive configuration using Config package
	comprehensiveConfiguration()
}

// basicConfiguration demonstrates the simplest way to configure the client
// with just the essential options.
func basicConfiguration() {
	fmt.Println("Example 1: Basic Configuration")
	fmt.Println("-----------------------------")

	// Create a client with minimal configuration
	c, err := client.New(
		client.WithAuthToken("example-token"),
		client.UseAllAPIs(),
	)

	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// In a real application, you would use the client here
	fmt.Printf("Client created successfully with auth token: %s\n", c.GetConfig().AuthToken)
	fmt.Printf("Using onboarding URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Using transaction URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceTransaction])
	fmt.Println()
}

// environmentBasedConfiguration demonstrates how to configure the client
// for different deployment environments.
func environmentBasedConfiguration() {
	fmt.Println("Example 2: Environment-Based Configuration")
	fmt.Println("----------------------------------------")

	// Local development environment
	localClient, err := client.New(
		client.WithAuthToken("local-token"),
		client.WithEnvironment(config.EnvironmentLocal),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create local client: %v", err)
	}

	// Staging/Development environment
	stagingClient, err := client.New(
		client.WithAuthToken("staging-token"),
		client.WithEnvironment(config.EnvironmentDevelopment),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create staging client: %v", err)
	}

	// Production environment
	productionClient, err := client.New(
		client.WithAuthToken("production-token"),
		client.WithEnvironment(config.EnvironmentProduction),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create production client: %v", err)
	}

	// Display the different URLs for each environment
	fmt.Println("Local Environment URLs:")
	fmt.Printf("  Onboarding: %s\n", localClient.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("  Transaction: %s\n", localClient.GetConfig().ServiceURLs[config.ServiceTransaction])

	fmt.Println("Staging Environment URLs:")
	fmt.Printf("  Onboarding: %s\n", stagingClient.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("  Transaction: %s\n", stagingClient.GetConfig().ServiceURLs[config.ServiceTransaction])

	fmt.Println("Production Environment URLs:")
	fmt.Printf("  Onboarding: %s\n", productionClient.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("  Transaction: %s\n", productionClient.GetConfig().ServiceURLs[config.ServiceTransaction])
	fmt.Println()
}

// configurationFromEnvironment demonstrates how to load configuration
// from environment variables.
func configurationFromEnvironment() {
	fmt.Println("Example 3: Configuration from Environment Variables")
	fmt.Println("------------------------------------------------")

	// Set environment variables for demonstration
	// In a real application, these would be set externally
	os.Setenv("MIDAZ_AUTH_TOKEN", "env-token")
	os.Setenv("MIDAZ_ENVIRONMENT", "development")
	os.Setenv("MIDAZ_DEBUG", "true")

	// Create a client that loads configuration from environment variables
	c, err := client.New(
		// No explicit configuration needed - it will be loaded from environment
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Display the configuration loaded from environment variables
	fmt.Printf("Auth Token (from env): %s\n", c.GetConfig().AuthToken)
	fmt.Printf("Environment (from env): %s\n", c.GetConfig().Environment)
	fmt.Printf("Debug Mode (from env): %t\n", c.GetConfig().Debug)
	fmt.Printf("Onboarding URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Transaction URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceTransaction])
	fmt.Println()

	// Clean up environment variables after demonstration
	os.Unsetenv("MIDAZ_AUTH_TOKEN")
	os.Unsetenv("MIDAZ_ENVIRONMENT")
	os.Unsetenv("MIDAZ_DEBUG")
}

// advancedHttpConfiguration demonstrates how to configure the client
// with custom HTTP settings.
func advancedHttpConfiguration() {
	fmt.Println("Example 4: Advanced HTTP Configuration")
	fmt.Println("------------------------------------")

	// Create a custom transport with specific settings
	customTransport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// Create a custom HTTP client with the transport
	customClient := &http.Client{
		Transport: customTransport,
		Timeout:   45 * time.Second,
	}

	// Create a client with the custom HTTP client
	c, err := client.New(
		client.WithAuthToken("http-token"),
		client.WithHTTPClient(customClient),
		client.WithTimeout(45*time.Second), // Can be redundant if set on HTTPClient
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Display the HTTP client settings
	fmt.Printf("HTTP Client Timeout: %s\n", c.GetConfig().HTTPClient.Timeout)
	fmt.Println()

	// Example usage with the configured client
	ctx := context.Background()
	fmt.Printf("Client ready for API calls with timeout of %s\n", c.GetConfig().HTTPClient.Timeout)

	// In a real application, you would make API calls here
	_ = ctx // Using ctx to avoid unused variable warning
	fmt.Println()
}

// comprehensiveConfiguration demonstrates how to use the Config package
// directly for advanced configuration scenarios.
func comprehensiveConfiguration() {
	fmt.Println("Example 5: Comprehensive Configuration Using Config Package")
	fmt.Println("-------------------------------------------------------")

	// Create a configuration with extensive options
	cfg, err := config.NewConfig(
		config.WithAuthToken("advanced-token"),
		config.WithEnvironment(config.EnvironmentProduction),
		config.WithUserAgent("MyApp/1.0"),
		config.WithTimeout(45*time.Second),
		config.WithRetryConfig(3, 500*time.Millisecond, 5*time.Second),
		config.WithDebug(true),
		config.WithIdempotency(true),
	)
	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	// Use the config in the client
	c, err := client.New(
		client.WithConfig(cfg),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Display the comprehensive configuration settings
	fmt.Printf("Auth Token: %s\n", c.GetConfig().AuthToken)
	fmt.Printf("Environment: %s\n", c.GetConfig().Environment)
	fmt.Printf("User Agent: %s\n", c.GetConfig().UserAgent)
	fmt.Printf("Timeout: %s\n", c.GetConfig().Timeout)
	fmt.Printf("Max Retries: %d\n", c.GetConfig().MaxRetries)
	fmt.Printf("Retry Wait Min: %s\n", c.GetConfig().RetryWaitMin)
	fmt.Printf("Retry Wait Max: %s\n", c.GetConfig().RetryWaitMax)
	fmt.Printf("Enable Retries: %t\n", c.GetConfig().EnableRetries)
	fmt.Printf("Debug Mode: %t\n", c.GetConfig().Debug)
	fmt.Printf("Enable Idempotency: %t\n", c.GetConfig().EnableIdempotency)
	fmt.Printf("Onboarding URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Transaction URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceTransaction])
	fmt.Println()
}
