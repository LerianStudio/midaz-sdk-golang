// Package observability provides utilities for adding observability capabilities
// to the Midaz SDK, including metrics, logging, and distributed tracing.
package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Constants for attribute keys used in spans and metrics
const (
	// General attributes
	KeySDKVersion     = "midaz.sdk.version"
	KeySDKLanguage    = "midaz.sdk.language"
	KeyOperationName  = "midaz.operation.name"
	KeyOperationType  = "midaz.operation.type"
	KeyResourceType   = "midaz.resource.type"
	KeyResourceID     = "midaz.resource.id"
	KeyOrganizationID = "midaz.organization_id"
	KeyLedgerID       = "midaz.ledger_id"
	KeyAccountID      = "midaz.account_id"

	// HTTP request attributes
	KeyHTTPMethod   = "http.method"
	KeyHTTPPath     = "http.path"
	KeyHTTPStatus   = "http.status_code"
	KeyHTTPHost     = "http.host"
	KeyErrorCode    = "error.code"
	KeyErrorMessage = "error.message"

	// Metric names
	MetricRequestTotal        = "midaz.sdk.request.total"
	MetricRequestDuration     = "midaz.sdk.request.duration"
	MetricRequestErrorTotal   = "midaz.sdk.request.error.total"
	MetricRequestSuccess      = "midaz.sdk.request.success"
	MetricRequestRetryTotal   = "midaz.sdk.request.retry.total"
	MetricRequestBatchSize    = "midaz.sdk.request.batch.size"
	MetricRequestBatchLatency = "midaz.sdk.request.batch.latency"
)

// Provider is the interface for observability providers.
// It allows for consistent access to tracing, metrics, and logging capabilities.
type Provider interface {
	// Tracer returns a tracer for creating spans
	Tracer() trace.Tracer

	// Meter returns a meter for creating metrics
	Meter() metric.Meter

	// Logger returns a logger
	Logger() Logger

	// Shutdown gracefully shuts down the provider
	Shutdown(ctx context.Context) error

	// IsEnabled returns true if observability is enabled
	IsEnabled() bool
}

// Config holds the configuration for the observability provider
type Config struct {
	// ServiceName is the name of the service using the SDK
	ServiceName string

	// ServiceVersion is the version of the service using the SDK
	ServiceVersion string

	// SDKVersion is the version of the Midaz Go SDK
	SDKVersion string

	// Environment is the environment where the service is running
	Environment string

	// CollectorEndpoint is the endpoint for the OpenTelemetry collector
	CollectorEndpoint string

	// LogLevel is the minimum log level to record
	LogLevel LogLevel

	// LogOutput is where to write logs (defaults to os.Stderr)
	LogOutput io.Writer

	// TraceSampleRate is the sampling rate for traces (0.0 to 1.0)
	TraceSampleRate float64

	// EnabledComponents controls which observability components are enabled
	EnabledComponents EnabledComponents

	// Attributes are additional attributes to add to all telemetry
	Attributes []attribute.KeyValue

	// Propagators for context propagation
	Propagators []propagation.TextMapPropagator

	// Headers to extract for trace context propagation
	PropagationHeaders []string

	// RegisterGlobally controls whether to register providers as global OpenTelemetry providers.
	// When true (default), providers are registered globally via otel.Set*Provider calls.
	// When false, providers are only available via this MidazProvider instance, avoiding
	// conflicts when multiple SDK instances are used in the same process.
	RegisterGlobally bool
}

// EnabledComponents controls which observability components are enabled
type EnabledComponents struct {
	Tracing bool
	Metrics bool
	Logging bool
}

// Option defines a function that configures the observability Config
type Option func(*Config) error

// WithServiceName sets the service name for observability
func WithServiceName(name string) Option {
	return func(c *Config) error {
		if name == "" {
			return errors.New("service name cannot be empty")
		}

		c.ServiceName = name

		return nil
	}
}

