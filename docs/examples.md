# Midaz Go SDK examples

This guide shows the current SDK access patterns for configuration, entity services, pagination, transactions, tracing, and the mass demo generator.

## Client initialization

Environment loading is explicit. Use `config.FromEnvironment()` when you want process environment values to configure the SDK. If you use a `.env` file, load it first with your application's dotenv loader:

```go
import (
    "context"

    client "github.com/LerianStudio/midaz-sdk-golang/v2"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
)

cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := client.New(
    client.WithConfig(cfg),
    client.UseAllAPIs(),
)
if err != nil {
    return err
}
defer c.Shutdown(context.Background())
```

For local testing with a direct base URL:

```go
c, err := client.New(
    client.WithBaseURL("http://localhost:3002"),
    client.UseAllAPIs(),
)
```

## Working with organizations

Prefer model builders for wrapped Midaz model inputs:

```go
orgInput := models.NewCreateOrganizationInput("Example Organization", "123456789").
    WithDoingBusinessAs("Example").
    WithAddress(models.Address{
        Line1:   "123 Main St",
        City:    "San Francisco",
        State:   "CA",
        Country: "US",
        ZipCode: "94105",
    }).
    WithMetadata(map[string]any{
        "industry": "fintech",
        "size":     "startup",
    })

org, err := c.Entity.Organizations.CreateOrganization(ctx, orgInput)
if err != nil {
    return err
}

org, err = c.Entity.Organizations.GetOrganization(ctx, org.ID)
if err != nil {
    return err
}

orgs, err := c.Entity.Organizations.ListOrganizations(ctx,
    models.NewListOptions().WithLimit(20).WithFilter("status", "ACTIVE"),
)
```

## Account and asset management

```go
assetInput := models.NewCreateAssetInputWithType("US Dollar", "USD", "currency").
    WithMetadata(map[string]any{
        "symbol":  "$",
        "country": "US",
    })

asset, err := c.Entity.Assets.CreateAsset(ctx, orgID, ledgerID, assetInput)
if err != nil {
    return err
}

accountInput := models.NewCreateAccountInput("Customer Checking Account", asset.Code, "deposit").
    WithAlias("customer-123").
    WithMetadata(map[string]any{
        "customer_id": "cust-123",
        "tier":        "premium",
    })

account, err := c.Entity.Accounts.CreateAccount(ctx, orgID, ledgerID, accountInput)
if err != nil {
    return err
}

balance, err := c.Entity.Accounts.GetBalance(ctx, orgID, ledgerID, account.ID)
```

Use `BalancesService` when you need balance records by balance ID, all balances for an account, history, alias lookup, or external-code lookup:

```go
balances, err := c.Entity.Balances.ListAccountBalances(ctx, orgID, ledgerID, account.ID, nil)
```

## Transaction processing

The current transaction contract uses a `send` payload. You can build it explicitly:

```go
txInput := models.NewCreateTransactionInput("USD", "100.00").
    WithDescription("Payment from customer to merchant").
    WithSend(&models.SendInput{
        Asset: "USD",
        Value: "100.00",
        Source: &models.SourceInput{
            From: []models.FromToInput{
                {
                    Account: customerAccountAlias,
                    Amount: models.AmountInput{Asset: "USD", Value: "100.00"},
                },
            },
        },
        Distribute: &models.DistributeInput{
            To: []models.FromToInput{
                {
                    Account: merchantAccountAlias,
                    Amount: models.AmountInput{Asset: "USD", Value: "100.00"},
                },
            },
        },
    }).
	WithMetadata(map[string]any{
		"payment_id":  "pay-123",
		"customer_id": "cust-123",
	})
txInput.IdempotencyKey = "payment-2026-05-03-0001"

tx, err := c.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, txInput)
```

For DSL-style structured transactions, use `TransactionDSLInput`:

```go
tx, err := c.Entity.Transactions.CreateTransactionWithDSL(ctx, orgID, ledgerID, &models.TransactionDSLInput{
    Description: "Split payment transaction",
    Send: &models.DSLSend{
        Asset: "USD",
        Value: "100.00",
        Source: &models.DSLSource{
            From: []models.DSLFromTo{
                {Account: customerAccountAlias, Amount: &models.DSLAmount{Asset: "USD", Value: "100.00"}},
            },
        },
        Distribute: &models.DSLDistribute{
            To: []models.DSLFromTo{
                {Account: merchantAccountAlias, Amount: &models.DSLAmount{Asset: "USD", Value: "85.00"}},
                {Account: platformFeeAlias, Amount: &models.DSLAmount{Asset: "USD", Value: "10.00"}},
                {Account: processorFeeAlias, Amount: &models.DSLAmount{Asset: "USD", Value: "5.00"}},
            },
        },
    },
})
```

