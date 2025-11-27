package generator

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	t.Run("Scale defaults", func(t *testing.T) {
		assert.Equal(t, 2, cfg.Organizations)
		assert.Equal(t, 2, cfg.LedgersPerOrg)
		assert.Equal(t, 3, cfg.AssetsPerLedger)
		assert.Equal(t, 50, cfg.AccountsPerLedger)
		assert.Equal(t, 20, cfg.TransactionsPerAccount)
		assert.Equal(t, 2, cfg.SegmentsPerLedger)
		assert.Equal(t, 2, cfg.PortfoliosPerLedger)
	})

	t.Run("Performance defaults", func(t *testing.T) {
		expectedConcurrency := maxInt(2, runtime.NumCPU()*2)
		assert.Equal(t, expectedConcurrency, cfg.ConcurrencyLevel)
		assert.Equal(t, 50, cfg.BatchSize)
		assert.False(t, cfg.EnableCircuitBreaker)
		assert.Equal(t, 3, cfg.MaxRetries)
		assert.Equal(t, 200, cfg.RetryBackoffMs)
	})

	t.Run("Pattern defaults", func(t *testing.T) {
		assert.Equal(t, []string{"payment", "refund", "transfer", "fee", "fx"}, cfg.TransactionPatterns)
		assert.Equal(t, []string{"checking", "savings", "creditCard", "expense"}, cfg.AccountTypes)
		assert.Equal(t, []string{"currency", "crypto", "points"}, cfg.AssetTypes)
	})

	t.Run("Idempotency defaults", func(t *testing.T) {
		assert.True(t, cfg.EnableIdempotency)
		assert.True(t, cfg.UseExternalIDs)
		assert.NotZero(t, cfg.GenerationSeed)
	})

	t.Run("Circuit breaker defaults", func(t *testing.T) {
		assert.Equal(t, 5, cfg.CircuitBreakerFailureThreshold)
		assert.Equal(t, 2, cfg.CircuitBreakerSuccessThreshold)
		assert.Equal(t, 5*time.Second, cfg.CircuitBreakerOpenTimeout)
	})
}

