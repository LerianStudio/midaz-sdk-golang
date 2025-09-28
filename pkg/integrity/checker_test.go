package integrity

import (
	"context"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestNewChecker(t *testing.T) {
	entity := &entities.Entity{}
	checker := NewChecker(entity)

	assert.NotNil(t, checker)
	assert.Equal(t, entity, checker.e)
	assert.Equal(t, time.Duration(0), checker.sleepBetweenAccountLookups)
}

func TestCheckerWithAccountLookupDelay(t *testing.T) {
	entity := &entities.Entity{}
	checker := NewChecker(entity)

	tests := []struct {
		name     string
		delay    time.Duration
		expected time.Duration
	}{
		{
			name:     "valid delay",
			delay:    2 * time.Second,
			expected: 2 * time.Second,
		},
		{
			name:     "negative delay clamped to zero",
			delay:    -1 * time.Second,
			expected: 0,
		},
		{
			name:     "excessive delay clamped to max",
			delay:    10 * time.Second,
			expected: maxAccountLookupDelay,
		},
		{
			name:     "zero delay",
			delay:    0,
			expected: 0,
		},
		{
			name:     "max allowed delay",
			delay:    maxAccountLookupDelay,
			expected: maxAccountLookupDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.WithAccountLookupDelay(tt.delay)

			// Should return the same checker instance for chaining
			assert.Equal(t, checker, result)
			assert.Equal(t, tt.expected, checker.sleepBetweenAccountLookups)
		})
	}
}

func TestGenerateLedgerReport_EntitiesNotInitialized(t *testing.T) {
	tests := []struct {
		name   string
		entity *entities.Entity
	}{
		{
			name:   "nil entity",
			entity: nil,
		},
		{
			name: "entity with nil balances service",
			entity: &entities.Entity{
				Accounts: nil, // Both services are nil
				Balances: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &Checker{e: tt.entity}

			report, err := checker.GenerateLedgerReport(context.TODO(), "org-1", "ledger-1")

			assert.Nil(t, report)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "entities not initialized")
		})
	}
}

func TestMaxAccountLookupDelay(t *testing.T) {
	// Test that the constant is set to a reasonable value
	expectedMax := 5 * time.Second
	assert.Equal(t, expectedMax, maxAccountLookupDelay)
}

func TestBalanceTotalsType(t *testing.T) {
	// Test creating and using BalanceTotals
	totals := &BalanceTotals{
		Asset:            "USD",
		Accounts:         5,
		TotalAvailable:   decimal.NewFromInt(1000),
		TotalOnHold:      decimal.NewFromInt(100),
		InternalNetTotal: decimal.NewFromInt(900),
		Overdrawn:        []string{"account-1", "account-2"},
	}

	assert.Equal(t, "USD", totals.Asset)
	assert.Equal(t, 5, totals.Accounts)
	assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromInt(1000)))
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromInt(100)))
	assert.True(t, totals.InternalNetTotal.Equal(decimal.NewFromInt(900)))
	assert.Len(t, totals.Overdrawn, 2)
	assert.Contains(t, totals.Overdrawn, "account-1")
	assert.Contains(t, totals.Overdrawn, "account-2")
}

func TestReportType(t *testing.T) {
	// Test creating and using Report
	totals := map[string]*BalanceTotals{
		"USD": {
			Asset:            "USD",
			Accounts:         3,
			TotalAvailable:   decimal.NewFromInt(1500),
			TotalOnHold:      decimal.NewFromInt(150),
			InternalNetTotal: decimal.NewFromInt(1350),
			Overdrawn:        []string{},
		},
		"EUR": {
			Asset:            "EUR",
			Accounts:         2,
			TotalAvailable:   decimal.NewFromInt(800),
			TotalOnHold:      decimal.NewFromInt(80),
			InternalNetTotal: decimal.NewFromInt(720),
			Overdrawn:        []string{"account-3"},
		},
	}

	report := &Report{
		LedgerID:      "ledger-123",
		TotalsByAsset: totals,
	}

	assert.Equal(t, "ledger-123", report.LedgerID)
	assert.Len(t, report.TotalsByAsset, 2)
	assert.Contains(t, report.TotalsByAsset, "USD")
	assert.Contains(t, report.TotalsByAsset, "EUR")

	usdTotals := report.TotalsByAsset["USD"]
	assert.Equal(t, "USD", usdTotals.Asset)
	assert.Equal(t, 3, usdTotals.Accounts)

	eurTotals := report.TotalsByAsset["EUR"]
	assert.Equal(t, "EUR", eurTotals.Asset)
	assert.Equal(t, 2, eurTotals.Accounts)
	assert.Len(t, eurTotals.Overdrawn, 1)
}

