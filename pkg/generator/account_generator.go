package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/stats"
)

type accountGenerator struct {
	e   *entities.Entity
	obs observability.Provider
	mc  *observability.MetricsCollector
}

// NewAccountGenerator creates a new AccountGenerator backed by entities API.
func NewAccountGenerator(e *entities.Entity, obs observability.Provider) AccountGenerator {
	var mc *observability.MetricsCollector

	if obs != nil && obs.IsEnabled() {
		if c, err := observability.NewMetricsCollector(obs); err == nil {
			mc = c
		}
	}

	return &accountGenerator{e: e, obs: obs, mc: mc}
}

func (g *accountGenerator) Generate(ctx context.Context, orgID, ledgerID, assetCode string, t data.AccountTemplate) (*models.Account, error) {
	if err := g.validateInputs(orgID, ledgerID, assetCode); err != nil {
		return nil, err
	}

	in := g.buildAccountInput(t, assetCode)
	g.applyTemplateFields(in, t)
	g.setupAccountTypeMetadata(in, t)

	return g.createAccount(ctx, orgID, ledgerID, in)
}

// validateInputs validates the required inputs for account generation
func (g *accountGenerator) validateInputs(orgID, ledgerID, assetCode string) error {
	if g.e == nil || g.e.Accounts == nil {
		return errors.New("entity accounts service not initialized")
	}

	if orgID == "" || ledgerID == "" {
		return errors.New("organization and ledger IDs are required")
	}

	if assetCode == "" {
		return errors.New("asset code is required for account creation")
	}

	return nil
}

// buildAccountInput creates the basic account input from template
func (*accountGenerator) buildAccountInput(t data.AccountTemplate, assetCode string) *models.CreateAccountInput {
	accountClass := mapAccountClass(t.Type)

	return models.NewCreateAccountInput(t.Name, assetCode, accountClass).
		WithStatus(t.Status).
		WithMetadata(t.Metadata)
}

// applyTemplateFields applies optional template fields to the account input
func (*accountGenerator) applyTemplateFields(in *models.CreateAccountInput, t data.AccountTemplate) {
	if t.Alias != nil && *t.Alias != "" {
		*in = *in.WithAlias(*t.Alias)
	}

	if t.ParentAccountID != nil && *t.ParentAccountID != "" {
		*in = *in.WithParentAccountID(*t.ParentAccountID)
	}

	if t.PortfolioID != nil && *t.PortfolioID != "" {
		*in = *in.WithPortfolioID(*t.PortfolioID)
	}

	if t.SegmentID != nil && *t.SegmentID != "" {
		*in = *in.WithSegmentID(*t.SegmentID)
	}

	if t.EntityID != nil && *t.EntityID != "" {
		*in = *in.WithEntityID(*t.EntityID)
	}
}

// setupAccountTypeMetadata configures account type metadata based on template
func (g *accountGenerator) setupAccountTypeMetadata(in *models.CreateAccountInput, t data.AccountTemplate) {
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}

	if t.AccountTypeKey != nil && *t.AccountTypeKey != "" {
		g.applyProvidedAccountTypeKey(in, *t.AccountTypeKey, t.Type)
	} else {
		g.applyInferredAccountTypeKey(in, t.Type)
	}
}

// applyProvidedAccountTypeKey applies a provided account type key with validation
func (g *accountGenerator) applyProvidedAccountTypeKey(in *models.CreateAccountInput, key, templateType string) {
	if isSupportedAccountTypeKey(key) {
		in.Metadata["account_type_key"] = key
	} else {
		// Fallback to inferred key if invalid provided
		g.applyInferredAccountTypeKey(in, templateType)
	}
}

// applyInferredAccountTypeKey applies an inferred account type key
func (*accountGenerator) applyInferredAccountTypeKey(in *models.CreateAccountInput, templateType string) {
	if k := inferAccountTypeKey(templateType); k != "" {
		in.Metadata["account_type_key"] = k
	}
}

// createAccount creates the account with observability and error handling
func (g *accountGenerator) createAccount(ctx context.Context, orgID, ledgerID string, in *models.CreateAccountInput) (*models.Account, error) {
	var out *models.Account

	err := observability.WithSpan(ctx, g.obs, "GenerateAccount", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				acc, err := g.e.Accounts.CreateAccount(ctx, orgID, ledgerID, in)
				if err != nil {
					return err
				}

				out = acc

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (g *accountGenerator) GenerateBatch(ctx context.Context, orgID, ledgerID, assetCode string, templates []data.AccountTemplate) ([]*models.Account, error) {
	if len(templates) == 0 {
		return []*models.Account{}, nil
	}

	var timer *observability.Timer
	if g.mc != nil {
		timer = g.mc.NewTimer(ctx, "accounts.batch.create", "accounts")
	}

	counter := stats.NewCounter()

	items := make([]int, len(templates))
	for i := range templates {
		items[i] = i
	}

	workers := getWorkers(ctx)
	buf := workers * 2
	results := concurrent.WorkerPool(ctx, items, func(ctx context.Context, idx int) (*models.Account, error) {
		acc, err := g.Generate(ctx, orgID, ledgerID, assetCode, templates[idx])
		if err == nil {
			counter.RecordSuccess()
		}

		return acc, err
	}, concurrent.WithWorkers(workers), concurrent.WithBufferSize(buf))

	out := make([]*models.Account, 0, len(templates))

	var errs []error

	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, r.Error)
			continue
		}

		out = append(out, r.Value)
	}

	if timer != nil {
		timer.StopBatch(len(out))
	}

	if g.obs != nil && g.obs.IsEnabled() && g.obs.Logger() != nil {
		g.obs.Logger().Infof("accounts: created=%d tps=%.2f", counter.SuccessCount(), counter.TPS())
	}

	if len(errs) > 0 {
		// Aggregate errors while returning successful creations
		// Use errors.Join when multiple errors occurred
		// Fallback to first error if Join not available (Go >=1.20 supports Join)
		return out, errorsJoin(errs...)
	}

	return out, nil
}

// mapAccountClass maps a domain-specific template type to an accounting class.
// Defaults to ASSET when uncertain to ensure account creation succeeds in demos.
func mapAccountClass(t string) string {
	switch t {
	case "expense":
		return "EXPENSE"
	case "revenue":
		return "REVENUE"
	case "liability":
		return "LIABILITY"
	case "equity":
		return "EQUITY"
	case "creditCard":
		return "LIABILITY"
	default:
		return "ASSET"
	}
}

// inferAccountTypeKey maps a domain template type to a default AccountType key.
// Returns empty string when no mapping exists.
func inferAccountTypeKey(t string) string {
	switch t {
	case "deposit", "marketplace":
		return AccountTypeKeyChecking
	case "savings":
		return AccountTypeKeySavings
	case "creditCard":
		return AccountTypeKeyCreditCard
	case "expense":
		return AccountTypeKeyExpense
	case "revenue":
		return AccountTypeKeyRevenue
	case "liability":
		return AccountTypeKeyLiability
	case "equity":
		return AccountTypeKeyEquity
	default:
		return ""
	}
}

var supportedAccountTypeKeys = []string{
	AccountTypeKeyChecking,
	AccountTypeKeySavings,
	AccountTypeKeyCreditCard,
	AccountTypeKeyExpense,
	AccountTypeKeyRevenue,
	AccountTypeKeyLiability,
	AccountTypeKeyEquity,
}

func isSupportedAccountTypeKey(k string) bool {
	for _, key := range supportedAccountTypeKeys {
		if k == key {
			return true
		}
	}

	return false
}
