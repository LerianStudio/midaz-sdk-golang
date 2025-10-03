# OpenTelemetry Tracing Propagation in Midaz SDK

This document describes how OpenTelemetry tracing is automatically propagated through Midaz APIs and provides examples of how to use this functionality.

## Overview

The Midaz Go SDK now includes comprehensive OpenTelemetry tracing support with automatic trace propagation across service boundaries. This enables distributed tracing throughout your application stack.

## Features

- **Automatic Trace Propagation**: HTTP clients automatically inject trace context into outgoing requests
- **Server-side Context Extraction**: Utilities to extract trace context from incoming requests  
- **Baggage Support**: Cross-service correlation data propagation
- **Comprehensive Testing**: Full test coverage for tracing scenarios
- **Performance Optimized**: Minimal overhead when tracing is disabled

## Quick Start

### 1. Enable Tracing in Midaz Client

```go
import (
    "context"
    "net/http"
    "time"
    
    "github.com/LerianStudio/midaz-sdk-golang/v2/client"
    "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
)

// Create observability provider with tracing enabled
provider, err := observability.New(context.Background(),
    observability.WithServiceName("my-service"),
    observability.WithServiceVersion("1.0.0"),
    observability.WithEnvironment("production"),
    observability.WithComponentEnabled(true, true, true), // tracing, metrics, logging
    observability.WithCollectorEndpoint("http://localhost:4317"), // Optional OTEL collector
    observability.WithTraceSampleRate(0.1), // Sample 10% of traces
)
if err != nil {
    log.Fatal(err)
}
defer provider.Shutdown(context.Background())

// Create Midaz client with observability
midazClient, err := client.New(
    client.WithBaseURL("https://api.midaz.com"),
    client.WithAuth("Bearer your-api-token"),
    client.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
    client.WithObservabilityProvider(provider), // Enables automatic tracing
)
if err != nil {
    log.Fatal(err)
}
```

### 2. Using Tracing in Your Application

```go
// Start a trace for your business operation
tracer := provider.Tracer()
ctx, span := tracer.Start(context.Background(), "create_organization_workflow")
defer span.End()

// Add custom attributes
span.SetAttributes(
    attribute.String("workflow.type", "organization_creation"),
    attribute.String("business.unit", "onboarding"),
)

// API calls will automatically include trace context
organization, err := midazClient.Organizations().Create(ctx, orgInput)
if err != nil {
    span.SetStatus(codes.Error, err.Error())
    span.RecordError(err)
    return err
}

span.SetAttributes(
    attribute.String("organization.id", organization.ID),
)
span.SetStatus(codes.Ok, "Organization created successfully")
```

### 3. Server-side Trace Context Extraction

```go
// Extract trace context from incoming HTTP requests
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract trace context from headers
    headers := make(map[string]string)
    for name, values := range r.Header {
        if len(values) > 0 {
            headers[name] = values[0]
        }
    }
    ctx := observability.ExtractContext(r.Context(), headers)
    
    // Start child span
    tracer := provider.Tracer()
    ctx, span := tracer.Start(ctx, "handle_request")
    defer span.End()
    
    // Your business logic here - trace context is now available
    result, err := processRequest(ctx)
    if err != nil {
        span.RecordError(err)
        return
    }
    
    // Return response with trace ID for correlation
    w.Header().Set("X-Trace-ID", observability.TraceID(ctx))
    json.NewEncoder(w).Encode(result)
}
```

## Advanced Features

### Baggage for Cross-service Correlation

```go
// Add correlation data that persists across service boundaries
ctx, err := observability.WithBaggageItem(ctx, "user-id", "user-123")
ctx, err = observability.WithBaggageItem(ctx, "request-id", "req-456")

// Later, extract baggage in another service
userID := observability.GetBaggageItem(ctx, "user-id")
requestID := observability.GetBaggageItem(ctx, "request-id")
```

### Custom HTTP Middleware

```go
// For server applications
func tracingMiddleware(provider observability.Provider) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract trace context
            headers := make(map[string]string)
            for name, values := range r.Header {
                if len(values) > 0 {
                    headers[name] = values[0]
                }
            }
            ctx := observability.ExtractContext(r.Context(), headers)
            
            // Start span
            tracer := provider.Tracer()
            ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
            defer span.End()
            
            // Add request attributes
            span.SetAttributes(
                attribute.String("http.method", r.Method),
                attribute.String("http.url", r.URL.String()),
            )
            
            // Process request
            r = r.WithContext(ctx)
            next.ServeHTTP(w, r)
        })
    }
}
```

