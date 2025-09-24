# Mass Demo Data Generator - Enhanced Implementation Plan

## Overview
Create a comprehensive demo data generator that populates Midaz with realistic financial data across multiple organizations, ledgers, accounts, and transactions. This generator will create persistent data (unlike the workflow example that deletes everything at the end) and leverage advanced SDK patterns for optimal performance.

## Deep Architecture Analysis

### SDK Architecture Insights
1. **Service-Oriented Design**
   - Each entity has a dedicated service interface (AccountsService, TransactionsService, etc.)
   - HTTPClient abstraction with configurable retry logic and observability
   - Functional options pattern throughout for flexible configuration

2. **Transaction DSL Support**
   - Full DSL implementation with SendInput/DistributeInput patterns
   - Support for complex multi-leg transactions
   - Share-based distributions and remaining account handling
   - Rate configurations for currency conversions

3. **Concurrency & Performance**
   - Built-in concurrent package with worker pools
   - Performance monitoring with TPS metrics
   - Observability integration (tracing, metrics, logging)
   - Retry mechanism with exponential backoff and jitter

4. **Validation & Constraints**
   - Metadata validation: max 100 chars for keys, 2000 for values
   - No nested metadata allowed (nonested validation)
   - UUID validation for all IDs
   - Alias validation with prohibited external account prefix rules
   - Idempotency key support for transaction deduplication

5. **Pagination Patterns**
   - Cursor-based pagination (preferred for consistency)
   - Offset-based pagination (alternative)
   - ListOptions with filters: startDate, endDate, metadata, sortOrder
   - Configurable page sizes (typically 1-100 items)

### Midaz Server Domain Model Insights

1. **Account Model Constraints**
   - Type field is required (checking, savings, creditCard, expense)
   - Status enum: active, inactive, pending
   - Parent-child account hierarchy support
   - Portfolio and Segment associations (optional)
   - EntityID for external system linking

2. **Transaction Processing**
   - Status states: PENDING, COMPLETED, FAILED, CANCELED
   - ChartOfAccountsGroupName for categorization
   - Template support for standardized transactions
   - External ID for idempotency and external references
   - Pending transactions require explicit commitment

3. **Asset Configuration**
   - Asset codes with standard formats (USD, EUR, BTC, etc.)
   - Scale/precision configuration per asset
   - Asset rate management for conversions
   - Type categorization (currency, security, commodity)

4. **Metadata Patterns**
   - All entities support metadata map[string]any
   - Common patterns: tags, references, categories, custom_fields
   - Used for filtering in list operations
   - Maximum 100 character keys, 2000 character values

## Key Findings from Deep Analysis

### Available in SDK & Server (with constraints)
- ✅ Organizations (CRUD) - with address, tax ID, metadata validation
- ✅ Ledgers (CRUD) - with status management and metadata filtering
- ✅ Assets (CRUD) - with scale, type, and rate configuration
- ✅ Accounts (CRUD) - with hierarchy, alias, type requirements
- ✅ Portfolios (CRUD) - with account association capabilities
- ✅ Segments (CRUD) - for business categorization
- ✅ Transactions (Create, Commit, Revert, Update) - with DSL support
- ✅ Operations (Get, List, Update) - individual transaction entries
- ✅ Balances (Get, List, Update, Delete) - real-time balance tracking
- ✅ Asset Rates (Create/Update, Get) - for multi-currency support

### All Features Available in Both SDK and Server
- ✅ Account Types - Available under "routing" auth scope in onboarding service
- ✅ Operation Routes - Available under "routing" auth scope in transaction service
- ✅ Transaction Routes - Available under "routing" auth scope in transaction service

### Critical Implementation Considerations

1. **Idempotency Requirements**
   - Always use IdempotencyKey for transactions
   - ExternalID for preventing duplicate entities
   - UUID generation for unique identifiers

2. **Performance Optimization**
   - Use concurrent.WorkerPool for parallel processing
   - Batch operations where possible
   - Implement circuit breakers for API protection
   - Monitor TPS with performance.Metrics

