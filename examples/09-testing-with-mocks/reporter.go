// Package main demonstrates unit-testing your code against the Midaz
// SDK using a consumer-defined interface.
//
// The idiomatic v4 pattern: depend on a NARROW interface you declare
// yourself — only the methods you actually call — rather than the
// concrete *midaz.Client or a broad SDK interface. In production you
// pass c.V2.Accounts (the facade satisfies your interface structurally);
// in tests you pass a tiny local mock. "Accept interfaces, return
// structs."
//
// This file defines a small AccountReporter that totals balances across
// every account in a ledger. The companion reporter_test.go exercises
// it against a hand-written mock — no network, no Midaz instance, no
// flaky integration tests.
package main

import (
	"context"
	"fmt"
	"iter"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// accountSource is the narrow slice of the Accounts facade this reporter
// needs. c.V2.Accounts satisfies it in production; a local stub satisfies it
// in tests. Declaring the interface on the consumer side (not importing a
// broad SDK interface) is the idiomatic v4 testing pattern.
type accountSource interface {
	All(ctx context.Context, orgID, ledgerID string, opts models.AccountsListOpts) iter.Seq2[models.Account, error]
	GetByAlias(ctx context.Context, orgID, ledgerID, alias string) (*models.Account, error)
}

// AccountReporter computes summary statistics across a ledger's
// accounts. It depends only on the narrow accountSource interface,
// which makes it trivially mockable.
type AccountReporter struct {
	accounts accountSource
}

// NewAccountReporter wires a reporter to any accountSource —
// in production, c.V2.Accounts; in tests, a local mock.
func NewAccountReporter(svc accountSource) *AccountReporter {
	return &AccountReporter{accounts: svc}
}

// CountAccounts walks every account in a ledger using the iter.Seq2
// trio's All variant. It demonstrates the recommended shape for code
// that needs collection-level totals: think 'collection,' not 'page.'
func (r *AccountReporter) CountAccounts(ctx context.Context, organizationID, ledgerID string) (int, error) {
	count := 0
	for _, err := range r.accounts.All(ctx, organizationID, ledgerID, models.AccountsListOpts{
		PageListOpts: models.PageListOpts{Limit: 50},
	}) {
		if err != nil {
			return count, fmt.Errorf("list accounts: %w", err)
		}
		count++
	}
	return count, nil
}

// FindByAlias retrieves a single account by its alias. Demonstrates
// straightforward error propagation and the typed *models.Account
// return shape.
func (r *AccountReporter) FindByAlias(ctx context.Context, organizationID, ledgerID, alias string) (*models.Account, error) {
	acc, err := r.accounts.GetByAlias(ctx, organizationID, ledgerID, alias)
	if err != nil {
		return nil, fmt.Errorf("get account by alias %q: %w", alias, err)
	}
	return acc, nil
}

// main is intentionally empty — this example is exercised through
// reporter_test.go via `go test`. There is nothing to demonstrate at
// runtime; the lesson is in the test file.
func main() {
	fmt.Println("This example demonstrates unit-testing patterns.")
	fmt.Println("Run: go test ./examples/09-testing-with-mocks/...")
}
