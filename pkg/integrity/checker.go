package integrity

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/LerianStudio/midaz-sdk-golang/v2/entities"
    "github.com/LerianStudio/midaz-sdk-golang/v2/models"
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
    LedgerID   string
    TotalsByAsset map[string]*BalanceTotals
}

// Checker provides data integrity checks and balance verification.
type Checker struct {
    e *entities.Entity
    // Optional delay between account lookups to avoid overwhelming services on large ledgers
    sleepBetweenAccountLookups time.Duration
}

// NewChecker creates a new Checker.
func NewChecker(e *entities.Entity) *Checker { return &Checker{e: e} }

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

    totals := map[string]*BalanceTotals{}
    accountAliasCache := map[string]string{}

    opts := models.NewListOptions().WithLimit(100)
    for {
        resp, err := c.e.Balances.ListBalances(ctx, orgID, ledgerID, opts)
        if err != nil {
            return nil, err
        }
        for _, b := range resp.Items {
            t, ok := totals[b.AssetCode]
            if !ok {
                t = &BalanceTotals{Asset: b.AssetCode, TotalAvailable: decimal.Zero, TotalOnHold: decimal.Zero, InternalNetTotal: decimal.Zero}
                totals[b.AssetCode] = t
            }
            t.Accounts++
            t.TotalAvailable = t.TotalAvailable.Add(b.Available)
            t.TotalOnHold = t.TotalOnHold.Add(b.OnHold)

            // Internal net excludes external aliases
            alias, ok := accountAliasCache[b.AccountID]
            if !ok {
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
                        return nil, ctx.Err()
                    }
                }
                acc, err := c.e.Accounts.GetAccount(ctx, orgID, ledgerID, b.AccountID)
                if err != nil {
                    return nil, fmt.Errorf("failed to get account %s: %w", b.AccountID, err)
                }
                if acc != nil && acc.Alias != nil {
                    alias = *acc.Alias
                } else {
                    alias = ""
                }
                accountAliasCache[b.AccountID] = alias
            }
            if !strings.HasPrefix(alias, "@external/") {
                t.InternalNetTotal = t.InternalNetTotal.Add(b.Available.Add(b.OnHold))
            }
            if b.Available.IsNegative() {
                id := alias
                if id == "" { id = b.AccountID }
                t.Overdrawn = append(t.Overdrawn, id)
            }
        }
        if resp.Pagination.NextCursor == "" {
            break
        }
        opts = models.NewListOptions().WithCursor(resp.Pagination.NextCursor).WithLimit(100)
    }

    return &Report{LedgerID: ledgerID, TotalsByAsset: totals}, nil
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
