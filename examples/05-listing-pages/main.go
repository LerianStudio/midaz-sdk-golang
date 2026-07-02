// Package main demonstrates page-based pagination in the v3 SDK using
// the iter.Seq2 trio: List, ListAll, ListPages.
//
// The v3 pagination contract:
//
//	List      — one page, you decide when to advance.
//	ListAll   — iter.Seq2[T, error] — yields every item, hides paging.
//	ListPages — iter.Seq2[*ListResponse[T], error] — yields full page
//	            envelopes (with metadata, e.g., total, hasNext) so you
//	            can short-circuit, log per-page progress, or implement
//	            custom backpressure.
//
// Page-based endpoints (Organizations, Ledgers, Accounts, Assets, etc.)
// support a Page field on the typed list-opts struct. Cursor-based
// endpoints (Transactions, Operations, OperationRoutes,
// TransactionRoutes) use cursor opts instead — see examples/04-listing-cursor.
//
// Usage:
//
//	go run ./examples/05-listing-pages
//
// Requires a local Midaz stack with auth disabled and at least one
// organization + ledger + a few accounts (run examples/03-end-to-end first
// to seed data).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v4"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/config"
)

func main() {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		log.Fatalf("midaz.New: %v", err)
	}
	defer func() {
		if err := c.Shutdown(context.Background()); err != nil {
			log.Printf("client shutdown: %v", err)
		}
	}()

	ctx := context.Background()

	// Resolve an organization + ledger to list accounts under. Real code
	// would carry these IDs from configuration, the URL, or an auth token.
	orgID, ledgerID, err := resolveOrgAndLedger(ctx, c)
	if err != nil {
		log.Fatalf("resolve org/ledger: %v\n  Run examples/03-end-to-end first to seed data.", err)
	}

	demoOnePage(ctx, c, orgID, ledgerID)
	demoEveryItem(ctx, c, orgID, ledgerID)
	demoEveryPage(ctx, c, orgID, ledgerID)
}

// demoOnePage uses ListAccounts: returns exactly one page. The caller
// decides whether to fetch more by examining the response and re-calling
// with an incremented Page. This is the right shape when the caller has
// a UI 'next page' button or wants explicit per-page control.
func demoOnePage(ctx context.Context, c *midaz.Client, orgID, ledgerID string) {
	fmt.Println("--- ListAccounts (one page) ---")

	page, err := c.Accounts.List(ctx, orgID, ledgerID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{
			Limit: 5,
			Page:  1,
		},
	})
	if err != nil {
		log.Printf("ListAccounts: %v", err)
		return
	}

	fmt.Printf("page 1 returned %d accounts (total available may be larger)\n", len(page.Items))
	for _, acc := range page.Items {
		fmt.Printf("  - %s (%s)\n", acc.Name, acc.ID)
	}
}

// demoEveryItem uses ListAccountsAll: a single iter.Seq2 that walks
// every account across every page. The SDK handles paging internally.
// This is the right shape for batch jobs, exports, or anything where
// the caller wants to think 'collection,' not 'page.'
func demoEveryItem(ctx context.Context, c *midaz.Client, orgID, ledgerID string) {
	fmt.Println("--- ListAccountsAll (every item) ---")

	count := 0
	for acc, err := range c.Accounts.All(ctx, orgID, ledgerID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 50},
	}) {
		if err != nil {
			log.Printf("iteration error: %v", err)
			return
		}
		count++
		fmt.Printf("  [%d] %s (%s)\n", count, acc.Name, acc.ID)

		if count >= 10 {
			fmt.Println("  ...stopping early (early-termination is just `return`)")
			return
		}
	}

	fmt.Printf("walked %d accounts total\n", count)
}

// demoEveryPage uses ListAccountsPages: yields *ListResponse envelopes,
// not individual items. Use when you need page-level metadata
// (Pagination.NextPage, the count of items in this batch, etc.) — for
// example, to log per-page checkpoints in a long-running export, or to
// short-circuit before pulling the next page based on some condition.
func demoEveryPage(ctx context.Context, c *midaz.Client, orgID, ledgerID string) {
	fmt.Println("--- ListAccountsPages (every page envelope) ---")

	pageNum := 0
	for batch, err := range c.Accounts.Pages(ctx, orgID, ledgerID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 5},
	}) {
		if err != nil {
			log.Printf("iteration error: %v", err)
			return
		}
		pageNum++
		fmt.Printf("  page %d: %d items\n", pageNum, len(batch.Items))

		if pageNum >= 3 {
			fmt.Println("  ...stopping early after 3 pages")
			return
		}
	}
}

// resolveOrgAndLedger resolves an organization and ledger to list accounts
// under. Real code reads these from configuration; this example just
// grabs the first available pair to keep the demo self-contained.
func resolveOrgAndLedger(ctx context.Context, c *midaz.Client) (orgID, ledgerID string, err error) {
	orgs, err := c.Organizations.List(ctx, models.OrganizationsListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	})
	if err != nil {
		return "", "", fmt.Errorf("list organizations: %w", err)
	}
	if len(orgs.Items) == 0 {
		return "", "", errors.New("no organizations available")
	}
	orgID = orgs.Items[0].ID

	ledgers, err := c.Ledgers.List(ctx, orgID, models.LedgersListOpts{
		PageListOpts: models.PageListOpts{Limit: 1},
	})
	if err != nil {
		return "", "", fmt.Errorf("list ledgers: %w", err)
	}
	if len(ledgers.Items) == 0 {
		return "", "", errors.New("no ledgers available")
	}
	return orgID, ledgers.Items[0].ID, nil
}
