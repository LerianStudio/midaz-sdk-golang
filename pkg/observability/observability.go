package observability

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	obsconstants "github.com/LerianStudio/lib-observability/constants"
	obslog "github.com/LerianStudio/lib-observability/log"
	obsmetrics "github.com/LerianStudio/lib-observability/metrics"
	obstracing "github.com/LerianStudio/lib-observability/tracing"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
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

	// HTTP semantic convention attributes
	KeyHTTPRequestMethod      = "http.request.method"
	KeyHTTPResponseStatusCode = "http.response.status_code"
	KeyURLFull                = "url.full"
	KeyURLPath                = "url.path"
	KeyURLScheme              = "url.scheme"
	KeyServerAddress          = "server.address"
	KeyServerPort             = "server.port"
	KeyNetworkProtocolVersion = "network.protocol.version"
	KeyErrorType              = "error.type"

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
//
// # Non-nil contract
//
// The Tracer, Meter, and Logger accessors MUST return non-nil values for any
// implementation that may be installed via
// [github.com/LerianStudio/midaz-sdk-golang/v5.WithObservabilityProvider].
// Use the OTel no-op providers (e.g. [go.opentelemetry.io/otel/trace/noop])
// or [NewNoopLogger] for disabled-component configurations. The in-tree
// [MidazProvider] follows this contract.
//
// SDK call sites use these accessors directly (e.g. provider.Tracer().Start)
// without nil-checking the returned values — codifying the invariant in the
// interface keeps consumers free of defensive guards.
type Provider interface {
	// Tracer returns a tracer for creating spans. Implementations MUST return
	// a non-nil Tracer; use a no-op tracer (e.g. noop.NewTracerProvider().Tracer(""))
	// when tracing is disabled.
	Tracer() trace.Tracer

	// Meter returns a meter for creating metrics. Implementations MUST return
	// a non-nil Meter; use a no-op meter (e.g. metricnoop.NewMeterProvider().Meter(""))
	// when metrics are disabled.
	Meter() metric.Meter

	// Logger returns a logger. Implementations MUST return a non-nil Logger;
	// use [NewNoopLogger] when logging is disabled.
	Logger() Logger

	// Shutdown gracefully shuts down the provider
	Shutdown(ctx context.Context) error

	// IsEnabled returns true if observability is enabled
	IsEnabled() bool
}

