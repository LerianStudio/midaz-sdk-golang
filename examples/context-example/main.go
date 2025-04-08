// Package main provides an example of how to use context for timeout and cancellation in the Midaz SDK.
//
// This example demonstrates the following:
// 1. Setting a timeout on API operations
// 2. Manually cancelling operations
// 3. Handling context cancellation errors
// 4. Using the client's WithContext method for operation groups
// 5. Proper resource cleanup on cancellation
// 6. Real-world API call cancellation
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

func main() {
	fmt.Println("Context Propagation Examples")
	fmt.Println("===========================")

	// Create a client with a default auth token for examples
	c, err := client.New(
		client.WithAuthToken("test-token"),
		client.WithEnvironment(config.EnvironmentLocal),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example 1: Using a timeout context
	timeoutExample(c)

	// Example 2: Manually cancelling operations
	cancellationExample(c)

	// Example 3: Using WithContext for operation groups
	operationGroupExample(c)

	// Example 4: Proper resource cleanup
	resourceCleanupExample(c)

	// Example 5: Real-world API call cancellation
	realWorldCancellationExample(c)
}

// timeoutExample demonstrates how to set a timeout on API operations.
func timeoutExample(c *client.Client) {
	fmt.Println("\nExample 1: Using a timeout context")
	fmt.Println("----------------------------------")

	// Create a context with a 100ms timeout (intentionally short for the example)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel() // Always call cancel to release resources

	// Simulate an operation that takes longer than the timeout
	// In a real app, this would be an API call to the Midaz service
	go func() {
		time.Sleep(200 * time.Millisecond) // Simulate slow operation
		fmt.Println("Operation would have completed now, but already timed out")
	}()

	// Attempt to call an API with the timeout context
	fmt.Println("Starting operation with a 100ms timeout...")
	_, err := c.Entity.Organizations.GetOrganization(ctx, "org-id")

	// Handle the timeout error
	handleContextError(err)
}

// cancellationExample demonstrates how to manually cancel operations.
func cancellationExample(c *client.Client) {
	fmt.Println("\nExample 2: Manually cancelling operations")
	fmt.Println("----------------------------------------")

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine to cancel the context after a delay
	go func() {
		time.Sleep(50 * time.Millisecond) // Wait a bit
		fmt.Println("Cancelling the operation manually...")
		cancel() // Cancel the operation
	}()

	// Attempt to call an API with the context that will be cancelled
	fmt.Println("Starting operation that will be cancelled...")
	_, err := c.Entity.Organizations.GetOrganization(ctx, "org-id")

	// Handle the cancellation error
	handleContextError(err)
}

// operationGroupExample demonstrates using WithContext for multiple operations.
func operationGroupExample(c *client.Client) {
	fmt.Println("\nExample 3: Using context for operation groups")
	fmt.Println("-----------------------------------------------")

	// Create a context with a deadline
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create a new client with the timeout context
	timeoutClient, err := client.New(
		client.WithAuthToken("test-token"),
		client.WithEnvironment(config.EnvironmentLocal),
		client.WithContext(ctx), // Set the context on the client
		client.UseAllAPIs(),
	)
	if err != nil {
		fmt.Printf("Failed to create client with context: %v\n", err)
		return
	}

	// Now all operations on timeoutClient will use the timeout context
	fmt.Println("Creating a client with a 2 second timeout for all operations")
	fmt.Println("All operations with this client will respect the timeout")

	// Example operations (would typically be API calls)
	fmt.Println("First operation using the timeout client")
	_, err1 := timeoutClient.Entity.Organizations.GetOrganization(ctx, "org-id")
	handleContextError(err1)

	fmt.Println("Second operation using the timeout client")
	_, err2 := timeoutClient.Entity.Ledgers.GetLedger(ctx, "org-id", "ledger-id")
	handleContextError(err2)

	// You can also override the client context for specific operations
	fmt.Println("Custom context for a specific operation")
	customCtx, customCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer customCancel()
	_, err3 := timeoutClient.Entity.Organizations.GetOrganization(customCtx, "org-id")
	handleContextError(err3)
}

// resourceCleanupExample demonstrates proper resource cleanup on cancellation.
func resourceCleanupExample(c *client.Client) {
	fmt.Println("\nExample 4: Proper resource cleanup")
	fmt.Println("----------------------------------")

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate resource acquisition
	fmt.Println("Acquiring resources...")

	// Set up cleanup to run on cancellation
	go func() {
		<-ctx.Done() // Wait for cancellation
		fmt.Println("Context cancelled, cleaning up resources...")
		// In a real app, this would release connections, close files, etc.
		fmt.Println("Resources cleaned up")
	}()

	// Cancel the context after a delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Cancelling context...")
		cancel()
	}()

	// Attempt to perform an operation
	fmt.Println("Performing operation...")
	_, err := c.Entity.Organizations.GetOrganization(ctx, "org-id")
	handleContextError(err)

	// Wait a bit to see the cleanup happen
	time.Sleep(150 * time.Millisecond)
}

// realWorldCancellationExample demonstrates a real-world API call with cancellation.
func realWorldCancellationExample(c *client.Client) {
	fmt.Println("\nExample 5: Real-world API call cancellation")
	fmt.Println("------------------------------------------")

	// Create a context with a deadline for a batch operation
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Set up a channel to receive results from multiple concurrent operations
	resultChan := make(chan struct {
		result *models.Account
		err    error
	})

	// Start multiple concurrent operations
	fmt.Println("Starting multiple concurrent operations...")

	// Operation 1: Create an account (will likely timeout)
	go func() {
		account, err := c.Entity.Accounts.CreateAccount(
			ctx,
			"org-id",
			"ledger-id",
			&models.CreateAccountInput{
				Name:      "Test Account",
				AssetCode: "USD",
				Type:      "ASSET",
			},
		)
		resultChan <- struct {
			result *models.Account
			err    error
		}{account, err}
	}()

	// Operation 2: Get an existing account
	go func() {
		time.Sleep(100 * time.Millisecond) // Simulate some processing
		account, err := c.Entity.Accounts.GetAccount(
			ctx,
			"org-id",
			"ledger-id",
			"account-id",
		)
		resultChan <- struct {
			result *models.Account
			err    error
		}{account, err}
	}()

	// Wait for results with a timeout
	var results int
	timeout := time.After(600 * time.Millisecond)

	for results < 2 {
		select {
		case result := <-resultChan:
			results++
			if result.err != nil {
				fmt.Printf("Operation %d result: ", results)
				handleContextError(result.err)
			} else {
				fmt.Printf("Operation %d succeeded\n", results)
			}
		case <-timeout:
			fmt.Println("Timed out waiting for all results")
			return
		}
	}

	fmt.Println("All operations completed (with or without errors)")
}

// handleContextError demonstrates how to properly handle context-related errors.
func handleContextError(err error) {
	if err == nil {
		fmt.Println("Operation completed successfully")
		return
	}

	switch {
	case errors.IsCancellationError(err):
		fmt.Printf("Operation was cancelled: %v\n", err)
	case errors.IsTimeoutError(err):
		fmt.Printf("Operation timed out: %v\n", err)
	case errors.IsNetworkError(err):
		fmt.Printf("Network error: %v\n", err)
	default:
		fmt.Printf("Other error: %v\n", err)
	}
}
