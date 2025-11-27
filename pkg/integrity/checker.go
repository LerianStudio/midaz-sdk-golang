package integrity

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/shopspring/decimal"
)

// Maximum allowed delay between account lookups to avoid accidental excessive throttling.
const maxAccountLookupDelay time.Duration = 5 * time.Second

// BalanceTotals holds aggregated balances per asset.
type BalanceTotals struct {
	Asset            string
	Accounts         int
	TotalAvailable   decimal.Decimal
	TotalOnHold      decimal.Decimal
	InternalNetTotal decimal.Decimal // excludes accounts whose alias starts with "@external/"
	Overdrawn        []string
}

// Report captures integrity results for a ledger.
type Report struct {
	LedgerID      string
	TotalsByAsset map[string]*BalanceTotals
}

// Checker provides data integrity checks and balance verification.
type Checker struct {
	e *entities.Entity
	// Optional observability provider for logging and tracing
	obs observability.Provider
	// Optional delay between account lookups to avoid overwhelming services on large ledgers
	sleepBetweenAccountLookups time.Duration
}

// NewChecker creates a new Checker.
func NewChecker(e *entities.Entity) *Checker { return &Checker{e: e} }

// WithObservability sets the observability provider for logging and tracing.
func (c *Checker) WithObservability(obs observability.Provider) *Checker {
	c.obs = obs
	return c
}

// WithAccountLookupDelay sets an optional delay inserted before each account lookup.
// Useful to rate-limit calls when processing very large ledgers.
func (c *Checker) WithAccountLookupDelay(d time.Duration) *Checker {
	// Clamp to a sensible range [0, maxAccountLookupDelay]
	if d < 0 {
		d = 0
	}

	if d > maxAccountLookupDelay {
		d = maxAccountLookupDelay
	}

	c.sleepBetweenAccountLookups = d

	return c
}

// GenerateLedgerReport aggregates balances and performs lightweight double-entry checks.
func (c *Checker) GenerateLedgerReport(ctx context.Context, orgID, ledgerID string) (*Report, error) {
	if c.e == nil || c.e.Balances == nil || c.e.Accounts == nil {
		return nil, fmt.Errorf("entities not initialized for integrity checks")
	}

	c.logDebug("Starting ledger integrity report generation for ledger %s", ledgerID)

	totals := map[string]*BalanceTotals{}
	accountAliasCache := map[string]string{}

	var report *Report
	err := observability.WithSpan(ctx, c.obs, "GenerateLedgerReport", func(ctx context.Context) error {
		if err := c.processBalances(ctx, orgID, ledgerID, totals, accountAliasCache); err != nil {
			c.logError("Failed to process balances for ledger %s: %v", ledgerID, err)
			return err
		}
		report = &Report{LedgerID: ledgerID, TotalsByAsset: totals}
		return nil
	})
	if err != nil {
		return nil, err
	}

	c.logInfo("Completed ledger integrity report for ledger %s: %d assets processed", ledgerID, len(totals))

	return report, nil
}

// processBalances processes all balances with pagination
func (c *Checker) processBalances(ctx context.Context, orgID, ledgerID string, totals map[string]*BalanceTotals, accountAliasCache map[string]string) error {
	opts := models.NewListOptions().WithLimit(100)

	for {
		resp, err := c.e.Balances.ListBalances(ctx, orgID, ledgerID, opts)
		if err != nil {
			return err
		}

		for _, b := range resp.Items {
			if err := c.processBalance(ctx, orgID, ledgerID, b, totals, accountAliasCache); err != nil {
				return err
			}
		}

		if resp.Pagination.NextCursor == "" {
			break
		}

		opts = models.NewListOptions().WithCursor(resp.Pagination.NextCursor).WithLimit(100)
	}

	return nil
}

// processBalance processes a single balance entry
func (c *Checker) processBalance(ctx context.Context, orgID, ledgerID string, b models.Balance, totals map[string]*BalanceTotals, accountAliasCache map[string]string) error {
	t := c.getOrCreateBalanceTotals(totals, b.AssetCode)
	c.updateBalanceTotals(t, b)

	alias, err := c.getAccountAlias(ctx, orgID, ledgerID, b.AccountID, accountAliasCache)
	if err != nil {
		return err
	}

	c.updateInternalNetTotal(t, b, alias)
	c.checkForOverdraft(t, b, alias)

	return nil
}

