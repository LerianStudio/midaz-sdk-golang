# Midaz Go SDK Examples

This guide provides brief examples of common operations with the Midaz Go SDK.

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
```

## Working with Organizations

```go
// Create an organization
org, err := c.Entity.Organizations.CreateOrganization(ctx, &models.CreateOrganizationInput{
	LegalName:       "Example Organization",
	LegalDocument:   "123456789",
	DoingBusinessAs: "Example",
})

// Get an organization
org, err := c.Entity.Organizations.GetOrganization(ctx, "org-id")

// List organizations
orgs, err := c.Entity.Organizations.ListOrganizations(ctx, nil)
```

## Using Pagination

```go
// Create a paginator for accounts
paginator := c.Entity.Accounts.GetAccountPaginator(ctx, "org-id", "ledger-id", &models.ListOptions{
	Limit: 10,
})

// Iterate through all pages
for paginator.HasNext() {
	accounts, err := paginator.Next()
	if err != nil {
		// Handle error
	}
	
	for _, account := range accounts.Items {
		// Process each account
	}
}
```

For more detailed examples, refer to the `examples` directory in the SDK repository.

## Mass Demo Generator

The `examples/mass-demo-generator` app creates realistic demo data (organizations, ledgers, accounts, transactions) using the SDK.

- Build and run interactively:

```
cd examples/mass-demo-generator
go run .
```

- Non-interactive mode with flags:

```
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

- Flags of interest:
  - `--org-locale` (`us`|`br`): toggles EIN vs CNPJ generation during org creation (also asked in interactive mode).
  - `--patterns` (true|false): enables DSL pattern demos (e.g., Subscription, SplitPayment) during transaction generation.
  - Account Types: generator creates default account types (CHECKING, SAVINGS, CREDIT_CARD, EXPENSE, REVENUE, LIABILITY, EQUITY) and links accounts via metadata `account_type_key` for validation.

- Pattern demos included when `--patterns=true`:
  - Subscription: recurring merchant charge pattern.
  - SplitPayment: customer payment split across multiple recipients by percentage.

### Routing Features (Phase 6)

The generator also creates default Operation Routes and Transaction Routes to enable route-based validation and orchestration:

- Operation Routes (examples):
  - Source: Customer (CHECKING)
  - Destination: Merchant (CHECKING)
  - Destination: Platform Fee (alias: `platform_fee`)
  - Destination: Settlement Pool (alias: `settlement_pool`)
  - Destination: Customer (CHECKING) — used for refunds

- Transaction Routes (examples):
  - Payment Flow: Customer → Merchant (+ Platform Fee)
  - Refund Flow: Merchant → Customer
  - Transfer Flow: Checking → Checking

These routes are created automatically after account types. They can be inspected via your Midaz environment or used by advanced workflows.

### Integrity Checks (Phase 7)

After batch generation, the demo runs a balance/integrity check and adds a per-asset summary to the report:

- Aggregates balances per asset (available, on-hold)
- Flags overdrawn accounts (negative available)
- Computes an internal net total excluding aliases prefixed with `@external/`
- Marks `doubleEntryBalanced` when internal net is zero

The summary is embedded in the JSON/HTML report under `dataSummary.balanceSummaries`.

Example non-interactive BR locale + patterns disabled:

```
cd examples/mass-demo-generator
DEMO_NON_INTERACTIVE=1 go run . --org-locale=br --patterns=false
```