func TestGeneratorConfig_WithOverrides(t *testing.T) {
	tests := []struct {
		name     string
		base     GeneratorConfig
		override GeneratorConfig
		check    func(t *testing.T, result GeneratorConfig)
	}{
		{
			name: "Override scale parameters",
			base: DefaultConfig(),
			override: GeneratorConfig{
				Organizations:          10,
				LedgersPerOrg:          5,
				AssetsPerLedger:        7,
				AccountsPerLedger:      100,
				TransactionsPerAccount: 50,
				SegmentsPerLedger:      4,
				PortfoliosPerLedger:    3,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 10, result.Organizations)
				assert.Equal(t, 5, result.LedgersPerOrg)
				assert.Equal(t, 7, result.AssetsPerLedger)
				assert.Equal(t, 100, result.AccountsPerLedger)
				assert.Equal(t, 50, result.TransactionsPerAccount)
				assert.Equal(t, 4, result.SegmentsPerLedger)
				assert.Equal(t, 3, result.PortfoliosPerLedger)
			},
		},
		{
			name: "Override performance parameters",
			base: DefaultConfig(),
			override: GeneratorConfig{
				ConcurrencyLevel:     16,
				BatchSize:            100,
				EnableCircuitBreaker: true,
				MaxRetries:           5,
				RetryBackoffMs:       500,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 16, result.ConcurrencyLevel)
				assert.Equal(t, 100, result.BatchSize)
				assert.True(t, result.EnableCircuitBreaker)
				assert.Equal(t, 5, result.MaxRetries)
				assert.Equal(t, 500, result.RetryBackoffMs)
			},
		},
		{
			name: "Override pattern parameters",
			base: DefaultConfig(),
			override: GeneratorConfig{
				TransactionPatterns: []string{"custom_pattern"},
				AccountTypes:        []string{"custom_type"},
				AssetTypes:          []string{"custom_asset"},
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"custom_pattern"}, result.TransactionPatterns)
				assert.Equal(t, []string{"custom_type"}, result.AccountTypes)
				assert.Equal(t, []string{"custom_asset"}, result.AssetTypes)
			},
		},
		{
			name: "Override tracking parameters",
			base: DefaultConfig(),
			override: GeneratorConfig{
				EnableIdempotency: true,
				UseExternalIDs:    true,
				GenerationSeed:    12345,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.True(t, result.EnableIdempotency)
				assert.True(t, result.UseExternalIDs)
				assert.Equal(t, int64(12345), result.GenerationSeed)
			},
		},
		{
			name: "Override circuit breaker parameters",
			base: DefaultConfig(),
			override: GeneratorConfig{
				CircuitBreakerFailureThreshold: 10,
				CircuitBreakerSuccessThreshold: 3,
				CircuitBreakerOpenTimeout:      10 * time.Second,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 10, result.CircuitBreakerFailureThreshold)
				assert.Equal(t, 3, result.CircuitBreakerSuccessThreshold)
				assert.Equal(t, 10*time.Second, result.CircuitBreakerOpenTimeout)
			},
		},
		{
			name: "Zero values do not override",
			base: GeneratorConfig{
				Organizations:             5,
				LedgersPerOrg:             3,
				ConcurrencyLevel:          8,
				BatchSize:                 25,
				MaxRetries:                2,
				RetryBackoffMs:            100,
				TransactionPatterns:       []string{"existing"},
				GenerationSeed:            999,
				CircuitBreakerOpenTimeout: 3 * time.Second,
			},
			override: GeneratorConfig{
				Organizations:  0,
				LedgersPerOrg:  0,
				GenerationSeed: 0,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 5, result.Organizations)
				assert.Equal(t, 3, result.LedgersPerOrg)
				assert.Equal(t, 8, result.ConcurrencyLevel)
				assert.Equal(t, int64(999), result.GenerationSeed)
			},
		},
		{
			name: "Empty slices do not override",
			base: GeneratorConfig{
				TransactionPatterns: []string{"payment"},
				AccountTypes:        []string{"checking"},
				AssetTypes:          []string{"currency"},
			},
			override: GeneratorConfig{
				TransactionPatterns: nil,
				AccountTypes:        []string{},
				AssetTypes:          nil,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"payment"}, result.TransactionPatterns)
				assert.Equal(t, []string{"checking"}, result.AccountTypes)
				assert.Equal(t, []string{"currency"}, result.AssetTypes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.WithOverrides(tt.override)
			tt.check(t, cfg)
		})
	}
}

func TestGeneratorConfig_ApplyScaleOverrides(t *testing.T) {
	tests := []struct {
		name     string
		base     GeneratorConfig
		override GeneratorConfig
		expected GeneratorConfig
	}{
		{
			name:     "All scale fields override",
			base:     GeneratorConfig{Organizations: 1, LedgersPerOrg: 1, AssetsPerLedger: 1, AccountsPerLedger: 1, TransactionsPerAccount: 1, SegmentsPerLedger: 1, PortfoliosPerLedger: 1},
			override: GeneratorConfig{Organizations: 10, LedgersPerOrg: 20, AssetsPerLedger: 30, AccountsPerLedger: 40, TransactionsPerAccount: 50, SegmentsPerLedger: 60, PortfoliosPerLedger: 70},
			expected: GeneratorConfig{Organizations: 10, LedgersPerOrg: 20, AssetsPerLedger: 30, AccountsPerLedger: 40, TransactionsPerAccount: 50, SegmentsPerLedger: 60, PortfoliosPerLedger: 70},
		},
		{
			name:     "Partial scale override",
			base:     GeneratorConfig{Organizations: 1, LedgersPerOrg: 2, AssetsPerLedger: 3},
			override: GeneratorConfig{Organizations: 10},
			expected: GeneratorConfig{Organizations: 10, LedgersPerOrg: 2, AssetsPerLedger: 3},
		},
		{
			name:     "Zero values preserved",
			base:     GeneratorConfig{Organizations: 5, LedgersPerOrg: 5},
			override: GeneratorConfig{Organizations: 0, LedgersPerOrg: 0},
			expected: GeneratorConfig{Organizations: 5, LedgersPerOrg: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.applyScaleOverrides(tt.override)
			assert.Equal(t, tt.expected.Organizations, cfg.Organizations)
			assert.Equal(t, tt.expected.LedgersPerOrg, cfg.LedgersPerOrg)
			assert.Equal(t, tt.expected.AssetsPerLedger, cfg.AssetsPerLedger)
			assert.Equal(t, tt.expected.AccountsPerLedger, cfg.AccountsPerLedger)
			assert.Equal(t, tt.expected.TransactionsPerAccount, cfg.TransactionsPerAccount)
			assert.Equal(t, tt.expected.SegmentsPerLedger, cfg.SegmentsPerLedger)
			assert.Equal(t, tt.expected.PortfoliosPerLedger, cfg.PortfoliosPerLedger)
		})
	}
}

