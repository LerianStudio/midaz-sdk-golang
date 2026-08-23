# 07-retries

Demonstrates the v3 retry surface — three modes the SDK supports for
HTTP-level retries.

## What this demonstrates

1. **Default policy (recommended)** — exponential backoff with jitter on
   5xx, 408, 425, 429, and transport errors. Triggered automatically by
   `midaz.New(...)` with no extra options.

2. **Custom policy** — `WithRetryOptions(...retry.Option)` to tune
   `MaxRetries`, `InitialBackoff`, `MaxBackoff`, `BackoffFactor`, the
   `RetryableStatusCodes` set, `RetryableErrors`, and the `OnRetry`
   callback hook. Or `WithCustomRetryPolicy(func(*Response, error) bool)`
   to define an arbitrary predicate.

3. **Disabled retries** — `WithoutRetries()` for the rare case where
   the caller orchestrates retries elsewhere (workflow engine, saga
   coordinator, etc.) and wants the SDK to fail fast.

## When to use this pattern

The default works for ~all use cases. Reach for custom policies when:

- You have specific SLA bounds (cap at 3 retries, max 2s total)
- The downstream service requires a specific backoff curve
- You need to instrument retry attempts for SRE dashboards via `OnRetry`

Reach for `WithoutRetries()` when retries are someone else's job.

## How to run

```bash
go run ./examples/07-retries
```

Requires a local Midaz stack. The example exercises three independently-
configured clients to show the contrast.

## Expected output

```
--- Default retry policy ---
request succeeded (or failed cleanly after retries)
--- Custom retry policy with OnRetry hook ---
attempt 1: status=503
attempt 2: status=200
request succeeded
--- Retries disabled ---
request failed fast: ...
```

## Related

- [`pkg/retry` godoc](https://pkg.go.dev/github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry)
- `midaz.WithRetryOptions`, `midaz.WithCustomRetryPolicy`, `midaz.WithoutRetries`
