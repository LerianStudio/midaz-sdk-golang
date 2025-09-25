package generator

import (
	"context"
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/stats"
)

type orgGenerator struct {
	e   *entities.Entity
	obs observability.Provider
	mc  *observability.MetricsCollector
}

// NewOrganizationGenerator creates a new OrganizationGenerator backed by entities API.
func NewOrganizationGenerator(e *entities.Entity, obs observability.Provider) OrganizationGenerator {
	var mc *observability.MetricsCollector
	if obs != nil && obs.IsEnabled() {
		if c, err := observability.NewMetricsCollector(obs); err == nil {
			mc = c
		}
	}
	return &orgGenerator{e: e, obs: obs, mc: mc}
}

func (g *orgGenerator) Generate(ctx context.Context, template data.OrgTemplate) (*models.Organization, error) {
	if g.e == nil || g.e.Organizations == nil {
		return nil, fmt.Errorf("entity organizations service not initialized")
	}

	input := models.NewCreateOrganizationInput(template.LegalName).
		WithDoingBusinessAs(template.TradeName).
		WithLegalDocument(template.TaxID).
		WithStatus(template.Status).
		WithAddress(template.Address).
		WithMetadata(template.Metadata)

	var out *models.Organization
	err := observability.WithSpan(ctx, g.obs, "GenerateOrganization", func(ctx context.Context) error {
		// Respect retry + circuit breaker options from context if present
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				org, err := g.e.Organizations.CreateOrganization(ctx, input)
				if err != nil {
					return err
				}
				out = org
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (g *orgGenerator) GenerateBatch(ctx context.Context, count int) ([]*models.Organization, error) {
	if count <= 0 {
		return []*models.Organization{}, nil
	}

	// Prepare work items (templates)
	items := make([]int, count)
	for i := 0; i < count; i++ {
		items[i] = i
	}

	// Metrics timer for the whole batch
	var timer *observability.Timer
	if g.mc != nil {
		timer = g.mc.NewTimer(ctx, "organizations.batch.create", "organizations")
	}
	counter := stats.NewCounter()

	// Decide workers and buffering
	workers := getWorkers(ctx)
	buf := workers * 2
	results := concurrent.WorkerPool(ctx, items, func(ctx context.Context, i int) (*models.Organization, error) {
		t := data.OrgTemplate{
			LegalName: fmt.Sprintf("Demo Org %d", i+1),
			TradeName: fmt.Sprintf("DemoOrg%d", i+1),
			TaxID:     fmt.Sprintf("00-000%04d", i+1),
			Address:   models.NewAddress("100 Demo St", "00000", "Demo City", "DC", "US"),
			Status:    models.NewStatus(models.StatusActive),
			Metadata:  map[string]any{"source": "generator", "created_at": time.Now().Format(time.RFC3339)},
			Industry:  "demo",
			Size:      "small",
		}
		org, err := g.Generate(ctx, t)
		if err == nil {
			counter.RecordSuccess()
		}
		return org, err
	}, concurrent.WithWorkers(workers), concurrent.WithBufferSize(buf))

	// Collect results and check errors
	out := make([]*models.Organization, 0, count)
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

	// Log batch TPS
	if g.obs != nil && g.obs.IsEnabled() && g.obs.Logger() != nil {
		g.obs.Logger().Infof("organizations: created=%d tps=%.2f", counter.SuccessCount(), counter.TPS())
	}

	if len(errs) > 0 {
		return out, errorsJoin(errs...)
	}
	return out, nil
}