func TestGeneratorConfig_ApplyPerformanceOverrides(t *testing.T) {
	tests := []struct {
		name     string
		base     GeneratorConfig
		override GeneratorConfig
		check    func(t *testing.T, result GeneratorConfig)
	}{
		{
			name:     "All performance fields override",
			base:     GeneratorConfig{ConcurrencyLevel: 4, BatchSize: 25, MaxRetries: 2, RetryBackoffMs: 100},
			override: GeneratorConfig{ConcurrencyLevel: 16, BatchSize: 100, EnableCircuitBreaker: true, MaxRetries: 5, RetryBackoffMs: 500},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 16, result.ConcurrencyLevel)
				assert.Equal(t, 100, result.BatchSize)
				assert.True(t, result.EnableCircuitBreaker)
				assert.Equal(t, 5, result.MaxRetries)
				assert.Equal(t, 500, result.RetryBackoffMs)
			},
		},
		{
			name:     "Circuit breaker OR logic",
			base:     GeneratorConfig{EnableCircuitBreaker: true},
			override: GeneratorConfig{EnableCircuitBreaker: false},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.True(t, result.EnableCircuitBreaker)
			},
		},
		{
			name:     "Zero concurrency preserved",
			base:     GeneratorConfig{ConcurrencyLevel: 8},
			override: GeneratorConfig{ConcurrencyLevel: 0},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 8, result.ConcurrencyLevel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.applyPerformanceOverrides(tt.override)
			tt.check(t, cfg)
		})
	}
}

func TestGeneratorConfig_ApplyPatternOverrides(t *testing.T) {
	tests := []struct {
		name     string
		base     GeneratorConfig
		override GeneratorConfig
		check    func(t *testing.T, result GeneratorConfig)
	}{
		{
			name: "Override transaction patterns",
			base: GeneratorConfig{
				TransactionPatterns: []string{"payment", "refund"},
			},
			override: GeneratorConfig{
				TransactionPatterns: []string{"custom"},
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"custom"}, result.TransactionPatterns)
			},
		},
		{
			name: "Override account types",
			base: GeneratorConfig{
				AccountTypes: []string{"checking"},
			},
			override: GeneratorConfig{
				AccountTypes: []string{"savings", "credit"},
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"savings", "credit"}, result.AccountTypes)
			},
		},
		{
			name: "Override asset types",
			base: GeneratorConfig{
				AssetTypes: []string{"currency"},
			},
			override: GeneratorConfig{
				AssetTypes: []string{"crypto", "nft"},
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"crypto", "nft"}, result.AssetTypes)
			},
		},
		{
			name: "Nil slices do not override",
			base: GeneratorConfig{
				TransactionPatterns: []string{"payment"},
				AccountTypes:        []string{"checking"},
				AssetTypes:          []string{"currency"},
			},
			override: GeneratorConfig{
				TransactionPatterns: nil,
				AccountTypes:        nil,
				AssetTypes:          nil,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"payment"}, result.TransactionPatterns)
				assert.Equal(t, []string{"checking"}, result.AccountTypes)
				assert.Equal(t, []string{"currency"}, result.AssetTypes)
			},
		},
		{
			name: "Override makes a copy",
			base: GeneratorConfig{
				TransactionPatterns: []string{"original"},
			},
			override: GeneratorConfig{
				TransactionPatterns: []string{"new"},
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, []string{"new"}, result.TransactionPatterns)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.applyPatternOverrides(tt.override)
			tt.check(t, cfg)
		})
	}
}

