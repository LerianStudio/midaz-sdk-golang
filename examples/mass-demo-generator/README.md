# mass-demo-generator

The **production-shaped demo data generator**. Creates realistic
organizations, ledgers, assets, accounts, balances, routes, and
transactions; emits a JSON entity manifest and an HTML report.

This is the largest example and the most realistic shape of an
SDK-using service: bounded concurrency, retries, idempotency keys,
observability, error handling, and progress reporting.

## What this demonstrates

- `examples/internal/quickstart`-style bootstrap (env-driven config,
  graceful shutdown)
- Concurrent resource creation with bounded parallelism
- Idempotency keys for at-least-once safety
- Locale-aware fake data generation (BR + US)
- Structured progress reporting + final HTML/JSON reports
- Error aggregation + non-fatal continuation patterns
- Live coverage of the **V2-only** families, with a fatal money-path
  assertion (see below)

## Phases

A full run walks these in order:

1. **Organizations, ledgers, assets** — locale-aware legal documents.
2. **Account types, routes, accounts, portfolio, segments.**
3. **Account hierarchy** (`DEMO_CREATE_HIERARCHY`).
4. **`/v1` transaction batch** (`DEMO_RUN_BATCH`) — bounded-concurrency
   funding transactions, asserted against the configured target count.
5. **`/v2`-only phase** (`DEMO_RUN_V2`) — see the next section.
6. **Reports** — JSON + HTML + the entity ID manifest. Written when
   **either** phase ran, so a `DEMO_RUN_BATCH=false DEMO_RUN_V2=true`
   run still gets artifacts naming everything it created. The batch
   summary is the one section that needs the batch.

## The V2-only phase (`DEMO_RUN_V2`, default `true`)

`/v1` is deprecated server-side, and several families exist *only* on
`/v2`. This phase runs after the `/v1` batch and exercises them against a
live stack, per ledger:

| Step | Surface | Fatal? |
| --- | --- | --- |
| CRM holders + holder-owned accounts with instruments, one instrument written directly | `V2.Holders`, `V2.Composition`, `V2.Instruments` | no |
| Fund → transfer → hold/commit → hold/cancel, then assert balances | `V2.Accounts`, `V2.Transactions`, `V2.Balances` | **yes** |
| Fee package + fee estimate (dry run) | `V2.FeePackages`, `V2.FeeEstimates` | no |
| Billing package + billing calculation | `V2.BillingPackages`, `V2.BillingCalculations` | no |
| Metadata index create → list → delete (once per run) | `V2.MetadataIndexes` | no |
| Encryption provisioning status, protection audit trail | `V2.Encryption`, `V2.ProtectionAudit` | no |

### What the transaction proof asserts

It opens two dedicated accounts (so the balances are predictable — the
`/v1` batch funds its accounts with random amounts), then posts these
amounts in whole units of the configured asset (`100.00` USD, not `100`
cents; the generator converts to the asset's minor units internally):

- fund source `100`, fund destination `50`, both from `@external/<asset>`
- a settled transfer of `30` source → destination
- a `20` hold source → destination, then a **commit**
- a `10` hold source → destination, then a **cancel**

and then asserts **exactly** `source = 50` and `destination = 100`, both
with nothing on hold. A mismatch fails the whole run. The canceled hold
is what makes the release path load-bearing: its `10` has to come back
to the source, so a cancel that stranded the value on hold or completed
it onto the destination fails the run. The read is retried on a bounded
interval so a slower stack reports a real mismatch rather than a race;
the expected values are computed once from the posted quantities and are
never adjusted from what a balance read came back with.

Everything else in the phase logs and continues, matching how the rest
of the generator treats a failed step.

### Notes

- **Instruments are written both ways.** Most go through the composition
  endpoint, which links the instrument to the account it opens in one
  call; one is written directly through `V2.Instruments.Create`, which
  covers the write side of that family against a live stack. The create
  payload names the ledger and the account the instrument belongs to,
  carries banking details and metadata, and carries nothing else — the
  endpoint rejects any property it does not declare.
  The direct write deliberately targets one of the `/v1` base accounts
  rather than a holder-owned one: the server allows a single instrument
  per account, and every account the composition endpoint opens already
  has its own. Pointing the direct create at one of those returns 409.
- **Encryption is read-only here.** Provisioning writes real key
  material into the deployment's KMS and keyset store; the status read
  answers the same question for free.
- Fee, billing and CRM all live in the `ledger` binary, so a stock local
  stack serves them with no extra containers.

## When to use this pattern

- Seeding a Midaz instance for development / QA / demos
- As a reference for production-shape SDK usage at non-trivial scale

## How to run

```bash
# Interactive mode (defaults + prompts for parameters)
go run ./examples/mass-demo-generator

# Non-interactive with locale override
DEMO_NON_INTERACTIVE=1 go run ./examples/mass-demo-generator --org-locale=br

# Or via the project Makefile
make demo-data
```

Configuration via `examples/mass-demo-generator/default.yaml` and the
`DEMO_*` environment variable family. See `main.go` for the full
flag/env contract.

## Output artifacts

- `mass-demo-entities.json` — full IDs of every resource created
- `mass-demo-report.html` — human-readable summary with timing / counts
- `mass-demo-report.json` — same data, machine-readable

They are written to the **current** directory, which is the repository
root when the generator is run the documented way. Both locations are
gitignored — see the project root `.gitignore`. They carry live resource
UUIDs and should not be shared.

## Related

- [`workflow-with-entities/`](../workflow-with-entities/) — same shape, smaller scope
- [`07-retries/`](../07-retries/) — retry contract this generator depends on
- [`06-idempotency/`](../06-idempotency/) — idempotency shape this generator depends on
