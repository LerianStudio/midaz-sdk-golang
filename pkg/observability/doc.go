// Package observability wires the Midaz Go SDK into OpenTelemetry —
// distributed tracing, metrics, and logs with a single [Provider].
//
// The provider is the unit of lifecycle: build one at process boot,
// hand it to [github.com/LerianStudio/midaz-sdk-golang/v4.WithObservabilityProvider],
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
//   - [NewHTTPMiddleware] — client-side transport middleware that wraps
//     a [net/http.RoundTripper] to propagate W3C trace context onto
//     outbound requests and record per-call spans/metrics.
//
// # Two-layer surface
//
// observability options have a parallel surface on the root midaz
// package: [github.com/LerianStudio/midaz-sdk-golang/v4.WithObservabilityOptions]
// (build a provider inline) and
// [github.com/LerianStudio/midaz-sdk-golang/v4.WithObservabilityProvider]
// (use a pre-built provider). Most consumers want the latter.
//
// # Logging surfaces
//
// The SDK exposes two distinct logger surfaces, configured independently and
// backed by different handler chains:
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v4.Client.Logger] returns a
//     *slog.Logger — the Go-stdlib idiom. Used by the SDK for retry traces
//     and other internal lines that are not span-correlated. Configured via
//     midaz.WithLogger / Config.Debug.
//   - [Provider.Logger] returns the bespoke [Logger] — OTel-correlated.
//     Call [Logger.WithSpan] to attach trace_id and span_id to every log
//     line. Used inside SDK call paths that already hold a live span.
//
// They are not the same handler. The slog surface predates this package and
// covers application-side use cases; this package's Logger predates slog.go
// and covers OTel correlation. Use slog for application code, Provider.Logger
// when you need span-aware logging from inside an SDK callback.
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v4.WithObservabilityProvider]
//   - [github.com/LerianStudio/midaz-sdk-golang/v4.WithObservabilityOptions]
//   - docs/logging.md — logging and trace/span ID guidance
//   - examples/10-observability-otel — runnable end-to-end demo
//   - examples/tracing — client-side tracing reference
//   - examples/tracing-server — server-side trace context extraction
package observability
