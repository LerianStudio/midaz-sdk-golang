# OpenTelemetry tracing in the Midaz Go SDK

The SDK can propagate OpenTelemetry trace context through outbound entity HTTP requests. Tracing is configured with `pkg/observability` and attached to the client with `client.WithObservabilityProvider` or SDK-created observability options.

## What the SDK provides

- OpenTelemetry tracer, meter, and logger provider abstraction.
- Automatic trace context and baggage injection on outbound entity HTTP requests.
- Context extraction and injection helpers for HTTP boundaries, including `http.Header` convenience helpers.
- Baggage helpers for cross-service correlation.
- HTTP middleware helpers for server applications.
- No-op behavior when observability is disabled.

## Client setup

Import the root module and alias it as `client`:

```go
import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "time"

    client "github.com/LerianStudio/midaz-sdk-golang/v2"
    "github.com/LerianStudio/midaz-sdk-golang/v2/models"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)
```

Create an observability provider and enable entity APIs:

```go
provider, err := observability.New(context.Background(),
    observability.WithServiceName("my-service"),
    observability.WithServiceVersion("1.0.0"),
    observability.WithEnvironment("production"),
    observability.WithComponentEnabled(true, true, true),
    observability.WithCollectorEndpoint("localhost:4317"),
    observability.WithTraceSampleRate(0.1),
)
if err != nil {
    log.Fatal(err)
}
defer provider.Shutdown(context.Background())

midazClient, err := client.New(
    client.WithBaseURL("https://api.midaz.com"),
    client.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
    client.WithObservabilityProvider(provider),
    client.UseAllAPIs(),
)
if err != nil {
    log.Fatal(err)
}
```

`client.UseAllAPIs()` or `client.UseEntityAPI()` is required before accessing `midazClient.Entity`.

Authentication is configured through SDK config and Access Manager, not through a `client.WithAuth` option.

## Tracing an operation

```go
tracer := provider.Tracer()
ctx, span := tracer.Start(context.Background(), "create_organization_workflow")
defer span.End()

orgInput := models.NewCreateOrganizationInput("Example Corporation", "123456789").
    WithDoingBusinessAs("Example Inc.")

organization, err := midazClient.Entity.Organizations.CreateOrganization(ctx, orgInput)
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
    return err
}

span.SetAttributes(attribute.String("organization.id", organization.ID))
span.SetStatus(codes.Ok, "organization created")
```

Outbound SDK calls automatically create one SDK client span and inject trace headers from `ctx`. The SDK honors the provider configured on the context or client, so propagation works even when `observability.WithRegisterGlobally(false)` is used.

Do not wrap the SDK HTTP transport with `observability.NewHTTPMiddleware(provider)` when the client is already configured with `client.WithObservabilityProvider(provider)` or `client.WithObservability(...)`. The entity HTTP client is the SDK instrumentation point; wrapping the same transport adds nested client spans for the same outbound request.

For applications that manage multiple OpenTelemetry providers in one process, keep the SDK provider local and attach it to request contexts explicitly:

```go
provider, err := observability.New(ctx,
    observability.WithServiceName("payments-api"),
    observability.WithRegisterGlobally(false),
)
if err != nil {
    return err
}

midazClient, err := client.New(
    client.WithObservabilityProvider(provider),
    client.UseAllAPIs(),
)
```

## Server-side context extraction

```go
func handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx := observability.WithProvider(r.Context(), provider)
    ctx = observability.ExtractHTTPContext(ctx, r.Header)
    tracer := provider.Tracer()
    ctx, span := tracer.Start(ctx, "handle_request")
    defer span.End()

    result, err := processRequest(ctx)
    if err != nil {
        span.RecordError(err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("X-Trace-ID", observability.TraceID(ctx))
    _ = json.NewEncoder(w).Encode(result)
}
```

For non-HTTP boundaries, use `observability.ExtractContext(ctx, map[string]string)` and `observability.InjectContext(ctx, map[string]string)`. For HTTP boundaries, prefer `ExtractHTTPContext` and `InjectHTTPContext` so multi-value headers and provider-specific propagators are handled consistently.

`observability.WithPropagationHeaders(...)` limits which inbound headers are considered during extraction. By default, W3C `traceparent`, `tracestate`, and `baggage` are accepted along with request/correlation IDs.

