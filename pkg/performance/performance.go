// Package performance provides utilities for optimizing the performance of the Midaz SDK.
//
// This package implements various performance optimizations, including:
// - Configurable batch processing for efficiently handling bulk operations
// - HTTP connection pooling to reduce connection overhead
// - JSON processing optimizations to minimize serialization costs
//
// Use Cases:
//
//  1. High-Volume Transaction Processing:
//     When processing thousands of transactions per second, use batch processing
//     to reduce API call overhead and connection pooling to maintain efficiency:
//
//     ```go
//     // Configure for high-volume transaction processing
//     options := performance.Options{
//     BatchSize: 100,            // Process 100 transactions at once
//     EnableHTTPPooling: true,   // Reuse connections
//     MaxIdleConnsPerHost: 20,   // Keep more idle connections
//     }
//     performance.ApplyGlobalPerformanceOptions(options)
//     ```
//
//  2. Memory-Constrained Environments:
//     When running in environments with limited memory (e.g., serverless functions),
//     tune settings to minimize memory usage:
//
//     ```go
//     // Configure for memory-efficient processing
//     options := performance.Options{
//     BatchSize: 25,             // Smaller batches to reduce memory spikes
//     EnableHTTPPooling: true,   // Reuse connections
//     MaxIdleConnsPerHost: 5,    // Limit idle connections
//     }
//     performance.ApplyGlobalPerformanceOptions(options)
//     ```
//
//  3. Processing Large Data Volumes:
//     When working with large datasets, use optimal batch sizing to increase throughput:
//
//     ```go
//     // Calculate ideal batch size for a large export operation
//     totalRecords := 10000
//     // Find optimal batch size that divides records evenly
//     batchSize := performance.GetOptimalBatchSize(totalRecords, 200)
//     ```
package performance

import (
	"fmt"
)

// Options represents global performance configuration for the SDK
type Options struct {
	// BatchSize is the default batch size to use for batch operations.
	// Larger values reduce API call overhead but increase memory usage.
	// Recommended range: 10-200, depending on record size and API limits.
	BatchSize int

	// EnableHTTPPooling enables connection pooling for HTTP clients.
	// Set to true for long-running services making repeated API calls.
	// Set to false for short-lived processes or serverless functions.
	EnableHTTPPooling bool

	// MaxIdleConnsPerHost is the maximum number of idle connections to keep per host.
	// Higher values reduce connection setup time for high QPS applications.
	// Lower values reduce memory usage for infrequent API calls.
	MaxIdleConnsPerHost int

	// UseJSONIterator enables the use of jsoniter for faster JSON parsing.
	// Set to true for applications with high JSON parsing overhead.
	// May slightly increase binary size due to additional dependency.
	UseJSONIterator bool
}

// Option defines a function that configures performance options
type Option func(*Options) error

// WithBatchSize sets the default batch size for batch operations
//
// Example use case: When processing large numbers of account updates,
// increasing batch size reduces the number of API calls required:
//
//	options, _ := performance.NewOptions(performance.WithBatchSize(100))
//	// Now 100 account updates can be processed in a single API call
func WithBatchSize(size int) Option {
	return func(o *Options) error {
		if size <= 0 {
			return fmt.Errorf("batch size must be greater than 0, got %d", size)
		}

		o.BatchSize = size

		return nil
	}
}

// WithHTTPPooling enables or disables HTTP connection pooling
func WithHTTPPooling(enabled bool) Option {
	return func(o *Options) error {
		o.EnableHTTPPooling = enabled

		return nil
	}
}

// WithMaxIdleConnsPerHost sets the maximum number of idle connections per host
func WithMaxIdleConnsPerHost(maxIdle int) Option {
	return func(o *Options) error {
		if maxIdle < 0 {
			return fmt.Errorf("max idle connections per host must be non-negative, got %d", maxIdle)
		}

		o.MaxIdleConnsPerHost = maxIdle

		return nil
	}
}

// WithJSONIterator enables or disables the use of jsoniter for JSON parsing
func WithJSONIterator(enabled bool) Option {
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
func NewOptions(opts ...Option) (*Options, error) {
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
func ApplyGlobalOptions(opts ...Option) error {
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

// GetOptimalBatchSize calculates an optimal batch size based on the total count and maximum batch size.
//
// This function is particularly useful for data export/import operations where processing
// in evenly-sized batches improves progress tracking and error handling.
//
// Example use case: When exporting 10,000 transaction records with a max batch size of 200:
//
//	totalRecords := 10000
//	batchSize := performance.GetOptimalBatchSize(totalRecords, 200)
//	// If 200 divides 10000 evenly, batchSize will be 200
//	// Otherwise, it finds the largest divisor of 10000 that is <= 200
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