type propagatorProvider interface {
	TextMapPropagator() propagation.TextMapPropagator
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

	// CollectorInsecure opts out of TLS for OTLP gRPC exporters. Default false.
	CollectorInsecure bool

	// LogLevel is the minimum log level to record
	LogLevel LogLevel

	// LogOutput is where to write logs (defaults to os.Stderr)
	LogOutput io.Writer

	// TraceSampleRate is retained for source compatibility. Trace sampling is
	// currently owned by lib-observability's telemetry lifecycle and this value
	// is not applied by the SDK provider.
	TraceSampleRate float64

	// EnabledComponents controls which observability components are enabled
	EnabledComponents EnabledComponents

	// Attributes are additional attributes attached to the SDK logger resource.
	// The lib-observability telemetry lifecycle currently owns exported
	// trace/metric resource attributes.
	Attributes []attribute.KeyValue

	// Propagators for context propagation
	Propagators []propagation.TextMapPropagator

	// Headers to extract for trace context propagation
	PropagationHeaders         []string
	propagationHeadersExplicit bool

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

// WithCollectorInsecure opts OTLP gRPC exporters into plaintext transport.
// The default is false, so collector connections use TLS unless callers
// explicitly opt out for local or trusted in-cluster deployments.
func WithCollectorInsecure(insecure bool) Option {
	return func(c *Config) error {
		c.CollectorInsecure = insecure
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

// WithLogOutput sets the writer for logs.
//
// Primary use: redirecting log output in tests so callers can capture and
// assert log content. Production code typically wires logging through the
// observability provider's resource configuration; this option exists for
// the inversion-of-output scenario.
func WithLogOutput(output io.Writer) Option {
	return func(c *Config) error {
		if output == nil {
			return errors.New("log output cannot be nil")
		}

		c.LogOutput = output

		return nil
	}
}

// WithTraceSampleRate stores the requested trace sampling rate for source compatibility.
//
// Compatibility note: lib-observability v1.0.0 owns the tracer provider
// lifecycle and does not expose sampler configuration through TelemetryConfig.
// The option is validated and retained on Config, but it does not change
// exported sampling.
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

// WithAttributes adds additional attributes to the SDK logger resource.
//
// Compatibility note: exported trace/metric resource attributes are currently
// owned by lib-observability's telemetry lifecycle. These attributes remain
// available to the SDK logger resource for source compatibility.
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

		c.Propagators = append([]propagation.TextMapPropagator(nil), propagators...)

		return nil
	}
}

// WithPropagationHeaders sets the headers to extract for trace context propagation
func WithPropagationHeaders(headers ...string) Option {
	return func(c *Config) error {
		if len(headers) == 0 {
			return errors.New("at least one propagation header must be provided")
		}

		c.PropagationHeaders = append([]string(nil), headers...)
		c.propagationHeadersExplicit = true

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

// WithHighTracingSampling stores the legacy high sampling request.
//
// Compatibility note: sampling is currently owned by lib-observability; see
// [WithTraceSampleRate].
func WithHighTracingSampling() Option {
	return WithTraceSampleRate(0.5)
}

// WithFullTracingSampling stores the legacy full sampling request.
//
// Compatibility note: sampling is currently owned by lib-observability; see
// [WithTraceSampleRate].
func WithFullTracingSampling() Option {
	return WithTraceSampleRate(1.0)
}

// WithDevelopmentDefaults sets reasonable defaults for development environments
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
			obsconstants.HeaderTraceparent,
			obsconstants.MetadataTracestate,
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
	lifecycleMu    sync.RWMutex
	config         *Config
	telemetry      *obstracing.Telemetry
	logger         Logger
	tracer         trace.Tracer
	meter          metric.Meter
	metricsFactory *obsmetrics.MetricsFactory
	enabled        bool

	// propagationHeadersOnce + propagationHeadersAllow cache the lowercased
	// allow-set used by filterPropagationHeaders / filterPropagationMap.
	// Building the set on every header filter call (which is itself on the
	// hot path of every outbound request) walked PropagationHeaders +
	// strings.ToLower for each header on each call. Caching once per
	// provider lifetime is correct because PropagationHeaders is set at
	// construction and the propagator's own Fields() are stable too.
	propagationHeadersOnce  sync.Once
	propagationHeadersAllow map[string]struct{}
}

// New creates a new observability provider with the given options
func New(_ context.Context, opts ...Option) (Provider, error) {
	// Start with default configuration
	config := DefaultConfig()

	// Apply all options
	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("observability option cannot be nil")
		}

		if err := opt(config); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	provider := &MidazProvider{
		config:  config,
		enabled: true,
	}

	// Create a resource with service information
	res, err := provider.createResource()
	if err != nil {
		return nil, err
	}

	// Initialize logging if enabled
	if config.EnabledComponents.Logging {
		provider.initLogging(res)
	}

	// Initialize OTel lifecycle through lib-observability when tracing or
	// metrics are enabled. Logging keeps the SDK's public Logger facade, while
	// lib-observability owns exporter/provider setup and shutdown.
	if config.EnabledComponents.Tracing || config.EnabledComponents.Metrics {
		if err := provider.initTelemetry(); err != nil {
			return nil, fmt.Errorf("failed to initialize telemetry: %w", err)
		}
	}

	// When no collector endpoint is configured, lib-observability is not
	// initialized. Keep the legacy global propagator behavior only for that
	// no-telemetry path; otherwise ApplyGlobals owns global propagation.
	if provider.telemetry == nil {
		provider.setupPropagation()
	}

	return provider, nil
}

// createResource creates an OpenTelemetry resource with service information.
func (p *MidazProvider) createResource() (*sdkresource.Resource, error) {
	attributes := make([]attribute.KeyValue, 0, 5+len(p.config.Attributes))
	attributes = append(attributes,
		semconv.ServiceNameKey.String(p.config.ServiceName),
		semconv.ServiceVersionKey.String(p.config.ServiceVersion),
		attribute.String(KeySDKVersion, p.config.SDKVersion),
		attribute.String(KeySDKLanguage, "go"),
		semconv.DeploymentEnvironmentNameKey.String(p.config.Environment),
	)

	// Add custom attributes
	attributes = append(attributes, p.config.Attributes...)

	defaultResource := sdkresource.Default()
	custom := sdkresource.NewWithAttributes(
		defaultResource.SchemaURL(),
		attributes...,
	)

	res, err := sdkresource.Merge(defaultResource, custom)
	if err != nil {
		return nil, fmt.Errorf("failed to merge OpenTelemetry default resource: %w", err)
	}

	return res, nil
}

// initTelemetry initializes OpenTelemetry through lib-observability so exporter
// security policy, redacting span processors, provider globals, metrics factory,
// and shutdown lifecycle stay centralized in the shared library.
func (p *MidazProvider) initTelemetry() error {
	if p == nil || p.config == nil {
		return nil
	}

	var globals telemetryGlobals
	if !p.config.RegisterGlobally {
		globals = captureTelemetryGlobals()
	}

	telemetry, err := obstracing.NewTelemetry(obstracing.TelemetryConfig{
		LibraryName:               "github.com/LerianStudio/midaz-sdk-golang/v5",
		ServiceName:               p.config.ServiceName,
		ServiceVersion:            p.config.ServiceVersion,
		DeploymentEnv:             p.config.Environment,
		CollectorExporterEndpoint: p.config.CollectorEndpoint,
		EnableTelemetry:           true,
		InsecureExporter:          p.config.CollectorInsecure,
		Logger:                    obslog.NewNop(), //nolint:forbidigo // lib-observability/tracing requires a lib-observability logger.
		Propagator:                p.textMapPropagatorFromConfig(),
		Redactor:                  obstracing.NewDefaultRedactor(),
	})
	if err != nil && (!errors.Is(err, obstracing.ErrEmptyEndpoint) || telemetry == nil) {
		return err
	}
	if !p.config.RegisterGlobally {
		restoreTelemetryGlobals(globals)
	}

	if p.config.RegisterGlobally {
		if err := telemetry.ApplyGlobals(); err != nil {
			return err
		}
	}

	p.telemetry = telemetry
	p.metricsFactory = telemetry.MetricsFactory

	if p.config.EnabledComponents.Tracing {
		tracer, err := telemetry.Tracer("github.com/LerianStudio/midaz-sdk-golang/v5")
		if err != nil {
			return err
		}
		p.tracer = tracer
	}

	if p.config.EnabledComponents.Metrics {
		meter, err := telemetry.Meter("github.com/LerianStudio/midaz-sdk-golang/v5")
		if err != nil {
			return err
		}
		p.meter = meter
	}

	return nil
}

type telemetryGlobals struct {
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	loggerProvider otellog.LoggerProvider //nolint:forbidigo // Required to snapshot/restore the OTel log global provider.
	propagator     propagation.TextMapPropagator
}

func captureTelemetryGlobals() telemetryGlobals {
	return telemetryGlobals{
		tracerProvider: otel.GetTracerProvider(),
		meterProvider:  otel.GetMeterProvider(),
		loggerProvider: otellogglobal.GetLoggerProvider(),
		propagator:     otel.GetTextMapPropagator(),
	}
}

func restoreTelemetryGlobals(globals telemetryGlobals) {
	if globals.tracerProvider != nil {
		otel.SetTracerProvider(globals.tracerProvider)
	}
	if globals.meterProvider != nil {
		otel.SetMeterProvider(globals.meterProvider)
	}
	if globals.loggerProvider != nil {
		otellogglobal.SetLoggerProvider(globals.loggerProvider)
	}
	if globals.propagator != nil {
		otel.SetTextMapPropagator(globals.propagator)
	}
}

// initLogging initializes structured logging.
func (p *MidazProvider) initLogging(res *sdkresource.Resource) {
	// Create logger
	p.logger = NewLogger(p.config.LogLevel, p.config.LogOutput, res)
}

// setupPropagation configures context propagation for distributed tracing
func (p *MidazProvider) setupPropagation() {
	// Only set global propagator if RegisterGlobally is true
	if p == nil || p.config == nil || !p.config.RegisterGlobally || !p.config.EnabledComponents.Tracing {
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
	if p == nil || p.config == nil || !p.isEnabled() || !p.config.EnabledComponents.Tracing || p.tracer == nil {
		// Return a no-op tracer if tracing is disabled
		return noop.NewTracerProvider().Tracer("")
	}

	return p.tracer
}

// Meter returns a meter for creating metrics
func (p *MidazProvider) Meter() metric.Meter {
	if p == nil || p.config == nil || !p.isEnabled() || !p.config.EnabledComponents.Metrics || p.meter == nil {
		return metricnoop.NewMeterProvider().Meter("")
	}

	return p.meter
}

// Logger returns the OTel-correlated logger associated with this provider.
//
// The return value is always non-nil — when logging is disabled or the
// provider is shut down, [NewNoopLogger] is returned. This honours the
// non-nil contract on [Provider.Logger].
//
// This logger is distinct from
// [github.com/LerianStudio/midaz-sdk-golang/v5.Client.Logger]: that method
// returns the canonical *slog.Logger used for retry/internal lines, while
// this method returns the bespoke observability.Logger that integrates with
// the provider's tracing pipeline (call WithSpan(span) to inject trace_id /
// span_id). See the package overview in doc.go for the two-surface design.
func (p *MidazProvider) Logger() Logger {
	if p == nil || p.config == nil || !p.isEnabled() || !p.config.EnabledComponents.Logging || p.logger == nil {
		// Return a no-op logger if logging is disabled
		return NewNoopLogger()
	}

	return p.logger
}

// Shutdown gracefully shuts down the provider and all its components
func (p *MidazProvider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}

	p.lifecycleMu.Lock()
	if !p.enabled {
		p.lifecycleMu.Unlock()
		return nil
	}

	p.enabled = false
	telemetry := p.telemetry
	p.lifecycleMu.Unlock()

	if telemetry != nil {
		return telemetry.ShutdownTelemetryWithContext(ctx)
	}

	return nil
}

// IsEnabled returns true if observability is enabled
func (p *MidazProvider) IsEnabled() bool {
	return p.isEnabled()
}

// TextMapPropagator returns the provider-specific propagator without requiring
// this method on the public Provider interface.
func (p *MidazProvider) TextMapPropagator() propagation.TextMapPropagator {
	if p == nil || p.config == nil || !p.isEnabled() || !p.config.EnabledComponents.Tracing {
		return propagation.NewCompositeTextMapPropagator()
	}

	return p.textMapPropagatorFromConfig()
}

func (p *MidazProvider) textMapPropagatorFromConfig() propagation.TextMapPropagator {
	if p == nil || p.config == nil {
		return defaultTextMapPropagator()
	}

	if len(p.config.Propagators) > 0 {
		return propagation.NewCompositeTextMapPropagator(p.config.Propagators...)
	}

	return defaultTextMapPropagator()
}

func (p *MidazProvider) isEnabled() bool {
	if p == nil {
		return false
	}

	p.lifecycleMu.RLock()
	defer p.lifecycleMu.RUnlock()

	return p.enabled
}

// WithSpan creates a new span and executes the function within the context of that span.
// It automatically ends the span when the function returns.
func WithSpan(ctx context.Context, provider Provider, name string, fn func(context.Context) error, opts ...trace.SpanStartOption) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if fn == nil {
		return nil
	}

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
		sanitizedErr := sanitizeSensitiveString(err.Error())
		span.SetStatus(codes.Error, sanitizedErr)
		span.RecordError(errors.New(sanitizedErr))
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

	if ctx == nil {
		ctx = context.Background()
	}

	factory, err := metricsFactoryForProvider(provider)
	if err != nil {
		logProviderMetricError(provider, "Failed to create metrics factory for metric %s: %v", name, err)
		return
	}

	counterValue, err := metricCounterValue(value)
	if err != nil {
		logProviderMetricError(provider, "Failed to record counter metric %s: %v", name, err)
		return
	}

	counter, err := factory.Counter(obsmetrics.Metric{Name: name, Unit: "1"})
	if err != nil {
		logProviderMetricError(provider, "Failed to create counter for metric %s: %v", name, err)
		return
	}

	if err := counter.WithAttributes(filterMetricAttributes(attrs)...).Add(ctx, counterValue); err != nil {
		logProviderMetricError(provider, "Failed to record counter metric %s: %v", name, err)
	}
}

