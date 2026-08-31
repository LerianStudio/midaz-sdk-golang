package workflows

import (
	"context"
	"fmt"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v6"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

// testListMethods tests various List methods of the Midaz SDK
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

func testListMethods(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n📋 STEP 12: TESTING LIST METHODS WITH PAGINATION AND ERROR HANDLING")
	fmt.Println(strings.Repeat("=", 50))

	if err := testListOrganizations(ctx, midazClient); err != nil {
		return err
	}

	if err := testListLedgers(ctx, midazClient, orgID); err != nil {
		return err
	}

	if err := testListAccountsWithPagination(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	if err := testListPortfolios(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	if err := testListSegments(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	fmt.Println("\n✅ All List methods tested successfully with pagination and error handling")

	return nil
}

// testListOrganizations tests the ListOrganizations method with pagination
func testListOrganizations(ctx context.Context, midazClient *midaz.Client) error {
	fmt.Println("\n🔍 Testing ListOrganizations with pagination...")

	orgOptions := models.OrganizationsListOpts{
		PageListOpts: models.PageListOpts{
			Limit:         5,
			SortDirection: models.SortAscending,
		},
	}

	orgsResponse, err := midazClient.V2.Organizations.List(ctx, orgOptions)
	if err != nil {
		return handleOrganizationError(err)
	}

	printOrganizationsResults(orgsResponse)

	return nil
}

// handleOrganizationError handles organization-specific errors
func handleOrganizationError(err error) error {
	if pkgerrors.IsNotFoundError(err) {
		return fmt.Errorf("no organizations found: %w", err)
	}

	if pkgerrors.IsAuthenticationError(err) {
		return fmt.Errorf("authentication failed: %w", err)
	}

	return fmt.Errorf("failed to list organizations: %w", err)
}

// printOrganizationsResults prints the organization results
func printOrganizationsResults(orgsResponse *models.ListResponse[models.Organization]) {
	page, totalPages := pageStats(orgsResponse.Pagination)
	if totalPages > 0 {
		fmt.Printf("✅ Found %d organizations (page %d of %d)\n",
			len(orgsResponse.Items), page, totalPages)
	} else {
		fmt.Printf("✅ Found %d organizations (page %d)\n",
			len(orgsResponse.Items), page)
	}

	for i, org := range orgsResponse.Items {
		fmt.Printf("   %d. %s (ID: %s)\n", i+1, org.LegalName, org.ID)
	}

	if orgsResponse.Pagination.HasMore() {
		fmt.Println("   (More organizations available on next page)")
	}
}

// pageStats computes the current 1-based page number and the total
// page count from a Pagination shape. Returns totalPages == 0 when the
// server did not report Total (cursor-paginated endpoints typically
// omit it). Callers should check the return value before formatting
// "page N of M" strings.
//
// Replaces the deleted Pagination.CurrentPage() and Pagination.TotalPages()
// methods, which silently returned misleading values when Total was zero.
func pageStats(p models.Pagination) (page, totalPages int) {
	page = 1
	if p.Page > 0 {
		page = p.Page
	} else if p.Limit > 0 {
		page = (p.Offset / p.Limit) + 1
	}

	if p.TotalKnown() && p.Limit > 0 {
		totalPages = (p.Total + p.Limit - 1) / p.Limit
	}

	return page, totalPages
}

// testListLedgers tests the ListLedgers method with filtering
func testListLedgers(ctx context.Context, midazClient *midaz.Client, orgID string) error {
	fmt.Println("\n🔍 Testing ListLedgers with filtering...")

	ledgerOptions := models.LedgersListOpts{
		Filters: models.LedgersFilters{Status: models.StatusActive},
	}

	ledgersResponse, err := midazClient.V2.Ledgers.List(ctx, orgID, ledgerOptions)
	if err != nil {
		return fmt.Errorf("ledger listing failed: %s", pkgerrors.FormatErrorDetails(err))
	}

	fmt.Printf("✅ Found %d active ledgers\n", len(ledgersResponse.Items))

	for i, ledger := range ledgersResponse.Items {
		fmt.Printf("   %d. %s (ID: %s)\n", i+1, ledger.Name, ledger.ID)
	}

	return nil
}

// testListAccountsWithPagination tests V2.Accounts.List using typed
// AccountsListOpts and demonstrates iter.Seq2 transparent pagination via
// V2.Accounts.Pages.
func testListAccountsWithPagination(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n🔍 Testing V2.Accounts.List with pagination and filtering...")

	accountOptions := models.AccountsListOpts{
		PageListOpts: models.PageListOpts{
			Limit:         3,
			SortDirection: models.SortDescending,
		},
		Filters: models.AccountsFilters{Type: "CUSTOMER"},
	}

	accountsResponse, err := midazClient.V2.Accounts.List(ctx, orgID, ledgerID, accountOptions)
	if err != nil {
		return handleAccountError(err)
	}

	printAccountsResults(accountsResponse)

	if accountsResponse.Pagination.HasMore() {
		return demonstrateAccountPagination(ctx, midazClient, orgID, ledgerID, accountOptions)
	}

	return nil
}

// handleAccountError handles account-specific errors
func handleAccountError(err error) error {
	switch {
	case pkgerrors.IsValidationError(err):
		return fmt.Errorf("invalid parameters: %w", err)
	case pkgerrors.IsNotFoundError(err):
		return fmt.Errorf("ledger or organization not found: %w", err)
	default:
		return fmt.Errorf("account listing failed: %w", err)
	}
}

// printAccountsResults prints the accounts results
func printAccountsResults(accountsResponse *models.ListResponse[models.Account]) {
	page, totalPages := pageStats(accountsResponse.Pagination)
	if totalPages > 0 {
		fmt.Printf("✅ Found %d customer accounts (page %d of %d)\n",
			len(accountsResponse.Items), page, totalPages)
	} else {
		fmt.Printf("✅ Found %d customer accounts (page %d)\n",
			len(accountsResponse.Items), page)
	}

	for i, account := range accountsResponse.Items {
		fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
	}
}

// demonstrateAccountPagination demonstrates multi-page iteration through accounts
// via V2.Accounts.Pages iter.Seq2. Limits to 3 pages for demo purposes.
func demonstrateAccountPagination(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string, opts models.AccountsListOpts) error {
	fmt.Println("\n📚 Demonstrating multi-page iteration through accounts...")

	pageCount := 0
	for currentPage, err := range midazClient.V2.Accounts.Pages(ctx, orgID, ledgerID, opts) {
		if err != nil {
			return fmt.Errorf("failed to fetch page %d: %w", pageCount+1, err)
		}

		pageCount++

		page, _ := pageStats(currentPage.Pagination)
		fmt.Printf("\n📄 Page %d:\n", page)

		for i, account := range currentPage.Items {
			fmt.Printf("   %d. %s (ID: %s, Type: %s)\n", i+1, account.Name, account.ID, account.Type)
		}

		if pageCount >= 3 {
			break
		}
	}

	fmt.Printf("✅ Iterated through %d pages of accounts\n", pageCount)

	return nil
}

// testListPortfolios tests the ListPortfolios method
func testListPortfolios(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n🔍 Testing ListPortfolios...")

	portfoliosResponse, err := midazClient.V2.Portfolios.List(ctx, orgID, ledgerID, models.PortfoliosListOpts{})
	if err != nil {
		return fmt.Errorf("failed to list portfolios: %w", err)
	}

	fmt.Printf("✅ Found %d portfolios\n", len(portfoliosResponse.Items))

	for i, portfolio := range portfoliosResponse.Items {
		fmt.Printf("   %d. %s (ID: %s)\n", i+1, portfolio.Name, portfolio.ID)
	}

	return nil
}

// testListSegments tests the ListSegments method with date range filtering
func testListSegments(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n🔍 Testing ListSegments with date range filtering...")

	segmentOptions := models.SegmentsListOpts{
		PageListOpts: models.PageListOpts{
			StartDate: "2023-01-01",
			EndDate:   "2100-12-31",
		},
	}

	segmentsResponse, err := midazClient.V2.Segments.List(ctx, orgID, ledgerID, segmentOptions)
	if err != nil {
		return fmt.Errorf("failed to list segments: %w", err)
	}

	fmt.Printf("✅ Found %d segments created between 2023-01-01 and 2100-12-31\n",
		len(segmentsResponse.Items))

	for i, segment := range segmentsResponse.Items {
		region := extractSegmentRegion(segment)
		fmt.Printf("   %d. %s (ID: %s, Region: %s)\n", i+1, segment.Name, segment.ID, region)
	}

	return nil
}

// extractSegmentRegion extracts the region metadata from a segment
func extractSegmentRegion(segment models.Segment) string {
	if segment.Metadata != nil && segment.Metadata["region"] != nil {
		return fmt.Sprintf("%v", segment.Metadata["region"])
	}

	return "N/A"
}