3. **Error Handling Patterns**
   - Retry with exponential backoff for transient errors
   - Specific error codes: insufficient_funds, validation_error, etc.
   - Context-aware error propagation
   - Observability integration for error tracking

4. **Data Integrity**
   - Double-entry accounting validation
   - Balance consistency checks
   - Transaction atomicity guarantees
   - Referential integrity for relationships

## Implementation Phases

### Phase 1: Project Setup & Configuration (Enhanced)
- [x] Create new `examples/mass-demo-generator/` directory
- [x] Set up main.go with SDK best practices
  - [x] Use config.FromEnvironment() for configuration
  - [x] Implement observability.Provider setup
  - [x] Configure retry.Options with exponential backoff
  - [x] Set up context with timeout and tracing
- [x] Create enhanced config structure
  ```go
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
      ConcurrencyLevel       int  // Worker pool size
      BatchSize             int  // Items per batch
      EnableCircuitBreaker  bool
      MaxRetries            int
      RetryBackoffMs        int

      // Data patterns
      TransactionPatterns   []string // payment, refund, transfer, etc.
      AccountTypes         []string // checking, savings, credit, expense
      AssetTypes           []string // currency, crypto, points

      // Idempotency & tracking
      EnableIdempotency     bool
      UseExternalIDs        bool
      GenerationSeed        int64 // For reproducible data
  }
  ```
- [x] Create generator package structure with interfaces
  - [x] `pkg/generator/` - Core generation interfaces and orchestration
  - [x] `pkg/data/` - Data templates with realistic patterns
  - [x] `pkg/stats/` - Metrics collection and TPS monitoring
  - [x] `pkg/utils/` - UUID generation, validation helpers
  - [x] `pkg/concurrent/` - Worker pools and parallel processing

### Phase 2: Data Templates & Patterns (Enhanced with Constraints)
- [x] Create realistic organization templates with proper metadata
  ```go
  type OrgTemplate struct {
      LegalName    string
      TradeName    string
      TaxID        string // CNPJ/EIN format validation
      Address      models.Address
      Status       models.Status // active, inactive, pending
      Metadata     map[string]any // max 100 char keys, 2000 values
      Industry     string // for metadata categorization
      Size         string // small, medium, large, enterprise
  }
  ```
  - [x] Tech companies (SaaS, marketplace, fintech)
  - [x] E-commerce businesses (B2C, B2B, marketplace)
  - [x] Financial institutions (bank, credit union, payment processor)
  - [x] Healthcare organizations (hospital, clinic, insurance)
  - [x] Retail chains (physical, online, hybrid)

- [x] Create asset templates with scale configuration
  ```go
  type AssetTemplate struct {
      Name      string
      Type      string // currency, security, commodity
      Code      string // ISO codes for currencies
      Scale     int    // Decimal places (2 for USD, 8 for BTC)
      Metadata  map[string]any
  }
  ```
  - [x] Fiat currencies with proper scale (USD:2, EUR:2, BRL:2, JPY:0)
  - [x] Cryptocurrencies with precision (BTC:8, ETH:18, USDT:6)
  - [x] Loyalty points (POINTS:0, MILES:0)
  - [x] Store credits (CREDIT:2)
  - [x] Custom tokens with configurable scale

- [x] Create account templates with type requirements
  ```go
  type AccountTemplate struct {
      Name            string
      Type            string // REQUIRED: checking, savings, creditCard, expense
      Status          models.Status
      Alias           *string // Must not start with @external/
      ParentAccountID *string // For hierarchy
      PortfolioID     *string
      SegmentID       *string
      EntityID        *string // External system reference
      Metadata        map[string]any
  }
  ```
  - [x] Customer accounts (checking, savings types)
  - [x] Merchant accounts (checking type with merchant metadata)
  - [x] Fee accounts (expense type)
  - [x] Settlement accounts (checking type with settlement metadata)
  - [x] Escrow accounts (savings type with hold metadata)
  - [x] Revenue accounts (categorized via metadata)
  - [x] Expense accounts (categorized via metadata)