// RecordDuration records a duration metric using the provided meter
func RecordDuration(ctx context.Context, provider Provider, name string, start time.Time, attrs ...attribute.KeyValue) {
	// If provider is nil or observability is disabled, just return
	if provider == nil || !provider.IsEnabled() {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	duration := time.Since(start).Milliseconds()

	factory, err := metricsFactoryForProvider(provider)
	if err != nil {
		logProviderMetricError(provider, "Failed to create metrics factory for duration metric %s: %v", name, err)
		return
	}

	histogram, err := factory.Histogram(obsmetrics.Metric{Name: name, Unit: "ms"})
	if err != nil {
		logProviderMetricError(provider, "Failed to create histogram for metric %s: %v", name, err)
		return
	}

	if err := histogram.WithAttributes(filterMetricAttributes(attrs)...).Record(ctx, duration); err != nil {
		logProviderMetricError(provider, "Failed to record histogram metric %s: %v", name, err)
	}
}

func metricsFactoryForProvider(provider Provider) (*obsmetrics.MetricsFactory, error) {
	if midazProvider, ok := provider.(*MidazProvider); ok && midazProvider != nil && midazProvider.metricsFactory != nil {
		return midazProvider.metricsFactory, nil
	}

	meter := provider.Meter()
	if meter == nil {
		return nil, obsmetrics.ErrNilMeter
	}

	return obsmetrics.NewMetricsFactory(meter, nil)
}

func logProviderMetricError(provider Provider, format string, args ...any) {
	if provider == nil {
		return
	}

	if logger := provider.Logger(); logger != nil {
		logger.Errorf(format, args...)
	}
}

// ExtractContext extracts context from HTTP headers for distributed tracing
func ExtractContext(ctx context.Context, headers map[string]string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if headers == nil {
		return ctx
	}

	return textMapPropagatorForContext(ctx).Extract(ctx, propagation.MapCarrier(filterPropagationMap(ctx, headers)))
}

// InjectContext injects context into HTTP headers for distributed tracing
func InjectContext(ctx context.Context, headers map[string]string) {
	if ctx == nil || headers == nil {
		return
	}

	textMapPropagatorForContext(ctx).Inject(ctx, propagation.MapCarrier(headers))
}

// ExtractHTTPContext extracts distributed tracing context from HTTP headers.
func ExtractHTTPContext(ctx context.Context, headers http.Header) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if headers == nil {
		return ctx
	}

	return textMapPropagatorForContext(ctx).Extract(ctx, propagation.HeaderCarrier(filterPropagationHeaders(ctx, headers)))
}

