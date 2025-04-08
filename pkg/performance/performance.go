package performance

import (
	"fmt"
)

// Options represents global performance configuration for the SDK
type Options struct {
	// BatchSize is the default batch size to use for batch operations
	BatchSize int

	// EnableHTTPPooling enables connection pooling for HTTP clients
	EnableHTTPPooling bool

	// MaxIdleConnsPerHost is the maximum number of idle connections to keep per host
	MaxIdleConnsPerHost int

	// UseJSONIterator enables the use of jsoniter for faster JSON parsing
	UseJSONIterator bool
}

// PerformanceOption defines a function that configures performance options
type PerformanceOption func(*Options) error

// WithBatchSize sets the default batch size for batch operations
func WithBatchSize(size int) PerformanceOption {
	return func(o *Options) error {
		if size <= 0 {
			return fmt.Errorf("batch size must be greater than 0, got %d", size)
		}
		o.BatchSize = size
		return nil
	}
}

// WithHTTPPooling enables or disables HTTP connection pooling
func WithHTTPPooling(enabled bool) PerformanceOption {
	return func(o *Options) error {
		o.EnableHTTPPooling = enabled
		return nil
	}
}

// WithMaxIdleConnsPerHost sets the maximum number of idle connections per host
func WithMaxIdleConnsPerHost(max int) PerformanceOption {
	return func(o *Options) error {
		if max < 0 {
			return fmt.Errorf("max idle connections per host must be non-negative, got %d", max)
		}
		o.MaxIdleConnsPerHost = max
		return nil
	}
}

// WithJSONIterator enables or disables the use of jsoniter for JSON parsing
func WithJSONIterator(enabled bool) PerformanceOption {
	return func(o *Options) error {
		o.UseJSONIterator = enabled
		return nil
	}
}

// Default options
var defaultOptions = Options{
	BatchSize:           50,
	EnableHTTPPooling:   true,
	MaxIdleConnsPerHost: 10,
	UseJSONIterator:     true,
}

// globalOptions holds the global performance options
var globalOptions = defaultOptions

// DefaultOptions returns a copy of the default performance options
func DefaultOptions() Options {
	return defaultOptions
}

// NewOptions creates a new Options instance with the given options
func NewOptions(opts ...PerformanceOption) (*Options, error) {
	// Start with default options
	options := defaultOptions

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, fmt.Errorf("failed to apply performance option: %w", err)
		}
	}

	return &options, nil
}

// ApplyGlobalPerformanceOptions applies performance options globally
func ApplyGlobalPerformanceOptions(options Options) {
	// Apply non-zero options
	if options.BatchSize > 0 {
		globalOptions.BatchSize = options.BatchSize
	}

	// Apply boolean options explicitly set
	globalOptions.EnableHTTPPooling = options.EnableHTTPPooling

	if options.MaxIdleConnsPerHost > 0 {
		globalOptions.MaxIdleConnsPerHost = options.MaxIdleConnsPerHost
	}

	globalOptions.UseJSONIterator = options.UseJSONIterator
}

// ApplyGlobalOptions applies the given options to the global configuration
func ApplyGlobalOptions(opts ...PerformanceOption) error {
	options, err := NewOptions(opts...)
	if err != nil {
		return err
	}

	ApplyGlobalPerformanceOptions(*options)
	return nil
}

// ApplyBatchingOptions applies options specific to batching operations
func ApplyBatchingOptions(options Options) {
	// Only update batch size if provided
	if options.BatchSize > 0 {
		globalOptions.BatchSize = options.BatchSize
	}
}

// GetGlobalOptions returns the current global performance options
func GetGlobalOptions() Options {
	return globalOptions
}

// GetBatchSize returns the current global batch size
func GetBatchSize() int {
	return globalOptions.BatchSize
}

// GetOptimalBatchSize calculates an optimal batch size based on the total count and maximum batch size
func GetOptimalBatchSize(totalCount, maxBatchSize int) int {
	// If no maximum is provided, use the global batch size
	if maxBatchSize <= 0 {
		return globalOptions.BatchSize
	}

	// If total count is less than the maximum, use the total count
	if totalCount <= maxBatchSize {
		return totalCount
	}

	// Calculate a reasonable batch size that divides the total count evenly
	// Try to find a divisor of totalCount that is close to but not larger than maxBatchSize
	for i := maxBatchSize; i > 1; i-- {
		if totalCount%i == 0 {
			return i
		}
	}

	// If no exact divisor is found, just use the maximum batch size
	return maxBatchSize
}
