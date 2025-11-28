package observability

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// =============================================================================
// Option Validation Tests
// =============================================================================

func TestWithServiceNameValidation(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid service name",
			serviceName: "test-service",
			wantErr:     false,
		},
		{
			name:        "empty service name",
			serviceName: "",
			wantErr:     true,
			errContains: "service name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithServiceName(tt.serviceName)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.serviceName, config.ServiceName)
			}
		})
	}
}

func TestWithServiceVersionValidation(t *testing.T) {
	tests := []struct {
		name           string
		serviceVersion string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "valid version",
			serviceVersion: "1.0.0",
			wantErr:        false,
		},
		{
			name:           "empty version",
			serviceVersion: "",
			wantErr:        true,
			errContains:    "service version cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithServiceVersion(tt.serviceVersion)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.serviceVersion, config.ServiceVersion)
			}
		})
	}
}

func TestWithSDKVersionValidation(t *testing.T) {
	tests := []struct {
		name        string
		sdkVersion  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid SDK version",
			sdkVersion: "2.0.0",
			wantErr:    false,
		},
		{
			name:        "empty SDK version",
			sdkVersion:  "",
			wantErr:     true,
			errContains: "SDK version cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithSDKVersion(tt.sdkVersion)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.sdkVersion, config.SDKVersion)
			}
		})
	}
}

func TestWithEnvironmentValidation(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantErr     bool
		errContains string
	}{
		{
			name:        "production environment",
			environment: "production",
			wantErr:     false,
		},
		{
			name:        "development environment",
			environment: "development",
			wantErr:     false,
		},
		{
			name:        "empty environment",
			environment: "",
			wantErr:     true,
			errContains: "environment cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithEnvironment(tt.environment)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.environment, config.Environment)
			}
		})
	}
}

func TestWithCollectorEndpointValidation(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid endpoint",
			endpoint: "localhost:4317",
			wantErr:  false,
		},
		{
			name:        "empty endpoint",
			endpoint:    "",
			wantErr:     true,
			errContains: "collector endpoint cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithCollectorEndpoint(tt.endpoint)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.endpoint, config.CollectorEndpoint)
			}
		})
	}
}

func TestWithLogLevelValidation(t *testing.T) {
	tests := []struct {
		name        string
		level       LogLevel
		wantErr     bool
		errContains string
	}{
		{
			name:    "debug level",
			level:   DebugLevel,
			wantErr: false,
		},
		{
			name:    "info level",
			level:   InfoLevel,
			wantErr: false,
		},
		{
			name:    "warn level",
			level:   WarnLevel,
			wantErr: false,
		},
		{
			name:    "error level",
			level:   ErrorLevel,
			wantErr: false,
		},
		{
			name:    "fatal level",
			level:   FatalLevel,
			wantErr: false,
		},
		{
			name:        "invalid level below range",
			level:       LogLevel(-1),
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name:        "invalid level above range",
			level:       LogLevel(100),
			wantErr:     true,
			errContains: "invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithLogLevel(tt.level)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.level, config.LogLevel)
			}
		})
	}
}

func TestWithLogOutputValidation(t *testing.T) {
	t.Run("valid output", func(t *testing.T) {
		var buf bytes.Buffer

		config := DefaultConfig()
		err := WithLogOutput(&buf)(config)
		require.NoError(t, err)
		assert.Equal(t, &buf, config.LogOutput)
	})

	t.Run("nil output", func(t *testing.T) {
		config := DefaultConfig()
		err := WithLogOutput(nil)(config)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log output cannot be nil")
	})
}

func TestWithTraceSampleRateValidation(t *testing.T) {
	tests := []struct {
		name        string
		rate        float64
		wantErr     bool
		errContains string
	}{
		{
			name:    "zero rate",
			rate:    0.0,
			wantErr: false,
		},
		{
			name:    "half rate",
			rate:    0.5,
			wantErr: false,
		},
		{
			name:    "full rate",
			rate:    1.0,
			wantErr: false,
		},
		{
			name:        "negative rate",
			rate:        -0.1,
			wantErr:     true,
			errContains: "trace sample rate must be between",
		},
		{
			name:        "rate above 1",
			rate:        1.5,
			wantErr:     true,
			errContains: "trace sample rate must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithTraceSampleRate(tt.rate)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.rate, config.TraceSampleRate, 0.001)
			}
		})
	}
}

func TestWithPropagatorsValidation(t *testing.T) {
	tests := []struct {
		name        string
		propagators []propagation.TextMapPropagator
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid propagator",
			propagators: []propagation.TextMapPropagator{propagation.TraceContext{}},
			wantErr:     false,
		},
		{
			name:        "multiple propagators",
			propagators: []propagation.TextMapPropagator{propagation.TraceContext{}, propagation.Baggage{}},
			wantErr:     false,
		},
		{
			name:        "empty propagators",
			propagators: []propagation.TextMapPropagator{},
			wantErr:     true,
			errContains: "at least one propagator must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithPropagators(tt.propagators...)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Len(t, config.Propagators, len(tt.propagators))
			}
		})
	}
}

