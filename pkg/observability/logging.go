package observability

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// DebugLevel is the most verbose logging level, used for debugging information
	DebugLevel LogLevel = iota
	// InfoLevel is for general information about normal operation
	InfoLevel
	// WarnLevel is for potentially problematic situations that don't cause errors
	WarnLevel
	// ErrorLevel is for errors that may still allow the application to continue
	ErrorLevel
	// FatalLevel is for severe errors that will likely cause the application to terminate
	FatalLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger is the interface for all logging operations
type Logger interface {
	// Debug logs a message at debug level
	Debug(args ...any)
	// Debugf logs a formatted message at debug level
	Debugf(format string, args ...any)
	// Info logs a message at info level
	Info(args ...any)
	// Infof logs a formatted message at info level
	Infof(format string, args ...any)
	// Warn logs a message at warn level
	Warn(args ...any)
	// Warnf logs a formatted message at warn level
	Warnf(format string, args ...any)
	// Error logs a message at error level
	Error(args ...any)
	// Errorf logs a formatted message at error level
	Errorf(format string, args ...any)
	// Fatal logs a message at fatal level
	Fatal(args ...any)
	// Fatalf logs a formatted message at fatal level
	Fatalf(format string, args ...any)
	// With returns a logger with added structured fields
	With(fields map[string]any) Logger
	// WithContext returns a logger with context information (trace ID, etc.)
	WithContext(ctx trace.SpanContext) Logger
	// WithSpan returns a logger with span information (span ID, trace ID, etc.)
	WithSpan(span trace.Span) Logger
}

// LoggerImpl is the standard implementation of the Logger interface
type LoggerImpl struct {
	level  LogLevel
	output io.Writer
	fields map[string]any
}

// NewLogger creates a new logger with the specified level and output
func NewLogger(level LogLevel, output io.Writer, resource *sdkresource.Resource) Logger {
	if output == nil {
		output = os.Stderr
	}

	fields := make(map[string]any)

	if resource != nil {
		// Add resource attributes to fields
		for _, kv := range resource.Attributes() {
			fields[string(kv.Key)] = kv.Value.AsString()
		}
	}

	return &LoggerImpl{
		level:  level,
		output: output,
		fields: fields,
	}
}

// log logs a message at the specified level
func (l *LoggerImpl) log(level LogLevel, msg string) {
	if level < l.level {
		return
	}

	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}
	// Extract just the filename, not the full path
	if lastSlash := strings.LastIndex(file, "/"); lastSlash >= 0 {
		file = file[lastSlash+1:]
	}

	// Create log entry
	entry := map[string]any{
		"timestamp": time.Now().Format(time.RFC3339),
		"level":     level.String(),
		"message":   msg,
		"caller":    fmt.Sprintf("%s:%d", file, line),
	}

	// Add fields
	for k, v := range l.fields {
		entry[k] = v
	}

	// Encode as JSON
	b, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	// Write to output
	b = append(b, '\n')

	_, err = l.output.Write(b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log entry: %v\n", err)
	}

	// If fatal, exit the program
	if level == FatalLevel {
		os.Exit(1)
	}
}

// Debug logs a message at debug level
func (l *LoggerImpl) Debug(args ...any) {
	l.log(DebugLevel, fmt.Sprint(args...))
}

// Debugf logs a formatted message at debug level
func (l *LoggerImpl) Debugf(format string, args ...any) {
	l.log(DebugLevel, fmt.Sprintf(format, args...))
}

// Info logs a message at info level
func (l *LoggerImpl) Info(args ...any) {
	l.log(InfoLevel, fmt.Sprint(args...))
}

// Infof logs a formatted message at info level
func (l *LoggerImpl) Infof(format string, args ...any) {
	l.log(InfoLevel, fmt.Sprintf(format, args...))
}

// Warn logs a message at warn level
func (l *LoggerImpl) Warn(args ...any) {
	l.log(WarnLevel, fmt.Sprint(args...))
}

// Warnf logs a formatted message at warn level
func (l *LoggerImpl) Warnf(format string, args ...any) {
	l.log(WarnLevel, fmt.Sprintf(format, args...))
}

// Error logs a message at error level
func (l *LoggerImpl) Error(args ...any) {
	l.log(ErrorLevel, fmt.Sprint(args...))
}

// Errorf logs a formatted message at error level
func (l *LoggerImpl) Errorf(format string, args ...any) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...))
}

// Fatal logs a message at fatal level
func (l *LoggerImpl) Fatal(args ...any) {
	l.log(FatalLevel, fmt.Sprint(args...))
}

// Fatalf logs a formatted message at fatal level
func (l *LoggerImpl) Fatalf(format string, args ...any) {
	l.log(FatalLevel, fmt.Sprintf(format, args...))
}

// With returns a logger with added structured fields
func (l *LoggerImpl) With(fields map[string]any) Logger {
	// Create a new map with existing fields
	newFields := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	// Add new fields, overwriting existing ones if needed
	for k, v := range fields {
		newFields[k] = v
	}

	return &LoggerImpl{
		level:  l.level,
		output: l.output,
		fields: newFields,
	}
}

// WithContext returns a logger with context information (trace ID, etc.)
func (l *LoggerImpl) WithContext(ctx trace.SpanContext) Logger {
	if !ctx.IsValid() {
		return l
	}

	fields := map[string]any{
		"trace_id": ctx.TraceID().String(),
	}

	if ctx.HasSpanID() {
		fields["span_id"] = ctx.SpanID().String()
	}

	if ctx.IsSampled() {
		fields["sampled"] = true
	}

	return l.With(fields)
}

// WithSpan returns a logger with span information (span ID, trace ID, etc.)
func (l *LoggerImpl) WithSpan(span trace.Span) Logger {
	if span == nil {
		return l
	}

	return l.WithContext(span.SpanContext())
}

// NoopLogger is a no-op implementation of the Logger interface
type NoopLogger struct{}

// NewNoopLogger creates a new no-op logger
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

// Debug is a no-op
func (l *NoopLogger) Debug(args ...any) {}

// Debugf is a no-op
func (l *NoopLogger) Debugf(format string, args ...any) {}

// Info is a no-op
func (l *NoopLogger) Info(args ...any) {}

// Infof is a no-op
func (l *NoopLogger) Infof(format string, args ...any) {}

// Warn is a no-op
func (l *NoopLogger) Warn(args ...any) {}

// Warnf is a no-op
func (l *NoopLogger) Warnf(format string, args ...any) {}

// Error is a no-op
func (l *NoopLogger) Error(args ...any) {}

// Errorf is a no-op
func (l *NoopLogger) Errorf(format string, args ...any) {}

// Fatal is a no-op
func (l *NoopLogger) Fatal(args ...any) {}

// Fatalf is a no-op
func (l *NoopLogger) Fatalf(format string, args ...any) {}

// With returns the same no-op logger
func (l *NoopLogger) With(fields map[string]any) Logger {
	return l
}

// WithContext returns the same no-op logger
func (l *NoopLogger) WithContext(ctx trace.SpanContext) Logger {
	return l
}

// WithSpan returns the same no-op logger
func (l *NoopLogger) WithSpan(span trace.Span) Logger {
	return l
}
