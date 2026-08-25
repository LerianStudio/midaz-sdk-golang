# Midaz Go SDK examples

This guide shows the current SDK access patterns for configuration, entity services, pagination, transactions, tracing, and the mass demo generator.

## Client initialization

Environment loading is explicit. Use `config.FromEnvironment()` when you want process environment values to configure the SDK. If you use a `.env` file, load it first with your application's dotenv loader:

```go
import (
    "context"

    "github.com/LerianStudio/midaz-sdk-golang/v5"
    "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
)

cfg, err := config.NewConfig(config.FromEnvironment())
if err != nil {
    return err
}

c, err := midaz.New(
    midaz.WithConfig(cfg),
    midaz.WithAnonymous(),
)
if err != nil {
    return err
}
defer func() {
    _ = c.Shutdown(context.Background())
}()
```

For local testing with a direct base URL:

```go
c, err := midaz.New(
    midaz.WithBaseURL("http://localhost"),
    midaz.WithAnonymous(),
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

org, err := c.V2.Organizations.Create(ctx, orgInput)
if err != nil {
    return err
}

org, err = c.V2.Organizations.Get(ctx, org.ID)
if err != nil {
    return err
}

orgs, err := c.V2.Organizations.List(ctx, models.OrganizationsListOpts{
    PageListOpts: models.PageListOpts{Limit: 20},
    Filters:      models.OrganizationsFilters{Status: "ACTIVE"},
})
```

## Account and asset management

```go
assetInput := models.NewCreateAssetInputWithType("US Dollar", "USD", "currency").
    WithMetadata(map[string]any{
        "symbol":  "$",
        "country": "US",
    })

asset, err := c.V2.Assets.Create(ctx, orgID, ledgerID, assetInput)
if err != nil {
    return err
}

accountInput := models.NewCreateAccountInput("Customer Checking Account", asset.Code, "deposit").
    WithAlias("customer-123").
    WithMetadata(map[string]any{
        "customer_id": "cust-123",
        "tier":        "premium",
    })

account, err := c.V2.Accounts.Create(ctx, orgID, ledgerID, accountInput)
if err != nil {
    return err
}

// Account balances are listed (cursor-paginated); pick the first for a single-asset account.
balances, err := c.V2.Balances.ListAccountBalances(ctx, orgID, ledgerID, account.ID, models.BalancesListOpts{})
if err != nil {
    return err
}
balance := balances.Items[0]
```

The account-scoped balance read lives on `Balances`, not on `Accounts`. On /v1
it is spelled on both (`c.V1.Accounts.ListBalances` and
`c.V1.Balances.ListAccountBalances` are the same wire call); /v2 spells each
endpoint exactly once, on the accessor named after what it returns.

`Balances` also serves balance records by balance ID, point-in-time history,
and the alias and external-code lookups:

```go
bal, err := c.V2.Balances.GetBalance(ctx, orgID, ledgerID, balanceID)
hist, err := c.V2.Balances.GetBalanceHistory(ctx, orgID, ledgerID, balanceID, "2026-06-30")

// The alias and external-code lookups take no options and are not paginated:
// the endpoint accepts no query parameters and answers with a fixed page.
byAlias, err := c.V2.Balances.ListBalancesByAccountAlias(ctx, orgID, ledgerID, "@customer-1")
```

## Transaction processing

The two surfaces have **different transaction contracts**, not different paths
to one contract.

On /v2 — the surface to build against — the request is flat: an asset, a total,
and two leg arrays. The action lives in the URL, so the SDK spells it as a
method: `CreateDirect` settles immediately, `CreateHold` reserves value for a
later `Commit`.

```go
txInput := &models.CreateTransactionV2Input{
    Asset:       "USD",
    Amount:      "100.00",
    Description: "Payment from customer to merchant",
    Debits:      []models.TransactionV2Leg{{Alias: customerAccountAlias, Amount: "100.00"}},
    Credits:     []models.TransactionV2Leg{{Alias: merchantAccountAlias, Amount: "100.00"}},
    Metadata: map[string]any{
        "payment_id":  "pay-123",
        "customer_id": "cust-123",
    },
}
txInput.IdempotencyKey = "payment-2026-05-03-0001"

tx, err := c.V2.Transactions.CreateDirect(ctx, orgID, ledgerID, txInput)
```

Each leg names the organization and ledger its account belongs to; the facade
stamps them from the pair passed to `CreateDirect`, into a copy, so one input
can be reused against a second ledger. A leg naming a *different* pair is
refused rather than posted into the wrong ledger.

A leg carries **exactly one** value expression — an explicit `Amount`, or a
`Share` of the total. Both, or neither, is refused before the request leaves.

A structured split is simply more `Credits` entries:

```go
splitInput := &models.CreateTransactionV2Input{
    Asset:       "USD",
    Amount:      "100.00",
    Description: "Split payment transaction",
    Debits:      []models.TransactionV2Leg{{Alias: customerAccountAlias, Amount: "100.00"}},
    Credits: []models.TransactionV2Leg{
        {Alias: merchantAccountAlias, Amount: "85.00"},
        {Alias: platformFeeAlias, Amount: "10.00"},
        {Alias: processorFeeAlias, Amount: "5.00"},
    },
}

tx, err := c.V2.Transactions.CreateDirect(ctx, orgID, ledgerID, splitInput)
```