func TestWithPropagationHeadersValidation(t *testing.T) {
	tests := []struct {
		name        string
		headers     []string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid headers",
			headers: []string{"traceparent", "tracestate"},
			wantErr: false,
		},
		{
			name:    "single header",
			headers: []string{"x-request-id"},
			wantErr: false,
		},
		{
			name:        "empty headers",
			headers:     []string{},
			wantErr:     true,
			errContains: "at least one propagation header must be provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()

			err := WithPropagationHeaders(tt.headers...)(config)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.headers, config.PropagationHeaders)
			}
		})
	}
}

func TestWithRegisterGlobally(t *testing.T) {
	t.Run("register globally true", func(t *testing.T) {
		config := DefaultConfig()
		config.RegisterGlobally = false
		err := WithRegisterGlobally(true)(config)
		require.NoError(t, err)
		assert.True(t, config.RegisterGlobally)
	})

	t.Run("register globally false", func(t *testing.T) {
		config := DefaultConfig()
		err := WithRegisterGlobally(false)(config)
		require.NoError(t, err)
		assert.False(t, config.RegisterGlobally)
	})
}

func TestWithComponentEnabled(t *testing.T) {
	tests := []struct {
		name                                    string
		tracing, metrics, logging               bool
		expectTracing, expectMetrics, expectLog bool
	}{
		{
			name:          "all enabled",
			tracing:       true,
			metrics:       true,
			logging:       true,
			expectTracing: true,
			expectMetrics: true,
			expectLog:     true,
		},
		{
			name:          "all disabled",
			tracing:       false,
			metrics:       false,
			logging:       false,
			expectTracing: false,
			expectMetrics: false,
			expectLog:     false,
		},
		{
			name:          "only tracing",
			tracing:       true,
			metrics:       false,
			logging:       false,
			expectTracing: true,
			expectMetrics: false,
			expectLog:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			err := WithComponentEnabled(tt.tracing, tt.metrics, tt.logging)(config)
			require.NoError(t, err)
			assert.Equal(t, tt.expectTracing, config.EnabledComponents.Tracing)
			assert.Equal(t, tt.expectMetrics, config.EnabledComponents.Metrics)
			assert.Equal(t, tt.expectLog, config.EnabledComponents.Logging)
		})
	}
}

func TestWithAttributes(t *testing.T) {
	config := DefaultConfig()
	attrs := []attribute.KeyValue{
		attribute.String("key1", "value1"),
		attribute.Int("key2", 42),
		attribute.Bool("key3", true),
	}

	err := WithAttributes(attrs...)(config)
	require.NoError(t, err)
	assert.Len(t, config.Attributes, 3)
}

// =============================================================================
// Provider Tests
// =============================================================================

func TestProviderDisabledComponents(t *testing.T) {
	t.Run("disabled tracing returns noop tracer", func(t *testing.T) {
		provider, err := New(context.Background(),
			WithComponentEnabled(false, false, false),
			WithRegisterGlobally(false),
		)
		require.NoError(t, err)

		defer func() { _ = provider.Shutdown(context.Background()) }()

		tracer := provider.Tracer()
		assert.NotNil(t, tracer)

		ctx, span := tracer.Start(context.Background(), "test")
		assert.NotNil(t, span)
		assert.NotNil(t, ctx)
		span.End()
	})

	t.Run("disabled metrics returns default meter", func(t *testing.T) {
		provider, err := New(context.Background(),
			WithComponentEnabled(false, false, false),
			WithRegisterGlobally(false),
		)
		require.NoError(t, err)

		defer func() { _ = provider.Shutdown(context.Background()) }()

		meter := provider.Meter()
		assert.NotNil(t, meter)
	})

	t.Run("disabled logging returns noop logger", func(t *testing.T) {
		provider, err := New(context.Background(),
			WithComponentEnabled(false, false, false),
			WithRegisterGlobally(false),
		)
		require.NoError(t, err)

		defer func() { _ = provider.Shutdown(context.Background()) }()

		logger := provider.Logger()
		assert.NotNil(t, logger)

		// Should not panic
		logger.Debug("test")
		logger.Info("test")
		logger.Warn("test")
		logger.Error("test")
	})
}

