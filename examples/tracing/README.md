# tracing

Reference example for **client-side OpenTelemetry tracing** — the SDK
emits spans for every HTTP request and the example wraps them in
business-level spans.

## What this demonstrates

- Building an `observability.Provider` with tracing enabled
- Wrapping an SDK call in a parent span via `provider.Tracer().Start(ctx, "...")`
- Auto-instrumented HTTP spans from the SDK with proper parent-child
  relationships
- Span attributes carrying business context (tenant ID, request type, etc.)

## When to use this pattern

When you want correlated traces from your service through the SDK to
the Midaz backend. Combined with [`tracing-server/`](../tracing-server/)
on the receiving end, you get end-to-end distributed traces.

## How to run

```bash
# Optional: configure an OTLP exporter
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

go run ./examples/tracing
```

## Related

- [`tracing-server/`](../tracing-server/) — the inbound side
- [`10-observability-otel/`](../10-observability-otel/) — full OTel surface
- [`docs/tracing.md`](../../docs/tracing.md) — tracing contract