- [x] Create transaction DSL patterns
  ```go
  type TransactionPattern struct {
      ChartOfAccountsGroupName string
      Description             string
      DSLTemplate            string // DSL script template
      RequiresCommit         bool   // For pending transactions
      IdempotencyKey         string // UUID for dedup
      ExternalID             string // External reference
      Metadata               map[string]any
  }
  ```
  - [x] Payment transactions (simple send/distribute)
  - [x] Refund transactions (reverse of payment)
  - [x] Transfer transactions (between accounts)
  - [x] Fee collection (with percentage calculations)
  - [x] Currency exchange (rate application placeholder)
  - [x] Batch settlements (multi-leg transactions)
  - [x] Split payments (share-based distribution)

### Phase 3: Core Entity Generators (With SDK Patterns)

#### 3.1 Organization Generator with Retry & Observability
```go
type OrganizationGenerator interface {
    Generate(ctx context.Context, template OrgTemplate) (*models.Organization, error)
    GenerateBatch(ctx context.Context, count int) ([]*models.Organization, error)
}
```
- [x] Implement with SDK best practices
  - [x] Use retry.Do() for resilient API calls
  - [x] Add observability.StartSpan() for tracing
  - [x] Apply metadata constraints (100/2000 char limits)
- [x] Generate organizations with realistic data
  - [ ] Legal names with faker library
  - [ ] Trade names variations
- [ ] Tax IDs with proper format (CNPJ: 14 digits, EIN: 9 digits)
  - [x] Addresses using models.NewAddress()
  - [x] Status using models.NewStatus()
  - [ ] Industry-specific metadata
- [x] Implement concurrent batch creation
  - [x] Use concurrent.WorkerPool for parallel creation
  - [x] Monitor with metrics/TPS
  - [x] Circuit breaker pattern for API protection

#### 3.2 Ledger Generator with Pagination Support
```go
type LedgerGenerator interface {
    Generate(ctx context.Context, orgID string, template LedgerTemplate) (*models.Ledger, error)
    GenerateForOrg(ctx context.Context, orgID string, count int) ([]*models.Ledger, error)
    ListWithPagination(ctx context.Context, opts *models.ListOptions) (*models.ListResponse[models.Ledger], error)
}
```
- [x] Generate ledgers with proper configuration
  - [ ] Multi-currency ledgers (multiple asset support)
  - [ ] Single-currency ledgers (USD-only, EUR-only, etc.)
  - [ ] Special purpose ledgers (fees, settlements, escrow)
  - [ ] Status management (active, inactive)
- [ ] Set up ledger hierarchies and relationships
- [x] Configure ledger metadata for filtering
  - [ ] Purpose: operational, settlement, fees
  - [ ] Currency_scope: single, multi
  - [ ] Region: us, eu, apac, latam

#### 3.3 Asset Generator with Rate Management
```go
type AssetGenerator interface {
    Generate(ctx context.Context, ledgerID string, template AssetTemplate) (*models.Asset, error)
    GenerateWithRates(ctx context.Context, ledgerID string, baseAsset string) error
    UpdateRates(ctx context.Context, ledgerID string, rates map[string]float64) error
}
```
- [x] Generate diverse assets per ledger
  - [x] Use models.NewCreateAssetInput() builder
  - [x] Encode scale/precision in metadata
- [ ] Set up asset exchange rates (pending SDK/server API)
  - [ ] Create base currency pairs (USD as base)
  - [ ] Configure conversion rates with AssetRateService
  - [ ] Support multi-hop conversions
- [x] Configure asset metadata (ISO/symbol/scale)

### Phase 4: Account Structure Generators

#### 4.1 Account Type Generator
- [x] Create account type definitions using AccountTypeService
  - [x] Checking account types
  - [x] Savings account types
  - [x] Credit card account types
  - [x] Expense account types
- [ ] Configure account type metadata and validation rules

#### 4.2 Account Generator
- [x] Generate accounts with proper mapping to accounting classes
  - [ ] Asset accounts
  - [ ] Liability accounts
  - [ ] Equity accounts
  - [ ] Revenue accounts
  - [ ] Expense accounts