func TestProviderShutdown(t *testing.T) {
	t.Run("shutdown twice", func(t *testing.T) {
		provider, err := New(context.Background(),
			WithRegisterGlobally(false),
		)
		require.NoError(t, err)

		err = provider.Shutdown(context.Background())
		require.NoError(t, err)
		assert.False(t, provider.IsEnabled())

		// Second shutdown should be safe
		err = provider.Shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("components after shutdown", func(t *testing.T) {
		provider, err := New(context.Background(),
			WithRegisterGlobally(false),
		)
		require.NoError(t, err)

		err = provider.Shutdown(context.Background())
		require.NoError(t, err)

		// Should return no-op components after shutdown
		tracer := provider.Tracer()
		assert.NotNil(t, tracer)

		meter := provider.Meter()
		assert.NotNil(t, meter)

		logger := provider.Logger()
		assert.NotNil(t, logger)
	})
}

func TestProviderWithNonGlobalRegistration(t *testing.T) {
	provider, err := New(context.Background(),
		WithServiceName("test-service"),
		WithServiceVersion("1.0.0"),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	assert.True(t, provider.IsEnabled())

	// Provider should work normally
	tracer := provider.Tracer()
	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, span)
	span.End()

	logger := provider.Logger()
	logger.Info("test message")

	_ = ctx
}

// =============================================================================
// WithSpan Tests
// =============================================================================

func TestWithSpanNilProvider(t *testing.T) {
	var executed bool

	err := WithSpan(context.Background(), nil, "test-span", func(_ context.Context) error {
		executed = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, executed)
}

func TestWithSpanDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	// Shutdown to disable
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	var executed bool

	err = WithSpan(context.Background(), provider, "test-span", func(_ context.Context) error {
		executed = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, executed)
}

func TestWithSpanErrorHandling(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	expectedErr := errors.New("test error")
	err = WithSpan(context.Background(), provider, "error-span", func(_ context.Context) error {
		return expectedErr
	})

	assert.Equal(t, expectedErr, err)
}

// =============================================================================
// RecordMetric and RecordDuration Tests
// =============================================================================

func TestRecordMetricNilProvider(_ *testing.T) {
	// Should not panic
	RecordMetric(context.Background(), nil, "test.metric", 1.0)
}

func TestRecordMetricDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	// Should not panic
	RecordMetric(context.Background(), provider, "test.metric", 1.0)
}

func TestRecordDurationNilProvider(_ *testing.T) {
	// Should not panic
	RecordDuration(context.Background(), nil, "test.duration", time.Now())
}

func TestRecordDurationDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	// Should not panic
	RecordDuration(context.Background(), provider, "test.duration", time.Now())
}

// =============================================================================
// Context Functions Tests
// =============================================================================

//nolint:staticcheck // SA1012: intentionally testing nil context behavior
func TestGetProviderNilContext(t *testing.T) {
	provider := GetProvider(nil)
	assert.Nil(t, provider)
}

func TestGetProviderNoValue(t *testing.T) {
	ctx := context.Background()
	provider := GetProvider(ctx)
	assert.Nil(t, provider)
}

func TestWithProviderAndGetProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx := WithProvider(context.Background(), provider)
	retrieved := GetProvider(ctx)

	assert.Equal(t, provider, retrieved)
}

func TestWithSpanAttributesNoRecording(t *testing.T) {
	// Use a context without a recording span
	ctx := context.Background()
	resultCtx := WithSpanAttributes(ctx, attribute.String("key", "value"))
	assert.NotNil(t, resultCtx)
}

func TestAddSpanAttributesNoRecording(_ *testing.T) {
	// Use a context without a recording span
	ctx := context.Background()
	// Should not panic
	AddSpanAttributes(ctx, attribute.String("key", "value"))
}

func TestAddSpanEventNoRecording(_ *testing.T) {
	// Use a context without a recording span
	ctx := context.Background()
	// Should not panic
	AddSpanEvent(ctx, "test-event", attribute.String("key", "value"))
}

func TestWithBaggageItemNewBaggage(t *testing.T) {
	ctx := context.Background()
	newCtx, err := WithBaggageItem(ctx, "key1", "value1")
	require.NoError(t, err)

	value := GetBaggageItem(newCtx, "key1")
	assert.Equal(t, "value1", value)
}

func TestWithBaggageItemExistingBaggage(t *testing.T) {
	ctx := context.Background()

	// Add first item
	ctx, err := WithBaggageItem(ctx, "key1", "value1")
	require.NoError(t, err)

	// Add second item
	ctx, err = WithBaggageItem(ctx, "key2", "value2")
	require.NoError(t, err)

	// Both should exist
	assert.Equal(t, "value1", GetBaggageItem(ctx, "key1"))
	assert.Equal(t, "value2", GetBaggageItem(ctx, "key2"))
}

func TestGetBaggageItemNotFound(t *testing.T) {
	ctx := context.Background()
	value := GetBaggageItem(ctx, "nonexistent")
	assert.Empty(t, value)
}

func TestStartWithProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx := WithProvider(context.Background(), provider)
	newCtx, span := Start(ctx, "test-span")

	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

func TestStartWithoutProvider(t *testing.T) {
	ctx := context.Background()
	newCtx, span := Start(ctx, "test-span")

	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

func TestLogWithProvider(t *testing.T) {
	var buf bytes.Buffer

	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, true),
		WithLogOutput(&buf),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx := WithProvider(context.Background(), provider)
	logger := Log(ctx)

	assert.NotNil(t, logger)
	logger.Info("test message")
}

func TestLogWithoutProvider(t *testing.T) {
	ctx := context.Background()
	logger := Log(ctx)

	assert.NotNil(t, logger)
	// Should be a no-op logger, shouldn't panic
	logger.Info("test message")
}

func TestTraceIDWithValidSpan(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	traceID := TraceID(ctx)
	// Trace ID should be non-empty for a valid span
	assert.NotEmpty(t, traceID)
}

func TestTraceIDWithoutSpan(t *testing.T) {
	ctx := context.Background()
	traceID := TraceID(ctx)
	assert.Empty(t, traceID)
}

