package entities_test

import (
	"context"
	"fmt"

	client "github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
)

// Example_accountsListAll demonstrates the canonical v4 pattern for
// traversing every account in a ledger. The Accounts facade's All method
// returns an iter.Seq2 that auto-advances pages and yields one Account at a
// time, so a normal range-over-func loop just works.
//
// For one-page-at-a-time access (page metadata, custom batching), use
// c.V1.Accounts.List; for page-level iteration with metadata, use
// c.V1.Accounts.Pages.
func Example_accountsListAll() {
	cfg, err := config.NewConfig(config.FromEnvironment())
	if err != nil {
		fmt.Println("config error")
		return
	}

	c, err := client.New(client.WithConfig(cfg), client.WithAnonymous())
	if err != nil {
		fmt.Println("client error")
		return
	}

	ctx := context.Background()
	opts := models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 100},
		Filters:      models.AccountsFilters{Status: "ACTIVE", AssetCode: "USD"},
	}

	count := 0
	for acct, err := range c.V1.Accounts.All(ctx, "org-123", "ledger-456", opts) {
		if err != nil {
			fmt.Printf("iteration error: %v\n", err)
			return
		}

		_ = acct
		count++
	}

	fmt.Printf("traversed %d accounts\n", count)
}

// Example_transactionsListAll demonstrates cursor-based iteration. Transactions
// use a cursor — the TransactionsListOpts struct has Cursor but no Page/Offset
// by construction, so the v2 footgun where setting WithPage on a cursor
// endpoint silently dropped the value is structurally impossible.
//
// The iterator handles cursor advance internally; the caller writes the same
// shape of code regardless of pagination style.
func Example_transactionsListAll() {
	cfg, err := config.NewConfig(config.FromEnvironment())
	if err != nil {
		fmt.Println("config error")
		return
	}

	c, err := client.New(client.WithConfig(cfg), client.WithAnonymous())
	if err != nil {
		fmt.Println("client error")
		return
	}

	ctx := context.Background()
	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 50},
		Filters:        models.TransactionsFilters{Status: "APPROVED", AssetCode: "USD"},
	}

	count := 0
	for tx, err := range c.V1.Transactions.All(ctx, "org-123", "ledger-456", opts) {
		if err != nil {
			fmt.Printf("iteration error: %v\n", err)
			return
		}

		_ = tx
		count++
	}

	fmt.Printf("traversed %d transactions\n", count)
}
