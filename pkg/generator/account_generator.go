package generator

import (
	"context"
	"errors"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/data"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/stats"
)

// accountsAPI is the narrow slice of the accounts facade this generator needs.
// Consumer-side interface (Epic 5.3 swap: client.Accounts is now a concrete
// *entities.accountsFacade); tests inject a mock satisfying just this.
type accountsAPI interface {
	Create(ctx context.Context, orgID, ledgerID string, input *models.CreateAccountInput) (*models.Account, error)
}

type accountGenerator struct {
	accounts accountsAPI
	obs      observability.Provider
	mc       *observability.MetricsCollector
}

// NewAccountGenerator creates a new AccountGenerator backed by entities API.
func NewAccountGenerator(e *entities.Entity, obs observability.Provider) AccountGenerator {
	var mc *observability.MetricsCollector

	if obs != nil && obs.IsEnabled() {
		if c, err := observability.NewMetricsCollector(obs); err == nil {
			mc = c
		}
	}

	g := &accountGenerator{obs: obs, mc: mc}
	if e != nil && e.Accounts != nil {
		g.accounts = e.Accounts
	}

	return g
}

// Generate creates a single account from the provided template.
func (g *accountGenerator) Generate(ctx context.Context, organizationID, ledgerID, assetCode string, t data.AccountTemplate) (*models.Account, error) {
	ctx = normalizeContext(ctx)

	if err := g.validateInputs(organizationID, ledgerID, assetCode); err != nil {
		return nil, err
	}

	in := g.buildAccountInput(t, assetCode)
	g.applyTemplateFields(in, t)
	g.setupAccountTypeMetadata(in, t)

	return g.createAccount(ctx, organizationID, ledgerID, in)
}

// validateInputs validates the required inputs for account generation
func (g *accountGenerator) validateInputs(organizationID, ledgerID, assetCode string) error {
	if g.accounts == nil {
		return errors.New("entity accounts service not initialized")
	}

	if organizationID == "" || ledgerID == "" {
		return errors.New("organization and ledger IDs are required")
	}

	if assetCode == "" {
		return errors.New("asset code is required for account creation")
	}

	return nil
}

// buildAccountInput creates the basic account input from template
func (*accountGenerator) buildAccountInput(t data.AccountTemplate, assetCode string) *models.CreateAccountInput {
	return models.NewCreateAccountInput(t.Name, assetCode, t.Type).
		WithStatus(t.Status).
		WithMetadata(t.Metadata)
}

func accountTypeKeyForTemplate(t data.AccountTemplate) string {
	if t.AccountTypeKey != nil && isSupportedAccountTypeKey(*t.AccountTypeKey) {
		return *t.AccountTypeKey
	}

	if key := inferAccountTypeKey(t.Type); key != "" {
		return key
	}

	return AccountTypeKeyChecking
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
func (g *accountGenerator) createAccount(ctx context.Context, organizationID, ledgerID string, in *models.CreateAccountInput) (*models.Account, error) {
	var out *models.Account

	err := observability.WithSpan(ctx, g.obs, "GenerateAccount", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				acc, err := g.accounts.Create(ctx, organizationID, ledgerID, in)
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

	if out == nil {
		return nil, errNilGenerated("account")
	}

	return out, nil
}

// GenerateBatch creates multiple accounts concurrently from the provided templates.
func (g *accountGenerator) GenerateBatch(ctx context.Context, organizationID, ledgerID, assetCode string, templates []data.AccountTemplate) ([]*models.Account, error) {
	ctx = normalizeContext(ctx)

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
		acc, err := g.Generate(ctx, organizationID, ledgerID, assetCode, templates[idx])
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

		if r.Value == nil {
			errs = append(errs, errNilGenerated("account"))
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
