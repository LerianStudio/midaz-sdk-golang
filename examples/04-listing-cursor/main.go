// Package main demonstrates v3 cursor pagination idioms across the Midaz SDK.
//
// Cursor-based endpoints (Transactions, Operations, OperationRoutes,
// TransactionRoutes) advance pages by setting opts.Cursor =
// page.Pagination.NextCursor — they have no Page/Offset field by design.
// This compile-time guarantee replaces v2's silent-drop footgun (audit
// finding 5.5).
//
// This file shows three idioms in increasing convenience:
//
//  1. Manual cursor loop — calling List* once per page, advancing the
//     cursor by hand. The wire-level shape; useful when you need explicit
//     control over per-page behavior.
//
//  2. ListTransactionsPages — an iter.Seq2 over *ListResponse[T] pages.
//     Use when you want page-level metadata (Pagination, ItemCount) but
//     don't want to manage the cursor yourself.
//
//  3. ListTransactionsAll — an iter.Seq2 over individual T values.
//     The flattened, range-loop-friendly form. Use this 90% of the time.
//
// The same patterns apply to ListOperations*, ListOperationRoutes*, and
// ListTransactionRoutes* — substitute the entity name and Filters type.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3"
	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
)

func main() {
	orgID := getEnv("MIDAZ_ORG_ID", "org-123")
	ledgerID := getEnv("MIDAZ_LEDGER_ID", "ledger-456")
	accountID := getEnv("MIDAZ_ACCOUNT_ID", "account-789")

	// v3 requires exactly one auth source. When PLUGIN_AUTH_ENABLED is unset
	// (typical for local dev), FromEnvironment doesn't install one — add the
	// explicit Anonymous opt-out at the config layer so NewConfig validates.
	cfgOpts := []config.Option{config.FromEnvironment()}
	if os.Getenv("PLUGIN_AUTH_ENABLED") != "true" {
		cfgOpts = append(cfgOpts, config.WithAnonymous())
	}

	cfg, err := config.NewConfig(cfgOpts...)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	c, err := midaz.New(midaz.WithConfig(cfg))
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Cursor pagination examples")
	fmt.Println("==========================")

	if err := manualCursorLoop(ctx, c, orgID, ledgerID); err != nil {
		log.Printf("manualCursorLoop: %v", err)
	}

	if err := pageIterator(ctx, c, orgID, ledgerID); err != nil {
		log.Printf("pageIterator: %v", err)
	}

	if err := flatItemIterator(ctx, c, orgID, ledgerID, accountID); err != nil {
		log.Printf("flatItemIterator: %v", err)
	}

	if err := earlyTermination(ctx, c, orgID, ledgerID); err != nil {
		log.Printf("earlyTermination: %v", err)
	}
}

// manualCursorLoop demonstrates the wire-level cursor advance pattern.
// Useful when you need to inspect Pagination metadata between pages or
// implement custom per-page behavior (logging, rate limiting, batching).
func manualCursorLoop(ctx context.Context, c *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n[1] Manual cursor loop — explicit page advance")

	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 50},
		Filters:        models.TransactionsFilters{Status: "APPROVED"},
	}

	pageNum := 0
	for {
		page, err := c.Transactions.ListTransactions(ctx, orgID, ledgerID, opts)
		if err != nil {
			return fmt.Errorf("list transactions page %d: %w", pageNum, err)
		}

		pageNum++
		fmt.Printf("  page %d: %d items, hasMore=%v\n", pageNum, len(page.Items), page.Pagination.HasMore())

		if page.Pagination.NextCursor == "" {
			break
		}

		opts.Cursor = page.Pagination.NextCursor
	}

	return nil
}

// pageIterator demonstrates ListTransactionsPages — the iter.Seq2 over
// *ListResponse pages. The cursor advance is fully automated; you keep
// page-level metadata access.
func pageIterator(ctx context.Context, c *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n[2] ListTransactionsPages — page-level iter.Seq2")

	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 50},
		Filters:        models.TransactionsFilters{AssetCode: "USD"},
	}

	pageNum := 0
	for page, err := range c.Transactions.ListTransactionsPages(ctx, orgID, ledgerID, opts) {
		if err != nil {
			return fmt.Errorf("page iter: %w", err)
		}

		pageNum++
		fmt.Printf("  page %d: %d items\n", pageNum, len(page.Items))
	}

	return nil
}

// flatItemIterator demonstrates ListOperationsAll — the most idiomatic v3
// pattern. Cursor advance and item flattening are handled internally; the
// caller writes a normal range loop. This is the form that should be used
// in 90%+ of cases.
func flatItemIterator(ctx context.Context, c *midaz.Client, orgID, ledgerID, accountID string) error {
	fmt.Println("\n[3] ListOperationsAll — flat item iter.Seq2")

	opts := models.OperationsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 100},
		Filters:        models.OperationsFilters{Type: "debit"},
	}

	count := 0
	for op, err := range c.Operations.ListOperationsAll(ctx, orgID, ledgerID, accountID, opts) {
		if err != nil {
			return fmt.Errorf("operation iter: %w", err)
		}

		_ = op
		count++

		if count%500 == 0 {
			fmt.Printf("  processed %d operations\n", count)
		}
	}

	fmt.Printf("  total: %d operations\n", count)
	return nil
}

// earlyTermination shows that range-over-func honors break — the cursor
// loop stops cleanly when the caller is done. No leaked goroutines, no
// dangling HTTP connections.
func earlyTermination(ctx context.Context, c *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n[4] Early termination — break stops cursor advance")

	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 25},
	}

	for tx, err := range c.Transactions.ListTransactionsAll(ctx, orgID, ledgerID, opts) {
		if err != nil {
			return fmt.Errorf("tx iter: %w", err)
		}

		if tx.Status.Code == "REJECTED" {
			fmt.Printf("  found first rejected tx %s, stopping\n", tx.ID)
			break
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
