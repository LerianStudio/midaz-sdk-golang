// Package main provides examples of using retry mechanisms in the Midaz SDK.
//
// This example demonstrates the following:
// 1. Configuring retry settings with backoff
// 2. Using custom retry policies
// 3. Handling retryable errors
// 4. Disabling retries
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

func main() {
	fmt.Println("Retry Mechanism Examples")
	fmt.Println("=======================")

	// Example 1: Using default retry settings
	defaultRetryExample()

	// Example 2: Configuring custom retry settings
	customRetryConfigExample()

	// Example 3: Using a custom retry policy
	customRetryPolicyExample()

	// Example 4: Disabling retries
	disableRetriesExample()
}

// defaultRetryExample demonstrates using the default retry settings.
func defaultRetryExample() {
	fmt.Println("\nExample 1: Using default retry settings")
	fmt.Println("-------------------------------------")

	// Create a client with default retry settings
	c, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// By default, the SDK will retry up to 3 times with exponential backoff
	fmt.Println("Default settings:")
	fmt.Printf("  - Max retries: %d\n", c.GetConfig().MaxRetries)
	fmt.Printf("  - Min wait: %v\n", c.GetConfig().RetryWaitMin)
	fmt.Printf("  - Max wait: %v\n", c.GetConfig().RetryWaitMax)
	fmt.Printf("  - Retries enabled: %v\n", c.GetConfig().EnableRetries)

	// In production code, the retry mechanism automatically handles:
	// - Network errors (connection timeouts, DNS issues)
	// - Server errors (500, 502, 503, 504)
	// - Rate limiting (429 Too Many Requests)
	fmt.Println("\nThe retry mechanism will automatically handle:")
	fmt.Println("  - Network errors (connection issues)")
	fmt.Println("  - Server errors (500, 502, 503, 504)")
	fmt.Println("  - Rate limiting (429 Too Many Requests)")

	// Example call that would be retried if it failed with a retryable error
	fmt.Println("\nExample call that would be retried if it failed:")
	_, err = c.Entity.Organizations.GetOrganization(context.Background(), "org-id")
	fmt.Printf("Result: %v\n", err)
}

// customRetryConfigExample demonstrates configuring custom retry settings.
func customRetryConfigExample() {
	fmt.Println("\nExample 2: Configuring custom retry settings")
	fmt.Println("------------------------------------------")

	// Create a client with custom retry settings
	c, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		// Configure a more aggressive retry strategy
		client.WithRetries(5, 200*time.Millisecond, 10*time.Second),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Show the custom settings
	fmt.Println("Custom retry settings:")
	fmt.Printf("  - Max retries: %d\n", c.GetConfig().MaxRetries)
	fmt.Printf("  - Min wait: %v\n", c.GetConfig().RetryWaitMin)
	fmt.Printf("  - Max wait: %v\n", c.GetConfig().RetryWaitMax)

	// Example call with custom retry settings
	fmt.Println("\nExample call with custom retry settings:")
	_, err = c.Entity.Organizations.GetOrganization(context.Background(), "org-id")
	fmt.Printf("Result: %v\n", err)
}

// customRetryPolicyExample demonstrates using a custom retry policy.
func customRetryPolicyExample() {
	fmt.Println("\nExample 3: Using a custom retry policy")
	fmt.Println("------------------------------------")

	// Define a custom retry policy function
	customShouldRetry := func(resp *http.Response, err error) bool {
		// Retry on network errors
		if err != nil {
			fmt.Println("  - Custom policy: Retrying network error")
			return true
		}

		// Only retry on 500 and 503 status codes
		if resp != nil {
			if resp.StatusCode == http.StatusInternalServerError ||
				resp.StatusCode == http.StatusServiceUnavailable {
				fmt.Printf("  - Custom policy: Retrying status %d\n", resp.StatusCode)
				return true
			}
			fmt.Printf("  - Custom policy: Not retrying status %d\n", resp.StatusCode)
		}

		return false
	}

	// Create a client with the custom retry policy
	c, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		client.WithRetries(3, 100*time.Millisecond, 1*time.Second),
		client.WithCustomRetryPolicy(customShouldRetry),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("Custom retry policy configured to retry only on:")
	fmt.Println("  - Network errors")
	fmt.Println("  - 500 Internal Server Error")
	fmt.Println("  - 503 Service Unavailable")

	// Example call with custom retry policy
	fmt.Println("\nExample call with custom retry policy:")
	_, err = c.Entity.Organizations.GetOrganization(context.Background(), "org-id")
	fmt.Printf("Result: %v\n", err)
}

// disableRetriesExample demonstrates disabling retries.
func disableRetriesExample() {
	fmt.Println("\nExample 4: Disabling retries")
	fmt.Println("-------------------------")

	// Create a client with retries disabled
	c, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		client.DisableRetries(),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Show retry settings
	fmt.Println("Retry settings:")
	fmt.Printf("  - Max retries: %d\n", c.GetConfig().MaxRetries)
	fmt.Printf("  - Retries enabled: %v\n", c.GetConfig().EnableRetries)

	// Example call with retries disabled
	fmt.Println("\nExample call with retries disabled:")
	_, err = c.Entity.Organizations.GetOrganization(context.Background(), "org-id")

	// Handle the error without retries
	if err != nil {
		if errors.IsNetworkError(err) {
			fmt.Println("  - Network error, would not be retried")
		} else if errors.IsTimeoutError(err) {
			fmt.Println("  - Timeout error, would not be retried")
		} else {
			fmt.Printf("  - Other error: %v\n", err)
		}
	} else {
		fmt.Println("  - Operation completed successfully")
	}
}
