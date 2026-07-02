package generator

import (
	"context"
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v4/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
)

// accountTypesAPI is the narrow slice of the account-types facade this generator needs.
type accountTypesAPI interface {
	Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountTypeInput) (*models.AccountType, error)
}

type accountTypeGenerator struct {
	accountTypes accountTypesAPI
	obs          observability.Provider
}

// NewAccountTypeGenerator creates a new account type generator.
func NewAccountTypeGenerator(e *entities.Entity, obs observability.Provider) AccountTypeGenerator {
	g := &accountTypeGenerator{obs: obs}
	if e != nil && e.AccountTypes != nil {
		g.accountTypes = e.AccountTypes
	}

	return g
}

// Generate creates a new account type with the specified name, key, and metadata.
func (g *accountTypeGenerator) Generate(ctx context.Context, organizationID, ledgerID string, name, key string, metadata map[string]any) (*models.AccountType, error) {
	if g.accountTypes == nil {
		return nil, errors.New("entity account types service not initialized")
	}

	input := models.NewCreateAccountTypeInput(name, key).WithMetadata(metadata)

	var out *models.AccountType

	err := observability.WithSpan(ctx, g.obs, "GenerateAccountType", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				at, err := g.accountTypes.Create(ctx, organizationID, ledgerID, input)
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
func (g *accountTypeGenerator) GenerateDefaults(ctx context.Context, organizationID, ledgerID string) ([]*models.AccountType, error) {
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
		at, err := g.Generate(ctx, organizationID, ledgerID, d.name, d.key, d.meta)
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
