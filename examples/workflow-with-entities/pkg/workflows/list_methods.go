package workflows

import (
	"context"
	"fmt"
	"strings"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// TestListMethods tests various List methods of the Midaz SDK
// demonstrating standardized pagination and error handling
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//
// Returns:
//   - error: Any error encountered during the operation
func init() {
	TestListMethods = testListMethods
}

func testListMethods(ctx context.Context, midazClient *client.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n📋 STEP 12: TESTING LIST METHODS WITH PAGINATION AND ERROR HANDLING")
	fmt.Println(strings.Repeat("=", 50))

	// Test ListOrganizations with pagination
	fmt.Println("\n🔍 Testing ListOrganizations with pagination...")

	// Create pagination options with the fluent API
	orgOptions := models.NewListOptions().
		WithLimit(5).
		WithOrderBy("legalName").
		WithOrderDirection(models.SortAscending)

	orgsResponse, err := midazClient.Entity.Organizations.ListOrganizations(ctx, orgOptions)
	if err != nil {
		// Demonstrate standardized error handling
		if sdkerrors.IsNotFoundError(err) {
			return fmt.Errorf("no organizations found: %w", err)
		} else if sdkerrors.IsAuthenticationError(err) {
			return fmt.Errorf("authentication failed: %w", err)
		} else {
			return fmt.Errorf("failed to list organizations: %w", err)
		}
	}

	// Demonstrate pagination information
	fmt.Printf("✅ Found %d organizations (page %d of %d)\n",
		len(orgsResponse.Items),
		orgsResponse.Pagination.CurrentPage(),
		orgsResponse.Pagination.TotalPages())

	for i, org := range orgsResponse.Items {
		fmt.Printf("   %d. %s (ID: %s)\n", i+1, org.LegalName, org.ID)
	}

	// Show pagination navigation if available
	if orgsResponse.Pagination.HasNextPage() {
		fmt.Println("   (More organizations available on next page)")
	}

	// Test ListLedgers with filtering
	fmt.Println("\n🔍 Testing ListLedgers with filtering...")

	// Create options with filtering
	ledgerOptions := models.NewListOptions().
		WithFilter("status", models.StatusActive)

	ledgersResponse, err := midazClient.Entity.Ledgers.ListLedgers(ctx, orgID, ledgerOptions)
	if err != nil {
		// Demonstrate standardized error handling using FormatErrorForDisplay
		return fmt.Errorf("ledger listing failed: %s", sdkerrors.FormatErrorDetails(err))
	}

	fmt.Printf("✅ Found %d active ledgers\n", len(ledgersResponse.Items))
	for i, ledger := range ledgersResponse.Items {
		fmt.Printf("   %d. %s (ID: %s)\n", i+1, ledger.Name, ledger.ID)
	}

	// Test ListAccounts with pagination and filtering
	fmt.Println("\n🔍 Testing ListAccounts with pagination and filtering...")

	// Create options with pagination and filtering
	accountOptions := models.NewListOptions().
		WithLimit(3).
		WithOrderBy("createdAt").
		WithOrderDirection(models.SortDescending).
		WithFilter("type", "CUSTOMER")

	// Demonstrate error handling pattern for API calls
	accountsResponse, err := midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, accountOptions)
	if err != nil {
		switch {
		case sdkerrors.IsValidationError(err):
			return fmt.Errorf("invalid parameters: %w", err)
		case sdkerrors.IsNotFoundError(err):
			return fmt.Errorf("ledger or organization not found: %w", err)
		default:
			return fmt.Errorf("account listing failed: %w", err)
		}
	}

	fmt.Printf("✅ Found %d customer accounts (page %d of %d)\n",
		len(accountsResponse.Items),
		accountsResponse.Pagination.CurrentPage(),
		accountsResponse.Pagination.TotalPages())

	for i, account := range accountsResponse.Items {
		fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
	}

	// Demonstrate iterating through all pages if there are multiple
	if accountsResponse.Pagination.HasNextPage() {
		fmt.Println("\n📚 Demonstrating multi-page iteration through accounts...")

		// Start with our first page of results
		currentPage := accountsResponse
		pageCount := 1

		// Continue fetching pages while there are more
		for currentPage.Pagination.HasNextPage() && pageCount < 3 { // Limit to 3 pages for demo
			// Get options for the next page
			nextOptions := currentPage.Pagination.NextPageOptions()

			// Fetch the next page
			var err error
			currentPage, err = midazClient.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, nextOptions)
			if err != nil {
				return fmt.Errorf("failed to fetch page %d: %w", pageCount+1, err)
			}

			pageCount++
			fmt.Printf("\n📄 Page %d (offset %d):\n",
				currentPage.Pagination.CurrentPage(),
				currentPage.Pagination.Offset)

			for i, account := range currentPage.Items {
				fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
			}
		}

		fmt.Printf("✅ Iterated through %d pages of accounts\n", pageCount)
	}

	// Test ListPortfolios
	fmt.Println("\n🔍 Testing ListPortfolios...")
	portfoliosResponse, err := midazClient.Entity.Portfolios.ListPortfolios(ctx, orgID, ledgerID, models.NewListOptions())
	if err != nil {
		return fmt.Errorf("failed to list portfolios: %w", err)
	}
	fmt.Printf("✅ Found %d portfolios\n", len(portfoliosResponse.Items))
	for i, portfolio := range portfoliosResponse.Items {
		fmt.Printf("   %d. %s (ID: %s)\n", i+1, portfolio.Name, portfolio.ID)
	}

	// Test ListSegments with date range filtering
	fmt.Println("\n🔍 Testing ListSegments with date range filtering...")

	// Create options with date range
	segmentOptions := models.NewListOptions().
		WithDateRange("2023-01-01", "2100-12-31") // Wide range to ensure results

	segmentsResponse, err := midazClient.Entity.Segments.ListSegments(ctx, orgID, ledgerID, segmentOptions)
	if err != nil {
		return fmt.Errorf("failed to list segments: %w", err)
	}

	fmt.Printf("✅ Found %d segments created between 2023-01-01 and 2100-12-31\n",
		len(segmentsResponse.Items))

	for i, segment := range segmentsResponse.Items {
		region := "N/A"
		if segment.Metadata != nil && segment.Metadata["region"] != nil {
			region = fmt.Sprintf("%v", segment.Metadata["region"])
		}
		fmt.Printf("   %d. %s (ID: %s, Region: %s)\n", i+1, segment.Name, segment.ID, region)
	}

	fmt.Println("\n✅ All List methods tested successfully with pagination and error handling")
	return nil
}