func TestSpanIDWithValidSpan(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	spanID := SpanID(ctx)
	// Span ID should be non-empty for a valid span
	assert.NotEmpty(t, spanID)
}

func TestSpanIDWithoutSpan(t *testing.T) {
	ctx := context.Background()
	spanID := SpanID(ctx)
	assert.Empty(t, spanID)
}

// =============================================================================
// Span Utility Functions Tests
// =============================================================================

func TestAddAttributeTypes(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	// Test different types
	AddAttribute(ctx, "string-key", "string-value")
	AddAttribute(ctx, "int-key", 42)
	AddAttribute(ctx, "int64-key", int64(42))
	AddAttribute(ctx, "float64-key", 3.14)
	AddAttribute(ctx, "bool-key", true)
	AddAttribute(ctx, "other-key", struct{ Name string }{"test"})
}

func TestAddAttributeNonRecordingSpan(_ *testing.T) {
	ctx := context.Background()
	// Should not panic
	AddAttribute(ctx, "key", "value")
}

func TestRecordErrorNilError(_ *testing.T) {
	ctx := context.Background()
	// Should not panic
	RecordError(ctx, nil, "event-name")
}

func TestRecordErrorNonRecordingSpan(_ *testing.T) {
	ctx := context.Background()
	err := errors.New("test error")
	// Should not panic
	RecordError(ctx, err, "error-event")
}

func TestRecordErrorWithAttrs(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	testErr := errors.New("test error")
	attrs := map[string]string{
		"operation": "test",
		"retry":     "1",
	}
	RecordError(ctx, testErr, "error-event", attrs)
}

func TestAddEventWithAttrs(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	attrs := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	AddEvent(ctx, "test-event", attrs)
}

func TestAddEventNonRecordingSpan(_ *testing.T) {
	ctx := context.Background()
	attrs := map[string]string{"key": "value"}
	// Should not panic
	AddEvent(ctx, "test-event", attrs)
}

func TestRecordSpanMetric(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	// Record metric with span context
	RecordSpanMetric(ctx, "test.metric", 1.0)
}

func TestRecordSpanMetricNoSpan(_ *testing.T) {
	ctx := context.Background()
	// Should not panic
	RecordSpanMetric(ctx, "test.metric", 1.0)
}

func TestWithTraceID(t *testing.T) {
	ctx := context.Background()
	newCtx := WithTraceID(ctx, "test-trace-id")
	// Currently returns the same context
	assert.NotNil(t, newCtx)
}

// =============================================================================
// Logging Tests
// =============================================================================

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{FatalLevel, "FATAL"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestLoggerAllLevels(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil)

	logger.Debug("debug message")
	logger.Debugf("debug %s", "formatted")
	logger.Info("info message")
	logger.Infof("info %s", "formatted")
	logger.Warn("warn message")
	logger.Warnf("warn %s", "formatted")
	logger.Error("error message")
	logger.Errorf("error %s", "formatted")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "debug formatted")
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "info formatted")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "warn formatted")
	assert.Contains(t, output, "error message")
	assert.Contains(t, output, "error formatted")
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(WarnLevel, &buf, nil)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	assert.NotContains(t, output, "debug message")
	assert.NotContains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil)

	fieldsLogger := logger.With(map[string]any{
		"request_id": "12345",
		"user_id":    "abc",
	})

	fieldsLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "12345")
	assert.Contains(t, output, "user_id")
	assert.Contains(t, output, "abc")
}

func TestLoggerReservedFieldsProtection(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil)

	// Try to overwrite reserved fields
	fieldsLogger := logger.With(map[string]any{
		"timestamp": "malicious-timestamp",
		"level":     "malicious-level",
		"message":   "malicious-message",
		"caller":    "malicious-caller",
		"safe_key":  "safe_value",
	})

	fieldsLogger.Info("test message")

	output := buf.String()
	// Reserved fields should not be overwritten
	assert.NotContains(t, output, "malicious-timestamp")
	assert.NotContains(t, output, "malicious-level")
	assert.NotContains(t, output, "malicious-message")
	assert.NotContains(t, output, "malicious-caller")
	// Safe field should be present
	assert.Contains(t, output, "safe_key")
	assert.Contains(t, output, "safe_value")
}

func TestLoggerWithContext(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	// Create a valid span context for testing
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	_, span := provider.Tracer().Start(context.Background(), "test")
	defer span.End()

	spanCtx := span.SpanContext()
	contextLogger := logger.WithContext(spanCtx)

	contextLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "trace_id")
}

func TestLoggerWithContextInvalid(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	// Use invalid span context
	contextLogger := logger.WithContext(trace.SpanContext{})

	// Should return the same logger
	assert.Equal(t, logger, contextLogger)
}

func TestLoggerWithSpan(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	_, span := provider.Tracer().Start(context.Background(), "test")
	defer span.End()

	spanLogger := logger.WithSpan(span)

	spanLogger.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "trace_id")
}

func TestLoggerWithSpanNil(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	spanLogger := logger.WithSpan(nil)

	// Should return the same logger
	assert.Equal(t, logger, spanLogger)
}

