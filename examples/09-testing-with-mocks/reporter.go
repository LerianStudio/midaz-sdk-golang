// Package main demonstrates unit-testing your code against the Midaz
// SDK using go.uber.org/mock.
//
// The pattern: depend on the SDK's service INTERFACE (e.g.,
// entities.AccountsService) rather than the concrete *midaz.Client.
// In production you pass c.Accounts; in tests you pass a generated mock.
//
// This file defines a small AccountReporter that totals balances across
// every account in a ledger. The companion reporter_test.go exercises
// it against MockAccountsService — no network, no Midaz instance, no
// flaky integration tests.
package main

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v4/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
)

// AccountReporter computes summary statistics across a ledger's
// accounts. It depends only on the SDK's AccountsService interface,
// which makes it trivially mockable.
type AccountReporter struct {
	accounts entities.AccountsService
}

// NewAccountReporter wires a reporter to any AccountsService —
// in production, c.Accounts; in tests, a generated mock.
func NewAccountReporter(svc entities.AccountsService) *AccountReporter {
	return &AccountReporter{accounts: svc}
}

// CountAccounts walks every account in a ledger using the iter.Seq2
// trio's ListAccountsAll variant. It demonstrates the recommended
// shape for code that needs collection-level totals: think 'collection,'
// not 'page.'
func (r *AccountReporter) CountAccounts(ctx context.Context, organizationID, ledgerID string) (int, error) {
	count := 0
	for _, err := range r.accounts.ListAccountsAll(ctx, organizationID, ledgerID, models.AccountsListOpts{
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
	acc, err := r.accounts.GetAccountByAlias(ctx, organizationID, ledgerID, alias)
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