### The /v1 creation styles

/v1 nests a `send` envelope behind four endpoints — `CreateJSON`,
`CreateInflow`, `CreateOutflow`, `CreateAnnotation`. **These have no /v2 twin**,
so they stay reachable on `c.V1.Transactions` for as long as Midaz serves /v1:

```go
txInput := models.NewCreateTransactionInput("USD", "100.00").
    WithDescription("Payment from customer to merchant").
    WithSend(&models.SendInput{
        Asset: "USD",
        Value: "100.00",
        Source: &models.SourceInput{
            From: []models.FromToInput{
                {
                    AccountAlias: customerAccountAlias,
                    Amount: models.AmountInput{Asset: "USD", Value: "100.00"},
                },
            },
        },
        Distribute: &models.DistributeInput{
            To: []models.FromToInput{
                {
                    AccountAlias: merchantAccountAlias,
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

tx, err := c.V1.Transactions.CreateJSON(ctx, orgID, ledgerID, txInput)
```

On /v1 a structured split adds multiple entries to `Distribute.To`, the same
way /v2 adds `Credits` entries.

## Waiting for settlement

A `201` from a create means the transaction was recorded, not that the
ledger balance reflects it. Wait on the balance effect with
`transaction.WaitForSettlement`, passing a predicate that decides what
"settled" means for your case (here, the account's `USD` balance version
advancing). Pin the asset on a multi-asset account:

```go
import "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/transaction"

settled, err := transaction.WaitForSettlement(
    ctx,
    c.Balances, // satisfies the balance reader structurally
    orgID, ledgerID, accountID,
    func(b models.Balance) bool {
        return b.AssetCode == "USD" && b.Version >= 2
    },
    transaction.WithTimeout(15*time.Second),
)
if errors.Is(err, transaction.ErrSettlementTimeout) {
    // balance did not settle within the deadline
    return err
}
if err != nil {
    return err
}
log.Printf("settled: available=%s version=%d", settled.Available, settled.Version)
```

## Fee estimation and billing

Fee, billing, and holder-account composition are **/v2 only** — Midaz removed
the fee and billing families from /v1 — and they are **ledger-scoped** there:
the family moved from organization scope to ledger scope in /v2, so every
method takes a ledger ID.

Feed a billing period into `NewBillingCalculateInput` and call the
`BillingCalculations` accessor; an empty result set (no packages matched) is a
success, not an error. The ledger in the path and the `ledgerId` in the body
are reconciled before the request leaves the SDK — filled when empty, refused
when they contradict:

```go
calcInput := models.NewBillingCalculateInput(ledgerID, "2026-06").
    WithType("volume")

result, err := c.V2.BillingCalculations.CalculateBilling(ctx, orgID, ledgerID, calcInput)
if err != nil {
    return err
}
log.Printf("billing results=%d net=%s", len(result.Results), result.Summary.TotalNetAmount)
```

Open a holder-owned account (optionally with its instrument) in one call with
`NewCreateHolderAccountInput` and the `Composition` accessor. A populated
`InstrumentError` on success means the account committed but the instrument
write did not — a success, not an error:

```go
acctInput := models.NewCreateHolderAccountInput("USD", "deposit").
    WithName("Primary settlement account")

resp, err := c.V2.Composition.CreateHolderAccount(ctx, orgID, ledgerID, holderID, acctInput)
if err != nil {
    return err
}
if resp.InstrumentError != nil {
    log.Printf("account created; instrument failed: %s", resp.InstrumentError.Reason)
}
```

## Using pagination

The primary entity accessors ship every paginated list method in a trio: `List` (one page), `All` (every item across pages), and `Pages` (every page envelope). `MetadataIndexes.ListMetadataIndexes` is intentionally non-paginated. Use `iter.Seq2` for auto-paging — the SDK advances cursors and pages internally:

```go
opts := models.AccountsListOpts{
    PageListOpts: models.PageListOpts{
        Limit:         25,
        SortDirection: models.SortDescending,
    },
    Filters: models.AccountsFilters{Status: "ACTIVE"},
}

for account, err := range c.V2.Accounts.All(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return fmt.Errorf("list accounts: %w", err)
    }
    process(account)
}
```

When you need page-level metadata (cursor, total, page number) — for checkpointing, batching, or stopping mid-collection — use the `Pages` variant:

```go
for page, err := range c.V2.Accounts.Pages(ctx, orgID, ledgerID, opts) {
    if err != nil {
        return err
    }
    log.Printf("page=%d items=%d next_cursor=%q",
        page.Pagination.Page, len(page.Items), page.Pagination.NextCursor)
    for _, account := range page.Items {
        process(account)
    }
}
```

For one-page-at-a-time control (UI pagination, manual replay), call `List` directly and inspect `page.Pagination.HasMore()`:

