# context

Reference example for **`context.Context` usage** with the SDK —
deadlines, cancellation, and request-scoped values.

## What this demonstrates

- `context.WithTimeout` for per-request deadlines
- `context.WithCancel` for graceful cancellation across goroutines
- The `pkg/sdkctx` helpers for SDK-specific context values:
  - `WithIdempotencyKey` / `WithoutAutoIdempotency`
  - `WithRequestTenantID` (per-request tenant override)
  - `WithIncludeDeleted` (soft-delete visibility)

## When to use this pattern

Always. Every SDK call takes `ctx context.Context` as its first
argument. The SDK respects cancellation, propagates timeouts to the
underlying HTTP client, and reads its own request-scoped values from
context.

## How to run

```bash
go run ./examples/context
```

Requires a local Midaz stack with auth disabled.

## Related

- [`pkg/sdkctx`](../../pkg/sdkctx/) — request-scoped context helpers
- [`06-idempotency/`](../06-idempotency/) — idempotency through context
- [`docs/multi-tenancy.md`](../../docs/multi-tenancy.md) — tenant routing through context