func TestGeneratorConfig_ApplyTrackingOverrides(t *testing.T) {
	tests := []struct {
		name     string
		base     GeneratorConfig
		override GeneratorConfig
		check    func(t *testing.T, result GeneratorConfig)
	}{
		{
			name:     "Override generation seed",
			base:     GeneratorConfig{GenerationSeed: 100},
			override: GeneratorConfig{GenerationSeed: 999},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, int64(999), result.GenerationSeed)
			},
		},
		{
			name:     "Zero seed does not override",
			base:     GeneratorConfig{GenerationSeed: 100},
			override: GeneratorConfig{GenerationSeed: 0},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, int64(100), result.GenerationSeed)
			},
		},
		{
			name:     "Idempotency OR logic",
			base:     GeneratorConfig{EnableIdempotency: true},
			override: GeneratorConfig{EnableIdempotency: false},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.True(t, result.EnableIdempotency)
			},
		},
		{
			name:     "UseExternalIDs OR logic",
			base:     GeneratorConfig{UseExternalIDs: true},
			override: GeneratorConfig{UseExternalIDs: false},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.True(t, result.UseExternalIDs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.applyTrackingOverrides(tt.override)
			tt.check(t, cfg)
		})
	}
}

func TestGeneratorConfig_ApplyCircuitBreakerOverrides(t *testing.T) {
	tests := []struct {
		name     string
		base     GeneratorConfig
		override GeneratorConfig
		check    func(t *testing.T, result GeneratorConfig)
	}{
		{
			name: "All circuit breaker fields override",
			base: GeneratorConfig{
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerSuccessThreshold: 2,
				CircuitBreakerOpenTimeout:      5 * time.Second,
			},
			override: GeneratorConfig{
				CircuitBreakerFailureThreshold: 10,
				CircuitBreakerSuccessThreshold: 5,
				CircuitBreakerOpenTimeout:      30 * time.Second,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 10, result.CircuitBreakerFailureThreshold)
				assert.Equal(t, 5, result.CircuitBreakerSuccessThreshold)
				assert.Equal(t, 30*time.Second, result.CircuitBreakerOpenTimeout)
			},
		},
		{
			name: "Zero values preserved",
			base: GeneratorConfig{
				CircuitBreakerFailureThreshold: 5,
				CircuitBreakerSuccessThreshold: 2,
				CircuitBreakerOpenTimeout:      5 * time.Second,
			},
			override: GeneratorConfig{
				CircuitBreakerFailureThreshold: 0,
				CircuitBreakerSuccessThreshold: 0,
				CircuitBreakerOpenTimeout:      0,
			},
			check: func(t *testing.T, result GeneratorConfig) {
				t.Helper()
				assert.Equal(t, 5, result.CircuitBreakerFailureThreshold)
				assert.Equal(t, 2, result.CircuitBreakerSuccessThreshold)
				assert.Equal(t, 5*time.Second, result.CircuitBreakerOpenTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.base
			cfg.applyCircuitBreakerOverrides(tt.override)
			tt.check(t, cfg)
		})
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{"a greater than b", 10, 5, 10},
		{"b greater than a", 5, 10, 10},
		{"equal values", 7, 7, 7},
		{"negative values", -5, -10, -5},
		{"zero and positive", 0, 5, 5},
		{"zero and negative", 0, -5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maxInt(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