// WithServiceVersion sets the service version for observability
func WithServiceVersion(ver string) Option {
	return func(c *Config) error {
		if ver == "" {
			return errors.New("service version cannot be empty")
		}

		c.ServiceVersion = ver

		return nil
	}
}

// WithSDKVersion sets the SDK version for observability
func WithSDKVersion(ver string) Option {
	return func(c *Config) error {
		if ver == "" {
			return errors.New("SDK version cannot be empty")
		}

		c.SDKVersion = ver

		return nil
	}
}

// WithEnvironment sets the environment for observability (e.g., "production", "staging", "development")
func WithEnvironment(env string) Option {
	return func(c *Config) error {
		if env == "" {
			return errors.New("environment cannot be empty")
		}

		c.Environment = env

		return nil
	}
}

// WithCollectorEndpoint sets the endpoint for the OpenTelemetry collector
func WithCollectorEndpoint(endpoint string) Option {
	return func(c *Config) error {
		if endpoint == "" {
			return errors.New("collector endpoint cannot be empty")
		}

		c.CollectorEndpoint = endpoint

		return nil
	}
}

// WithLogLevel sets the minimum log level to record
func WithLogLevel(level LogLevel) Option {
	return func(c *Config) error {
		if level < DebugLevel || level > FatalLevel {
			return fmt.Errorf("invalid log level: %d", level)
		}

		c.LogLevel = level

		return nil
	}
}

// WithLogOutput sets the writer for logs
func WithLogOutput(output io.Writer) Option {
	return func(c *Config) error {
		if output == nil {
			return errors.New("log output cannot be nil")
		}

		c.LogOutput = output

		return nil
	}
}

// WithTraceSampleRate sets the sampling rate for traces (0.0 to 1.0)
func WithTraceSampleRate(rate float64) Option {
	return func(c *Config) error {
		if rate < 0.0 || rate > 1.0 {
			return fmt.Errorf("trace sample rate must be between 0.0 and 1.0, got %f", rate)
		}

		c.TraceSampleRate = rate

		return nil
	}
}

// WithComponentEnabled enables or disables specific observability components
func WithComponentEnabled(tracing, metrics, logging bool) Option {
	return func(c *Config) error {
		c.EnabledComponents.Tracing = tracing
		c.EnabledComponents.Metrics = metrics
		c.EnabledComponents.Logging = logging

		return nil
	}
}

// WithAttributes adds additional attributes to all telemetry
func WithAttributes(attrs ...attribute.KeyValue) Option {
	return func(c *Config) error {
		c.Attributes = append(c.Attributes, attrs...)

		return nil
	}
}

// WithPropagators sets the propagators for context propagation
func WithPropagators(propagators ...propagation.TextMapPropagator) Option {
	return func(c *Config) error {
		if len(propagators) == 0 {
			return errors.New("at least one propagator must be provided")
		}

		c.Propagators = propagators

		return nil
	}
}

// WithPropagationHeaders sets the headers to extract for trace context propagation
func WithPropagationHeaders(headers ...string) Option {
	return func(c *Config) error {
		if len(headers) == 0 {
			return errors.New("at least one propagation header must be provided")
		}

		c.PropagationHeaders = headers

		return nil
	}
}

// WithRegisterGlobally controls whether to register providers as global OpenTelemetry providers.
// When true (default), providers are registered globally via otel.Set*Provider calls.
// When false, providers are only available via this MidazProvider instance, avoiding
// conflicts when multiple SDK instances are used in the same process.
func WithRegisterGlobally(register bool) Option {
	return func(c *Config) error {
		c.RegisterGlobally = register

		return nil
	}
}

// WithHighTracingSampling sets a high trace sampling rate (0.5) for development environments
func WithHighTracingSampling() Option {
	return WithTraceSampleRate(0.5)
}

// WithFullTracingSampling sets a full trace sampling rate (1.0) for testing environments
func WithFullTracingSampling() Option {
	return WithTraceSampleRate(1.0)
}