- [x] Set up account aliases
- [x] Configure account metadata
  - [ ] Customer IDs
  - [x] Account purposes
  - [x] Risk levels
- [ ] Link accounts to their account types

#### 4.2 Portfolio Generator
- [x] Create portfolio structures
  - [x] Customer portfolios
  - [ ] Merchant portfolios
  - [ ] Internal portfolios
- [x] Link accounts to portfolios
- [ ] Set up portfolio hierarchies

#### 4.3 Segment Generator
- [x] Create business segments
  - [ ] Geographic segments
  - [ ] Product segments
  - [ ] Customer segments
- [x] Associate accounts with segments
- [x] Configure segment metadata

### Phase 5: Transaction Generators (DSL-Based with Idempotency)

#### 5.1 DSL Transaction Generator with TPS Monitoring
```go
type TransactionGenerator interface {
    GenerateWithDSL(ctx context.Context, orgID, ledgerID string, pattern TransactionPattern) (*models.Transaction, error)
    GenerateBatch(ctx context.Context, orgID, ledgerID string, patterns []TransactionPattern, tps float64) ([]*models.Transaction, error)
}
```
- [x] Implement DSL-based transaction creation (DSL file endpoint)
- [x] Configure Send/Distribute via generated DSL scripts
- [ ] Programmatic Send/Distribute builders (optional)
- [x] Idempotency header injection (SDK HTTP path)
- [x] ExternalID support
- [ ] Generate realistic transaction amounts
  - [ ] Normal distribution for typical payments
  - [ ] Power law distribution for e-commerce
  - [ ] Respect asset scale/precision
- [ ] Create time-distributed transactions
  - [ ] Historical data with realistic patterns
  - [ ] Daily/weekly/monthly cycles
  - [ ] Peak hour simulations
  - [ ] Real-time streaming mode

#### 5.2 Complex Transaction Patterns with DSL
```go
// Multi-leg transaction DSL example
dsl := `
send [USD 100] (
  source = @customer
)
distribute [USD 100] (
  destination = {
    85% to @merchant
    10% to @platform-fee
    5% to @payment-processor
  }
)
`
```
- [x] Multi-leg transactions with proper balancing (via DSL)
- [ ] Currency exchange with rate application
  - [ ] Use asset rates for conversions
  - [ ] Handle rate spreads
- [ ] Fee-bearing transactions
  - [ ] Percentage-based fees
  - [ ] Fixed fees
  - [ ] Tiered fee structures
- [x] Batch payment processing
  - [x] Use concurrent.WorkerPool
  - [x] Monitor with metrics/TPS
- [ ] Recurring transactions with templates
- [ ] Split payments with share distributions

#### 5.3 Transaction Lifecycle Management
```go
type TransactionLifecycle interface {
    CreatePending(ctx context.Context, input *models.CreateTransactionInput) (*models.Transaction, error)
    Commit(ctx context.Context, txID string) error
    Revert(ctx context.Context, txID string) error
    HandleInsufficientFunds(ctx context.Context, err error) error
}
```
- [ ] Create pending transactions
  - [ ] Set Pending: true in input
  - [ ] Track pending transaction IDs
- [ ] Commit transactions with retry
  - [ ] Use CommitTransaction API
  - [ ] Handle commit failures
- [ ] Handle insufficient funds scenarios
  - [ ] Detect specific error codes
  - [ ] Implement retry with backoff
  - [ ] Create compensating transactions
- [ ] Implement reversals
  - [ ] Use RevertTransaction API
  - [ ] Maintain reversal audit trail
- [ ] Rich transaction metadata
  - [ ] Reference IDs for external systems
  - [ ] Descriptive tags and categories
  - [ ] Compliance metadata

### Phase 6: Routing Features Implementation

#### 6.1 Account Types Integration (Available in Server)
- [ ] Create account type templates using AccountTypeService
  - [ ] Checking accounts with overdraft rules
  - [ ] Savings accounts with interest calculations
  - [ ] Credit accounts with credit limits
  - [ ] Investment accounts with portfolio tracking
