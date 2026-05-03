package observability

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Default global provider for simple usage
var defaultProvider Provider

type traceIDContextKey struct{}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}

	if defaultProvider == nil {
		provider, err := New(ctx,
			WithComponentEnabled(true, false, false),
			WithFullTracingSampling(),
			WithRegisterGlobally(false),
		)
		if err == nil {
			defaultProvider = provider
		}
	}

	// Use the default provider if initialized
	if defaultProvider != nil && defaultProvider.IsEnabled() {
		return defaultProvider.Tracer().Start(ctx, name)
	}

	return noop.NewTracerProvider().Tracer("github.com/LerianStudio/midaz-sdk-golang/v2").Start(ctx, name)
}

// AddAttribute adds an attribute to the current span in the context
func AddAttribute(ctx context.Context, key string, value any) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	// Convert the value to the appropriate attribute type
	var attr attribute.KeyValue

	switch v := value.(type) {
	case string:
		attr = attribute.String(key, v)
	case int:
		attr = attribute.Int(key, v)
	case int64:
		attr = attribute.Int64(key, v)
	case float64:
		attr = attribute.Float64(key, v)
	case bool:
		attr = attribute.Bool(key, v)
	default:
		// For other types, convert to string
		attr = attribute.String(key, fmt.Sprintf("%v", v))
	}

	span.SetAttributes(attr)
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, eventName string, attrs ...map[string]string) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	// Set error status
	sanitizedErr := sanitizeSensitiveString(err.Error())
	span.SetStatus(codes.Error, sanitizedErr)

	// Convert map attributes to attribute.KeyValue slice
	var eventAttrs []attribute.KeyValue

	if len(attrs) > 0 {
		for k, v := range attrs[0] {
			eventAttrs = append(eventAttrs, attribute.String(k, sanitizeSensitiveString(v)))
		}
	}

	// Add error details as event
	span.AddEvent(eventName, trace.WithAttributes(
		append(eventAttrs, attribute.String("error.message", sanitizedErr))...,
	))

	// Record error
	span.RecordError(errors.New(sanitizedErr))
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	// Convert map attributes to attribute.KeyValue slice
	eventAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		eventAttrs = append(eventAttrs, attribute.String(k, sanitizeSensitiveString(v)))
	}

	span.AddEvent(name, trace.WithAttributes(eventAttrs...))
}

// RecordSpanMetric records a metric with the given name and value
func RecordSpanMetric(ctx context.Context, name string, value float64) {
	if defaultProvider == nil {
		return
	}

	// Use RecordMetric from the provider
	RecordMetric(ctx, defaultProvider, name, value)
}

// WithTraceID adds a trace ID to the context for correlation
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if traceID == "" {
		return ctx
	}

	return context.WithValue(ctx, traceIDContextKey{}, traceID)
}