// WithDevelopmentDefaults sets reasonable defaults for development environments
// - High trace sampling rate (0.5)
// - Debug log level
// - Development environment
func WithDevelopmentDefaults() Option {
	return func(c *Config) error {
		if err := WithEnvironment("development")(c); err != nil {
			return err
		}

		if err := WithLogLevel(DebugLevel)(c); err != nil {
			return err
		}

		return WithTraceSampleRate(0.5)(c)
	}
}

// WithProductionDefaults sets reasonable defaults for production environments
// - Low trace sampling rate (0.1)
// - Info log level
// - Production environment
func WithProductionDefaults() Option {
	return func(c *Config) error {
		if err := WithEnvironment("production")(c); err != nil {
			return err
		}

		if err := WithLogLevel(InfoLevel)(c); err != nil {
			return err
		}

		return WithTraceSampleRate(0.1)(c)
	}
}

// DefaultConfig returns a default configuration for the observability provider
func DefaultConfig() *Config {
	return &Config{
		ServiceName:     version.SDKName,
		ServiceVersion:  version.Version,
		SDKVersion:      version.Version,
		Environment:     "production",
		LogLevel:        InfoLevel,
		TraceSampleRate: 0.1,
		EnabledComponents: EnabledComponents{
			Tracing: true,
			Metrics: true,
			Logging: true,
		},
		PropagationHeaders: []string{
			"traceparent",
			"tracestate",
			"baggage",
			"x-request-id",
			"x-correlation-id",
		},
		RegisterGlobally: true,
	}
}

// MidazProvider is the main implementation of the Provider interface
// It provides access to OpenTelemetry tracing, metrics, and logging
type MidazProvider struct {
	config            *Config
	tracerProvider    *sdktrace.TracerProvider
	meterProvider     *sdkmetric.MeterProvider
	logger            Logger
	tracer            trace.Tracer
	meter             metric.Meter
	enabled           bool
	shutdownFunctions []func(context.Context) error
}

// New creates a new observability provider with the given options
func New(ctx context.Context, opts ...Option) (Provider, error) {
	// Start with default configuration
	config := DefaultConfig()

	// Apply all options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	provider := &MidazProvider{
		config:            config,
		shutdownFunctions: []func(context.Context) error{},
		enabled:           true,
	}

	// Create a resource with service information
	res := provider.createResource()

	// Initialize tracing if enabled
	if config.EnabledComponents.Tracing {
		if err := provider.initTracing(ctx, res); err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
	}

	// Initialize metrics if enabled
	if config.EnabledComponents.Metrics {
		if err := provider.initMetrics(ctx, res); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
	}

	// Initialize logging if enabled
	if config.EnabledComponents.Logging {
		if err := provider.initLogging(res); err != nil {
			return nil, fmt.Errorf("failed to initialize logging: %w", err)
		}
	}

	// Set up context propagation
	provider.setupPropagation()

	return provider, nil
}

// NewWithConfig creates a new observability provider with the given configuration
// This is provided for backward compatibility with existing code
func NewWithConfig(ctx context.Context, config *Config) (Provider, error) {
	if config == nil {
		return New(ctx)
	}

	// Convert the config to options
	var opts []Option

	if config.ServiceName != "" {
		opts = append(opts, WithServiceName(config.ServiceName))
	}

	if config.ServiceVersion != "" {
		opts = append(opts, WithServiceVersion(config.ServiceVersion))
	}

	if config.SDKVersion != "" {
		opts = append(opts, WithSDKVersion(config.SDKVersion))
	}

	if config.Environment != "" {
		opts = append(opts, WithEnvironment(config.Environment))
	}

	if config.CollectorEndpoint != "" {
		opts = append(opts, WithCollectorEndpoint(config.CollectorEndpoint))
	}

	if config.LogOutput != nil {
		opts = append(opts, WithLogOutput(config.LogOutput))
	}

	// Always set log level, as it has a valid zero value
	opts = append(opts, WithLogLevel(config.LogLevel))

	// Always set trace sample rate
	opts = append(opts, WithTraceSampleRate(config.TraceSampleRate))

	// Always set components
	opts = append(opts, WithComponentEnabled(
		config.EnabledComponents.Tracing,
		config.EnabledComponents.Metrics,
		config.EnabledComponents.Logging,
	))

	if len(config.Attributes) > 0 {
		opts = append(opts, WithAttributes(config.Attributes...))
	}

	if len(config.Propagators) > 0 {
		opts = append(opts, WithPropagators(config.Propagators...))
	}

	if len(config.PropagationHeaders) > 0 {
		opts = append(opts, WithPropagationHeaders(config.PropagationHeaders...))
	}

	// Always set RegisterGlobally
	opts = append(opts, WithRegisterGlobally(config.RegisterGlobally))

	return New(ctx, opts...)
}