- [ ] Configure account type metadata
- [ ] Link accounts to their types
- [ ] Validate account operations against type rules

#### 6.2 Operation Routes Integration (Available in Server)
- [ ] Create operation route templates using OperationRouteService
- [ ] Define source operation patterns
- [ ] Define destination operation patterns
- [ ] Configure route metadata for validation
- [ ] Link routes to account types
- [ ] Set up route-based transaction validation

#### 6.3 Transaction Routes Integration (Available in Server)
- [ ] Create transaction route templates using TransactionRouteService
- [ ] Build payment flow routes
- [ ] Build refund flow routes
- [ ] Build transfer flow routes
- [ ] Link transaction routes to operation routes
- [ ] Configure route-based transaction orchestration
- [ ] Set up multi-step transaction workflows

### Phase 7: Data Relationships & Integrity

#### 7.1 Relationship Builder
- [ ] Establish organization hierarchies
 - [x] Create inter-account relationships
- [ ] Set up portfolio compositions
 - [x] Configure segment associations

#### 7.2 Balance Verification
- [ ] Implement balance checker
- [ ] Verify double-entry consistency
- [ ] Generate balance reports
- [ ] Handle balance discrepancies

#### 7.3 Data Consistency
- [ ] Validate all foreign key relationships
- [ ] Ensure metadata consistency
- [ ] Verify transaction integrity
- [ ] Check account balance accuracy

### Phase 8: Performance & Scalability (Production-Grade)

#### 8.1 Concurrent Processing with SDK Patterns
```go
type ConcurrentProcessor struct {
    WorkerPool   *concurrent.WorkerPool
    Metrics      *performance.Metrics
    CircuitBreaker *CircuitBreaker
    RateLimiter  *RateLimiter
}
```
- [x] Implement parallel processing with SDK utilities
  - [x] Use concurrent.WorkerPool (size based on CPU cores)
  - [x] Monitor with metrics/TPS
  - [x] Add observability spans on critical paths
  - [x] Handle context cancellation properly
- [x] Organization creation parallelization
  - [ ] Batch size: 10-20 orgs per worker
  - [ ] Target TPS: 50-100 orgs/second
- [x] Concurrent ledger generation
  - [ ] Fan-out pattern per organization
  - [ ] Limit: 5 concurrent ledgers per org
- [x] Parallel account creation
  - [ ] Worker pool size: 20-50 workers
  - [ ] Batch size: 100 accounts per batch
- [x] Transaction batch generation (TPS throttle)
  - [ ] Target: 1000+ TPS (stretch)

#### 8.2 Batch Operations & API Optimization
```go
type BatchProcessor interface {
    ProcessBatch(ctx context.Context, items []interface{}, batchSize int) error
    OptimizeForAPI(ctx context.Context, rateLimit int) error
}
```
- [ ] Implement intelligent batching
  - [ ] Dynamic batch sizing based on API response times
  - [ ] Exponential backoff on rate limits
  - [x] Circuit breaker on consecutive failures
- [ ] API call optimization
  - [x] Reuse HTTP connections (shared HTTP client)
  - [ ] Enable HTTP/2 multiplexing
  - [ ] Implement request coalescing
  - [ ] Cache frequently accessed data
- [ ] Rate limiting implementation
  - [ ] Token bucket algorithm
  - [ ] Adaptive rate adjustment
  - [ ] Priority queue for critical operations

#### 8.3 Advanced Progress Monitoring
```go
type ProgressMonitor struct {
    TotalItems    int64
    ProcessedItems atomic.Int64
    FailedItems   atomic.Int64
    StartTime     time.Time
    Metrics       map[string]*performance.Metric
}
```
- [ ] Real-time progress visualization
  - [ ] Use github.com/schollz/progressbar/v3
  - [ ] Multi-bar support for parallel operations
  - [ ] Color-coded status indicators
- [ ] Performance metrics collection
  - [ ] TPS per entity type
  - [ ] P50/P95/P99 latencies
  - [ ] Error rates and types
  - [ ] Resource utilization (CPU, memory, network)
