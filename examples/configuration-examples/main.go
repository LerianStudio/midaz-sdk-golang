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

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
)

func main() {
	// For demonstration purposes only - in a real application, you would
	// use just one of these configuration approaches
	// Example 1: Basic configuration with functional options
	if err := basicConfiguration(); err != nil {
		log.Fatalf("Basic configuration failed: %v", err)
	}

	// Example 2: Environment-based configuration
	if err := environmentBasedConfiguration(); err != nil {
		log.Fatalf("Environment-based configuration failed: %v", err)
	}

	// Example 3: Configuration using environment variables
	if err := configurationFromEnvironment(); err != nil {
		log.Fatalf("Configuration from environment failed: %v", err)
	}

	// Example 4: Advanced configuration with custom HTTP settings
	if err := advancedHTTPConfiguration(); err != nil {
		log.Fatalf("Advanced HTTP configuration failed: %v", err)
	}

	// Example 5: Comprehensive configuration using Config package
	if err := comprehensiveConfiguration(); err != nil {
		log.Fatalf("Comprehensive configuration failed: %v", err)
	}
}

// basicConfiguration demonstrates the simplest way to configure the client
// with just the essential options.
func basicConfiguration() error {
	fmt.Println("Example 1: Basic Configuration")
	fmt.Println("-----------------------------")

	// Create a client with minimal configuration
	c, err := client.New(
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// In a real application, you would use the client here
	// Note: AuthToken is now stored internally and accessed through plugin auth configuration
	pluginAuth := c.GetConfig().GetPluginAuth()
	if pluginAuth.Enabled {
		fmt.Printf("Client created successfully with plugin auth at address: %s\n", pluginAuth.Address)
	} else {
		fmt.Printf("Client created successfully with no plugin auth enabled\n")
	}

	fmt.Printf("Using onboarding URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Using transaction URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceTransaction])
	fmt.Println()

	return nil
}

// environmentBasedConfiguration demonstrates how to configure the client
// for different deployment environments.
func environmentBasedConfiguration() error {
	fmt.Println("Example 2: Environment-Based Configuration")
	fmt.Println("----------------------------------------")

	// Local development environment
	localClient, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create local client: %w", err)
	}

	// Staging/Development environment
	stagingClient, err := client.New(
		client.WithEnvironment(config.EnvironmentDevelopment),
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create staging client: %w", err)
	}

	// Production environment
	productionClient, err := client.New(
		client.WithEnvironment(config.EnvironmentProduction),
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create production client: %w", err)
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

	return nil
}

// configurationFromEnvironment demonstrates how to load configuration
// from environment variables.
func configurationFromEnvironment() error {
	fmt.Println("Example 3: Configuration from Environment Variables")
	fmt.Println("------------------------------------------------")

	// Set environment variables for demonstration
	// In a real application, these would be set externally
	if err := os.Setenv("MIDAZ_CLIENT_ID", "1234567890"); err != nil {
		fmt.Printf("Error setting environment variable: %v\n", err)
	}

	if err := os.Setenv("MIDAZ_CLIENT_SECRET", "1234567890"); err != nil {
		fmt.Printf("Error setting environment variable: %v\n", err)
	}

	if err := os.Setenv("MIDAZ_ENVIRONMENT", "development"); err != nil {
		fmt.Printf("Error setting environment variable: %v\n", err)
	}

	if err := os.Setenv("MIDAZ_DEBUG", "true"); err != nil {
		fmt.Printf("Error setting environment variable: %v\n", err)
	}

	// Create a client that loads configuration from environment variables
	c, err := client.New(
		// No explicit configuration needed - it will be loaded from environment
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Display the configuration loaded from environment variables
	pluginAuth := c.GetConfig().GetPluginAuth()
	fmt.Printf("Plugin Auth Enabled (from env): %t\n", pluginAuth.Enabled)
	fmt.Printf("Plugin Auth Address (from env): %s\n", pluginAuth.Address)
	fmt.Printf("Environment (from env): %s\n", c.GetConfig().Environment)
	fmt.Printf("Debug Mode (from env): %t\n", c.GetConfig().Debug)
	fmt.Printf("Onboarding URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Transaction URL: %s\n", c.GetConfig().ServiceURLs[config.ServiceTransaction])
	fmt.Println()

	// Clean up environment variables after demonstration
	// Errors from Unsetenv are logged for visibility but don't stop the demo
	if err := os.Unsetenv("MIDAZ_CLIENT_ID"); err != nil {
		fmt.Printf("Warning: failed to unset MIDAZ_CLIENT_ID: %v\n", err)
	}

	if err := os.Unsetenv("MIDAZ_CLIENT_SECRET"); err != nil {
		fmt.Printf("Warning: failed to unset MIDAZ_CLIENT_SECRET: %v\n", err)
	}

	if err := os.Unsetenv("MIDAZ_ENVIRONMENT"); err != nil {
		fmt.Printf("Warning: failed to unset MIDAZ_ENVIRONMENT: %v\n", err)
	}

	if err := os.Unsetenv("MIDAZ_DEBUG"); err != nil {
		fmt.Printf("Warning: failed to unset MIDAZ_DEBUG: %v\n", err)
	}

	return nil
}

// advancedHTTPConfiguration demonstrates how to configure the client
// with custom HTTP settings.
func advancedHTTPConfiguration() error {
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
		client.WithHTTPClient(customClient),
		client.WithTimeout(45*time.Second), // Can be redundant if set on HTTPClient
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
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

	return nil
}

// comprehensiveConfiguration demonstrates how to use the Config package
// directly for advanced configuration scenarios.
func comprehensiveConfiguration() error {
	fmt.Println("Example 5: Comprehensive Configuration Using Config Package")
	fmt.Println("-------------------------------------------------------")

	// Create a configuration with extensive options
	cfg, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentProduction),
		config.WithUserAgent("MyApp/1.0"),
		config.WithTimeout(45*time.Second),
		config.WithRetryConfig(3, 500*time.Millisecond, 5*time.Second),
		config.WithDebug(true),
		config.WithIdempotency(true),
	)
	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Use the config in the client
	c, err := client.New(
		client.WithConfig(cfg),
		client.UseAllAPIs(),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Display the comprehensive configuration settings
	fmt.Printf("Plugin Auth Enabled: %t\n", c.GetConfig().GetPluginAuth().Enabled)
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

	return nil
}
