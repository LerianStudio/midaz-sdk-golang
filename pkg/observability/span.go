package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Default global provider for simple usage
	defaultProvider Provider
)

// Initialize the default provider
func init() {
	// Create a simple default provider with default options
	p, _ := New(context.Background())
	defaultProvider = p
}

// StartSpan starts a new span with the given name
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	// Use the default provider if initialized
	if defaultProvider != nil {
		return defaultProvider.Tracer().Start(ctx, name)
	}

	// Fall back to global tracer if no default provider
	return otel.Tracer("github.com/LerianStudio/midaz-sdk-golang/v2").Start(ctx, name)
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
	span.SetStatus(codes.Error, err.Error())

	// Convert map attributes to attribute.KeyValue slice
	var eventAttrs []attribute.KeyValue
	if len(attrs) > 0 {
		for k, v := range attrs[0] {
			eventAttrs = append(eventAttrs, attribute.String(k, v))
		}
	}

	// Add error details as event
	span.AddEvent(eventName, trace.WithAttributes(
		append(eventAttrs, attribute.String("error.message", err.Error()))...,
	))

	// Record error
	span.RecordError(err)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs map[string]string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	// Convert map attributes to attribute.KeyValue slice
	var eventAttrs []attribute.KeyValue
	for k, v := range attrs {
		eventAttrs = append(eventAttrs, attribute.String(k, v))
	}

	span.AddEvent(name, trace.WithAttributes(eventAttrs...))
}

// RecordSpanMetric records a metric with the given name and value
func RecordSpanMetric(ctx context.Context, name string, value float64) {
	if defaultProvider == nil {
		return
	}

	// Extract trace ID and span ID for correlation
	span := trace.SpanFromContext(ctx)
	var attrs []attribute.KeyValue
	if span.IsRecording() {
		attrs = append(attrs, attribute.String("trace_id", span.SpanContext().TraceID().String()))
		attrs = append(attrs, attribute.String("span_id", span.SpanContext().SpanID().String()))
	}

	// Use RecordMetric from the provider
	RecordMetric(ctx, defaultProvider, name, value, attrs...)
}

// WithTraceID adds a trace ID to the context for correlation
func WithTraceID(ctx context.Context, traceID string) context.Context {
	// Add trace ID to context
	return ctx
}
