package generator

import (
	"context"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type accountTypeGenerator struct {
	e   *entities.Entity
	obs observability.Provider
}

// NewAccountTypeGenerator creates a new account type generator.
func NewAccountTypeGenerator(e *entities.Entity, obs observability.Provider) AccountTypeGenerator {
	return &accountTypeGenerator{e: e, obs: obs}
}

// Generate creates a new account type with the specified name, key, and metadata.
func (g *accountTypeGenerator) Generate(ctx context.Context, orgID, ledgerID string, name, key string, metadata map[string]any) (*models.AccountType, error) {
	if g.e == nil || g.e.AccountTypes == nil {
		return nil, errors.New("entity account types service not initialized")
	}

	input := models.NewCreateAccountTypeInput(name, key).WithMetadata(metadata)

	var out *models.AccountType

	err := observability.WithSpan(ctx, g.obs, "GenerateAccountType", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				at, err := g.e.AccountTypes.CreateAccountType(ctx, orgID, ledgerID, input)
				if err != nil {
					return err
				}

				out = at

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// GenerateDefaults creates a default set of commonly used account types.
// Returns partial results along with any accumulated errors.
func (g *accountTypeGenerator) GenerateDefaults(ctx context.Context, orgID, ledgerID string) ([]*models.AccountType, error) {
	defs := []struct {
		name string
		key  string
		meta map[string]any
	}{
		{"Checking", AccountTypeKeyChecking, map[string]any{"category": "deposit", "overdraft": false}},
		{"Savings", AccountTypeKeySavings, map[string]any{"category": "savings", "interest": true}},
		{"Credit Card", AccountTypeKeyCreditCard, map[string]any{"category": "credit", "limit_supported": true}},
		{"Expense", AccountTypeKeyExpense, map[string]any{"category": "expense"}},
		{"Revenue", AccountTypeKeyRevenue, map[string]any{"category": "revenue"}},
		{"Liability", AccountTypeKeyLiability, map[string]any{"category": "liability"}},
		{"Equity", AccountTypeKeyEquity, map[string]any{"category": "equity"}},
	}

	out := make([]*models.AccountType, 0, len(defs))

	var errs []error

	for _, d := range defs {
		at, err := g.Generate(ctx, orgID, ledgerID, d.name, d.key, d.meta)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create account type %s: %w", d.key, err))
			continue
		}

		out = append(out, at)
	}

	if len(errs) > 0 {
		return out, errors.Join(errs...)
	}

	return out, nil
}
