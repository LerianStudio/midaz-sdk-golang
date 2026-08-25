# Midaz Go SDK — Examples

Every example here is a runnable `main` package. The numbered examples
(01-10) are **focused tutorials**: each one teaches exactly one concept
with the smallest possible body. The non-numbered examples are
**reference / advanced** material — comprehensive but unfocused.

If this is your first time with the SDK, read the numbered examples in
order. If you already know what you're looking for, jump to the relevant
section below.

## Which surface these examples use

The SDK serves **both** Midaz ledger surfaces, and the version lives in
the request path rather than in the base URL: `c.V1.<Service>` (14
services) and `c.V2.<Service>` (22 services). Tracer accessors are not
grouped — they stay flat on the client.

Midaz deprecated all of /v1, so **these examples use `c.V2` by default**.
Two flows deliberately stay on `c.V1`, because the endpoints they
demonstrate exist only there:

| Stays on V1 | Why |
|---|---|
| [`workflow-with-entities/`](workflow-with-entities/) transaction flows | `CreateJSON` and the nested send/source/distribute envelope: /v2 replaced the four /v1 creation styles with the flat top-level `direct`/`hold` actions |
| asset rates | /v2 dropped the resource entirely |

[`03-end-to-end/`](03-end-to-end/) shows the /v2 creation path.

## Start here

| Example | Demonstrates | Body size |
|---|---|---|
| [`01-hello-world/`](01-hello-world/) | Minimal init + first API call | ~17 lines |
| [`02-auth/`](02-auth/) | Access Manager authentication (production auth) | small |

## Common workflows

| Example | Demonstrates |
|---|---|
| [`03-end-to-end/`](03-end-to-end/) | Creating a transaction on /v2 (`CreateDirect`) |
| [`04-listing-cursor/`](04-listing-cursor/) | Cursor-based pagination with `iter.Seq2` |
| [`05-listing-pages/`](05-listing-pages/) | Page-based pagination — `List` / `ListAll` / `ListPages` |

## Behavior & resilience

| Example | Demonstrates |
|---|---|
| [`06-idempotency/`](06-idempotency/) | Auto / explicit / suppressed idempotency modes |
| [`07-retries/`](07-retries/) | Default policy, custom policy, disabled retries |
| [`08-logging-slog/`](08-logging-slog/) | `*slog.Logger` integration (the SDK's only logging surface) |

## Testing & observability

| Example | Demonstrates |
|---|---|
| [`09-testing-with-mocks/`](09-testing-with-mocks/) | `go.uber.org/mock` for unit testing your code against the SDK |
| [`10-observability-otel/`](10-observability-otel/) | Full OpenTelemetry surface (tracing + metrics + logs) |

## Reference / advanced

| Example | Demonstrates |
|---|---|
| [`concurrency/`](concurrency/) | Concurrent SDK usage with bounded parallelism |
| [`concurrency/balance-fetch/`](concurrency/balance-fetch/) | Concurrent balance fetching pattern |
| [`configuration/`](configuration/) | Full client / config setup variations |
| [`context/`](context/) | `context.Context` usage with deadlines, cancellation, request-scoped values |
| [`tracing/`](tracing/) | Client-side OTel trace propagation |
| [`tracing-server/`](tracing-server/) | Server-side trace context extraction |
| [`pkg-validation-demo/`](pkg-validation-demo/) | `pkg/validation` validators (does not use the client) |
| [`mass-demo-generator/`](mass-demo-generator/) | Production-shaped data generator at scale |
| [`workflow-with-entities/`](workflow-with-entities/) | Every public service through full CRUD |

## Shared infrastructure

- [`internal/quickstart/`](internal/quickstart/) — bootstrap helpers used by
  examples that don't need to teach client construction. NOT a public SDK
  helper — examples may reach in, your code may not.

## Conventions

- Every example has a `README.md` covering: what it demonstrates, when
  to use the pattern, how to run it, expected output, and related
  examples / docs.
- Every example compiles cleanly with `go build ./examples/...`.
- Every example targets a **local Midaz stack** by default. Examples
  needing production-style auth document the env vars they require.
- Examples may load a `.env` file via `godotenv` if present in the
  working directory; configuration via env vars is always supported as
  a fallback.

## Running everything

```bash
# Build everything (compile-time check)
go build ./examples/...

# Run a specific example
go run ./examples/01-hello-world

# Seed demo data (mass-demo-generator)
make demo-data
```
