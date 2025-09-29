package workflows

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// ListAccounts lists all accounts in the ledger with advanced demonstrations
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - midazClient: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - error: Any error encountered during the operation
func ListAccounts(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n📋 STEP 8: ACCOUNT LISTING")
	fmt.Println(strings.Repeat("=", 50))

	// Basic account listing demonstration
	if err := demonstrateBasicListing(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Advanced parallel fetching demonstration
	if err := demonstrateParallelFetching(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	// Context cancellation demonstration
	return demonstrateContextCancellation(ctx, midazClient, orgID, ledgerID)
}

// demonstrateBasicListing shows basic pagination functionality
func demonstrateBasicListing(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("Listing all accounts...")

	listOptions := createBasicListOptions()

	accounts, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, listOptions)
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	displayAccountsPage(accounts.Items, accounts.Pagination)

	return demonstratePagination(ctx, midazClient, orgID, ledgerID, accounts)
}

// createBasicListOptions creates standard list options for account fetching
func createBasicListOptions() *models.ListOptions {
	return models.NewListOptions().
		WithLimit(5).
		WithOrderBy("name").
		WithOrderDirection(models.SortAscending).
		WithFilter("status", models.StatusActive)
}

// displayAccountsPage prints account information for a page
func displayAccountsPage(accounts []models.Account, pagination models.Pagination) {
	fmt.Printf("✅ Found %d accounts (showing page %d of %d):\n",
		len(accounts), pagination.CurrentPage(), pagination.TotalPages())

	for i, account := range accounts {
		fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
	}
}

// demonstratePagination shows next/previous page navigation
func demonstratePagination(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accounts *models.ListResponse[models.Account]) error {
	if !accounts.Pagination.HasMorePages() {
		return nil
	}

	fmt.Println("\nDemonstrating pagination - fetching next page...")

	nextPage, err := fetchNextPage(ctx, midazClient, orgID, ledgerID, accounts)
	if err != nil {
		return err
	}

	displayAccountsPage(nextPage.Items, nextPage.Pagination)

	return demonstrateGoingBack(ctx, midazClient, orgID, ledgerID, nextPage)
}

// fetchNextPage retrieves the next page of accounts
func fetchNextPage(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, accounts *models.ListResponse[models.Account]) (*models.ListResponse[models.Account], error) {
	nextPageOptions := accounts.Pagination.NextPageOptions()
	nextPage, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, nextPageOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch next page: %w", err)
	}

	return nextPage, nil
}

// demonstrateGoingBack shows how to navigate to previous pages
func demonstrateGoingBack(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, nextPage *models.ListResponse[models.Account]) error {
	if !nextPage.Pagination.HasPrevPage() {
		return nil
	}

	fmt.Println("\nDemonstrating pagination - returning to first page...")

	prevPageOptions := nextPage.Pagination.PrevPageOptions()
	prevPage, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, prevPageOptions)
	if err != nil {
		return fmt.Errorf("failed to fetch previous page: %w", err)
	}

	fmt.Printf("✅ Back to first page with %d accounts\n", len(prevPage.Items))

	return nil
}

// demonstrateParallelFetching shows advanced concurrent account fetching
func demonstrateParallelFetching(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\nDemo: Advanced parallel account listing with retry and context handling...")

	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	firstPage, err := fetchFirstPage(listCtx, midazClient, orgID, ledgerID)
	if err != nil {
		return handleFirstPageError(err)
	}

	totalAccounts := len(firstPage.Items)
	fmt.Printf("✅ Page 1: %d accounts (total so far: %d)\n", len(firstPage.Items), totalAccounts)

	if firstPage.Pagination.HasMorePages() {
		totalAccounts = performParallelFetching(listCtx, midazClient, orgID, ledgerID, firstPage, totalAccounts)
	}

	fmt.Printf("✅ Iterated through all %d accounts\n", totalAccounts)

	return nil
}

// fetchFirstPage retrieves the first page for parallel demo
func fetchFirstPage(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) (*models.ListResponse[models.Account], error) {
	fmt.Println("1️⃣ Fetching first page to determine pagination...")

	iterationOptions := models.NewListOptions().WithLimit(2)

	return midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, iterationOptions)
}

// handleFirstPageError processes first page fetch errors
func handleFirstPageError(err error) error {
	if errors.IsCancellationError(err) {
		fmt.Println("⚠️ Operation cancelled due to timeout")
		return nil
	}

	return fmt.Errorf("failed to fetch first page: %w", err)
}