func TestBalanceTotalsDecimalOperations(t *testing.T) {
	// Test decimal operations with BalanceTotals
	totals := &BalanceTotals{
		Asset:            "BTC",
		Accounts:         1,
		TotalAvailable:   decimal.NewFromFloat(0.12345678),
		TotalOnHold:      decimal.NewFromFloat(0.00000001),
		InternalNetTotal: decimal.NewFromFloat(0.12345679),
		Overdrawn:        []string{},
	}

	// Test decimal precision
	assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromFloat(0.12345678)))
	assert.True(t, totals.TotalOnHold.Equal(decimal.NewFromFloat(0.00000001)))

	// Test arithmetic operations
	sum := totals.TotalAvailable.Add(totals.TotalOnHold)
	assert.True(t, sum.Equal(totals.InternalNetTotal))
}

func TestBalanceTotalsWithNegativeValues(t *testing.T) {
	// Test handling of negative balances (overdrawn accounts)
	totals := &BalanceTotals{
		Asset:            "USD",
		Accounts:         2,
		TotalAvailable:   decimal.NewFromInt(-500), // Negative total (overdrawn)
		TotalOnHold:      decimal.NewFromInt(100),
		InternalNetTotal: decimal.NewFromInt(-400),
		Overdrawn:        []string{"account-1", "account-2"},
	}

	assert.Equal(t, "USD", totals.Asset)
	assert.Equal(t, 2, totals.Accounts)
	assert.True(t, totals.TotalAvailable.IsNegative())
	assert.True(t, totals.InternalNetTotal.IsNegative())
	assert.Len(t, totals.Overdrawn, 2)
}

func TestReportWithMultipleAssets(t *testing.T) {
	// Test report with multiple different assets
	assets := []string{"USD", "EUR", "BTC", "POINTS"}
	report := &Report{
		LedgerID:      "multi-asset-ledger",
		TotalsByAsset: make(map[string]*BalanceTotals),
	}

	// Add totals for each asset
	for i, asset := range assets {
		report.TotalsByAsset[asset] = &BalanceTotals{
			Asset:            asset,
			Accounts:         i + 1,
			TotalAvailable:   decimal.NewFromInt(int64((i + 1) * 1000)),
			TotalOnHold:      decimal.NewFromInt(int64((i + 1) * 100)),
			InternalNetTotal: decimal.NewFromInt(int64((i + 1) * 1100)),
			Overdrawn:        []string{},
		}
	}

	assert.Equal(t, "multi-asset-ledger", report.LedgerID)
	assert.Len(t, report.TotalsByAsset, len(assets))

	// Verify each asset
	for i, asset := range assets {
		totals, exists := report.TotalsByAsset[asset]
		assert.True(t, exists, "Asset %s should exist in report", asset)
		assert.Equal(t, asset, totals.Asset)
		assert.Equal(t, i+1, totals.Accounts)
		assert.True(t, totals.TotalAvailable.Equal(decimal.NewFromInt(int64((i+1)*1000))))
	}
}

func TestCheckerChaining(t *testing.T) {
	// Test that WithAccountLookupDelay returns the same instance for method chaining
	entity := &entities.Entity{}
	checker := NewChecker(entity)

	result1 := checker.WithAccountLookupDelay(1 * time.Second)
	result2 := result1.WithAccountLookupDelay(2 * time.Second)

	// All should point to the same instance
	assert.Equal(t, checker, result1)
	assert.Equal(t, checker, result2)
	assert.Equal(t, result1, result2)

	// Final delay should be the last set value
	assert.Equal(t, 2*time.Second, checker.sleepBetweenAccountLookups)
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
