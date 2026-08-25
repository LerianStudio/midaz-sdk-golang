# 08-logging-slog

Demonstrates the canonical logging surface: an injected
`*slog.Logger` via `midaz.WithLogger`.

## What this demonstrates

- Building a `*slog.Logger` with a JSON handler at info level
- Passing it to `midaz.WithLogger(logger)` so the SDK uses your handler
  for every log record (debug, info, warn, error)
- The SDK is silent by default (discard handler) — `WithLogger` is opt-in
- Retry attempts are logged at info level via the SDK's internal
  `RecordRetry` call (Track 4)

## When to use this pattern

Always when you want SDK logs visible. `slog` is the Go 1.21+ standard
logging API and is the SDK's only logging surface. Adapters for zap,
zerolog, logrus, and others go through `slog.Handler` — there is no
custom Logger interface.

## How to run

```bash
go run ./examples/08-logging-slog
```

Requires a local Midaz stack with auth disabled.

## Expected output

```json
{"time":"...","level":"INFO","msg":"midaz: client constructed",...}
{"time":"...","level":"INFO","msg":"midaz: list organizations",...}
```

## Related

- [`docs/logging.md`](../../docs/logging.md) — full logging contract, adapter recipes
- `midaz.WithLogger`, `midaz.WithSlowCallThreshold`