- [ ] ETA calculations
  - [ ] Moving average for rate estimation
  - [ ] Adjust for time-of-day patterns
  - [ ] Account for API throttling

### Phase 9: Reporting & Statistics

#### 9.1 Generation Report
- [ ] Total entities created
  - [ ] Organizations count
  - [ ] Ledgers count
  - [ ] Assets count
  - [ ] Accounts count
  - [ ] Transactions count
  - [ ] Portfolios count
  - [ ] Segments count
- [ ] Time taken per phase
- [ ] API calls made
- [ ] Errors encountered

#### 9.2 Data Summary
- [ ] Transaction volume by organization
- [ ] Account distribution by type
- [ ] Asset usage statistics
- [ ] Balance summaries
- [ ] Top accounts by transaction count

#### 9.3 Export Capabilities
- [ ] Export generation report as JSON
- [ ] Export entity IDs for reference
- [ ] Create data dictionary
- [ ] Generate relationship map

### Phase 10: CLI & Configuration

#### 10.1 CLI Interface
- [x] Implement command-line flags (timeout, orgs, ledgers, accounts, tx, concurrency, batch, demo)
  - [x] `--orgs`: Number of organizations
  - [x] `--ledgers-per-org`: Ledgers per organization
  - [x] `--accounts-per-ledger`: Accounts per ledger
  - [ ] `--transactions`: Total transactions to generate
  - [ ] `--mode`: Sequential/parallel/distributed
  - [ ] `--dry-run`: Preview without creating
  - [ ] `--config`: Config file path
- [ ] Add interactive mode
- [ ] Implement resume capability

#### 10.2 Configuration File
- [ ] YAML/JSON configuration support
- [ ] Preset configurations
  - [ ] Small (demo/testing)
  - [ ] Medium (development)
  - [ ] Large (performance testing)
  - [ ] Custom
- [ ] Environment variable overrides

#### 10.3 Validation & Safety
- [ ] Validate configuration parameters
- [ ] Implement safety limits
- [ ] Add confirmation prompts for large datasets
- [ ] Create backup/restore capability

## Implementation Notes

### Priority Considerations
1. **Core First**: Focus on entities that are fully supported by both SDK and server
2. **Graceful Degradation**: Detect and skip features not available on server
3. **Modularity**: Each generator should be independent and reusable
4. **Idempotency**: Support re-running without duplicating data
5. **Observability**: Comprehensive logging and metrics

### Technical Decisions
- Use functional options pattern for all generators
- Implement retry logic with exponential backoff
- Use context for cancellation support
- Leverage Go routines for parallelization
- Implement circuit breakers for API calls

### Data Realism Goals
- Realistic organization names and details
- Proper geographic distribution
- Natural transaction patterns (daily/weekly cycles)
- Realistic amount distributions
- Proper account balance distributions
- Industry-appropriate metadata

### Testing Strategy
- Unit tests for each generator
- Integration tests with mock server
- Performance benchmarks
- Data validation tests
- Stress testing with large datasets

## Success Criteria
- ✅ Generate 10+ organizations with full hierarchy in < 5 minutes
- ✅ Create 1000+ accounts with proper categorization
- ✅ Process 10,000+ transactions maintaining consistency
- ✅ Zero data integrity violations
- ✅ Comprehensive generation report
- ✅ Resumable on failure
- ✅ Configurable for different scales

## Implementation Examples & Best Practices

