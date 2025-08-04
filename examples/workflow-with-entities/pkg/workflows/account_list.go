package workflows

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
)

// ListAccounts lists all accounts in the ledger
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - cfg: The configuration object
//   - client: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - error: Any error encountered during the operation
func ListAccounts(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n📋 STEP 8: ACCOUNT LISTING")
	fmt.Println(strings.Repeat("=", 50))

	fmt.Println("Listing all accounts...")

	// Create pagination options with default values
	// This will use the system's default page size and sort order
	listOptions := models.NewListOptions()

	// Customize pagination options as needed
	listOptions.WithLimit(5). // Limit to 5 accounts per page
					WithOrderBy("name").                      // Order by name
					WithOrderDirection(models.SortAscending). // Use ascending order
					WithFilter("status", models.StatusActive) // Filter by status

	// Call the API with our pagination options
	accounts, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	fmt.Printf("✅ Found %d accounts (showing page %d of %d):\n",
		len(accounts.Items),
		accounts.Pagination.CurrentPage(),
		accounts.Pagination.TotalPages())

	for i, account := range accounts.Items {
		fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
	}

	// Demonstrate pagination capabilities
	if accounts.Pagination.HasMorePages() {
		fmt.Println("\nDemonstrating pagination - fetching next page...")

		// Get options for the next page
		nextPageOptions := accounts.Pagination.NextPageOptions()
		nextPage, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, nextPageOptions)
		if err != nil {
			return fmt.Errorf("failed to fetch next page: %w", err)
		}

		fmt.Printf("✅ Next page contains %d accounts (page %d of %d):\n",
			len(nextPage.Items),
			nextPage.Pagination.CurrentPage(),
			nextPage.Pagination.TotalPages())

		for i, account := range nextPage.Items {
			fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
		}

		// If we have more than one page of results, demonstrate going back
		if nextPage.Pagination.HasPrevPage() {
			fmt.Println("\nDemonstrating pagination - returning to first page...")

			// Get options for the previous page (which is the first page in this case)
			prevPageOptions := nextPage.Pagination.PrevPageOptions()
			prevPage, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, prevPageOptions)
			if err != nil {
				return fmt.Errorf("failed to fetch previous page: %w", err)
			}

			fmt.Printf("✅ Back to first page with %d accounts\n", len(prevPage.Items))
		}
	}

	// Example of advanced usage: parallel fetching with timeout, retries, and cancellation
	fmt.Println("\nDemo: Advanced parallel account listing with retry and context handling...")

	// ------- PART 1: PARALLEL FETCHING WITH SHARED CONTEXT -------
	// Create a context with a longer timeout for this demonstration
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Configure pagination to show the feature
	iterationOptions := models.NewListOptions().WithLimit(2) // Small page size to demonstrate pagination
	totalAccounts := 0
	pageNum := 1

	// First page fetch to get pagination info
	fmt.Println("1️⃣ Fetching first page to determine pagination...")
	firstPage, err := midazClient.Entity.Accounts.ListAccounts(listCtx, orgID, ledgerID, iterationOptions)
	if err != nil {
		if errors.IsCancellationError(err) {
			fmt.Println("⚠️ Operation cancelled due to timeout")
			return nil // Don't return an error, this is just a demo
		}
		return fmt.Errorf("failed to fetch first page: %w", err)
	}

	// Process the first page
	totalAccounts += len(firstPage.Items)
	fmt.Printf("✅ Page %d: %d accounts (total so far: %d)\n",
		pageNum, len(firstPage.Items), totalAccounts)

	// ------- PART 2: CONCURRENT PAGE FETCHING WITH WORKER POOL -------
	if firstPage.Pagination.HasMorePages() {
		fmt.Println("\n2️⃣ Implementing parallel page fetching with WorkerPool...")

		// Calculate how many pages we need to fetch
		totalPages := firstPage.Pagination.TotalPages()
		remainingPages := totalPages - 1 // We already fetched page 1

		fmt.Printf("🔢 Total pages to fetch: %d (already fetched: 1, remaining: %d)\n",
			totalPages, remainingPages)

		if remainingPages > 0 {
			// Create pagination options for all remaining pages
			pageOptions := make([]*models.ListOptions, remainingPages)
			for i := 0; i < remainingPages; i++ {
				// Create a new options instance for each page
				pageOptions[i] = models.NewListOptions().
					WithLimit(iterationOptions.Limit).
					WithPage(i + 2) // Pages start at 1, and we already fetched page 1

				// Copy any filters from the original options
				for k, v := range iterationOptions.Filters {
					pageOptions[i].Filters[k] = v
				}
			}

			// Define a worker function to fetch a single page
			fetchPage := func(ctx context.Context, options *models.ListOptions) ([]models.Account, error) {
				// Add rate limiting to avoid overwhelming the API
				limiter := concurrent.NewRateLimiter(5, 5) // 5 ops/sec with burst of 5
				defer limiter.Stop()

				// Acquire a token from the rate limiter
				if err := limiter.Wait(ctx); err != nil {
					return nil, fmt.Errorf("rate limiter wait failed: %w", err)
				}

				// Add random delay to simulate network variability (just for demo)
				var randomBytes [2]byte
				_, err = rand.Read(randomBytes[:])
				if err != nil {
					// If crypto/rand fails, use a default value
					delay := time.Duration(125) * time.Millisecond
					time.Sleep(delay)
					fmt.Printf("🔄 Fetching page %d (with %dms default latency due to random generation error)...\n",
						options.Page, delay.Milliseconds())
				} else {
					// Convert random bytes to a number between 50 and 200
					randomNum := int(binary.BigEndian.Uint16(randomBytes[:]))%150 + 50
					delay := time.Duration(randomNum) * time.Millisecond
					time.Sleep(delay)
					fmt.Printf("🔄 Fetching page %d (with %dms simulated latency)...\n",
						options.Page, delay.Milliseconds())
				}

				// Log the attempt (this would normally be debug-level logging)
				fmt.Printf("🔄 Fetching page %d...\n",
					options.Page)

				// Fetch the page with automatic retries handled by SDK's HTTP client
				page, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
				if err != nil {
					return nil, err
				}

				return page.Items, nil
			}

			// Use concurrent.WorkerPool to fetch all pages in parallel
			fmt.Println("🚀 Launching parallel workers to fetch all pages concurrently...")
			results := concurrent.WorkerPool(
				listCtx,
				pageOptions,
				fetchPage,
				concurrent.WithWorkers(3),         // Use 3 workers (adjust based on API rate limits)
				concurrent.WithRateLimit(5),       // Limit to 5 ops/sec across all workers
				concurrent.WithUnorderedResults(), // Process results as they arrive
			)

			// Process all results
			successCount := 0
			errorCount := 0

			for i, result := range results {
				pageNumber := pageOptions[i].Page

				if result.Error != nil {
					errorCount++
					if errors.IsCancellationError(result.Error) {
						fmt.Printf("⏱️ Page %d fetch cancelled: %v\n", pageNumber, result.Error)
					} else if errors.IsRateLimitError(result.Error) {
						fmt.Printf("⚠️ Page %d hit rate limit: %v\n", pageNumber, result.Error)
					} else {
						fmt.Printf("❌ Error fetching page %d: %v\n", pageNumber, result.Error)
					}
					continue
				}

				// Successfully retrieved page
				successCount++
				accounts := result.Value
				totalAccounts += len(accounts)

				fmt.Printf("✅ Page %d: %d accounts (total so far: %d)\n",
					pageNumber, len(accounts), totalAccounts)
			}

			fmt.Printf("\n📊 Summary: Successfully fetched %d/%d pages (%d accounts total)\n",
				successCount, remainingPages, totalAccounts)
		}
	}

	// ------- PART 3: DEMONSTRATE CONTEXT CANCELLATION -------
	fmt.Println("\n3️⃣ Demonstrating context cancellation handling...")

	// Create a context that we'll cancel manually
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// Launch a goroutine to cancel the context after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond) // Short delay to allow the operation to start
		fmt.Println("🛑 Cancelling context deliberately...")
		cancelFunc() // Cancel the context deliberately
	}()

	// Attempt to fetch accounts with the context that will be cancelled
	_, err = midazClient.Entity.Accounts.ListAccounts(cancelCtx, orgID, ledgerID, models.NewListOptions())

	// Check for cancellation error
	if err != nil {
		if errors.IsCancellationError(err) {
			fmt.Println("✅ Context cancellation correctly detected and handled")
		} else {
			fmt.Printf("❓ Expected cancellation error, but got: %v\n", err)
		}
	} else {
		fmt.Println("❓ Expected an error due to context cancellation, but operation succeeded")
	}

	fmt.Printf("✅ Iterated through all %d accounts\n", totalAccounts)

	return nil
}