For raw DSL file content, use `CreateTransactionWithDSLFile(ctx, orgID, ledgerID, []byte(content))`. The SDK sends `POST /transactions/dsl` as multipart form data using field name `transaction`, filename `transaction.dsl`, and UTF-8 DSL content; empty, invalid UTF-8, and over-limit payloads are rejected before network I/O.

## Using pagination

```go
options := models.NewListOptions().
    WithLimit(25).
    WithOrderDirection(models.SortDescending).
    WithFilter("status", "ACTIVE")

for {
    page, err := c.Entity.Accounts.ListAccounts(ctx, orgID, ledgerID, options)
    if err != nil {
        return err
    }

    for _, account := range page.Items {
        process(account)
    }

    if !page.Pagination.HasNextPage() {
        break
    }

    options = page.Pagination.NextPageOptions()
}
```

See [pagination](./pagination.md) for details on `page`, `limit`, cursor behavior, and sort serialization.

## Example applications

Runnable examples live in `examples/`:

- `access-manager-example/` - Access Manager authentication configuration.
- `clean-transaction/` - Simple transaction flow.
- `concurrency-example/` - Concurrent SDK usage.
- `concurrency-example/balance-fetch/` - Concurrent balance fetching.
- `configuration-examples/` - Client and config setup patterns.
- `context-example/` - Context timeout and cancellation usage.
- `mass-demo-generator/` - End-to-end demo data generation.
- `observability-demo/` - Observability setup.
- `retry-example/` - Retry behavior.
- `tracing-example/` - Client-side tracing propagation.
- `tracing-server-example/` - Server-side tracing propagation.
- `validation-example/` - Input validation patterns.
- `workflow-with-entities/` - Complete entity workflow.

Most examples can be run with:

```bash
cd examples/example-name
go run .
```

## Mass demo generator

The `examples/mass-demo-generator` app creates realistic organizations, ledgers, assets, accounts, balances, routes, transactions, and reports. It uses concurrent processing and is the best starting point for demo data.

### Basic usage

Interactive mode:

```bash
cd examples/mass-demo-generator
go run .
```

Non-interactive mode:

```bash
cd examples/mass-demo-generator
DEMO_NON_INTERACTIVE=1 go run . \
  --orgs=3 \
  --ledgers=2 \
  --accounts=50 \
  --tx=100 \
  --concurrency=10 \
  --batch=25 \
  --org-locale=br
```

### Command line options

| Flag | Type | Default from `default.yaml` | Description |
| --- | --- | --- | --- |
| `--timeout` | int | `120` | Overall generation timeout in seconds |
| `--orgs` | int | `2` | Number of organizations to create |
| `--ledgers` | int | `2` | Ledgers per organization |
| `--accounts` | int | `100` | Accounts per ledger |
| `--tx` | int | `50` | Transactions per account for the demo batch |
| `--concurrency` | int | `32` | Worker pool size; `0` means auto when not overridden by defaults |
| `--batch` | int | `50` | Batch size for parallel operations |
| `--org-locale` | string | `us` | Organization locale (`us` or `br`) |

### Non-interactive environment controls

The generator also reads non-interactive defaults from `examples/mass-demo-generator/default.yaml`. Batch-demo controls include:

- `DEMO_RUN_BATCH` - Enable the send-based transfer batch demo.
- `DEMO_ASSET_CODE` - Asset code used by the batch demo.
- `DEMO_CHART_GROUP` - Chart of accounts group for transaction creation.

### Generated data structure

The generator creates:

- Organizations with locale-aware legal documents.
- Ledgers for each organization.
- Assets such as USD, EUR, and BTC depending on configuration.
- Account types and accounts.
- Portfolio and segment hierarchy when enabled.
- Operation routes and transaction routes.
- Transactions using the current send-based transaction contract.

### Reports and output

The generator writes report files in its working directory, including machine-readable entity references and generation summaries. Console output includes progress and performance metrics.

### Example scenarios

Small demo dataset:

```bash
DEMO_NON_INTERACTIVE=1 go run . \
  --orgs=1 \
  --ledgers=1 \
  --accounts=10 \
  --tx=20 \
  --org-locale=us
```

Larger performance dataset:

```bash
DEMO_NON_INTERACTIVE=1 go run . \
  --orgs=10 \
  --ledgers=3 \
  --accounts=100 \
  --tx=500 \
  --concurrency=20 \
  --batch=50
```

Brazilian organization demo:

```bash
DEMO_NON_INTERACTIVE=1 go run . --org-locale=br
```

CI-friendly bounded run:

```bash
timeout 600 go run . --timeout=600 --orgs=5 --tx=100
```