```go
page, err := c.V2.Accounts.List(ctx, orgID, ledgerID, opts)
if err != nil {
    return err
}
for _, account := range page.Items {
    process(account)
}
if page.Pagination.HasMore() {
    // For page-based: increment opts.Page; for cursor-based: copy
    // page.Pagination.NextCursor into opts.Cursor.
}
```

See [pagination](./pagination.md) for the full contract: page-based vs cursor-based endpoints, the `*All` / `*Pages` / `List` decision tree, and the typed-opts compile-time guarantees.

## Example applications

Runnable examples live in `examples/`:

**Start here**

- `01-hello-world/` - Minimal init + first API call (≤30 lines).
- `02-auth/` - Access Manager authentication configuration.

**Common workflows**

- `03-end-to-end/` - Org → ledger → account → transaction.
- `04-listing-cursor/` - Paginate transactions with `iter.Seq2`.
- `05-listing-pages/` - Paginate accounts with page metadata.

**Behavior & resilience**

- `06-idempotency/` - Auto, manual, and per-call opt-out.
- `07-retries/` - Default policy, custom policy, disabled.
- `08-logging-slog/` - `*slog.Logger` integration.

**Testing & observability**

- `09-testing-with-mocks/` - `go.uber.org/mock` for unit tests.
- `10-observability-otel/` - OpenTelemetry tracing + metrics + logs.

**Reference / advanced**

- `concurrency/` - Concurrent SDK usage.
- `concurrency/balance-fetch/` - Concurrent balance fetching.
- `configuration/` - Client and config setup patterns.
- `context/` - Context timeout and cancellation usage.
- `tracing/` - Client-side tracing propagation.
- `tracing-server/` - Server-side tracing propagation.
- `pkg-validation-demo/` - Input-validation patterns from `pkg/validation` (does not use the client).
- `mass-demo-generator/` - End-to-end demo data generation.
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
DEMO_AUTH_MODE=anonymous-local go run .
```

Non-interactive mode:

```bash
cd examples/mass-demo-generator
DEMO_NON_INTERACTIVE=1 DEMO_AUTH_MODE=anonymous-local go run . \
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

- `DEMO_TIMEOUT` - Overall timeout in seconds.
- `DEMO_ORGS` - Organizations to create.
- `DEMO_LEDGERS_PER_ORG` - Ledgers per organization.
- `DEMO_ACCOUNTS_PER_LEDGER` - Accounts per ledger.
- `DEMO_TX_PER_ACCOUNT` - Transactions per account.
- `DEMO_CONCURRENCY` - Worker pool size.
- `DEMO_BATCH_SIZE` - Batch size.
- `DEMO_ASSETS` - Assets per ledger.
- `DEMO_CREATE_HIERARCHY` - Enable account hierarchy generation.
- `DEMO_RUN_FLOW` - Enable the organization/ledger/account generation flow.
- `DEMO_RUN_BATCH` - Enable the send-based transfer batch demo.
- `DEMO_ASSET_CODE` - Asset code used by the batch demo.
- `DEMO_CHART_GROUP` - Chart of accounts group for transaction creation.
- `DEMO_LOCALE` - Organization locale (`us` or `br`).
- `DEMO_AUTH_MODE=anonymous-local` - Explicitly allow anonymous auth for an unsecured local Midaz stack when `PLUGIN_AUTH_ENABLED` is not `true`.

Configuration precedence is explicit CLI flag, then `DEMO_*` environment variable, then `default.yaml`, then hardcoded fallback.

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

The generator writes report files in its working directory, including machine-readable entity references and generation summaries. Console output includes progress and performance metrics. These files contain operational identifiers and should not be shared publicly. JSON/HTML reports summarize the full batch and retain a bounded sample of transaction results to avoid huge local artifacts.

Generated artifacts can be removed with:

```bash
rm -f examples/mass-demo-generator/mass-demo-report.* examples/mass-demo-generator/mass-demo-entities.json
```

### Example scenarios

Small demo dataset:

```bash
DEMO_NON_INTERACTIVE=1 DEMO_AUTH_MODE=anonymous-local go run . \
  --orgs=1 \
  --ledgers=1 \
  --accounts=10 \
  --tx=20 \
  --org-locale=us
```

Larger performance dataset:

```bash
DEMO_NON_INTERACTIVE=1 DEMO_AUTH_MODE=anonymous-local go run . \
	--timeout=1800 \
  --orgs=10 \
  --ledgers=3 \
  --accounts=100 \
  --tx=500 \
  --concurrency=20 \
  --batch=50
```

Brazilian organization demo:

```bash
DEMO_NON_INTERACTIVE=1 DEMO_AUTH_MODE=anonymous-local go run . --org-locale=br
```

CI-friendly bounded run:

```bash
DEMO_NON_INTERACTIVE=1 DEMO_AUTH_MODE=anonymous-local go run . \
  --timeout=120 \
  --orgs=1 \
  --ledgers=1 \
  --accounts=1 \
  --tx=0
```
