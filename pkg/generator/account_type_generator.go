package generator

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

type accountTypeGenerator struct {
	e   *entities.Entity
	obs observability.Provider
	mc  *observability.MetricsCollector
}

// NewAccountTypeGenerator creates a new account type generator.
func NewAccountTypeGenerator(e *entities.Entity, obs observability.Provider) AccountTypeGenerator {
	var mc *observability.MetricsCollector
	if obs != nil && obs.IsEnabled() {
		if c, err := observability.NewMetricsCollector(obs); err == nil {
			mc = c
		}
	}
	return &accountTypeGenerator{e: e, obs: obs, mc: mc}
}

func (g *accountTypeGenerator) Generate(ctx context.Context, orgID, ledgerID string, name, key string, metadata map[string]any) (*models.AccountType, error) {
	if g.e == nil || g.e.AccountTypes == nil {
		return nil, fmt.Errorf("entity account types service not initialized")
	}
	input := models.NewCreateAccountTypeInput(name, key).WithMetadata(metadata)
	var out *models.AccountType
	err := observability.WithSpan(ctx, g.obs, "GenerateAccountType", func(ctx context.Context) error {
		return retry.DoWithContext(ctx, func() error {
			at, err := g.e.AccountTypes.CreateAccountType(ctx, orgID, ledgerID, input)
			if err != nil {
				return err
			}
			out = at
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GenerateDefaults creates a default set of commonly used account types.
func (g *accountTypeGenerator) GenerateDefaults(ctx context.Context, orgID, ledgerID string) ([]*models.AccountType, error) {
    defs := []struct {
        name string
        key  string
        meta map[string]any
    }{
        {"Checking", "CHECKING", map[string]any{"category": "deposit", "overdraft": false}},
        {"Savings", "SAVINGS", map[string]any{"category": "savings", "interest": true}},
        {"Credit Card", "CREDIT_CARD", map[string]any{"category": "credit", "limit_supported": true}},
        {"Expense", "EXPENSE", map[string]any{"category": "expense"}},
        {"Revenue", "REVENUE", map[string]any{"category": "revenue"}},
        {"Liability", "LIABILITY", map[string]any{"category": "liability"}},
        {"Equity", "EQUITY", map[string]any{"category": "equity"}},
    }

	out := make([]*models.AccountType, 0, len(defs))
	for _, d := range defs {
		at, err := g.Generate(ctx, orgID, ledgerID, d.name, d.key, d.meta)
		if err != nil {
			return nil, err
		}
		out = append(out, at)
	}
	return out, nil
}