### Example: High-Performance Transaction Generation
```go
func (g *TransactionGenerator) GenerateHighVolume(ctx context.Context, config GeneratorConfig) error {
    // Set up observability
    ctx, span := observability.StartSpan(ctx, "HighVolumeTransactionGeneration")
    defer span.End()

    // Create worker pool with monitoring
    pool := concurrent.NewWorkerPool(config.ConcurrencyLevel)
    metrics := performance.NewMetrics("transactions")

    // Generate transactions with retry and idempotency
    for i := 0; i < config.TransactionsPerAccount; i++ {
        pool.Submit(func() error {
            return retry.Do(ctx, func() error {
                input := &models.CreateTransactionInput{
                    IdempotencyKey: uuid.New().String(),
                    ExternalID:     fmt.Sprintf("demo-%d-%d", time.Now().Unix(), i),
                    // ... transaction details
                }

                tx, err := g.client.CreateTransaction(ctx, orgID, ledgerID, input)
                if err != nil {
                    if errors.IsInsufficientFunds(err) {
                        // Handle specific error
                        return retry.Unrecoverable(err)
                    }
                    return err
                }

                metrics.RecordSuccess()
                return nil
            }, retry.WithMaxRetries(3))
        })
    }

    // Wait and report
    pool.Wait()
    fmt.Printf("Generated %d transactions at %.2f TPS\n",
        metrics.SuccessCount(), metrics.TPS())

    return nil
}
```

### Example: Metadata Validation
```go
func validateMetadata(metadata map[string]any) error {
    for key, value := range metadata {
        // Check key length (max 100)
        if len(key) > 100 {
            return fmt.Errorf("metadata key '%s' exceeds 100 characters", key)
        }

        // Check value length (max 2000)
        valueStr := fmt.Sprintf("%v", value)
        if len(valueStr) > 2000 {
            return fmt.Errorf("metadata value for '%s' exceeds 2000 characters", key)
        }

        // Check for nested structures (not allowed)
        if _, isMap := value.(map[string]any); isMap {
            return fmt.Errorf("nested metadata not allowed for key '%s'", key)
        }
    }
    return nil
}
```

### Best Practices Checklist
1. **Always use IdempotencyKey** for transactions to prevent duplicates
2. **Implement retry with exponential backoff** for all API calls
3. **Use context for cancellation** and timeout management
4. **Monitor TPS and latencies** with performance.Metrics
5. **Validate metadata constraints** before API calls
6. **Use cursor-based pagination** for large result sets
7. **Implement circuit breakers** to protect the API
8. **Add observability spans** for debugging and monitoring
9. **Handle specific error codes** (insufficient_funds, validation_error)
10. **Use worker pools** for concurrent operations

## Future Enhancements

## Current Status Summary

- Phases 1–2: Completed (project scaffolding, config, templates, DSL patterns).
- Phase 3: Organization, ledger, and asset generators implemented; concurrency + metrics in place; asset rates pending.
- Phase 4: Account types, accounts (batch), portfolios, segments implemented; hierarchy builder implemented; explicit type-links pending.
- Phase 5: DSL transaction generator implemented (single + batch with TPS); lifecycle (pending/commit, revert) pending.
- Phases 6–8: Routing rules, integrity checks, advanced batching/circuit breaker/rate limiting to be implemented.

## What’s Missing / Next Tasks

1. Asset Rate Management
   - Integrate AssetRateService when available; FX conversions and spreads.

2. Transaction Lifecycle
   - Pending transactions, CommitTransaction handling with retries.
   - Reversals and compensating transactions; insufficient funds flows.

3. Account Structure & Linking
   - Explicit linkage to account types.
   - Portfolio hierarchies; relationship maps.

4. Routing Features
   - Operation and Transaction Routes; route-based validation and orchestration.

5. Integrity & Reporting
   - Balance checker, double-entry consistency, reports and summaries.

6. Performance Hardening
   - Circuit breaker (implemented); token bucket rate limiting; dynamic batch sizing.

7. Programmatic DSL Builders
   - Helpers to build Send/Distribute patterns programmatically; recurring templates.

8. Idempotency Header Support
   - Inject X-Idempotency in Transactions HTTP path when supported.
- [ ] Data aging simulation (historical patterns)
- [ ] Anomaly injection for testing
- [ ] Custom data patterns via plugins
- [ ] Distributed generation across multiple clients
- [ ] Real-time streaming mode
- [ ] Integration with monitoring systems (Prometheus, Grafana)
- [ ] Data anonymization options
- [ ] Export to different formats (CSV, Parquet, JSON Lines)
- [ ] Webhook simulation for event-driven testing
- [ ] Chaos engineering support (random failures, delays)
