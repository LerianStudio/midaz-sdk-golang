package generator

import (
	"context"
	"fmt"

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
	if g.e == nil || g.e.Accounts == nil {
		return nil, fmt.Errorf("entity accounts service not initialized")
	}
	if orgID == "" || ledgerID == "" {
		return nil, fmt.Errorf("organization and ledger IDs are required")
	}
	if assetCode == "" {
		return nil, fmt.Errorf("asset code is required for account creation")
	}

	// Map template type to accounting class
	accountClass := mapAccountClass(t.Type)

	in := models.NewCreateAccountInput(t.Name, assetCode, accountClass).
		WithStatus(t.Status).
		WithMetadata(t.Metadata)
	if t.Alias != nil && *t.Alias != "" {
		in = in.WithAlias(*t.Alias)
	}
	if t.ParentAccountID != nil && *t.ParentAccountID != "" {
		in = in.WithParentAccountID(*t.ParentAccountID)
	}
	if t.PortfolioID != nil && *t.PortfolioID != "" {
		in = in.WithPortfolioID(*t.PortfolioID)
	}
	if t.SegmentID != nil && *t.SegmentID != "" {
		in = in.WithSegmentID(*t.SegmentID)
	}
	if t.EntityID != nil && *t.EntityID != "" {
		in = in.WithEntityID(*t.EntityID)
	}

	// Ensure linkage to AccountType via metadata if provided
	if t.AccountTypeKey != nil && *t.AccountTypeKey != "" {
		if in.Metadata == nil {
			in.Metadata = map[string]any{}
		}
		in.Metadata["account_type_key"] = *t.AccountTypeKey
	}

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
	case "creditCard":
		return "LIABILITY"
	default:
		return "ASSET"
	}
}
