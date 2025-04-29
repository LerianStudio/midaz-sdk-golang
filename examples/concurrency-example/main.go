// Package main provides examples of using concurrency helpers in the Midaz SDK.
//
// This example demonstrates the following:
// 1. Using worker pools for parallel processing
// 2. Processing items in batches
// 3. Using forEach for simple parallel operations
// 4. Rate limiting API calls
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"sync/atomic"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/config"
)

func main() {
	fmt.Println("Concurrency Helpers Examples")
	fmt.Println("===========================")

	// Create a client for use in examples
	c, err := client.New(
		client.WithEnvironment(config.EnvironmentLocal),
		client.UseAllAPIs(),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Example 1: Worker Pool for Parallel Processing
	workerPoolExample(c)

	// Example 2: Batch Processing
	batchProcessingExample(c)

	// Example 3: ForEach for Simple Operations
	forEachExample(c)

	// Example 4: Rate Limiting
	rateLimitingExample(c)
}

// workerPoolExample demonstrates using a worker pool to process items in parallel.
func workerPoolExample(_ *client.Client) {
	fmt.Println("\nExample 1: Worker Pool for Parallel Processing")
	fmt.Println("-------------------------------------------")

	// Create a list of account IDs to process
	accountIDs := []string{
		"account-1", "account-2", "account-3", "account-4", "account-5",
		"account-6", "account-7", "account-8", "account-9", "account-10",
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Printf("Processing %d accounts in parallel\n", len(accountIDs))

	// Use a worker pool to fetch account details in parallel
	startTime := time.Now()
	results := concurrent.WorkerPool(
		ctx,
		accountIDs,
		func(ctx context.Context, accountID string) (*models.Account, error) {
			// Simulate API call to get account details
			// In a real application, this would call the Midaz API
			time.Sleep(200 * time.Millisecond) // Simulate network delay

			// Simulate some failures to demonstrate error handling
			// Use crypto/rand for secure random number generation
			randomNum, err := rand.Int(rand.Reader, big.NewInt(10))
			if err != nil {
				log.Printf("Error generating random number: %v", err)
				// Fallback to a simple deterministic approach if random fails
				return &models.Account{
					ID:   accountID,
					Name: fmt.Sprintf("Account %s", accountID),
				}, nil
			}

			if randomNum.Int64() < 2 { // 20% chance of failure
				return nil, fmt.Errorf("failed to get account %s", accountID)
			}

			// Return mock account data
			return &models.Account{
				ID:   accountID,
				Name: fmt.Sprintf("Account %s", accountID),
			}, nil
		},
		// Configure the worker pool
		concurrent.WithWorkers(5),                  // Use 5 workers
		concurrent.WithBufferSize(len(accountIDs)), // Buffer all items
	)
	elapsed := time.Since(startTime)

	fmt.Printf("Processed %d accounts in %v\n", len(results), elapsed)

	// Process the results
	var successCount, errorCount int
	for _, result := range results {
		if result.Error != nil {
			errorCount++
			fmt.Printf("Error processing account %s: %v\n", result.Item, result.Error)
		} else {
			successCount++
			// In a real application, we would do something with the account data
			// fmt.Printf("Successfully processed account %s\n", result.Value.ID)
		}
	}

	fmt.Printf("Success: %d, Errors: %d\n", successCount, errorCount)

	// Compare to sequential processing
	fmt.Println("\nComparing to sequential processing:")
	startTime = time.Now()
	for range accountIDs {
		// Simulate API call
		time.Sleep(200 * time.Millisecond)
	}
	sequentialElapsed := time.Since(startTime)

	fmt.Printf("Sequential: %v, Parallel: %v, Speedup: %.2fx\n",
		sequentialElapsed, elapsed, float64(sequentialElapsed)/float64(elapsed))
}

// batchProcessingExample demonstrates processing items in batches.
func batchProcessingExample(_ *client.Client) {
	fmt.Println("\nExample 2: Batch Processing")
	fmt.Println("-------------------------")

	// Create a list of transaction IDs to process
	transactionIDs := make([]string, 50)
	for i := range transactionIDs {
		transactionIDs[i] = fmt.Sprintf("tx-%d", i+1)
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Processing %d transactions in batches\n", len(transactionIDs))

	// Use batch processing to process transactions in groups
	startTime := time.Now()
	results := concurrent.Batch(
		ctx,
		transactionIDs,
		10, // Process 10 transactions per batch
		func(ctx context.Context, batch []string) ([]string, error) {
			// Simulate a batch API call to process transactions
			// In a real application, this would call the Midaz API
			time.Sleep(300 * time.Millisecond) // Simulate network delay

			// Process the batch and return results
			processedIDs := make([]string, len(batch))
			for i, txID := range batch {
				processedIDs[i] = fmt.Sprintf("processed-%s", txID)
			}

			return processedIDs, nil
		},
		// Configure the worker pool for batch processing
		concurrent.WithWorkers(3), // Process 3 batches concurrently
	)
	elapsed := time.Since(startTime)

	fmt.Printf("Processed %d transactions in %d batches in %v\n",
		len(results), (len(transactionIDs)+9)/10, elapsed)

	// Process the results
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	fmt.Printf("Successfully processed %d transactions\n", successCount)

	// Explain the benefits
	fmt.Println("\nBenefits of batch processing:")
	fmt.Println("1. Reduces API call overhead by grouping requests")
	fmt.Println("2. Respects API rate limits while maximizing throughput")
	fmt.Println("3. Processes large datasets efficiently")
}

// forEachExample demonstrates using forEach for simple parallel operations.
func forEachExample(_ *client.Client) {
	fmt.Println("\nExample 3: ForEach for Simple Operations")
	fmt.Println("--------------------------------------")

	// Create a list of portfolio IDs to update
	portfolioIDs := []string{
		"portfolio-1", "portfolio-2", "portfolio-3", "portfolio-4", "portfolio-5",
		"portfolio-6", "portfolio-7", "portfolio-8", "portfolio-9", "portfolio-10",
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Printf("Updating %d portfolios in parallel\n", len(portfolioIDs))

	// Track processed items
	var processedCount int32

	// Use forEach to update portfolios in parallel
	startTime := time.Now()
	err := concurrent.ForEach(
		ctx,
		portfolioIDs,
		func(ctx context.Context, portfolioID string) error {
			// Simulate API call to update a portfolio
			// In a real application, this would call the Midaz API
			time.Sleep(150 * time.Millisecond) // Simulate network delay

			// Simulate a random failure
			randomNum, err := rand.Int(rand.Reader, big.NewInt(10))
			if err != nil {
				log.Printf("Error generating random number: %v", err)
				// Fallback to a simple deterministic approach if random fails
				return nil
			}
			if randomNum.Int64() < 1 { // 10% chance of failure
				return fmt.Errorf("failed to update portfolio %s", portfolioID)
			}

			// Increment processed count
			atomic.AddInt32(&processedCount, 1)
			return nil
		},
		// Configure the worker pool
		concurrent.WithWorkers(4), // Use 4 workers
	)
	elapsed := time.Since(startTime)

	fmt.Printf("Processed %d portfolios in %v\n", atomic.LoadInt32(&processedCount), elapsed)

	// Check for errors
	if err != nil {
		fmt.Printf("Error occurred: %v\n", err)
	} else {
		fmt.Println("All portfolios updated successfully")
	}

	// Explain the benefits
	fmt.Println("\nBenefits of forEach:")
	fmt.Println("1. Simpler API for fire-and-forget operations")
	fmt.Println("2. Stops on first error for fail-fast behavior")
	fmt.Println("3. No need to collect and process results when not needed")
}

// rateLimitingExample demonstrates using a rate limiter to control API call frequency.
func rateLimitingExample(_ *client.Client) {
	fmt.Println("\nExample 4: Rate Limiting")
	fmt.Println("---------------------")

	// Create a rate limiter for 5 operations per second
	rateLimiter := concurrent.NewRateLimiter(5, 5) // 5 ops/sec, burst of 5
	defer rateLimiter.Stop()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a list of operations to perform
	numOperations := 20
	fmt.Printf("Performing %d operations at a rate of 5 per second\n", numOperations)

	// Track completed operations
	var completedCount int32

	// Run operations with rate limiting
	startTime := time.Now()

	// Launch operations in parallel
	results := concurrent.WorkerPool(
		ctx,
		make([]int, numOperations), // Just need numOperations items
		func(ctx context.Context, _ int) (time.Time, error) {
			// Wait for a rate limiter token before proceeding
			if err := rateLimiter.Wait(ctx); err != nil {
				return time.Time{}, err
			}

			// Simulate API call
			// In a real application, this would call the Midaz API
			time.Sleep(50 * time.Millisecond) // Simulate processing time

			// Increment completed count
			atomic.AddInt32(&completedCount, 1)

			// Return completion time for analysis
			return time.Now(), nil
		},
		concurrent.WithWorkers(10),        // More workers than our rate limit
		concurrent.WithUnorderedResults(), // Get results as they complete
	)
	elapsed := time.Since(startTime)

	fmt.Printf("Completed %d operations in %v\n", atomic.LoadInt32(&completedCount), elapsed)

	// Analyze operation timing
	if len(results) > 1 {
		// Calculate average rate
		first := results[0].Value
		last := results[len(results)-1].Value
		duration := last.Sub(first)
		opsPerSecond := float64(len(results)-1) / duration.Seconds()

		fmt.Printf("Achieved rate: %.2f operations per second\n", opsPerSecond)

		// Verify the rate is close to our limit
		if opsPerSecond < 4.5 || opsPerSecond > 5.5 {
			fmt.Printf("Warning: Rate (%.2f ops/sec) is outside expected range (4.5-5.5 ops/sec)\n", opsPerSecond)
		} else {
			fmt.Println("Rate limiting is working as expected")
		}
	}

	// Explain the benefits
	fmt.Println("\nBenefits of rate limiting:")
	fmt.Println("1. Prevents API rate limit errors")
	fmt.Println("2. Ensures fair resource usage")
	fmt.Println("3. Smooths out traffic peaks")
	fmt.Println("4. Allows burst handling for efficiency")
}