// InjectHTTPContext injects distributed tracing context into HTTP headers.
func InjectHTTPContext(ctx context.Context, headers http.Header) {
	if ctx == nil || headers == nil {
		return
	}

	textMapPropagatorForContext(ctx).Inject(ctx, propagation.HeaderCarrier(headers))
}

func defaultTextMapPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func textMapPropagatorForContext(ctx context.Context) propagation.TextMapPropagator {
	return textMapPropagatorForProvider(GetProvider(ctx))
}

func textMapPropagatorForProvider(provider Provider) propagation.TextMapPropagator {
	if provider == nil {
		global := otel.GetTextMapPropagator()
		if global == nil || len(global.Fields()) == 0 {
			return defaultTextMapPropagator()
		}

		return global
	}

	if !provider.IsEnabled() {
		return propagation.NewCompositeTextMapPropagator()
	}

	if pp, ok := provider.(propagatorProvider); ok {
		return pp.TextMapPropagator()
	}

	return defaultTextMapPropagator()
}

func filterPropagationMap(ctx context.Context, headers map[string]string) map[string]string {
	allowed := propagationHeaderSet(ctx)
	if len(allowed) == 0 {
		return headers
	}

	filtered := make(map[string]string, len(headers))
	for key, value := range headers {
		if _, ok := allowed[strings.ToLower(key)]; ok {
			filtered[key] = value
		}
	}

	return filtered
}

