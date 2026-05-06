package workflows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3"
	"github.com/LerianStudio/midaz-sdk-golang/v3/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// ListAccounts lists all accounts in the ledger with v3 idiomatic demonstrations.
//
// The v3 contract demonstrated here:
//   - Typed AccountsListOpts (no pointer, concurrent-safe)
//   - ListAccountsAll iter.Seq2 for transparent pagination
//   - entities.Collect to materialize a bounded slice
//
// This file replaces the v2 multi-page-with-NextPageOptionsFrom demo
// that depended on the now-deleted Pagination.NextPageOptions and
// PrevPageOptions methods (Track 5 Batch 5B). Customers iterating
// page-by-page in v3 use ListAccountsPages or call ListAccounts
// repeatedly with opts.Page++.
func ListAccounts(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n\n📋 STEP 8: ACCOUNT LISTING")
	fmt.Println(strings.Repeat("=", 50))

	if err := demonstrateBasicListing(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	if err := demonstrateAllAccountsIteration(ctx, midazClient, orgID, ledgerID); err != nil {
		return err
	}

	return demonstrateContextCancellation(ctx, midazClient, orgID, ledgerID)
}

// demonstrateBasicListing shows ListAccounts on a single page.
func demonstrateBasicListing(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("Listing first page of accounts...")

	opts := models.AccountsListOpts{
		PageListOpts: models.PageListOpts{
			Limit:         5,
			OrderBy:       "name",
			SortDirection: models.SortAscending,
		},
		Filters: models.AccountsFilters{Status: models.StatusActive},
	}

	resp, err := midazClient.Accounts.ListAccounts(ctx, orgID, ledgerID, opts)
	if err != nil {
		return fmt.Errorf("failed to list accounts: %w", err)
	}

	fmt.Printf("✅ Found %d accounts on this page\n", len(resp.Items))

	for i, account := range resp.Items {
		fmt.Printf("   %d. %q (ID: %q, Type: %q)\n", i+1, account.Name, account.ID, account.Type)
	}

	if resp.Pagination.HasMore() {
		fmt.Println("   (More accounts available — see ListAccountsAll demo below)")
	}

	return nil
}

// demonstrateAllAccountsIteration shows ListAccountsAll iter.Seq2.
func demonstrateAllAccountsIteration(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\nDemonstrating ListAccountsAll (iter.Seq2 transparent pagination)...")

	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 5},
	}

	all, err := entities.Collect(
		midazClient.Accounts.ListAccountsAll(listCtx, orgID, ledgerID, opts),
		1000, // hard cap; example workload is bounded
	)
	if err != nil {
		if pkgerrors.IsCancellationError(err) {
			fmt.Println("⚠️ Operation cancelled due to timeout")
			return nil
		}

		return fmt.Errorf("failed to iterate all accounts: %w", err)
	}

	fmt.Printf("✅ Iterated through %d accounts via iter.Seq2\n", len(all))

	return nil
}

// demonstrateContextCancellation shows graceful cancellation handling.
func demonstrateContextCancellation(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\nDemonstrating context cancellation...")

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // cancel immediately

	_, err := midazClient.Accounts.ListAccounts(cancelCtx, orgID, ledgerID, models.AccountsListOpts{})
	if err == nil {
		return errors.New("expected cancellation error but got nil")
	}

	if pkgerrors.IsCancellationError(err) {
		fmt.Println("✅ Cancellation handled gracefully")
		return nil
	}

	fmt.Printf("✅ Got error from cancelled context: %v\n", err)

	return nil
}
