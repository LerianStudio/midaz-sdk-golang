# Midaz Go SDK Examples

This guide provides comprehensive examples of common operations with the Midaz Go SDK, including basic operations and advanced features.

## Table of Contents

- [Client Initialization](#client-initialization)
- [Working with Organizations](#working-with-organizations)
- [Account and Asset Management](#account-and-asset-management)
- [Transaction Processing](#transaction-processing)
- [Using Pagination](#using-pagination)
- [Example Applications](#example-applications)
- [Mass Demo Generator](#mass-demo-generator)

## Client Initialization

```go
import (
	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
)

// Create a client with default options
c, err := client.New(
	client.WithEnvironment(config.EnvironmentProduction),
	client.UseAllAPIs(),
)
if err != nil {
	// Handle error
}

// Create a client with observability enabled
c, err := client.New(
	client.WithEnvironment(config.EnvironmentLocal),
	client.WithObservability(true, true, true), // tracing, metrics, logging
	client.UseAllAPIs(),
)
if err != nil {
	// Handle error
}
```

## Working with Organizations

```go
// Create an organization
org, err := c.Entity.Organizations.CreateOrganization(ctx, &models.CreateOrganizationInput{
	LegalName:       "Example Organization",
	LegalDocument:   "123456789",
	DoingBusinessAs: "Example",
	Address: &models.Address{
		Line1:   "123 Main St",
		City:    "San Francisco",
		State:   "CA",
		Country: "US",
		ZipCode: "94105",
	},
	Metadata: map[string]any{
		"industry": "fintech",
		"size":     "startup",
	},
})

// Get an organization
org, err := c.Entity.Organizations.GetOrganization(ctx, "org-id")

// List organizations with filtering
orgs, err := c.Entity.Organizations.ListOrganizations(ctx, &models.ListOptions{
	Limit: 20,
	Filters: map[string]string{
		"status": "ACTIVE",
	},
})
```

## Account and Asset Management

```go
// Create an asset
asset, err := c.Entity.Assets.CreateAsset(ctx, orgID, ledgerID, &models.CreateAssetInput{
	Name: "US Dollar",
	Type: "currency",
	Code: "USD",
	Scale: 2,
	Metadata: map[string]any{
		"symbol": "$",
		"country": "US",
	},
})

// Create an account
account, err := c.Entity.Accounts.CreateAccount(ctx, orgID, ledgerID, &models.CreateAccountInput{
	Name:      "Customer Checking Account",
	Type:      "checking",
	AssetCode: "USD",
	Alias:     "customer-123",
	Metadata: map[string]any{
		"customer_id": "cust-123",
		"tier":        "premium",
	},
})

// Get account balance
balance, err := c.Entity.Balances.GetBalance(ctx, orgID, ledgerID, accountID)
```

## Transaction Processing

```go
// Create a simple transaction
tx, err := c.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, &models.CreateTransactionInput{
	Description: "Payment from customer to merchant",
	Operations: []models.CreateOperationInput{
		{
			Type:      "debit",
			AssetCode: "USD",
			Amount:    10000, // $100.00 (scale 2)
			AccountID: customerAccountID,
		},
		{
			Type:      "credit",
			AssetCode: "USD",
			Amount:    10000,
			AccountID: merchantAccountID,
		},
	},
})

// Create a transaction using DSL
dsl := `
send [USD 100] (
  source = @customer
)
distribute [USD 100] (
  destination = {
    85% to @merchant
    10% to @platform-fee
    5% to @processor-fee
  }
)
`

tx, err := c.Entity.Transactions.CreateTransactionWithDSL(ctx, orgID, ledgerID, &models.TransactionDSLInput{
	Script:      dsl,
	Description: "Split payment transaction",
	Metadata: map[string]any{
		"payment_id": "pay-123",
		"customer_id": "cust-123",
	},
})
```

## Using Pagination

```go
// Create pagination options
options := models.NewListOptions().
	WithLimit(25).
	WithOrderBy("createdAt").
	WithOrderDirection(models.SortDescending).
	WithFilter("status", "ACTIVE")

// List accounts with pagination
accounts, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
if err != nil {
	// Handle error
}

// Process first page
for _, account := range accounts.Items {
	// Process each account
}

// Check for more pages
if accounts.Pagination.HasNextPage() {
	nextPageOptions := accounts.Pagination.NextPageOptions()
	nextPage, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, nextPageOptions)
	// Process next page...
}

// Alternative: iterate through all pages
for {
	page, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
	if err != nil {
		// Handle error
		break
	}

	// Process items on this page
	for _, account := range page.Items {
		// Process each account
	}

	// Check if we've reached the last page
	if !page.Pagination.HasNextPage() {
		break
	}

	// Update options for the next page
	options = page.Pagination.NextPageOptions()
}
```

## Example Applications

The SDK includes several example applications in the `examples/` directory:

### Available Examples

- **`access-manager-example/`** - Authentication integration example
- **`clean-transaction/`** - Simple transaction processing
- **`concurrency-example/`** - Concurrent operations and balance fetching
- **`configuration-examples/`** - SDK configuration patterns
- **`context-example/`** - Context management and cancellation
- **`mass-demo-generator/`** - Comprehensive demo data generation (see below)
- **`observability-demo/`** - Observability and monitoring setup
- **`retry-example/`** - Retry mechanisms and error handling
- **`validation-example/`** - Input validation patterns
- **`workflow-with-entities/`** - Complete workflow implementation

### Running Examples

Most examples can be run with:

```bash
cd examples/example-name
go run .
```

For more detailed examples, refer to the `examples` directory in the SDK repository.

## Mass Demo Generator

The `examples/mass-demo-generator` app creates realistic demo data (organizations, ledgers, accounts, transactions) using advanced SDK features including concurrent processing, circuit breakers, and comprehensive reporting.

### Features

- **Concurrent Processing**: Uses worker pools for parallel entity creation
- **Circuit Breaker**: Protects against API overload with automatic failure detection
- **DSL Transaction Patterns**: Demonstrates complex transaction flows using DSL
- **Routing System**: Creates account types, operation routes, and transaction routes
- **Integrity Verification**: Performs balance checks and double-entry validation
- **Comprehensive Reporting**: Generates detailed HTML and JSON reports
- **Flexible Configuration**: Interactive and non-interactive modes with extensive options

### Basic Usage

#### Interactive Mode

```bash
cd examples/mass-demo-generator
go run .
```

The interactive mode will prompt you for:

- Number of organizations to create
- Ledgers per organization
- Accounts per ledger
- Transactions per account
- Concurrent workers count
- Organization locale (US/Brazil)
- Enable DSL pattern demonstrations
- Funding amount for initial balances

#### Non-Interactive Mode

```bash
cd examples/mass-demo-generator
DEMO_NON_INTERACTIVE=1 go run . \
  --orgs=3 \
  --ledgers-per-org=2 \
  --accounts-per-ledger=50 \
  --tx=100 \
  --concurrency=10 \
  --batch=25 \
  --org-locale=br \
  --patterns=true
```

### Command Line Options

| Flag                    | Type     | Default | Description                                              |
| ----------------------- | -------- | ------- | -------------------------------------------------------- |
| `--timeout`             | duration | 30m     | Overall generation timeout                               |
| `--orgs`                | int      | 2       | Number of organizations to create                        |
| `--ledgers-per-org`     | int      | 2       | Ledgers per organization                                 |
| `--accounts-per-ledger` | int      | 20      | Accounts per ledger                                      |
| `--tx`                  | int      | 50      | Transactions per account                                 |
| `--concurrency`         | int      | 5       | Worker pool size for parallel operations                 |
| `--batch`               | int      | 10      | Batch size for grouped operations                        |
| `--org-locale`          | string   | us      | Organization locale (`us` or `br`) - toggles EIN vs CNPJ |
| `--patterns`            | bool     | false   | Enable DSL pattern demonstrations                        |

### Generated Data Structure

The generator creates a complete financial ecosystem:

1. **Organizations** with realistic business data and addresses
2. **Ledgers** for each organization with proper metadata
3. **Assets** including USD, EUR, BTC, and loyalty points
4. **Account Types** (checking, savings, credit, expense, revenue, liability, equity)
5. **Accounts** linked to types with hierarchical relationships
6. **Portfolios** and **Segments** for organizing accounts
7. **Operation Routes** and **Transaction Routes** for validation
8. **Transactions** using both standard and DSL formats

### DSL Pattern Demonstrations

When `--patterns=true` is enabled, the generator demonstrates:

#### Subscription Pattern

```dsl
send [USD 29.99] (
  source = @customer
)
distribute [USD 29.99] (
  destination = {
    85% to @merchant
    10% to @platform-fee
    5% to @payment-processor
  }
)
```

#### Split Payment Pattern

```dsl
send [USD 100] (
  source = @customer
)
distribute [USD 100] (
  destination = {
    70% to @merchant-a
    25% to @merchant-b
    5% to @platform-fee
  }
)
```

### Routing System

The generator creates a complete routing infrastructure:

#### Account Types

- **CHECKING**: Standard transaction accounts
- **SAVINGS**: Interest-bearing accounts
- **CREDIT_CARD**: Credit accounts with limits
- **EXPENSE**: Business expense tracking
- **REVENUE**: Revenue recognition accounts
- **LIABILITY**: Liability tracking
- **EQUITY**: Equity accounts

#### Operation Routes

Routes define valid source and destination patterns:

- **Customer Routes**: Checking accounts for customer transactions
- **Merchant Routes**: Business account patterns
- **Fee Routes**: Platform and processor fee destinations
- **Settlement Routes**: Account settlement patterns

#### Transaction Routes

Higher-level transaction flows:

- **Payment Flow**: Customer → Merchant (+ fees)
- **Refund Flow**: Merchant → Customer
- **Transfer Flow**: Account-to-account transfers
- **Settlement Flow**: Multi-party settlements

### Integrity Verification

After generation, the system performs comprehensive checks:

#### Balance Aggregation

- Aggregates balances per asset across all accounts
- Tracks available and on-hold amounts separately
- Identifies overdrawn accounts (negative available balance)

#### Double-Entry Validation

- Computes internal net total (excluding `@external/` prefixed accounts)
- Validates double-entry consistency (net should be zero)
- Flags any discrepancies for investigation

#### Scale Consistency

- Verifies amounts respect asset scale configuration
- Reports any precision violations
- Ensures currency amounts use proper decimal places

### Reporting and Output

The generator produces comprehensive reports:

#### Console Output

Real-time progress indicators and performance metrics during generation.

#### JSON Report (`mass-demo-report.json`)

Machine-readable report containing:

- Entity counts and creation times
- API call statistics and performance metrics
- Transaction volumes by account
- Asset usage statistics
- Balance summaries with integrity status

#### HTML Report (`mass-demo-report.html`)

Human-readable report with:

- Executive summary and key metrics
- Data integrity status and warnings
- Performance analysis and recommendations
- Entity relationship visualizations

#### Entity Reference (`mass-demo-entities.json`)

Complete list of created entity IDs for reference and cleanup.

### Performance Characteristics

The generator is optimized for high-performance data generation:

- **Concurrent Processing**: Parallel entity creation with configurable worker pools
- **Circuit Breaker**: Automatic API protection with failure threshold detection
- **Rate Limiting**: Configurable throttling to prevent API overload
- **Batch Operations**: Grouped operations for efficiency
- **Progress Monitoring**: Real-time performance metrics and ETA calculation

### Example Scenarios

#### Small Demo Dataset

```bash
DEMO_NON_INTERACTIVE=1 go run . \
  --orgs=1 \
  --ledgers-per-org=1 \
  --accounts-per-ledger=10 \
  --tx=20 \
  --org-locale=us
```

#### Large Performance Test

```bash
DEMO_NON_INTERACTIVE=1 go run . \
  --orgs=10 \
  --ledgers-per-org=3 \
  --accounts-per-ledger=100 \
  --tx=500 \
  --concurrency=20 \
  --batch=50 \
  --patterns=true
```

#### Brazilian Organization Demo

```bash
DEMO_NON_INTERACTIVE=1 go run . \
  --org-locale=br \
  --patterns=false
```

### Integration with CI/CD

The generator can be used for:

- **Development Environment Setup**: Create realistic test data
- **Performance Testing**: Generate large datasets for load testing
- **Demo Environments**: Populate demo instances with sample data
- **Integration Testing**: Provide consistent test datasets

For CI/CD integration, use non-interactive mode with appropriate timeout settings:

```bash
timeout 600 go run . --timeout=10m --orgs=5 --tx=100
```
