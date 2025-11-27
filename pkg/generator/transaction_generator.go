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

type transactionGenerator struct {
	e   *entities.Entity
	obs observability.Provider
	mc  *observability.MetricsCollector
}

// NewTransactionGenerator creates a TransactionGenerator with observability and retry integration.
func NewTransactionGenerator(e *entities.Entity, obs observability.Provider) TransactionGenerator {
	var mc *observability.MetricsCollector

	if obs != nil && obs.IsEnabled() {
		if c, err := observability.NewMetricsCollector(obs); err == nil {
			mc = c
		}
	}

	return &transactionGenerator{e: e, obs: obs, mc: mc}
}

func (g *transactionGenerator) GenerateWithDSL(ctx context.Context, orgID, ledgerID string, pattern data.TransactionPattern) (*models.Transaction, error) {
	if g.e == nil || g.e.Transactions == nil {
		return nil, fmt.Errorf("entity transactions service not initialized")
	}

	if err := data.ValidateTransactionPattern(pattern); err != nil {
		return nil, err
	}

	var out *models.Transaction

	// Inject idempotency key into context so HTTP layer can add header
	if pattern.IdempotencyKey != "" {
		ctx = entities.WithIdempotencyKey(ctx, pattern.IdempotencyKey)
	}

	err := observability.WithSpan(ctx, g.obs, "GenerateTransactionDSL", func(ctx context.Context) error {
		return executeWithCircuitBreaker(ctx, func() error {
			return retry.DoWithContext(ctx, func() error {
				// Use DSL file endpoint for free-form DSL
				tx, err := g.e.Transactions.CreateTransactionWithDSLFile(ctx, orgID, ledgerID, []byte(pattern.DSLTemplate))
				if err != nil {
					return err
				}

				out = tx

				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	return out, nil
}

// setupThrottleTicker creates a ticker channel for TPS throttling.
// Returns the ticker channel (nil if no throttling) and a cleanup function.
func setupThrottleTicker(tps float64) (<-chan time.Time, func()) {
	if tps <= 0 {
		return nil, func() {}
	}

	interval := time.Duration(float64(time.Second) / tps)
	ticker := time.NewTicker(interval)

	return ticker.C, ticker.Stop
}

// collectBatchResults processes worker pool results and separates successes from errors.
func collectBatchResults(results []concurrent.Result[int, *models.Transaction]) ([]*models.Transaction, []error) {
	out := make([]*models.Transaction, 0, len(results))
	var errs []error

	for _, r := range results {
		if r.Error != nil {
			errs = append(errs, r.Error)
			continue
		}

		out = append(out, r.Value)
	}

	return out, errs
}

// GenerateBatch submits a list of DSL patterns with a target TPS throttle.
func (g *transactionGenerator) GenerateBatch(ctx context.Context, orgID, ledgerID string, patterns []data.TransactionPattern, tps float64) ([]*models.Transaction, error) {
	if len(patterns) == 0 {
		return []*models.Transaction{}, nil
	}

	var timer *observability.Timer
	if g.mc != nil {
		timer = g.mc.NewTimer(ctx, "transactions.batch.dsl", "transactions")
	}

	counter := stats.NewCounter()
	tick, stopTicker := setupThrottleTicker(tps)
	defer stopTicker()

	items := make([]int, len(patterns))
	for i := range patterns {
		items[i] = i
	}

	workers := getWorkers(ctx)
	buf := workers * 2
	results := concurrent.WorkerPool(ctx, items, func(ctx context.Context, i int) (*models.Transaction, error) {
		if err := g.waitForThrottle(ctx, tick); err != nil {
			return nil, err
		}

		tx, err := g.GenerateWithDSL(ctx, orgID, ledgerID, patterns[i])
		if err == nil {
			counter.RecordSuccess()
		}

		return tx, err
	}, concurrent.WithWorkers(workers), concurrent.WithBufferSize(buf))

	out, errs := collectBatchResults(results)
	g.finalizeBatch(ctx, timer, counter, len(out))

	if len(errs) > 0 {
		return out, errorsJoin(errs...)
	}

	return out, nil
}

// waitForThrottle waits for the throttle ticker or context cancellation.
func (g *transactionGenerator) waitForThrottle(ctx context.Context, tick <-chan time.Time) error {
	if tick == nil {
		return nil
	}

	select {
	case <-tick:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// finalizeBatch stops the timer and logs batch completion.
func (g *transactionGenerator) finalizeBatch(ctx context.Context, timer *observability.Timer, counter *stats.Counter, count int) {
	if timer != nil {
		timer.StopBatch(count)
	}

	if g.obs != nil && g.obs.IsEnabled() && g.obs.Logger() != nil {
		g.obs.Logger().Infof("transactions: created=%d tps=%.2f", counter.SuccessCount(), counter.TPS())
	}
}