// createResource creates an OpenTelemetry resource with service information
func (p *MidazProvider) createResource() *sdkresource.Resource {
	attributes := []attribute.KeyValue{
		semconv.ServiceNameKey.String(p.config.ServiceName),
		semconv.ServiceVersionKey.String(p.config.ServiceVersion),
		attribute.String(KeySDKVersion, p.config.SDKVersion),
		attribute.String(KeySDKLanguage, "go"),
		semconv.DeploymentEnvironmentNameKey.String(p.config.Environment),
	}

	// Add custom attributes
	attributes = append(attributes, p.config.Attributes...)

	// Create and return the resource without merging defaults to avoid schema URL conflicts
	// between different OpenTelemetry versions pulled by transitive deps during tests.
	// If needed, default attributes can be reintroduced by constructing a resource with
	// a consistent schema across both sources.
	return sdkresource.NewWithAttributes(
		semconv.SchemaURL,
		attributes...,
	)
}

// initTracing initializes OpenTelemetry tracing
func (p *MidazProvider) initTracing(ctx context.Context, res *sdkresource.Resource) error {
	var exporter *otlptrace.Exporter

	var err error

	// Set up exporter
	if p.config.CollectorEndpoint != "" {
		// Use OTLP exporter with gRPC if collector endpoint is provided
		exporter, err = otlptracegrpc.New(
			ctx,
			otlptracegrpc.WithEndpoint(p.config.CollectorEndpoint),
			otlptracegrpc.WithInsecure(),
		)
	} else {
		// Use stdout exporter (for development) if no collector endpoint is specified
		exporter = otlptracegrpc.NewUnstarted()
	}

	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Configure and create the trace provider
	p.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(p.config.TraceSampleRate)),
	)

	// Set the global trace provider only if RegisterGlobally is true
	if p.config.RegisterGlobally {
		otel.SetTracerProvider(p.tracerProvider)
	}

	// Create a tracer for this library
	p.tracer = p.tracerProvider.Tracer("github.com/LerianStudio/midaz-sdk-golang/v2")

	// Add shutdown function
	p.shutdownFunctions = append(p.shutdownFunctions, func(ctx context.Context) error {
		return p.tracerProvider.Shutdown(ctx)
	})

	return nil
}

// initMetrics initializes OpenTelemetry metrics
func (p *MidazProvider) initMetrics(ctx context.Context, res *sdkresource.Resource) error {
	// No default metrics exporter; skip metrics if no endpoint is provided
	if p.config.CollectorEndpoint == "" {
		return nil
	}

	// Use OTLP exporter with gRPC if collector endpoint is provided
	exporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(p.config.CollectorEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Configure and create the meter provider
	p.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)

	// Set the global meter provider only if RegisterGlobally is true
	if p.config.RegisterGlobally {
		otel.SetMeterProvider(p.meterProvider)
	}

	// Create a meter for this library
	p.meter = p.meterProvider.Meter("github.com/LerianStudio/midaz-sdk-golang/v2")

	// Add shutdown function
	p.shutdownFunctions = append(p.shutdownFunctions, func(ctx context.Context) error {
		return p.meterProvider.Shutdown(ctx)
	})

	return nil
}

// initLogging initializes structured logging
//
//nolint:unparam // Error return kept for future error handling
func (p *MidazProvider) initLogging(res *sdkresource.Resource) error {
	// Create logger
	p.logger = NewLogger(p.config.LogLevel, p.config.LogOutput, res)
	return nil
}

