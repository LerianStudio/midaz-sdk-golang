# concurrency

Reference example for **concurrent SDK usage** — multiple goroutines
sharing a single `*midaz.Client`.

## What this demonstrates

- The SDK client is goroutine-safe by construction; one client per
  process is the canonical shape
- A worker-pool pattern for fanning out independent reads/writes
- The companion `balance-fetch/` sub-example fetches account balances
  concurrently with bounded parallelism

## When to use this pattern

When throughput matters. Typical shape: one client at process boot,
shared via context or DI; per-request goroutines call methods in
parallel. The connection pool, retry policy, observability provider,
and rate limiter are all process-wide and concurrent-safe.

## How to run

```bash
go run ./examples/concurrency
go run ./examples/concurrency/balance-fetch
```

Requires a local Midaz stack with auth disabled and seeded data
(run [`03-end-to-end/`](../03-end-to-end/) first).

## Related

- [`pkg/concurrency`](../../pkg/concurrency/) — worker-pool helpers
- [`workflow-with-entities/`](../workflow-with-entities/) — concurrent
  resource creation with retry + idempotency
