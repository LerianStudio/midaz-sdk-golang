# Tracing implementation note

This note records the tracing propagation implementation in `midaz-sdk-golang`. It is a changelog-style implementation note, not the main tracing guide.

For usage guidance, keep `docs/tracing.md` as the user-facing guide.

## Status

Tracing propagation is implemented for outbound Entity API HTTP requests.

The SDK:

- Stores the configured observability provider on `entities.HTTPClient`.
- Starts SDK HTTP spans when the tracing component is enabled.
- Injects trace context into outbound request paths.
- Uses standard W3C Trace Context and Baggage propagation.
- Keeps existing SDK calls backward compatible when observability is disabled.

## Implementation summary

### `entities.HTTPClient`

`HTTPClient` stores the observability provider in the `observability` field.

`NewHTTPClient` accepts the provider and stores it on the client:

```go
func NewHTTPClient(client *http.Client, authToken string, provider observability.Provider) *HTTPClient
```

The client uses the provider for:

- Span creation through `setupObservabilityContext`.
- Trace context injection before request execution.
- Metrics collection when configured.

### Request paths with propagation

Trace context injection happens in these request paths:

- `doRequest`
- `doRawRequest`
- `doCountRequest`

`sendRequest` reuses `doRawRequest`, so requests created by service methods also receive trace context injection.

Each path uses:

```go
propagator := propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},
)
propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
```

This injects W3C-compatible propagation headers such as:

- `traceparent`
- `tracestate`
- `baggage`

## Propagation scope

The SDK-owned outbound request paths support standard W3C Trace Context and Baggage propagation only.

The observability config still stores propagation-related configuration, including `PropagationHeaders`, for compatibility with existing configuration shape. The current HTTP request injection path does not use that list to emit custom propagation headers.

The SDK does not currently provide custom trace propagation header support beyond stored configuration fields.

## Client example

Client examples should import the root module as `client` and enable Entity API access with `client.UseEntityAPI()` or `client.UseAllAPIs()`.

```go
package main

import (
    "context"
    "log"

    client "github.com/LerianStudio/midaz-sdk-golang/v2"
    "github.com/LerianStudio/midaz-sdk-golang/v2/models"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
)

func main() {
    ctx := context.Background()

    provider, err := observability.New(ctx,
        observability.WithServiceName("my-service"),
        observability.WithServiceVersion("1.0.0"),
        observability.WithEnvironment("development"),
        observability.WithComponentEnabled(true, true, true),
        observability.WithCollectorEndpoint("localhost:4317"),
        observability.WithTraceSampleRate(0.1),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer provider.Shutdown(ctx)

    c, err := client.New(
        client.WithBaseURL("https://api.midaz.io"),
        client.WithObservabilityProvider(provider),
        client.UseEntityAPI(),
    )
    if err != nil {
        log.Fatal(err)
    }

    tracer := provider.Tracer()
    ctx, span := tracer.Start(ctx, "create_organization_workflow")
    defer span.End()

    input := models.NewCreateOrganizationInput(
        "Example Corporation",
        "123456789",
    ).WithDoingBusinessAs("Example Inc.")

    organization, err := c.Entity.Organizations.CreateOrganization(ctx, input)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        log.Fatal(err)
    }

    span.SetAttributes(attribute.String("organization.id", organization.ID))
    span.SetStatus(codes.Ok, "organization created")
}
```

Collector endpoints must use `host:port` format, for example `localhost:4317`. Do not include `http://` or `https://`.

## Server example blocks

The following server snippets are fragments. They depend on imports for `context`, `encoding/json`, `net/http`, `client`, `models`, `observability`, and `go.opentelemetry.io/otel/trace`.

### Extract incoming trace context

```go
func extractTraceContext(r *http.Request) context.Context {
    headers := make(map[string]string)

    for name, values := range r.Header {
        if len(values) > 0 {
            headers[name] = values[0]
        }
    }

    return observability.ExtractContext(r.Context(), headers)
}
```

### Create a server span

```go
func tracingMiddleware(provider observability.Provider, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := extractTraceContext(r)

        tracer := provider.Tracer()
        ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()

        r = r.WithContext(ctx)
        next.ServeHTTP(w, r)
    })
}
```

### Propagate context into downstream SDK calls

```go
func createOrganizationHandler(c *client.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        input := models.NewCreateOrganizationInput(
            "Example Corporation",
            "123456789",
        )

        organization, err := c.Entity.Organizations.CreateOrganization(r.Context(), input)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        _ = json.NewEncoder(w).Encode(organization)
    }
}
```

When the request context contains a valid span, the SDK injects W3C trace context and baggage into the outbound Midaz request.

## Tests

Tracing coverage is verified by unit and integration-style tests in:

- `pkg/observability/tracing_test.go`
- `pkg/observability/middleware_test.go`
- `entities/http_tracing_test.go`

Run tracing-specific tests with:

```bash
go test ./pkg/observability -v -run TestTracingPropagation
go test ./pkg/observability -v -run TestHTTPMiddlewareDirectly
go test ./entities -v -run 'TestHTTPClientTracingIntegration|TestHTTPClientDistributedTracing'
```

Run the current repository test targets with:

```bash
make test-fast
make test
```

Use the full local pipeline before release work:

```bash
make ci
```

## Compatibility notes

- Existing clients continue to work without tracing changes.
- If observability is disabled, the SDK uses no-op tracing behavior.
- Entity API access still requires `client.UseEntityAPI()` or `client.UseAllAPIs()`.
- Authentication remains configured through SDK configuration and Access Manager paths, not through a tracing-specific client option.
- The SDK exports OTLP through the OpenTelemetry provider configuration. Route Jaeger, Zipkin, or vendor backends through an OpenTelemetry Collector.