// getOrCreateBalanceTotals gets or creates BalanceTotals for an asset
func (c *Checker) getOrCreateBalanceTotals(totals map[string]*BalanceTotals, assetCode string) *BalanceTotals {
	t, ok := totals[assetCode]
	if !ok {
		t = &BalanceTotals{Asset: assetCode, TotalAvailable: decimal.Zero, TotalOnHold: decimal.Zero, InternalNetTotal: decimal.Zero}
		totals[assetCode] = t
	}

	return t
}

// updateBalanceTotals updates the balance totals with the given balance
func (c *Checker) updateBalanceTotals(t *BalanceTotals, b models.Balance) {
	t.Accounts++
	t.TotalAvailable = t.TotalAvailable.Add(b.Available)
	t.TotalOnHold = t.TotalOnHold.Add(b.OnHold)
}

// getAccountAlias gets the account alias with caching and optional throttling
func (c *Checker) getAccountAlias(ctx context.Context, orgID, ledgerID, accountID string, accountAliasCache map[string]string) (string, error) {
	alias, ok := accountAliasCache[accountID]
	if !ok {
		if err := c.waitForThrottling(ctx); err != nil {
			return "", err
		}

		acc, err := c.e.Accounts.GetAccount(ctx, orgID, ledgerID, accountID)
		if err != nil {
			return "", fmt.Errorf("failed to get account %s: %w", accountID, err)
		}

		if acc != nil && acc.Alias != nil {
			alias = *acc.Alias
		} else {
			alias = ""
		}

		accountAliasCache[accountID] = alias
	}

	return alias, nil
}

// waitForThrottling implements the account lookup delay with cancellation
func (c *Checker) waitForThrottling(ctx context.Context) error {
	if c.sleepBetweenAccountLookups > 0 {
		timer := time.NewTimer(c.sleepBetweenAccountLookups)
		select {
		case <-timer.C:
			// continue
		case <-ctx.Done():
			if !timer.Stop() {
				// drain if fired concurrently
				select {
				case <-timer.C:
				default:
				}
			}

			return ctx.Err()
		}
	}

	return nil
}

// updateInternalNetTotal updates internal net total excluding external aliases
func (c *Checker) updateInternalNetTotal(t *BalanceTotals, b models.Balance, alias string) {
	if !strings.HasPrefix(alias, "@external/") {
		t.InternalNetTotal = t.InternalNetTotal.Add(b.Available.Add(b.OnHold))
	}
}

// checkForOverdraft checks for negative balances and tracks them
func (c *Checker) checkForOverdraft(t *BalanceTotals, b models.Balance, alias string) {
	if b.Available.IsNegative() {
		id := alias
		if id == "" {
			id = b.AccountID
		}

		t.Overdrawn = append(t.Overdrawn, id)
		c.logWarn("Detected overdrawn account %s for asset %s: available=%s", id, b.AssetCode, b.Available.String())
	}
}

// logDebug logs a debug message if observability is enabled.
func (c *Checker) logDebug(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Debugf(format, args...)
	}
}

// logInfo logs an info message if observability is enabled.
func (c *Checker) logInfo(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Infof(format, args...)
	}
}

// logWarn logs a warning message if observability is enabled.
func (c *Checker) logWarn(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Warnf(format, args...)
	}
}

// logError logs an error message if observability is enabled.
func (c *Checker) logError(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Errorf(format, args...)
	}
}

// ToSummaryMap renders a compact map suitable for report embedding (JSON-friendly).
func (r *Report) ToSummaryMap() map[string]map[string]any {
	out := map[string]map[string]any{}
	for asset, t := range r.TotalsByAsset {
		out[asset] = map[string]any{
			"accounts":            t.Accounts,
			"totalAvailable":      t.TotalAvailable.String(),
			"totalOnHold":         t.TotalOnHold.String(),
			"internalNetTotal":    t.InternalNetTotal.String(),
			"doubleEntryBalanced": t.InternalNetTotal.Equal(decimal.Zero),
			"overdrawnAccounts":   t.Overdrawn,
		}
	}

	return out
}