// performParallelFetching executes concurrent page fetching with WorkerPool
func performParallelFetching(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, firstPage *models.ListResponse[models.Account], totalAccounts int) int {
	fmt.Println("\n2️⃣ Implementing parallel page fetching with WorkerPool...")

	remainingPages := firstPage.Pagination.TotalPages() - 1
	fmt.Printf("🔢 Total pages to fetch: %d (already fetched: 1, remaining: %d)\n",
		firstPage.Pagination.TotalPages(), remainingPages)

	if remainingPages <= 0 {
		return totalAccounts
	}

	pageOptions := createPageOptions(remainingPages)
	fetchPageFunc := createFetchPageFunction(midazClient, orgID, ledgerID)

	fmt.Println("🚀 Launching parallel workers to fetch all pages concurrently...")

	results := concurrent.WorkerPool(
		ctx,
		pageOptions,
		fetchPageFunc,
		concurrent.WithWorkers(3),
		concurrent.WithRateLimit(5),
		concurrent.WithUnorderedResults(),
	)

	return processParallelResults(results, pageOptions, totalAccounts)
}

// createPageOptions generates list options for remaining pages
func createPageOptions(remainingPages int) []*models.ListOptions {
	iterationOptions := models.NewListOptions().WithLimit(2)
	pageOptions := make([]*models.ListOptions, remainingPages)

	for i := 0; i < remainingPages; i++ {
		pageOptions[i] = models.NewListOptions().
			WithLimit(iterationOptions.Limit).
			WithPage(i + 2)

		// Copy filters from the original options
		for k, v := range iterationOptions.Filters {
			pageOptions[i].Filters[k] = v
		}
	}

	return pageOptions
}

// createFetchPageFunction creates a worker function for page fetching
func createFetchPageFunction(midazClient *client.Client, orgID, ledgerID string) func(context.Context, *models.ListOptions) ([]models.Account, error) {
	return func(ctx context.Context, options *models.ListOptions) ([]models.Account, error) {
		limiter := concurrent.NewRateLimiter(5, 5)
		defer limiter.Stop()

		if err := limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter wait failed: %w", err)
		}

		simulateLatency(options.Page)

		page, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
		if err != nil {
			return nil, err
		}

		return page.Items, nil
	}
}

// simulateLatency adds artificial delay for demonstration
func simulateLatency(pageNum int) {
	var randomBytes [2]byte

	delay := time.Duration(125) * time.Millisecond

	if _, err := rand.Read(randomBytes[:]); err == nil {
		randomNum := int(binary.BigEndian.Uint16(randomBytes[:]))%150 + 50
		delay = time.Duration(randomNum) * time.Millisecond
		fmt.Printf("🔄 Fetching page %d (with %dms simulated latency)...\n", pageNum, delay.Milliseconds())
	} else {
		fmt.Printf("🔄 Fetching page %d (with %dms default latency)...\n", pageNum, delay.Milliseconds())
	}

	time.Sleep(delay)
	fmt.Printf("🔄 Fetching page %d...\n", pageNum)
}

// processParallelResults handles the results from parallel page fetching
func processParallelResults(results []concurrent.Result[*models.ListOptions, []models.Account], pageOptions []*models.ListOptions, totalAccounts int) int {
	successCount := 0
	errorCount := 0

	for i, result := range results {
		pageNumber := pageOptions[i].Page

		if result.Error != nil {
			errorCount++

			handleParallelError(result.Error, pageNumber)

			continue
		}

		successCount++
		accounts := result.Value
		totalAccounts += len(accounts)

		fmt.Printf("✅ Page %d: %d accounts (total so far: %d)\n",
			pageNumber, len(accounts), totalAccounts)
	}

	fmt.Printf("\n📊 Summary: Successfully fetched %d/%d pages (%d accounts total)\n",
		successCount, len(pageOptions), totalAccounts)

	return totalAccounts
}

// handleParallelError processes errors from parallel fetching
func handleParallelError(err error, pageNumber int) {
	if errors.IsCancellationError(err) {
		fmt.Printf("⏱️ Page %d fetch cancelled: %v\n", pageNumber, err)
	} else if errors.IsRateLimitError(err) {
		fmt.Printf("⚠️ Page %d hit rate limit: %v\n", pageNumber, err)
	} else {
		fmt.Printf("❌ Error fetching page %d: %v\n", pageNumber, err)
	}
}

// demonstrateContextCancellation shows context cancellation behavior
func demonstrateContextCancellation(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n3️⃣ Demonstrating context cancellation handling...")

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	// Launch goroutine to cancel context
	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("🛑 Cancelling context deliberately...")
		cancelFunc()
	}()

	// Attempt fetch with cancellable context
	_, err := midazClient.Entity.Accounts.ListAccounts(cancelCtx, orgID, ledgerID, models.NewListOptions())

	return handleCancellationResult(err)
}

// handleCancellationResult processes the result of the cancellation test
func handleCancellationResult(err error) error {
	if err != nil {
		if errors.IsCancellationError(err) {
			fmt.Println("✅ Context cancellation correctly detected and handled")
		} else {
			fmt.Printf("❓ Expected cancellation error, but got: %v\n", err)
		}
	} else {
		fmt.Println("❓ Expected an error due to context cancellation, but operation succeeded")
	}

	return nil
}
