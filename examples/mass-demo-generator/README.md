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

These are gitignored — see the project root `.gitignore`.

## Related

- [`workflow-with-entities/`](../workflow-with-entities/) — same shape, smaller scope
- [`07-retries/`](../07-retries/) — retry contract this generator depends on
- [`06-idempotency/`](../06-idempotency/) — idempotency shape this generator depends on
