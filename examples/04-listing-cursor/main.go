// Package main demonstrates cursor pagination idioms across the Midaz SDK.
//
// Cursor-based endpoints (Transactions, Operations, OperationRoutes,
// TransactionRoutes) advance pages by setting opts.Cursor =
// page.Pagination.NextCursor — they have no Page/Offset field by design.
// This compile-time guarantee replaces the silent-drop footgun of SDK v2
// — the SDK generation, not the /v2 ledger surface this file calls (audit
// finding 5.5).
//
// This file shows three idioms in increasing convenience:
//
//  1. Manual cursor loop — calling List* once per page, advancing the
//     cursor by hand. The wire-level shape; useful when you need explicit
//     control over per-page behavior.
//
//  2. V2.Transactions.Pages — an iter.Seq2 over *ListResponse[T] pages.
//     Use when you want page-level metadata (Pagination, ItemCount) but
//     don't want to manage the cursor yourself.
//
//  3. V2.Transactions.All — an iter.Seq2 over individual T values.
//     The flattened, range-loop-friendly form. Use this 90% of the time.
//
// The same three shapes exist on every cursor-paginated surface — operations,
// operation routes, transaction routes — as List / Pages / All under the
// matching accessor; substitute the entity name and Filters type.
//
// WHAT A TRANSACTION LIST CAN ACTUALLY NARROW BY. The date range, the sort
// direction and ONE metadata predicate — nothing else. Six fields on
// models.TransactionsFilters (Status, AssetCode, Reference, SourceAccount,
// DestinationAccount, Route) are refused before the request is built, on BOTH
// surfaces, because the ledger never honored them: it parses two of them and
// drops them on the floor, and never parses the other four. Setting one used
// to return the whole unfiltered ledger with a nil error. Status and Route are
// honored by Count — see countByStatus below.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
)

func main() {
	orgID := getEnv("MIDAZ_ORG_ID", "org-123")
	ledgerID := getEnv("MIDAZ_LEDGER_ID", "ledger-456")
	accountID := getEnv("MIDAZ_ACCOUNT_ID", "account-789")

	// The SDK requires exactly one auth source. When PLUGIN_AUTH_ENABLED is unset
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

	if err := countByStatus(ctx, c, orgID, ledgerID); err != nil {
		log.Printf("countByStatus: %v", err)
	}
}

// manualCursorLoop demonstrates the wire-level cursor advance pattern.
// Useful when you need to inspect Pagination metadata between pages or
// implement custom per-page behavior (logging, rate limiting, batching).
func manualCursorLoop(ctx context.Context, c *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n[1] Manual cursor loop — explicit page advance")

	// A date range is one of the three things this endpoint narrows by.
	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 50, StartDate: "2026-01-01"},
	}

	pageNum := 0
	for {
		page, err := c.V2.Transactions.List(ctx, orgID, ledgerID, opts)
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

// pageIterator demonstrates V2.Transactions.Pages — the iter.Seq2 over
// *ListResponse pages. The cursor advance is fully automated; you keep
// page-level metadata access.
func pageIterator(ctx context.Context, c *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n[2] V2.Transactions.Pages — page-level iter.Seq2")

	// The metadata pair is the ONLY content filter a transaction list honors,
	// which is why a correlation identifier belongs in the transaction's
	// metadata at creation time rather than in a field the list cannot see.
	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 50},
		Filters: models.TransactionsFilters{
			MetadataKey:   "settlementBatch",
			MetadataValue: "2026-01-15-EU",
		},
	}

	pageNum := 0
	for page, err := range c.V2.Transactions.Pages(ctx, orgID, ledgerID, opts) {
		if err != nil {
			return fmt.Errorf("page iter: %w", err)
		}

		pageNum++
		fmt.Printf("  page %d: %d items\n", pageNum, len(page.Items))
	}

	return nil
}

// flatItemIterator demonstrates ListOperationsAll — the most idiomatic
// pattern. Cursor advance and item flattening are handled internally; the
// caller writes a normal range loop. This is the form that should be used
// in 90%+ of cases.
func flatItemIterator(ctx context.Context, c *midaz.Client, orgID, ledgerID, accountID string) error {
	fmt.Println("\n[3] ListOperationsAll — flat item iter.Seq2")

	opts := models.OperationsListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 100},
		Filters:        models.OperationsFilters{Type: "DEBIT"},
	}

	count := 0
	for op, err := range c.V2.Operations.ListOperationsAll(ctx, orgID, ledgerID, accountID, opts) {
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

	for tx, err := range c.V2.Transactions.All(ctx, orgID, ledgerID, opts) {
		if err != nil {
			return fmt.Errorf("tx iter: %w", err)
		}

		// Stop at the first terminal cancelled transaction. CANCELED is a real
		// server status (a PENDING transaction cancelled before commit); the
		// server never emits REJECTED.
		if tx.Status.Code == string(models.TransactionStatusCanceled) {
			fmt.Printf("  found first canceled tx %s, stopping\n", tx.ID)
			break
		}
	}

	return nil
}

// countByStatus shows where Status and Route narrowing DID survive: the count
// endpoint honors both, and it is the only place on the transaction surface
// that does.
//
// The trap worth knowing: Count's default window is TODAY, not the ledger.
// Leave StartDate and EndDate unset and the server fills in the current UTC
// day, so a zero-options Count answers "how many transactions today" — a
// plausible-looking number a caller reading it as the ledger total will
// misread. The dates take the same YYYY-MM-DD spelling List takes, and both
// bounds name a whole, inclusive day.
//
// The window is RELATIVE to now (the last 30 UTC days) rather than a fixed
// calendar month. The seed flows create transactions when they run, so a hard
// date range demonstrates status filtering only for as long as that range
// happens to contain the seed data — after which the example prints a confident
// 0 and teaches the opposite of its own lesson. Override the span with
// DEMO_COUNT_DAYS.
func countByStatus(ctx context.Context, c *midaz.Client, orgID, ledgerID string) error {
	fmt.Println("\n[5] V2.Transactions.Count — where Status and Route do narrow")

	days := 30
	if parsed, err := strconv.Atoi(getEnv("DEMO_COUNT_DAYS", "30")); err == nil && parsed > 0 {
		days = parsed
	}

	// Both bounds are inclusive whole days, so a span of N dates subtracts
	// N-1: today alone is days=1, not days=0.
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -(days - 1))

	opts := models.TransactionsListOpts{
		CursorListOpts: models.CursorListOpts{
			StartDate: start.Format(time.DateOnly),
			EndDate:   end.Format(time.DateOnly),
		},
		Filters: models.TransactionsFilters{
			Status: string(models.TransactionStatusApproved),
		},
	}

	n, err := c.V2.Transactions.Count(ctx, orgID, ledgerID, opts)
	if err != nil {
		return fmt.Errorf("count transactions: %w", err)
	}

	fmt.Printf("  approved between %s and %s: %d\n", opts.StartDate, opts.EndDate, n)

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
