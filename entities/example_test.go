package entities_test

import (
	"context"
	"fmt"

	client "github.com/LerianStudio/midaz-sdk-golang/v4"
	"github.com/LerianStudio/midaz-sdk-golang/v4/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/config"
)

// Anchor identifiers that satisfy go vet's example-naming requirement
// (Example<Type>_<Method> needs <Type> to be in scope from this test
// package). The concrete implementations are unexported, so we cannot
// assert they satisfy the interface here — see the corresponding _test.go
// files in package entities for the concrete-type compile checks.
var (
	_ entities.AccountsService     = (entities.AccountsService)(nil)
	_ entities.TransactionsService = (entities.TransactionsService)(nil)
)

// ExampleAccountsService_ListAccountsAll demonstrates the canonical v3
// pattern for traversing every account in a ledger. ListAccountsAll
// returns an iter.Seq2 that auto-advances pages and yields one Account
// at a time, so a normal range-over-func loop just works.
//
// For one-page-at-a-time access (page metadata, custom batching), use
// ListAccounts; for page-level iteration with metadata, use
// ListAccountsPages.
func ExampleAccountsService_ListAccountsAll() {
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
	for acct, err := range c.Accounts.All(ctx, "org-123", "ledger-456", opts) {
		if err != nil {
			fmt.Printf("iteration error: %v\n", err)
			return
		}

		_ = acct
		count++
	}

	fmt.Printf("traversed %d accounts\n", count)
}

// ExampleTransactionsService_ListTransactionsAll demonstrates cursor-based
// iteration. Transactions use a cursor — the TransactionsListOpts struct
// has Cursor but no Page/Offset by construction, so the v2 footgun where
// setting WithPage on a cursor endpoint silently dropped the value is
// structurally impossible (audit finding 5.5).
//
// The iterator handles cursor advance internally; the caller writes the
// same shape of code regardless of pagination style.
func ExampleTransactionsService_ListTransactionsAll() {
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
	for tx, err := range c.Transactions.All(ctx, "org-123", "ledger-456", opts) {
		if err != nil {
			fmt.Printf("iteration error: %v\n", err)
			return
		}

		_ = tx
		count++
	}

	fmt.Printf("traversed %d transactions\n", count)
}
