package generator

import (
    "runtime"
    "time"
)

// GeneratorConfig defines scale, performance, and data pattern options
// for the mass demo data generator.
//
// Phase 1 focuses on establishing a comprehensive configuration surface
// that later phases will consume to drive generation behavior.
type GeneratorConfig struct {
    // Scale parameters
    Organizations           int
    LedgersPerOrg          int
    AssetsPerLedger        int
    AccountsPerLedger      int
    TransactionsPerAccount int
    SegmentsPerLedger      int
    PortfoliosPerLedger    int

    // Performance parameters
    ConcurrencyLevel      int  // Worker pool size
    BatchSize             int  // Items per batch
    EnableCircuitBreaker  bool
    MaxRetries            int
    RetryBackoffMs        int

    // Data patterns
    TransactionPatterns   []string // payment, refund, transfer, etc.
    AccountTypes          []string // checking, savings, credit, expense
    AssetTypes            []string // currency, crypto, points

    // Idempotency & tracking
    EnableIdempotency     bool
    UseExternalIDs        bool
    GenerationSeed        int64 // For reproducible data

    // Circuit breaker parameters
    CircuitBreakerFailureThreshold int
    CircuitBreakerSuccessThreshold int
    CircuitBreakerOpenTimeout      time.Duration
}

// DefaultConfig returns a sensible baseline configuration suitable for
// local development. Values can be overridden by flags or envs in examples.
func DefaultConfig() GeneratorConfig {
    return GeneratorConfig{
        // Scale
        Organizations:           2,
        LedgersPerOrg:          2,
        AssetsPerLedger:        3,
        AccountsPerLedger:      50,
        TransactionsPerAccount: 20,
        SegmentsPerLedger:      2,
        PortfoliosPerLedger:    2,

        // Performance
        ConcurrencyLevel:     max(2, runtime.NumCPU()*2),
        BatchSize:            50,
        EnableCircuitBreaker: false,
        MaxRetries:           3,
        RetryBackoffMs:       200,

        // Patterns
        TransactionPatterns: []string{"payment", "refund", "transfer", "fee", "fx"},
        AccountTypes:        []string{"checking", "savings", "creditCard", "expense"},
        AssetTypes:          []string{"currency", "crypto", "points"},

        // Idempotency
        EnableIdempotency: true,
        UseExternalIDs:    true,
        GenerationSeed:    time.Now().UnixNano(),

        // Circuit breaker defaults
        CircuitBreakerFailureThreshold: 5,
        CircuitBreakerSuccessThreshold: 2,
        CircuitBreakerOpenTimeout:      5 * time.Second,
    }
}

// WithOverrides applies non-zero/meaningful overrides from src onto dst.
// This is a lightweight helper to merge configuration sources.
func (dst *GeneratorConfig) WithOverrides(src GeneratorConfig) {
    // Scale
    if src.Organizations > 0 {
        dst.Organizations = src.Organizations
    }
    if src.LedgersPerOrg > 0 {
        dst.LedgersPerOrg = src.LedgersPerOrg
    }
    if src.AssetsPerLedger > 0 {
        dst.AssetsPerLedger = src.AssetsPerLedger
    }
    if src.AccountsPerLedger > 0 {
        dst.AccountsPerLedger = src.AccountsPerLedger
    }
    if src.TransactionsPerAccount > 0 {
        dst.TransactionsPerAccount = src.TransactionsPerAccount
    }
    if src.SegmentsPerLedger > 0 {
        dst.SegmentsPerLedger = src.SegmentsPerLedger
    }
    if src.PortfoliosPerLedger > 0 {
        dst.PortfoliosPerLedger = src.PortfoliosPerLedger
    }

    // Performance
    if src.ConcurrencyLevel > 0 {
        dst.ConcurrencyLevel = src.ConcurrencyLevel
    }
    if src.BatchSize > 0 {
        dst.BatchSize = src.BatchSize
    }
    dst.EnableCircuitBreaker = src.EnableCircuitBreaker || dst.EnableCircuitBreaker
    if src.MaxRetries > 0 {
        dst.MaxRetries = src.MaxRetries
    }
    if src.RetryBackoffMs > 0 {
        dst.RetryBackoffMs = src.RetryBackoffMs
    }

    // Patterns
    if len(src.TransactionPatterns) > 0 {
        dst.TransactionPatterns = append([]string{}, src.TransactionPatterns...)
    }
    if len(src.AccountTypes) > 0 {
        dst.AccountTypes = append([]string{}, src.AccountTypes...)
    }
    if len(src.AssetTypes) > 0 {
        dst.AssetTypes = append([]string{}, src.AssetTypes...)
    }

    // Idempotency & tracking
    dst.EnableIdempotency = src.EnableIdempotency || dst.EnableIdempotency
    dst.UseExternalIDs = src.UseExternalIDs || dst.UseExternalIDs
    if src.GenerationSeed != 0 {
        dst.GenerationSeed = src.GenerationSeed
    }

    // Circuit breaker
    if src.CircuitBreakerFailureThreshold > 0 {
        dst.CircuitBreakerFailureThreshold = src.CircuitBreakerFailureThreshold
    }
    if src.CircuitBreakerSuccessThreshold > 0 {
        dst.CircuitBreakerSuccessThreshold = src.CircuitBreakerSuccessThreshold
    }
    if src.CircuitBreakerOpenTimeout > 0 {
        dst.CircuitBreakerOpenTimeout = src.CircuitBreakerOpenTimeout
    }
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}
