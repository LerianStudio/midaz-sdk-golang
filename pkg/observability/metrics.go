package observability

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	obsmetrics "github.com/LerianStudio/lib-observability/metrics"
	"go.opentelemetry.io/otel/attribute"
)

// MetricsCollector provides convenient methods for recording common metrics
// related to the SDK operations.
type MetricsCollector struct {
	provider Provider

	// Counters
	requestCounter *obsmetrics.CounterBuilder
	errorCounter   *obsmetrics.CounterBuilder
	successCounter *obsmetrics.CounterBuilder
	retryCounter   *obsmetrics.CounterBuilder

	// Histograms
	requestDuration     *obsmetrics.HistogramBuilder
	requestBatchSize    *obsmetrics.HistogramBuilder
	requestBatchLatency *obsmetrics.HistogramBuilder
}

type collectorInstruments struct {
	requestCounter      *obsmetrics.CounterBuilder
	errorCounter        *obsmetrics.CounterBuilder
	successCounter      *obsmetrics.CounterBuilder
	retryCounter        *obsmetrics.CounterBuilder
	requestDuration     *obsmetrics.HistogramBuilder
	requestBatchSize    *obsmetrics.HistogramBuilder
	requestBatchLatency *obsmetrics.HistogramBuilder
}

// NewMetricsCollector creates a new MetricsCollector for recording SDK metrics
func NewMetricsCollector(provider Provider) (*MetricsCollector, error) {
	// If provider is not enabled, return a no-op collector
	if provider == nil || !provider.IsEnabled() {
		return &MetricsCollector{provider: provider}, nil
	}

	meter := provider.Meter()
	if meter == nil {
		return &MetricsCollector{provider: provider}, nil
	}

	factory, err := obsmetrics.NewMetricsFactory(meter, nil)
	if err != nil {
		return nil, err
	}

	instruments, err := newCollectorInstruments(factory)
	if err != nil {
		return nil, err
	}

	return &MetricsCollector{
		provider:            provider,
		requestCounter:      instruments.requestCounter,
		errorCounter:        instruments.errorCounter,
		successCounter:      instruments.successCounter,
		retryCounter:        instruments.retryCounter,
		requestDuration:     instruments.requestDuration,
		requestBatchSize:    instruments.requestBatchSize,
		requestBatchLatency: instruments.requestBatchLatency,
	}, nil
}

func newCollectorInstruments(factory *obsmetrics.MetricsFactory) (collectorInstruments, error) {
	requestCounter, err := newCounter(factory, MetricRequestTotal, "Total number of API requests made")
	if err != nil {
		return collectorInstruments{}, err
	}
	errorCounter, err := newCounter(factory, MetricRequestErrorTotal, "Total number of API request errors")
	if err != nil {
		return collectorInstruments{}, err
	}
	successCounter, err := newCounter(factory, MetricRequestSuccess, "Total number of successful API requests")
	if err != nil {
		return collectorInstruments{}, err
	}
	retryCounter, err := newCounter(factory, MetricRequestRetryTotal, "Total number of API request retries")
	if err != nil {
		return collectorInstruments{}, err
	}
	requestDuration, err := newHistogram(factory, MetricRequestDuration, "Duration of API requests in milliseconds", "ms")
	if err != nil {
		return collectorInstruments{}, err
	}
	requestBatchSize, err := newHistogram(factory, MetricRequestBatchSize, "Size of API request batches", "1")
	if err != nil {
		return collectorInstruments{}, err
	}
	requestBatchLatency, err := newHistogram(factory, MetricRequestBatchLatency, "Latency of API request batches in milliseconds", "ms")
	if err != nil {
		return collectorInstruments{}, err
	}

	return collectorInstruments{requestCounter, errorCounter, successCounter, retryCounter, requestDuration, requestBatchSize, requestBatchLatency}, nil
}

func newCounter(factory *obsmetrics.MetricsFactory, name, description string) (*obsmetrics.CounterBuilder, error) {
	return factory.Counter(obsmetrics.Metric{Name: name, Description: description, Unit: "1"})
}

func newHistogram(factory *obsmetrics.MetricsFactory, name, description, unit string) (*obsmetrics.HistogramBuilder, error) {
	return factory.Histogram(obsmetrics.Metric{Name: name, Description: description, Unit: unit})
}

// RecordRequest records a request with its result and duration
func (m *MetricsCollector) RecordRequest(ctx context.Context, operation, resourceType string, statusCode int, duration time.Duration, attrs ...attribute.KeyValue) {
	// If provider is not enabled, do nothing
	if m == nil || m.provider == nil || !m.provider.IsEnabled() || m.requestCounter == nil || m.requestDuration == nil {
		return
	}

	// Set base attributes
	baseAttrs := make([]attribute.KeyValue, 0, 4+len(attrs))
	baseAttrs = append(baseAttrs,
		attribute.String(KeyOperationName, operation),
		attribute.String(KeyOperationType, "api.request"),
		attribute.String(KeyResourceType, resourceType),
		attribute.Int(KeyHTTPResponseStatusCode, statusCode),
	)

	// Combine with additional attributes
	allAttrs := append(baseAttrs, attrs...)

	// Record request
	if err := m.requestCounter.WithAttributes(allAttrs...).AddOne(ctx); err != nil {
		m.logMetricError("record request counter", err)
	}

	// Record duration in milliseconds
	if err := m.requestDuration.WithAttributes(allAttrs...).Record(ctx, duration.Milliseconds()); err != nil {
		m.logMetricError("record request duration", err)
	}

	// Record success or error
	if statusCode >= http.StatusBadRequest {
		// Error
		if err := m.errorCounter.WithAttributes(allAttrs...).AddOne(ctx); err != nil {
			m.logMetricError("record request error counter", err)
		}
	} else {
		// Success
		if err := m.successCounter.WithAttributes(allAttrs...).AddOne(ctx); err != nil {
			m.logMetricError("record request success counter", err)
		}
	}
}

