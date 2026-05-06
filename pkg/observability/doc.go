// Package observability wires the Midaz Go SDK into OpenTelemetry —
// distributed tracing, metrics, and logs with a single [Provider].
//
// The provider is the unit of lifecycle: build one at process boot,
// hand it to [github.com/LerianStudio/midaz-sdk-golang/v3.WithObservabilityProvider],
// shut it down on exit. Every SDK call emits spans + metrics that nest
// correctly under whatever parent span the caller has open.
//
// # Quickstart
//
//	provider, err := observability.New(ctx,
//	    observability.WithServiceName("payments-api"),
//	    observability.WithEnvironment("production"),
//	    observability.WithComponentEnabled(true, true, true), // tracing, metrics, logs
//	)
//	if err != nil { return err }
//	defer provider.Shutdown(ctx)
//
//	c, err := midaz.New(
//	    midaz.WithEnvironment(midaz.EnvironmentProduction),
//	    midaz.WithAccessManager(am),
//	    midaz.WithObservabilityProvider(provider),
//	)
//
// # Public surface
//
//   - [Provider] — the lifecycle handle. Hands out [Provider.Tracer],
//     [Provider.Meter], [Provider.Logger].
//   - [New] — constructor. Functional options select OTLP exporter,
//     resource attributes, sampling, and component toggles.
//   - [WithSpan] — convenience wrapper that opens a span around a
//     callback, records errors as span attributes on failure, and
//     ends the span on return.
//   - [NewHTTPMiddleware] — server-side middleware that extracts W3C
//     trace context from inbound requests and binds it to the handler
//     context.
//
// # Two-layer surface
//
// observability options have a parallel surface on the root midaz
// package: [github.com/LerianStudio/midaz-sdk-golang/v3.WithObservabilityOptions]
// (build a provider inline) and
// [github.com/LerianStudio/midaz-sdk-golang/v3.WithObservabilityProvider]
// (use a pre-built provider). Most consumers want the latter.
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v3.WithObservabilityProvider]
//   - [github.com/LerianStudio/midaz-sdk-golang/v3.WithObservabilityOptions]
//   - docs/tracing.md — full tracing contract
//   - examples/10-observability-otel — runnable end-to-end demo
//   - examples/tracing — client-side tracing reference
//   - examples/tracing-server — server-side trace context extraction
package observability
