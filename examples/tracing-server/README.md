# tracing-server

Reference example for **server-side OpenTelemetry tracing** — your
HTTP server extracts incoming W3C trace context, enters a span for the
inbound request, and the SDK propagates that context through to its
own outbound calls.

## What this demonstrates

- Server-side trace context extraction (W3C `traceparent` header)
- Middleware-style span entry on inbound requests
- Passing the inbound context through to SDK calls so spans nest correctly
- Trace propagation across the boundary into Midaz services

## When to use this pattern

When your service is a participant in a larger distributed trace and
needs to (a) honor incoming traces and (b) propagate them to the
Midaz backend through the SDK.

## How to run

```bash
# Optional: configure OTLP exporter
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

go run ./examples/tracing-server
```

The example starts an HTTP server on a fixed port. Hit it with curl
including a `traceparent` header to see propagation, or call it from
[`tracing/`](../tracing/) for an end-to-end demo.

## Related

- [`tracing/`](../tracing/) — the outbound side
- [`docs/tracing.md`](../../docs/tracing.md) — tracing contract
