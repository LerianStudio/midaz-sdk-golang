# 10-observability-otel

Demonstrates the observability surface — OpenTelemetry tracing,
metrics, and logging wired through the SDK's
`pkg/observability.Provider`.

## What this demonstrates

- Building a `Provider` via `observability.New(ctx, ...)` with the
  functional-options pattern (Track 6 unified shape)
- Wiring it into the client via `midaz.WithObservabilityProvider(provider)`
- Custom span operations (`provider.Tracer().Start`)
- Custom metrics (counters, histograms, gauges)
- Baggage propagation
- HTTP middleware for inbound trace extraction

## When to use this pattern

Production. Every Midaz service emits OTel signals; matching that on the
client side means correlated traces across the boundary, p99 latency
histograms per endpoint, and structured logs co-located with span IDs.

## How to run

```bash
# Optional: point at an OTLP collector
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

go run ./examples/10-observability-otel
```

If no collector is configured, the SDK uses a no-op exporter — the
example still runs end-to-end, you just won't see signals downstream.

## Expected output

```
[INFO] Starting observability demo
[INFO] Demonstrating span operations
... (spans + metrics emitted to OTLP collector or no-op exporter)
[INFO] Observability demo completed
```

## Related

- [`docs/tracing.md`](../../docs/tracing.md) — full tracing contract
- `midaz.WithObservabilityOptions`, `midaz.WithObservabilityProvider`
