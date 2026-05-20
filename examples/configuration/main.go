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

	"github.com/LerianStudio/midaz-sdk-golang/v3"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
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

	// Create a client with minimal configuration. WithAnonymous opts out of
	// auth so the example builds without credentials; v3 requires exactly
	// one auth source (Anonymous or AccessManager) at construction.
	c, err := midaz.New(midaz.WithAnonymous())
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

	// Local development environment. WithAnonymous keeps the example
	// constructible without credentials; in real staging/production code
	// you would replace it with WithAccessManager(...).
	localClient, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		return fmt.Errorf("failed to create local client: %w", err)
	}

	// Staging/Development configuration. Access Manager placeholders show the
	// production-shaped auth posture without requiring a live token endpoint for
	// this runnable example.
	stagingConfig, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentDevelopment),
		config.WithAccessManager(auth.AccessManager{Address: "https://auth.dev.midaz.io", ClientID: "<client-id>", ClientSecret: "<client-secret>"}),
	)
	if err != nil {
		return fmt.Errorf("failed to create staging config: %w", err)
	}

	// Production configuration uses Access Manager. Do not use WithAnonymous for
	// production unless an external gateway owns auth and the risk is explicit.
	productionConfig, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentProduction),
		config.WithAccessManager(auth.AccessManager{Address: "https://auth.midaz.io", ClientID: "<client-id>", ClientSecret: "<client-secret>"}),
	)
	if err != nil {
		return fmt.Errorf("failed to create production config: %w", err)
	}

	// Display the different URLs for each environment
	fmt.Println("Local Environment URLs:")
	fmt.Printf("  Onboarding: %s\n", localClient.GetConfig().ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("  Transaction: %s\n", localClient.GetConfig().ServiceURLs[config.ServiceTransaction])

	fmt.Println("Staging Environment URLs:")
	fmt.Printf("  Onboarding: %s\n", stagingConfig.ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("  Transaction: %s\n", stagingConfig.ServiceURLs[config.ServiceTransaction])

	fmt.Println("Production Environment URLs:")
	fmt.Printf("  Onboarding: %s\n", productionConfig.ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("  Transaction: %s\n", productionConfig.ServiceURLs[config.ServiceTransaction])
	fmt.Println()

	return nil
}

// setupConfigurationEnv installs the env vars used by configurationFromEnvironment.
// PLUGIN_AUTH_ENABLED + PLUGIN_AUTH_ADDRESS install the Access Manager auth source,
// satisfying v3's "exactly one auth source" invariant.
func setupConfigurationEnv() {
	envs := map[string]string{
		"PLUGIN_AUTH_ENABLED": "true",
		"PLUGIN_AUTH_ADDRESS": "http://localhost:4000",
		"MIDAZ_CLIENT_ID":     "1234567890",
		"MIDAZ_CLIENT_SECRET": "1234567890",
		"MIDAZ_ENVIRONMENT":   "development",
		"MIDAZ_DEBUG":         "true",
	}

	for k, v := range envs {
		if err := os.Setenv(k, v); err != nil {
			fmt.Printf("Error setting %s: %v\n", k, err)
		}
	}
}

// cleanupConfigurationEnv removes the env vars installed by setupConfigurationEnv.
// Errors from Unsetenv are logged for visibility but don't stop the demo.
func cleanupConfigurationEnv() {
	keys := []string{
		"PLUGIN_AUTH_ENABLED",
		"PLUGIN_AUTH_ADDRESS",
		"MIDAZ_CLIENT_ID",
		"MIDAZ_CLIENT_SECRET",
		"MIDAZ_ENVIRONMENT",
		"MIDAZ_DEBUG",
	}

	for _, k := range keys {
		if err := os.Unsetenv(k); err != nil {
			fmt.Printf("Warning: failed to unset %s: %v\n", k, err)
		}
	}
}

// configurationFromEnvironment demonstrates how to load configuration
// from environment variables.
func configurationFromEnvironment() error {
	fmt.Println("Example 3: Configuration from Environment Variables")
	fmt.Println("------------------------------------------------")

	// Set environment variables for demonstration.
	// In a real application, these would be set externally.
	setupConfigurationEnv()
	defer cleanupConfigurationEnv()

	// Create a config that explicitly loads configuration from environment variables.
	cfg, err := config.NewConfig(config.FromEnvironment())
	if err != nil {
		return fmt.Errorf("failed to load environment config: %w", err)
	}

	// Display the configuration loaded from environment variables. We do not
	// construct a client here because midaz.New eagerly fetches an Access Manager
	// token, and this example uses placeholder local credentials.
	pluginAuth := cfg.GetPluginAuth()
	fmt.Printf("Plugin Auth Enabled (from env): %t\n", pluginAuth.Enabled)
	fmt.Printf("Plugin Auth Address (from env): %s\n", pluginAuth.Address)
	fmt.Printf("Environment (from env): %s\n", cfg.Environment)
	fmt.Printf("Debug Mode (from env): %t\n", cfg.Debug)
	fmt.Printf("Onboarding URL: %s\n", cfg.ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Transaction URL: %s\n", cfg.ServiceURLs[config.ServiceTransaction])
	fmt.Println()

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
	c, err := midaz.New(
		midaz.WithHTTPClient(customClient),
		midaz.WithAnonymous(),
		midaz.WithTimeout(45*time.Second), // Can be redundant if set on HTTPClient
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
		config.WithAccessManager(auth.AccessManager{Address: "https://auth.midaz.io", ClientID: "<client-id>", ClientSecret: "<client-secret>"}),
		config.WithUserAgent("MyApp/1.0"),
		config.WithTimeout(45*time.Second),
		// Retry knobs are now individual single-concern Options. Chain them
		// instead of the v2 WithRetryConfig(int, dur, dur) macro.
		config.WithMaxRetries(3),
		config.WithRetryWaitMin(500*time.Millisecond),
		config.WithRetryWaitMax(5*time.Second),
		config.WithIdempotency(true),
	)
	if err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	// Display the comprehensive configuration settings without contacting the
	// placeholder Access Manager endpoint.
	fmt.Printf("Plugin Auth Enabled: %t\n", cfg.GetPluginAuth().Enabled)
	fmt.Printf("Environment: %s\n", cfg.Environment)
	fmt.Printf("User Agent: %s\n", cfg.UserAgent)
	fmt.Printf("Timeout: %s\n", cfg.Timeout)
	fmt.Printf("Max Retries: %d\n", cfg.MaxRetries)
	fmt.Printf("Retry Wait Min: %s\n", cfg.RetryWaitMin)
	fmt.Printf("Retry Wait Max: %s\n", cfg.RetryWaitMax)
	fmt.Printf("Enable Retries: %t\n", cfg.MaxRetries > 0)
	fmt.Printf("Debug Mode: %t\n", cfg.Debug)
	fmt.Printf("Enable Idempotency: %t\n", cfg.EnableIdempotency)
	fmt.Printf("Onboarding URL: %s\n", cfg.ServiceURLs[config.ServiceOnboarding])
	fmt.Printf("Transaction URL: %s\n", cfg.ServiceURLs[config.ServiceTransaction])
	fmt.Println()

	return nil
}