### Trace Information Extraction

```go
// Get trace and span IDs for logging correlation
traceID := observability.TraceID(ctx)
spanID := observability.SpanID(ctx)

log.Printf("Processing request [trace_id=%s] [span_id=%s]", traceID, spanID)
```

## Configuration Options

### Environment Variables

- `MIDAZ_OTEL_ENDPOINT`: OpenTelemetry collector endpoint
- `MIDAZ_LOG_LEVEL`: Logging level (debug, info, warn, error)

### Programmatic Configuration

```go
provider, err := observability.New(context.Background(),
    // Service identification
    observability.WithServiceName("my-service"),
    observability.WithServiceVersion("1.0.0"),
    observability.WithEnvironment("production"),
    
    // Component enablement
    observability.WithComponentEnabled(true, true, true), // tracing, metrics, logging
    
    // Sampling configuration
    observability.WithTraceSampleRate(0.1), // 0.0 to 1.0
    observability.WithFullTracingSampling(), // For development (1.0)
    observability.WithHighTracingSampling(), // For development (0.5)
    
    // Propagation configuration
    observability.WithPropagationHeaders("traceparent", "tracestate", "baggage"),
    observability.WithPropagators(propagation.TraceContext{}, propagation.Baggage{}),
    
    // Export configuration  
    observability.WithCollectorEndpoint("http://localhost:4317"),
    
    // Convenience presets
    observability.WithDevelopmentDefaults(), // High sampling, debug logging
    observability.WithProductionDefaults(),  // Low sampling, info logging
)
```

## Testing

The SDK includes comprehensive tests for tracing propagation:

```bash
# Run tracing propagation tests
go test ./pkg/observability -v -run TestTracingPropagation

# Run HTTP client tracing integration tests  
go test ./entities -v -run TestHTTPClientTracingIntegration

# Run distributed tracing tests
go test ./entities -v -run TestHTTPClientDistributedTracing
```

## Examples

See the following example applications:

- [`examples/tracing-example/`](../examples/tracing-example/) - Client-side tracing with complex workflows
- [`examples/tracing-server-example/`](../examples/tracing-server-example/) - Server-side tracing and middleware

## Performance Considerations

- **Sampling**: Use appropriate sampling rates for production (typically 0.01-0.1)
- **Overhead**: When tracing is disabled, overhead is minimal (noop operations)
- **Memory**: Traces are batched and exported asynchronously
- **Network**: Consider collector placement to minimize latency

## Troubleshooting

### Common Issues

1. **Missing trace headers**: Ensure the observability provider is properly configured with the HTTP client
2. **Trace context not propagating**: Verify that the context is being passed through your application layers
3. **High resource usage**: Reduce sampling rate or disable tracing in high-throughput scenarios

### Debug Mode

Enable debug logging to see tracing operations:

```go
provider, err := observability.New(context.Background(),
    observability.WithLogLevel(observability.DebugLevel),
    // ... other options
)
```

### Verification

Check that traces are being generated correctly:

```go
// Check if tracing is enabled
if provider.IsEnabled() {
    log.Println("Tracing is enabled")
}

// Verify trace context exists
if traceID := observability.TraceID(ctx); traceID != "" {
    log.Printf("Current trace ID: %s", traceID)
}
```

## Integration with Popular Tools

### Jaeger

```bash
# Run Jaeger locally
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14250:14250 \
  jaegertracing/all-in-one:latest

# Configure SDK
observability.WithCollectorEndpoint("http://localhost:14250")
```

### Zipkin

```bash
# Run Zipkin locally  
docker run -d -p 9411:9411 openzipkin/zipkin

# Use OTLP HTTP endpoint
observability.WithCollectorEndpoint("http://localhost:9411/api/v2/spans")
```

### OTEL Collector

```yaml
# otel-collector.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  jaeger:
    endpoint: jaeger:14250
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger]
```

## Best Practices

1. **Use meaningful operation names**: `"create_organization"` vs `"POST /api/organizations"`
2. **Add relevant attributes**: Include business context like user IDs, operation types
3. **Handle errors properly**: Always record errors and set appropriate span status
4. **Use baggage sparingly**: Only include essential correlation data
5. **Monitor sampling rates**: Adjust based on traffic volume and storage costs
6. **Test trace propagation**: Verify traces flow correctly through your system