func filterPropagationHeaders(ctx context.Context, headers http.Header) http.Header {
	allowed := propagationHeaderSet(ctx)
	if len(allowed) == 0 {
		return headers
	}

	filtered := make(http.Header, len(headers))
	for key, values := range headers {
		if _, ok := allowed[strings.ToLower(key)]; ok {
			filtered[key] = append([]string(nil), values...)
		}
	}

	return filtered
}

func propagationHeaderSet(ctx context.Context) map[string]struct{} {
	provider := GetProvider(ctx)

	midazProvider, ok := provider.(*MidazProvider)
	if !ok || midazProvider == nil || midazProvider.config == nil || len(midazProvider.config.PropagationHeaders) == 0 {
		return nil
	}

	midazProvider.propagationHeadersOnce.Do(func() {
		midazProvider.propagationHeadersAllow = buildPropagationHeaderAllowSet(provider, midazProvider.config)
	})

	return midazProvider.propagationHeadersAllow
}

// buildPropagationHeaderAllowSet computes the lowercased set of permitted
// propagation header names. It is invoked exactly once per provider via
// sync.Once and the resulting map is shared read-only.
func buildPropagationHeaderAllowSet(provider Provider, config *Config) map[string]struct{} {
	allowed := make(map[string]struct{}, len(config.PropagationHeaders))
	for _, header := range config.PropagationHeaders {
		header = strings.ToLower(strings.TrimSpace(header))
		if header != "" {
			allowed[header] = struct{}{}
		}
	}

	if !config.propagationHeadersExplicit {
		for _, header := range textMapPropagatorForProvider(provider).Fields() {
			header = strings.ToLower(strings.TrimSpace(header))
			if header != "" {
				allowed[header] = struct{}{}
			}
		}
	}

	return allowed
}

func filterMetricAttributes(attrs []attribute.KeyValue) []attribute.KeyValue {
	filtered := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		switch string(attr.Key) {
		case "trace_id", "span_id":
			continue
		default:
			filtered = append(filtered, attr)
		}
	}

	return filtered
}