func TestLoggerFatal(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	// With nil exit function (default for library code), Fatal just logs without terminating
	logger.SetExitFunc(nil)

	// Should not panic - library code should not terminate the caller
	assert.NotPanics(t, func() {
		logger.Fatal("fatal message")
	})

	// Verify the message was logged
	assert.Contains(t, buf.String(), "fatal message")
	assert.Contains(t, buf.String(), "FATAL")
}

func TestLoggerFatalf(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	// With nil exit function (default for library code), Fatal just logs without terminating
	logger.SetExitFunc(nil)

	// Should not panic - library code should not terminate the caller
	assert.NotPanics(t, func() {
		logger.Fatalf("fatal %s", "formatted")
	})

	// Verify the message was logged
	assert.Contains(t, buf.String(), "fatal formatted")
	assert.Contains(t, buf.String(), "FATAL")
}

func TestLoggerFatalWithCustomExit(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	var (
		exitCalled bool
		exitCode   int
	)

	logger.SetExitFunc(func(code int) {
		exitCalled = true
		exitCode = code
	})

	logger.Fatal("fatal message")

	assert.True(t, exitCalled)
	assert.Equal(t, 1, exitCode)
}

func TestNoopLogger(t *testing.T) {
	logger := NewNoopLogger()

	// All methods should be no-ops
	logger.Debug("debug")
	logger.Debugf("debug %s", "formatted")
	logger.Info("info")
	logger.Infof("info %s", "formatted")
	logger.Warn("warn")
	logger.Warnf("warn %s", "formatted")
	logger.Error("error")
	logger.Errorf("error %s", "formatted")
	logger.Fatal("fatal")
	logger.Fatalf("fatal %s", "formatted")

	// With should return the same logger
	withLogger := logger.With(map[string]any{"key": "value"})
	assert.Equal(t, logger, withLogger)

	// WithContext should return the same logger
	contextLogger := logger.WithContext(trace.SpanContext{})
	assert.Equal(t, logger, contextLogger)

	// WithSpan should return the same logger
	spanLogger := logger.WithSpan(nil)
	assert.Equal(t, logger, spanLogger)
}

func TestLoggerNilOutput(t *testing.T) {
	// Should default to stderr
	logger := NewLogger(DebugLevel, nil, nil)
	assert.NotNil(t, logger)
}

// =============================================================================
// MetricsCollector Tests
// =============================================================================

func TestMetricsCollectorDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	collector, err := NewMetricsCollector(provider)
	require.NoError(t, err)
	assert.NotNil(t, collector)

	// All methods should be no-ops
	ctx := context.Background()
	collector.RecordRequest(ctx, "op", "resource", 200, time.Millisecond)
	collector.RecordBatchRequest(ctx, "op", "resource", 10, time.Millisecond)
	collector.RecordRetry(ctx, "op", "resource", 1)
}

