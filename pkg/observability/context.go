package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// ProviderKey is the context key for the observability provider
	ProviderKey ContextKey = "midaz-observability-provider"
)

// WithProvider returns a new context with the provider added
func WithProvider(ctx context.Context, provider Provider) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, ProviderKey, provider)
}

// GetProvider returns the provider from the context, if any
func GetProvider(ctx context.Context) Provider {
	if ctx == nil {
		return nil
	}

	if provider, ok := ctx.Value(ProviderKey).(Provider); ok {
		return provider
	}

	return nil
}

// WithSpanAttributes returns a new context with attributes added to the current span
func WithSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}

	return ctx
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	if ctx == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if ctx == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// WithBaggageItem returns a new context with a baggage item added
func WithBaggageItem(ctx context.Context, key, value string) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	item, err := baggage.NewMember(key, value)
	if err != nil {
		return ctx, err
	}

	currentBaggage := baggage.FromContext(ctx)
	if currentBaggage.Len() == 0 {
		newBaggage, err := baggage.New(item)
		if err != nil {
			return ctx, err
		}

		return baggage.ContextWithBaggage(ctx, newBaggage), nil
	}

	newBaggage, err := currentBaggage.SetMember(item)
	if err != nil {
		return ctx, err
	}

	return baggage.ContextWithBaggage(ctx, newBaggage), nil
}

// GetBaggageItem returns a baggage item from the context
func GetBaggageItem(ctx context.Context, key string) string {
	if ctx == nil {
		return ""
	}

	currentBaggage := baggage.FromContext(ctx)
	member := currentBaggage.Member(key)

	if member.Key() != "" {
		return member.Value()
	}

	return ""
}

// Start starts a new span from a context
func Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if ctx == nil {
		ctx = context.Background()
	}

	provider := GetProvider(ctx)
	if provider != nil && provider.IsEnabled() {
		return provider.Tracer().Start(ctx, name, opts...)
	}

	return noop.NewTracerProvider().Tracer("").Start(ctx, name, opts...)
}

// Log returns a logger from the context
func Log(ctx context.Context) Logger {
	if ctx == nil {
		return NewNoopLogger()
	}

	provider := GetProvider(ctx)
	if provider != nil && provider.IsEnabled() {
		span := trace.SpanFromContext(ctx)
		return provider.Logger().WithSpan(span)
	}

	return NewNoopLogger()
}

// TraceID returns the trace ID from the context, if available
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if traceID, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return traceID
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}

	return ""
}

// SpanID returns the span ID from the context, if available
func SpanID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		return spanCtx.SpanID().String()
	}

	return ""
}