## Unified client-app, SDK, and Midaz API trace

```go
func handlePayment(w http.ResponseWriter, r *http.Request) {
    ctx := observability.WithProvider(r.Context(), provider)
    ctx = observability.ExtractHTTPContext(ctx, r.Header)

    ctx, span := provider.Tracer().Start(ctx, "payments.create")
    defer span.End()

    tx, err := midazClient.Entity.Transactions.CreateTransaction(ctx, orgID, ledgerID, input)
    if err != nil {
        span.RecordError(err)
        http.Error(w, "transaction failed", http.StatusBadGateway)
        return
    }

    _ = json.NewEncoder(w).Encode(tx)
}
```

The inbound request span, SDK HTTP client span, and downstream Midaz API span share the same trace ID when the Midaz API also extracts W3C trace context.

## Business events

When logging is enabled, high-value mutation lifecycle methods emit structured business events such as `midaz.account.created`, `midaz.account.updated`, `midaz.transaction.created`, `midaz.transaction.updated`, `midaz.transaction.committed`, `midaz.transaction.reverted`, and `midaz.transaction.cancelled`. Read-only `Get*` methods do not emit mutation lifecycle events.

Business events include safe identifiers only, such as `organizationId`, `ledgerId`, `accountId`, `transactionId`, `operationId`, `assetId`, `status`, `operation`, and `event`. The SDK does not log payloads, metadata, names, documents, addresses, auth headers, idempotency keys, or raw request/response bodies.

## Baggage

Use baggage for small correlation values that must cross service boundaries:

```go
ctx, err := observability.WithBaggageItem(ctx, "user-id", "user-123")
if err != nil {
    return err
}

ctx, err = observability.WithBaggageItem(ctx, "request-id", "req-456")
if err != nil {
    return err
}

userID := observability.GetBaggageItem(ctx, "user-id")
requestID := observability.GetBaggageItem(ctx, "request-id")
```

Keep baggage small. Baggage is propagated on every downstream request.

## Trace IDs for logs

```go
traceID := observability.TraceID(ctx)
spanID := observability.SpanID(ctx)

logger := provider.Logger()
logger.Info("processing request", map[string]any{
    "trace_id": traceID,
    "span_id":  spanID,
})
```

## Configuration options

Common observability options:

- `observability.WithServiceName(string)`
- `observability.WithServiceVersion(string)`
- `observability.WithEnvironment(string)`
- `observability.WithComponentEnabled(tracing, metrics, logging bool)`
- `observability.WithTraceSampleRate(float64)`
- `observability.WithHighTracingSampling()`
- `observability.WithFullTracingSampling()`
- `observability.WithCollectorEndpoint(string)`
- `observability.WithDevelopmentDefaults()`
- `observability.WithProductionDefaults()`

Collector endpoint values are passed to the OTLP gRPC exporter as `host:port`, for example `localhost:4317`. Do not include `http://` in this value.

The SDK does not currently load observability configuration from `MIDAZ_OTEL_ENDPOINT` or `MIDAZ_LOG_LEVEL`. Configure observability programmatically.

## Local collector example

Run an OpenTelemetry Collector locally on OTLP gRPC port `4317`, then configure:

```go
observability.WithCollectorEndpoint("localhost:4317")
```

If you want Jaeger or Zipkin, route traces through the OpenTelemetry Collector. The SDK exports OTLP gRPC; it does not directly export Zipkin spans.

## Tests

Run tracing-related tests with:

```bash
go test ./pkg/observability -v -run TestTracingPropagation
go test ./entities -v -run 'TestHTTPClientTracingIntegration|TestHTTPClientDistributedTracing'
```

## Examples

- [`examples/tracing-example/`](../examples/tracing-example/) - Client-side tracing workflow.
- [`examples/tracing-server-example/`](../examples/tracing-server-example/) - Server-side trace extraction and middleware-style propagation.

## Best practices

- Pass `context.Context` from your incoming request through every SDK call.
- Use meaningful span names such as `create_organization_workflow`.
- Record errors on spans before returning.
- Add business identifiers as span attributes when they are safe to expose.
- Keep baggage limited to small correlation values.
- Use low sampling rates in production unless debugging a focused incident.