func TestMetricsCollectorRecordRequestError(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(false, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	collector, err := NewMetricsCollector(provider)
	require.NoError(t, err)

	// Record a request with error status
	ctx := context.Background()
	collector.RecordRequest(ctx, "test.op", "account", 500, 100*time.Millisecond)
	collector.RecordRequest(ctx, "test.op", "account", 404, 50*time.Millisecond)
}

func TestMetricsCollectorWithAdditionalAttrs(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(false, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	collector, err := NewMetricsCollector(provider)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with additional attributes
	collector.RecordRequest(ctx, "test.op", "account", 200, time.Millisecond,
		attribute.String("extra", "value"))

	collector.RecordBatchRequest(ctx, "test.batch", "account", 10, time.Millisecond,
		attribute.String("batch_id", "123"))

	collector.RecordRetry(ctx, "test.retry", "account", 2,
		attribute.String("reason", "timeout"))
}

func TestTimerWithAdditionalAttrs(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(false, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	collector, err := NewMetricsCollector(provider)
	require.NoError(t, err)

	ctx := context.Background()

	timer := collector.NewTimer(ctx, "test.op", "resource",
		attribute.String("initial", "attr"))
	timer.Stop(200, attribute.String("final", "attr"))

	batchTimer := collector.NewTimer(ctx, "test.batch", "resource")
	batchTimer.StopBatch(5, attribute.String("batch", "attr"))
}

// =============================================================================
// HTTP Middleware Tests
// =============================================================================

func TestHTTPMiddlewareOptions(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	t.Run("WithIgnoreHeaders empty", func(t *testing.T) {
		m := &httpMiddleware{}
		err := WithIgnoreHeaders()(m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one header must be provided")
	})

	t.Run("WithIgnorePaths empty", func(t *testing.T) {
		m := &httpMiddleware{}
		err := WithIgnorePaths()(m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one path must be provided")
	})

	t.Run("WithMaskedParams empty", func(t *testing.T) {
		m := &httpMiddleware{}
		err := WithMaskedParams()(m)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one parameter must be provided")
	})

	t.Run("WithDefaultSensitiveHeaders", func(t *testing.T) {
		m := &httpMiddleware{}
		err := WithDefaultSensitiveHeaders()(m)
		require.NoError(t, err)
		assert.Contains(t, m.ignoreHeaders, "authorization")
		assert.Contains(t, m.ignoreHeaders, "cookie")
	})

	t.Run("WithDefaultSensitiveParams", func(t *testing.T) {
		m := &httpMiddleware{}
		err := WithDefaultSensitiveParams()(m)
		require.NoError(t, err)
		assert.Contains(t, m.maskedParams, "access_token")
		assert.Contains(t, m.maskedParams, "password")
	})

	t.Run("WithHideRequestBody", func(t *testing.T) {
		m := &httpMiddleware{}
		err := WithHideRequestBody(true)(m)
		require.NoError(t, err)
		assert.True(t, m.hideBody)
	})
}

func TestHTTPMiddlewareIgnoredPath(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithIgnorePaths("/health"),
		)(http.DefaultTransport),
	}

	// Request to ignored path
	resp, err := client.Get(server.URL + "/health")
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPMiddlewareDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	// Shutdown to disable
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPMiddlewareErrorResponse(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHTTPMiddlewareRequestError(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Create a server that we immediately close to simulate connection refused
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	serverURL := server.URL
	server.Close() // Close immediately

	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
		Timeout:   100 * time.Millisecond,
	}

	// Request to closed server should fail
	resp, err := client.Get(serverURL)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	require.Error(t, err)
}

func TestHTTPMiddlewareHeaderFiltering(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.Header().Set("Authorization", "secret")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithIgnoreHeaders("authorization"),
		)(http.DefaultTransport),
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-Request-Header", "request-value")

	resp, err := client.Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTLSVersionString(t *testing.T) {
	tests := []struct {
		version  uint16
		expected string
	}{
		{0x0301, "TLS 1.0"},
		{0x0302, "TLS 1.1"},
		{0x0303, "TLS 1.2"},
		{0x0304, "TLS 1.3"},
		{0x0000, "unknown (0x0000)"},
		{0xFFFF, "unknown (0xffff)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tlsVersionString(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTLSCipherSuiteString(t *testing.T) {
	tests := []struct {
		suite    uint16
		expected string
	}{
		{0x1301, "TLS_AES_128_GCM_SHA256"},
		{0x1302, "TLS_AES_256_GCM_SHA384"},
		{0x1303, "TLS_CHACHA20_POLY1305_SHA256"},
		{0xc02b, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"},
		{0xc02c, "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
		{0xc02f, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		{0xc030, "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
		{0x0000, "unknown (0x0000)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tlsCipherSuiteString(tt.suite)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// NewWithConfig Tests
// =============================================================================

func TestNewWithConfigAllFields(t *testing.T) {
	var buf bytes.Buffer

	config := &Config{
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		SDKVersion:        "2.0.0",
		Environment:       "test",
		CollectorEndpoint: "", // Empty to avoid connection attempts
		LogLevel:          DebugLevel,
		LogOutput:         &buf,
		TraceSampleRate:   0.5,
		EnabledComponents: EnabledComponents{
			Tracing: true,
			Metrics: true,
			Logging: true,
		},
		Attributes: []attribute.KeyValue{
			attribute.String("custom", "attr"),
		},
		Propagators: []propagation.TextMapPropagator{
			propagation.TraceContext{},
		},
		PropagationHeaders: []string{"traceparent"},
		RegisterGlobally:   false,
	}

	provider, err := NewWithConfig(context.Background(), config)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	assert.True(t, provider.IsEnabled())
}

// =============================================================================
// DefaultConfig Tests
// =============================================================================

func TestDefaultConfigValues(t *testing.T) {
	config := DefaultConfig()

	assert.NotEmpty(t, config.ServiceName)
	assert.NotEmpty(t, config.ServiceVersion)
	assert.NotEmpty(t, config.SDKVersion)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, InfoLevel, config.LogLevel)
	assert.InDelta(t, 0.1, config.TraceSampleRate, 0.001)
	assert.True(t, config.EnabledComponents.Tracing)
	assert.True(t, config.EnabledComponents.Metrics)
	assert.True(t, config.EnabledComponents.Logging)
	assert.True(t, config.RegisterGlobally)
	assert.NotEmpty(t, config.PropagationHeaders)
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestProviderWithInvalidOption(t *testing.T) {
	// Create an option that returns an error
	badOption := func(_ *Config) error {
		return errors.New("intentional error")
	}

	_, err := New(context.Background(), badOption)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to apply option")
}

func TestIgnoreHeadersCaseInsensitive(t *testing.T) {
	m := &httpMiddleware{
		ignoreHeaders: []string{"authorization", "cookie"},
	}

	assert.True(t, m.isIgnoredHeader("Authorization"))
	assert.True(t, m.isIgnoredHeader("AUTHORIZATION"))
	assert.True(t, m.isIgnoredHeader("authorization"))
	assert.True(t, m.isIgnoredHeader("Cookie"))
	assert.False(t, m.isIgnoredHeader("X-Custom"))
}

func TestHTTPMiddlewareMergeIgnoreHeaders(t *testing.T) {
	m := &httpMiddleware{
		ignoreHeaders: []string{"authorization"},
	}

	err := WithIgnoreHeaders("cookie", "x-api-key")(m)
	require.NoError(t, err)

	// Should have all headers
	assert.True(t, m.isIgnoredHeader("authorization"))
	assert.True(t, m.isIgnoredHeader("cookie"))
	assert.True(t, m.isIgnoredHeader("x-api-key"))
}

func TestContextPropagationFunctions(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx, span := provider.Tracer().Start(context.Background(), "test-span")
	defer span.End()

	// Test inject
	headers := make(map[string]string)
	InjectContext(ctx, headers)

	// Should have trace context
	assert.NotEmpty(t, headers)

	// Test extract
	extractedCtx := ExtractContext(context.Background(), headers)
	assert.NotNil(t, extractedCtx)
}

func TestRoundTripperFuncInterface(t *testing.T) {
	called := false
	rtf := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	resp, err := rtf.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}

	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, 200, resp.StatusCode)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestFullObservabilityWorkflow(t *testing.T) {
	var logBuf bytes.Buffer

	// Create provider with all components
	provider, err := New(context.Background(),
		WithServiceName("integration-test"),
		WithServiceVersion("1.0.0"),
		WithEnvironment("test"),
		WithLogLevel(DebugLevel),
		WithLogOutput(&logBuf),
		WithFullTracingSampling(),
		WithComponentEnabled(true, true, true),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	// Store provider in context
	ctx := WithProvider(context.Background(), provider)

	// Create span using context
	ctx, span := Start(ctx, "parent-operation")

	// Add attributes
	AddSpanAttributes(ctx, attribute.String("operation", "test"))

	// Create child span
	childCtx, childSpan := Start(ctx, "child-operation")
	AddSpanEvent(childCtx, "processing-started", attribute.Int("batch", 1))

	// Simulate error
	testErr := errors.New("processing failed")
	RecordError(childCtx, testErr, "processing-error", map[string]string{
		"step": "validation",
	})

	childSpan.End()

	// Add event to parent
	AddSpanEvent(ctx, "child-completed")

	span.End()

	// Log some messages
	logger := Log(ctx)
	logger.Info("operation completed")

	// Get IDs
	traceID := TraceID(ctx)
	spanID := SpanID(ctx)

	// Cleanup
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	// Verify logs contain expected content
	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "operation completed")

	// Trace and span IDs should be non-empty when sampled
	_ = traceID
	_ = spanID
}

func TestHTTPClientWithObservability(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Create test server
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Note: Traceparent header may or may not be present depending on sampling
		_ = r.Header.Get("Traceparent")

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "OK")
	}))
	defer server.Close()

	// Create client with middleware
	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithSecurityDefaults(),
		)(http.DefaultTransport),
	}

	// Make multiple requests
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL + "/api/test")
		require.NoError(t, err)

		_ = resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	assert.Equal(t, 3, requestCount)
}

func TestStartSpanWithNilDefaultProvider(t *testing.T) {
	// Test StartSpan when defaultProvider might be initialized differently
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-operation")

	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

// Test that handles logger write errors gracefully
func TestLoggerWriteError(_ *testing.T) {
	// Create a writer that always fails
	failWriter := &failingWriter{}

	logger := NewLogger(DebugLevel, failWriter, nil)
	// Should not panic even if write fails
	logger.Info("test message")
}

type failingWriter struct{}

func (*failingWriter) Write(_ []byte) (n int, err error) {
	return 0, errors.New("write failed")
}

// Test logger JSON marshal error handling (difficult to trigger normally)
func TestLoggerWithNilFields(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	// Ensure nil fields map doesn't cause issues
	logger.fields = nil
	withLogger := logger.With(map[string]any{"key": "value"})

	assert.NotNil(t, withLogger)
}

func TestHTTPMiddlewareWithOptions(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Test with multiple ignore headers (should merge)
	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithIgnoreHeaders("x-custom-1"),
			WithIgnoreHeaders("x-custom-2"),
		)(http.DefaultTransport),
	}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHTTPMiddlewareIgnorePathPrefix(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(
			provider,
			WithIgnorePaths("/health"),
		)(http.DefaultTransport),
	}

	// Test path that starts with ignored prefix
	resp, err := client.Get(server.URL + "/health/live")
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProviderTracerAndMeterWhenEnabled(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, true),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Get tracer
	tracer := provider.Tracer()
	assert.NotNil(t, tracer)

	// Create a span
	ctx, span := tracer.Start(context.Background(), "test")
	span.End()

	_ = ctx

	// Get meter
	meter := provider.Meter()
	assert.NotNil(t, meter)
}

func TestHTTPMiddleware4xxStatusCode(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "notfound") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if strings.Contains(r.URL.Path, "badrequest") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
	}

	// Test 404
	resp, err := client.Get(server.URL + "/notfound")
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Test 400
	resp, err = client.Get(server.URL + "/badrequest")
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// =============================================================================
// Additional Coverage Tests
// =============================================================================

func TestNoopLoggerAllMethods(_ *testing.T) {
	logger := NewNoopLogger()

	// Test all noop methods explicitly
	logger.Info("info")
	logger.Infof("info %s", "formatted")
	logger.Warn("warn")
	logger.Warnf("warn %s", "formatted")
	logger.Error("error")
	logger.Errorf("error %s", "formatted")
	logger.Fatal("fatal")
	logger.Fatalf("fatal %s", "formatted")
}

func TestWithDevelopmentDefaultsErrors(t *testing.T) {
	// Test that development defaults apply correctly
	config := DefaultConfig()
	err := WithDevelopmentDefaults()(config)
	require.NoError(t, err)
	assert.Equal(t, "development", config.Environment)
	assert.Equal(t, DebugLevel, config.LogLevel)
	assert.InDelta(t, 0.5, config.TraceSampleRate, 0.001)
}

func TestWithProductionDefaultsErrors(t *testing.T) {
	// Test that production defaults apply correctly
	config := DefaultConfig()
	err := WithProductionDefaults()(config)
	require.NoError(t, err)
	assert.Equal(t, "production", config.Environment)
	assert.Equal(t, InfoLevel, config.LogLevel)
	assert.InDelta(t, 0.1, config.TraceSampleRate, 0.001)
}

func TestMeterProviderNilCase(t *testing.T) {
	// Create provider without meter initialization (no collector endpoint)
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Meter should return a valid meter even if meter provider is not initialized
	meter := provider.Meter()
	assert.NotNil(t, meter)
}

func TestRecordMetricWithError(t *testing.T) {
	// Create provider with metrics enabled
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, true),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx := context.Background()

	// Record metric - this exercises the success path
	RecordMetric(ctx, provider, "test.counter", 1.0,
		attribute.String("key", "value"))
}

func TestRecordDurationWithError(t *testing.T) {
	// Create provider with metrics enabled
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, true),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	ctx := context.Background()

	// Record duration - this exercises the success path
	RecordDuration(ctx, provider, "test.histogram", time.Now().Add(-100*time.Millisecond),
		attribute.String("key", "value"))
}

func TestShutdownWithErrors(t *testing.T) {
	// Create provider with all components
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, true),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	// First shutdown should work
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestLoggerWithSampledContext(t *testing.T) {
	var buf bytes.Buffer

	logger := NewLogger(DebugLevel, &buf, nil).(*LoggerImpl)

	provider, err := New(context.Background(),
		WithComponentEnabled(true, false, false),
		WithFullTracingSampling(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	_, span := provider.Tracer().Start(context.Background(), "test")
	defer span.End()

	spanCtx := span.SpanContext()

	// Test with sampled span
	if spanCtx.IsSampled() {
		contextLogger := logger.WithContext(spanCtx)
		contextLogger.Info("sampled message")

		output := buf.String()
		assert.Contains(t, output, "sampled")
	}
}

func TestRecordSpanMetricWithoutDefaultProvider(_ *testing.T) {
	ctx := context.Background()
	// Should not panic when defaultProvider might be nil or disabled
	RecordSpanMetric(ctx, "test.metric", 1.0)
}

func TestHTTPMiddlewareMetricsRecording(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &http.Client{
		Transport: NewHTTPMiddleware(provider)(http.DefaultTransport),
	}

	// Make request to trigger metrics recording
	resp, err := client.Get(server.URL + "/test")
	require.NoError(t, err)

	_ = resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestStartSpanWithDefaultProvider(t *testing.T) {
	ctx := context.Background()

	// Use StartSpan which uses defaultProvider
	newCtx, span := StartSpan(ctx, "test-span")
	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

func TestMetricsCollectorEnabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, true),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	collector, err := NewMetricsCollector(provider)
	require.NoError(t, err)
	assert.NotNil(t, collector)

	ctx := context.Background()

	// Test all metric recording methods
	collector.RecordRequest(ctx, "test.op", "resource", 200, 10*time.Millisecond)
	collector.RecordRequest(ctx, "test.op", "resource", 500, 10*time.Millisecond) // Error case
	collector.RecordBatchRequest(ctx, "test.batch", "resource", 5, 20*time.Millisecond)
	collector.RecordRetry(ctx, "test.retry", "resource", 1)
}

func TestHTTPMiddlewareWithNilResponse(t *testing.T) {
	provider, err := New(context.Background(),
		WithComponentEnabled(true, true, false),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	defer func() { _ = provider.Shutdown(context.Background()) }()

	// Create a test middleware instance directly
	m := &httpMiddleware{
		provider:      provider,
		ignoreHeaders: []string{"authorization"},
		maskedParams:  []string{"token"},
	}

	// Test recordRequestMetrics with nil response
	ctx := context.Background()
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	m.recordRequestMetrics(ctx, req, nil, errors.New("connection refused"), 100*time.Millisecond)
}

func TestStartWithDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	// Shutdown to disable
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	ctx := WithProvider(context.Background(), provider)

	// Start should return noop span
	newCtx, span := Start(ctx, "test-span")
	assert.NotNil(t, span)
	assert.NotNil(t, newCtx)
	span.End()
}

func TestLogWithDisabledProvider(t *testing.T) {
	provider, err := New(context.Background(),
		WithRegisterGlobally(false),
	)
	require.NoError(t, err)

	// Shutdown to disable
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)

	ctx := WithProvider(context.Background(), provider)

	// Should return noop logger
	logger := Log(ctx)
	assert.NotNil(t, logger)
	logger.Info("test")
}
