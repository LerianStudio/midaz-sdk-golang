package integrity

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/entities"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/shopspring/decimal"
)

// Maximum allowed delay between account lookups to avoid accidental excessive throttling.
const maxAccountLookupDelay time.Duration = 5 * time.Second

var logMessageSanitizer = strings.NewReplacer("\r", "\\r", "\n", "\\n")

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

// accountsGetter is the narrow slice of the accounts accessor the checker needs
// (consumer-side interface; client.Accounts is a concrete facade).
type accountsGetter interface {
	Get(ctx context.Context, orgID, ledgerID, accountID string) (*models.Account, error)
}

// balancesLister is the narrow slice of the balances accessor the checker needs.
// Same consumer-side pattern as accountsGetter, for the same reason:
// client.Balances is a concrete facade, so the seam that lets a caller stand in
// a fake has to live here.
type balancesLister interface {
	ListBalancesAll(ctx context.Context, orgID, ledgerID string, opts models.BalancesListOpts) iter.Seq2[models.Balance, error]
}

// Checker provides data integrity checks and balance verification.
type Checker struct {
	e        *entities.Entity
	accounts accountsGetter
	balances balancesLister
	// Optional observability provider for logging and tracing
	obs observability.Provider
	// Optional delay between account lookups to avoid overwhelming services on large ledgers
	sleepBetweenAccountLookups time.Duration
}

// NewChecker creates a new Checker.
func NewChecker(e *entities.Entity) *Checker {
	c := &Checker{e: e}
	if e != nil && e.Accounts != nil {
		c.accounts = e.Accounts
	}

	if e != nil && e.Balances != nil {
		c.balances = e.Balances
	}

	return c
}

// WithObservability sets the observability provider for logging and tracing.
func (c *Checker) WithObservability(obs observability.Provider) *Checker {
	if c == nil {
		return nil
	}

	c.obs = obs

	return c
}

// WithAccountLookupDelay sets an optional delay inserted before each account lookup.
// Useful to rate-limit calls when processing very large ledgers.
func (c *Checker) WithAccountLookupDelay(d time.Duration) *Checker {
	if c == nil {
		return nil
	}

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
func (c *Checker) GenerateLedgerReport(ctx context.Context, organizationID, ledgerID string) (*Report, error) {
	if c == nil {
		return nil, errors.New("checker is nil")
	}

	if c.e == nil || c.balances == nil || c.accounts == nil {
		return nil, errors.New("entities not initialized for integrity checks")
	}

	c.logDebug("Starting ledger integrity report generation for ledger %q", ledgerID)

	totals := map[string]*BalanceTotals{}
	accountAliasCache := map[string]string{}

	var report *Report

	err := observability.WithSpan(ctx, c.obs, "GenerateLedgerReport", func(ctx context.Context) error {
		if err := c.processBalances(ctx, organizationID, ledgerID, totals, accountAliasCache); err != nil {
			c.logError("Failed to process balances for ledger %q: %v", ledgerID, err)
			return err
		}

		report = &Report{LedgerID: ledgerID, TotalsByAsset: totals}

		return nil
	})
	if err != nil {
		return nil, err
	}

	c.logInfo("Completed ledger integrity report for ledger %q: %d assets processed", ledgerID, len(totals))

	return report, nil
}

// processBalances processes all balances with pagination
func (c *Checker) processBalances(ctx context.Context, organizationID, ledgerID string, totals map[string]*BalanceTotals, accountAliasCache map[string]string) error {
	opts := models.BalancesListOpts{CursorListOpts: models.CursorListOpts{Limit: 100}}

	for b, err := range c.balances.ListBalancesAll(ctx, organizationID, ledgerID, opts) {
		if err != nil {
			return err
		}

		if err := c.processBalance(ctx, organizationID, ledgerID, b, totals, accountAliasCache); err != nil {
			return err
		}
	}

	return nil
}

// processBalance processes a single balance entry
func (c *Checker) processBalance(ctx context.Context, organizationID, ledgerID string, b models.Balance, totals map[string]*BalanceTotals, accountAliasCache map[string]string) error {
	t := c.getOrCreateBalanceTotals(totals, b.AssetCode)
	c.updateBalanceTotals(t, b)

	alias, err := c.getAccountAlias(ctx, organizationID, ledgerID, b.AccountID, accountAliasCache)
	if err != nil {
		return err
	}

	c.updateInternalNetTotal(t, b, alias)
	c.checkForOverdraft(t, b, alias)

	return nil
}

// getOrCreateBalanceTotals gets or creates BalanceTotals for an asset
func (*Checker) getOrCreateBalanceTotals(totals map[string]*BalanceTotals, assetCode string) *BalanceTotals {
	t, ok := totals[assetCode]
	if !ok {
		t = &BalanceTotals{Asset: assetCode, TotalAvailable: decimal.Zero, TotalOnHold: decimal.Zero, InternalNetTotal: decimal.Zero}
		totals[assetCode] = t
	}

	return t
}

// updateBalanceTotals updates the balance totals with the given balance
func (*Checker) updateBalanceTotals(t *BalanceTotals, b models.Balance) {
	t.Accounts++
	t.TotalAvailable = t.TotalAvailable.Add(b.Available)
	t.TotalOnHold = t.TotalOnHold.Add(b.OnHold)
}

// getAccountAlias gets the account alias with caching and optional throttling
func (c *Checker) getAccountAlias(ctx context.Context, organizationID, ledgerID, accountID string, accountAliasCache map[string]string) (string, error) {
	if alias, ok := accountAliasCache[accountID]; ok {
		return alias, nil
	}

	alias, err := c.fetchAccountAlias(ctx, organizationID, ledgerID, accountID)
	if err != nil {
		return "", err
	}

	accountAliasCache[accountID] = alias

	return alias, nil
}

// fetchAccountAlias fetches the account alias from the API with throttling.
func (c *Checker) fetchAccountAlias(ctx context.Context, organizationID, ledgerID, accountID string) (string, error) {
	if err := c.waitForThrottling(ctx); err != nil {
		return "", err
	}

	acc, err := c.accounts.Get(ctx, organizationID, ledgerID, accountID)
	if err != nil {
		return "", fmt.Errorf("failed to get account %s: %w", accountID, err)
	}

	if acc != nil && acc.Alias != nil {
		return *acc.Alias, nil
	}

	return "", nil
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
func (*Checker) updateInternalNetTotal(t *BalanceTotals, b models.Balance, alias string) {
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
		c.logWarn("Detected overdrawn account %q for asset %q: available=%s", id, b.AssetCode, b.Available.String())
	}
}

// logDebug logs a debug message if observability is enabled.
func (c *Checker) logDebug(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Debug(formatLogMessage(format, args...))
	}
}

// logInfo logs an info message if observability is enabled.
func (c *Checker) logInfo(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Info(formatLogMessage(format, args...))
	}
}

// logWarn logs a warning message if observability is enabled.
func (c *Checker) logWarn(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Warn(formatLogMessage(format, args...))
	}
}

// logError logs an error message if observability is enabled.
func (c *Checker) logError(format string, args ...any) {
	if c.obs != nil && c.obs.IsEnabled() {
		c.obs.Logger().Error(formatLogMessage(format, args...))
	}
}

func formatLogMessage(format string, args ...any) string {
	message := fmt.Sprintf(format, args...)

	return logMessageSanitizer.Replace(message)
}

// ToSummaryMap renders a compact map suitable for report embedding (JSON-friendly).
func (r *Report) ToSummaryMap() map[string]map[string]any {
	if r == nil {
		return map[string]map[string]any{}
	}

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