// setupPropagation configures context propagation for distributed tracing
func (p *MidazProvider) setupPropagation() {
	// Only set global propagator if RegisterGlobally is true
	if !p.config.RegisterGlobally {
		return
	}

	// Set up propagators if provided, otherwise use defaults
	if len(p.config.Propagators) > 0 {
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			p.config.Propagators...,
		))
	} else {
		// Use default propagators
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
	}
}

// Tracer returns a tracer for creating spans
func (p *MidazProvider) Tracer() trace.Tracer {
	if !p.enabled || !p.config.EnabledComponents.Tracing {
		// Return a no-op tracer if tracing is disabled
		return noop.NewTracerProvider().Tracer("")
	}

	return p.tracer
}

// Meter returns a meter for creating metrics
func (p *MidazProvider) Meter() metric.Meter {
	if !p.enabled || !p.config.EnabledComponents.Metrics || p.meter == nil {
		// Return the default global meter if metrics are disabled
		return otel.GetMeterProvider().Meter("")
	}

	return p.meter
}

// Logger returns a logger
func (p *MidazProvider) Logger() Logger {
	if !p.enabled || !p.config.EnabledComponents.Logging {
		// Return a no-op logger if logging is disabled
		return NewNoopLogger()
	}

	return p.logger
}

// Shutdown gracefully shuts down the provider and all its components
func (p *MidazProvider) Shutdown(ctx context.Context) error {
	if !p.enabled {
		return nil
	}

	p.enabled = false

	// Call all shutdown functions
	var shutdownErrs []error

	for _, shutdownFn := range p.shutdownFunctions {
		if err := shutdownFn(ctx); err != nil {
			shutdownErrs = append(shutdownErrs, err)
		}
	}

	if len(shutdownErrs) > 0 {
		return fmt.Errorf("errors during shutdown: %v", shutdownErrs)
	}

	return nil
}

// IsEnabled returns true if observability is enabled
func (p *MidazProvider) IsEnabled() bool {
	return p.enabled
}

// WithSpan creates a new span and executes the function within the context of that span.
// It automatically ends the span when the function returns.
func WithSpan(ctx context.Context, provider Provider, name string, fn func(context.Context) error, opts ...trace.SpanStartOption) error {
	// If provider is nil or observability is disabled, just run the function
	if provider == nil || !provider.IsEnabled() {
		return fn(ctx)
	}

	// Start a new span
	ctx, span := provider.Tracer().Start(ctx, name, opts...)
	defer span.End()

	// Run the function and handle errors
	err := fn(ctx)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "Success")
	}

	return err
}

// RecordMetric records a metric using the provided meter
func RecordMetric(ctx context.Context, provider Provider, name string, value float64, attrs ...attribute.KeyValue) {
	// If provider is nil or observability is disabled, just return
	if provider == nil || !provider.IsEnabled() {
		return
	}

	counter, err := provider.Meter().Float64Counter(name)
	if err != nil {
		provider.Logger().Errorf("Failed to create counter for metric %s: %v", name, err)
		return
	}

	counter.Add(ctx, value, metric.WithAttributes(attrs...))
}

// RecordDuration records a duration metric using the provided meter
func RecordDuration(ctx context.Context, provider Provider, name string, start time.Time, attrs ...attribute.KeyValue) {
	// If provider is nil or observability is disabled, just return
	if provider == nil || !provider.IsEnabled() {
		return
	}

	duration := time.Since(start).Milliseconds()

	histogram, err := provider.Meter().Int64Histogram(name)
	if err != nil {
		provider.Logger().Errorf("Failed to create histogram for metric %s: %v", name, err)
		return
	}

	histogram.Record(ctx, duration, metric.WithAttributes(attrs...))
}

// ExtractContext extracts context from HTTP headers for distributed tracing
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(headers))
}

// InjectContext injects context into HTTP headers for distributed tracing
func InjectContext(ctx context.Context, headers map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))
}
