package generator

import (
    "context"
    "fmt"

    "github.com/LerianStudio/midaz-sdk-golang/v2/entities"
    "github.com/LerianStudio/midaz-sdk-golang/v2/models"
    data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/stats"
)

type ledgerGenerator struct {
    e           *entities.Entity
    obs         observability.Provider
    defaultOrg  string
    mc          *observability.MetricsCollector
}

// NewLedgerGenerator creates a new ledger generator; defaultOrg is used for list operations.
func NewLedgerGenerator(e *entities.Entity, obs observability.Provider, defaultOrg string) LedgerGenerator {
    var mc *observability.MetricsCollector
    if obs != nil && obs.IsEnabled() {
        if c, err := observability.NewMetricsCollector(obs); err == nil {
            mc = c
        }
    }
    return &ledgerGenerator{e: e, obs: obs, defaultOrg: defaultOrg, mc: mc}
}

func (g *ledgerGenerator) Generate(ctx context.Context, orgID string, template data.LedgerTemplate) (*models.Ledger, error) {
    if g.e == nil || g.e.Ledgers == nil {
        return nil, fmt.Errorf("entity ledgers service not initialized")
    }
    if orgID == "" {
        return nil, fmt.Errorf("organization id is required")
    }

    input := models.NewCreateLedgerInput(template.Name).
        WithStatus(template.Status).
        WithMetadata(template.Metadata)

    var out *models.Ledger
    err := observability.WithSpan(ctx, g.obs, "GenerateLedger", func(ctx context.Context) error {
        return executeWithCircuitBreaker(ctx, func() error {
            return retry.DoWithContext(ctx, func() error {
                ledger, err := g.e.Ledgers.CreateLedger(ctx, orgID, input)
                if err != nil {
                    return err
                }
                out = ledger
                return nil
            })
        })
    })
    if err != nil {
        return nil, err
    }
    return out, nil
}

func (g *ledgerGenerator) GenerateForOrg(ctx context.Context, orgID string, count int) ([]*models.Ledger, error) {
    if count <= 0 {
        return []*models.Ledger{}, nil
    }

    items := make([]int, count)
    for i := 0; i < count; i++ {
        items[i] = i
    }

    var timer *observability.Timer
    if g.mc != nil {
        timer = g.mc.NewTimer(ctx, "ledgers.batch.create", "ledgers")
    }
    counter := stats.NewCounter()

    workers := getWorkers(ctx)
    buf := workers * 2
    results := concurrent.WorkerPool(ctx, items, func(ctx context.Context, i int) (*models.Ledger, error) {
        t := data.LedgerTemplate{
            Name:     fmt.Sprintf("Demo Ledger %d", i+1),
            Status:   models.NewStatus(models.StatusActive),
            Metadata: map[string]any{"purpose": "operational"},
        }
        ledger, err := g.Generate(ctx, orgID, t)
        if err == nil {
            counter.RecordSuccess()
        }
        return ledger, err
    }, concurrent.WithWorkers(workers), concurrent.WithBufferSize(buf))

    out := make([]*models.Ledger, 0, count)
    for _, r := range results {
        if r.Error != nil {
            if timer != nil {
                timer.StopBatch(len(out))
            }
            return nil, r.Error
        }
        out = append(out, r.Value)
    }

    if timer != nil {
        timer.StopBatch(len(out))
    }

    if g.obs != nil && g.obs.IsEnabled() && g.obs.Logger() != nil {
        g.obs.Logger().Infof("ledgers: created=%d tps=%.2f", counter.SuccessCount(), counter.TPS())
    }

    return out, nil
}

func (g *ledgerGenerator) ListWithPagination(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error) {
    if g.defaultOrg == "" {
        return nil, fmt.Errorf("default organization id not configured for listing")
    }
    var out *models.ListResponse[models.Ledger]
    err := observability.WithSpan(ctx, g.obs, "ListLedgersWithPagination", func(ctx context.Context) error {
        return retry.DoWithContext(ctx, func() error {
            resp, err := g.e.Ledgers.ListLedgers(ctx, g.defaultOrg, opts)
            if err != nil {
                return err
            }
            out = resp
            return nil
        })
    })
    if err != nil {
        return nil, err
    }
    return out, nil
}