// RecordBatchRequest records a batch request with its size and latency
func (m *MetricsCollector) RecordBatchRequest(ctx context.Context, operation, resourceType string, batchSize int, duration time.Duration, attrs ...attribute.KeyValue) {
	// If provider is not enabled, do nothing
	if m == nil || m.provider == nil || !m.provider.IsEnabled() || m.requestBatchSize == nil || m.requestBatchLatency == nil {
		return
	}

	// Set base attributes
	baseAttrs := make([]attribute.KeyValue, 0, 3+len(attrs))
	baseAttrs = append(baseAttrs,
		attribute.String(KeyOperationName, operation),
		attribute.String(KeyOperationType, "api.batch"),
		attribute.String(KeyResourceType, resourceType),
	)

	// Combine with additional attributes
	allAttrs := append(baseAttrs, attrs...)

	// Record batch size
	if err := m.requestBatchSize.WithAttributes(allAttrs...).Record(ctx, int64(batchSize)); err != nil {
		m.logMetricError("record request batch size", err)
	}

	// Record batch latency in milliseconds
	if err := m.requestBatchLatency.WithAttributes(allAttrs...).Record(ctx, duration.Milliseconds()); err != nil {
		m.logMetricError("record request batch latency", err)
	}
}

// RecordRetry records a retry attempt
func (m *MetricsCollector) RecordRetry(ctx context.Context, operation, resourceType string, attempt int, attrs ...attribute.KeyValue) {
	// If provider is not enabled, do nothing
	if m == nil || m.provider == nil || !m.provider.IsEnabled() || m.retryCounter == nil {
		return
	}

	// Set base attributes
	baseAttrs := make([]attribute.KeyValue, 0, 4+len(attrs))
	baseAttrs = append(baseAttrs,
		attribute.String(KeyOperationName, operation),
		attribute.String(KeyOperationType, "api.retry"),
		attribute.String(KeyResourceType, resourceType),
		attribute.Int("retry.attempt", attempt),
	)

	// Combine with additional attributes
	allAttrs := append(baseAttrs, attrs...)

	// Record retry
	if err := m.retryCounter.WithAttributes(allAttrs...).AddOne(ctx); err != nil {
		m.logMetricError("record retry counter", err)
	}
}

func (m *MetricsCollector) logMetricError(operation string, err error) {
	if err == nil || m == nil || m.provider == nil {
		return
	}

	if logger := m.provider.Logger(); logger != nil {
		logger.Errorf("%s: %v", operation, err)
	}
}

func metricCounterValue(value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("metric counter value must be finite: %v", value)
	}
	if value < 0 {
		return 0, fmt.Errorf("metric counter value must be non-negative: %v", value)
	}
	if value != math.Trunc(value) {
		return 0, fmt.Errorf("metric counter value must be an integer: %v", value)
	}
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("metric counter value exceeds int64 range: %v", value)
	}

	return int64(value), nil
}

// Timer provides a convenient way to record the duration of an operation
type Timer struct {
	startTime    time.Time
	collector    *MetricsCollector
	ctx          context.Context
	operation    string
	resourceType string
	attrs        []attribute.KeyValue
}

// NewTimer creates a new timer for recording the duration of an operation
func (m *MetricsCollector) NewTimer(ctx context.Context, operation, resourceType string, attrs ...attribute.KeyValue) *Timer {
	return &Timer{
		startTime:    time.Now(),
		collector:    m,
		ctx:          ctx,
		operation:    operation,
		resourceType: resourceType,
		attrs:        attrs,
	}
}

// Stop records the duration of the operation with the result
func (t *Timer) Stop(statusCode int, additionalAttrs ...attribute.KeyValue) {
	if t == nil || t.collector == nil {
		return
	}

	duration := time.Since(t.startTime)
	allAttrs := append(t.attrs, additionalAttrs...)
	t.collector.RecordRequest(t.ctx, t.operation, t.resourceType, statusCode, duration, allAttrs...)
}

// StopBatch records the duration of a batch operation
func (t *Timer) StopBatch(batchSize int, additionalAttrs ...attribute.KeyValue) {
	if t == nil || t.collector == nil {
		return
	}

	duration := time.Since(t.startTime)
	allAttrs := append(t.attrs, additionalAttrs...)
	t.collector.RecordBatchRequest(t.ctx, t.operation, t.resourceType, batchSize, duration, allAttrs...)
}
