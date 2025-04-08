// Package retry provides utilities for implementing retry logic with exponential backoff,
// including specialized support for HTTP requests.
package retry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPResponse represents the result of an HTTP request.
type HTTPResponse struct {
	// Response is the HTTP response.
	Response *http.Response

	// Body is the response body as bytes.
	Body []byte

	// Error is any error that occurred during the request.
	Error error

	// Attempt is the attempt number (0 for first attempt, etc.).
	Attempt int
}

// HTTPOptions configures HTTP retry behavior.
type HTTPOptions struct {
	// MaxRetries is the maximum number of retries to attempt.
	MaxRetries int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// BackoffFactor is the factor by which to increase the delay after each retry.
	BackoffFactor float64

	// RetryableHTTPCodes is a list of HTTP status codes that should trigger a retry.
	RetryableHTTPCodes []int

	// RetryableNetworkErrors is a list of error strings that should trigger a retry.
	RetryableNetworkErrors []string

	// RetryAllServerErrors determines whether to retry all 5xx errors.
	RetryAllServerErrors bool

	// RetryOn4xx is a list of 4xx status codes that should trigger a retry.
	RetryOn4xx []int

	// PreRetryHook is called before each retry attempt.
	PreRetryHook func(ctx context.Context, req *http.Request, resp *HTTPResponse) error

	// JitterFactor is the amount of jitter to add to the delay (0.0-1.0)
	JitterFactor float64
}

// DefaultHTTPOptions returns the default HTTP retry options.
func DefaultHTTPOptions() *HTTPOptions {
	return &HTTPOptions{
		MaxRetries:             3,
		InitialDelay:           100 * time.Millisecond,
		MaxDelay:               10 * time.Second,
		BackoffFactor:          2.0,
		RetryableHTTPCodes:     DefaultRetryableHTTPCodes,
		RetryableNetworkErrors: DefaultRetryableErrors,
		RetryAllServerErrors:   true,
		RetryOn4xx:             []int{429}, // Too Many Requests
		JitterFactor:           0.25,
	}
}

// HTTPOption is a function that configures an HTTPOptions object
type HTTPOption func(*HTTPOptions) error

// WithHTTPMaxRetries returns an HTTPOption that sets the maximum number of retry attempts.
// The value must be non-negative.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPMaxRetries(5))
func WithHTTPMaxRetries(maxRetries int) HTTPOption {
	return func(o *HTTPOptions) error {
		if maxRetries < 0 {
			return fmt.Errorf("maxRetries must be non-negative, got %d", maxRetries)
		}
		o.MaxRetries = maxRetries
		return nil
	}
}

// WithHTTPInitialDelay returns an HTTPOption that sets the initial delay before the first retry.
// The value must be positive.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPInitialDelay(200*time.Millisecond))
func WithHTTPInitialDelay(delay time.Duration) HTTPOption {
	return func(o *HTTPOptions) error {
		if delay <= 0 {
			return fmt.Errorf("initialDelay must be positive, got %v", delay)
		}
		o.InitialDelay = delay
		return nil
	}
}

// WithHTTPMaxDelay returns an HTTPOption that sets the maximum delay between retries.
// The value must be greater than or equal to the initial delay.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPMaxDelay(30*time.Second))
func WithHTTPMaxDelay(delay time.Duration) HTTPOption {
	return func(o *HTTPOptions) error {
		if delay <= 0 {
			return fmt.Errorf("maxDelay must be positive, got %v", delay)
		}
		o.MaxDelay = delay
		return nil
	}
}

// WithHTTPBackoffFactor returns an HTTPOption that sets the factor by which to increase
// the delay after each retry. The value must be greater than or equal to 1.0.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPBackoffFactor(1.5))
func WithHTTPBackoffFactor(factor float64) HTTPOption {
	return func(o *HTTPOptions) error {
		if factor < 1.0 {
			return fmt.Errorf("backoffFactor must be at least 1.0, got %f", factor)
		}
		o.BackoffFactor = factor
		return nil
	}
}

// WithHTTPRetryableHTTPCodes returns an HTTPOption that sets the list of HTTP status
// codes that should trigger a retry.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPRetryableHTTPCodes([]int{
//	    http.StatusTooManyRequests,
//	    http.StatusServiceUnavailable,
//	}))
func WithHTTPRetryableHTTPCodes(codes []int) HTTPOption {
	return func(o *HTTPOptions) error {
		o.RetryableHTTPCodes = codes
		return nil
	}
}

// WithHTTPRetryableNetworkErrors returns an HTTPOption that sets the list of error strings
// that should trigger a retry.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPRetryableNetworkErrors([]string{
//	    "connection refused",
//	    "timeout",
//	}))
func WithHTTPRetryableNetworkErrors(errors []string) HTTPOption {
	return func(o *HTTPOptions) error {
		o.RetryableNetworkErrors = errors
		return nil
	}
}

// WithHTTPRetryAllServerErrors returns an HTTPOption that determines whether to
// retry all 5xx errors.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPRetryAllServerErrors(true))
func WithHTTPRetryAllServerErrors(retry bool) HTTPOption {
	return func(o *HTTPOptions) error {
		o.RetryAllServerErrors = retry
		return nil
	}
}

// WithHTTPRetryOn4xx returns an HTTPOption that sets the list of 4xx status codes
// that should trigger a retry.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPRetryOn4xx([]int{429}))
func WithHTTPRetryOn4xx(codes []int) HTTPOption {
	return func(o *HTTPOptions) error {
		o.RetryOn4xx = codes
		return nil
	}
}

// WithHTTPPreRetryHook returns an HTTPOption that sets a function to be called
// before each retry attempt. The hook can be used to modify the request or
// add additional logging.
//
// The hook function receives:
// - The context
// - The HTTP request about to be retried
// - The HTTPResponse from the previous attempt
//
// If the hook returns an error, the retry is aborted and the error is returned.
//
// Example:
//
//	hook := func(ctx context.Context, req *http.Request, resp *retry.HTTPResponse) error {
//	    log.Printf("Retrying request to %s after attempt %d with status %d",
//	        req.URL, resp.Attempt, resp.Response.StatusCode)
//	    return nil
//	}
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPPreRetryHook(hook))
func WithHTTPPreRetryHook(hook func(ctx context.Context, req *http.Request, resp *HTTPResponse) error) HTTPOption {
	return func(o *HTTPOptions) error {
		o.PreRetryHook = hook
		return nil
	}
}

// WithHTTPJitterFactor returns an HTTPOption that sets the amount of jitter to add to the
// delay to avoid thundering herd problems. The value must be between 0.0 and 1.0.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPJitterFactor(0.5))
func WithHTTPJitterFactor(factor float64) HTTPOption {
	return func(o *HTTPOptions) error {
		if factor < 0.0 || factor > 1.0 {
			return fmt.Errorf("jitterFactor must be between 0.0 and 1.0, got %f", factor)
		}
		o.JitterFactor = factor
		return nil
	}
}

// WithHTTPHighReliability returns an HTTPOption that configures HTTP retry options
// for high reliability. This increases timeouts, retry counts, and adds jitter
// for maximum resilience with HTTP requests.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPHighReliability())
func WithHTTPHighReliability() HTTPOption {
	return func(o *HTTPOptions) error {
		o.MaxRetries = 5
		o.InitialDelay = 200 * time.Millisecond
		o.MaxDelay = 30 * time.Second
		o.BackoffFactor = 2.5
		o.JitterFactor = 0.4
		o.RetryAllServerErrors = true
		o.RetryOn4xx = []int{408, 429}
		return nil
	}
}

// WithHTTPNoRetry returns an HTTPOption that disables retries for HTTP requests.
//
// Example:
//
//	resp, err := retry.DoHTTPRequest(ctx, client, req, retry.WithHTTPNoRetry())
func WithHTTPNoRetry() HTTPOption {
	return func(o *HTTPOptions) error {
		o.MaxRetries = 0
		return nil
	}
}

// DoHTTPRequest performs an HTTP request with retries.
// It handles connection errors, HTTP status codes, and reading the response body.
//
// Example:
//
//	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com", nil)
//	resp, err := retry.DoHTTPRequest(ctx, client, req,
//	    retry.WithHTTPMaxRetries(3),
//	    retry.WithHTTPInitialDelay(100*time.Millisecond))
func DoHTTPRequest(ctx context.Context, client *http.Client, req *http.Request, opts ...HTTPOption) (*HTTPResponse, error) {
	// Start with default options
	options := DefaultHTTPOptions()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, fmt.Errorf("failed to apply HTTP retry option: %w", err)
		}
	}

	return doHTTPRequestWithOptions(ctx, client, req, options)
}

// DoHTTPRequestWithContext performs an HTTP request with retries using options from the context.
// If no options are set in the context, it uses the default options.
//
// Example:
//
//	// Set options in the context
//	ctx = retry.WithHTTPOptionsContext(ctx, retry.DefaultHTTPOptions())
//
//	// Later, use the options from the context
//	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com", nil)
//	resp, err := retry.DoHTTPRequestWithContext(ctx, client, req)
func DoHTTPRequestWithContext(ctx context.Context, client *http.Client, req *http.Request) (*HTTPResponse, error) {
	options := GetHTTPOptionsFromContext(ctx)
	return doHTTPRequestWithOptions(ctx, client, req, options)
}

// doHTTPRequestWithOptions is an internal function that performs an HTTP request with retries.
func doHTTPRequestWithOptions(ctx context.Context, client *http.Client, req *http.Request, options *HTTPOptions) (*HTTPResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}

	var (
		resp           *http.Response
		respBody       []byte
		respErr        error
		statusCode     int
		lastErr        error
		lastStatusCode int
	)

	// Retry loop
	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// Check if context is done before executing
		if ctx.Err() != nil {
			return &HTTPResponse{
				Error:   fmt.Errorf("operation cancelled: %w", ctx.Err()),
				Attempt: attempt,
			}, ctx.Err()
		}

		// Clone the request if this is a retry (since the body may have been consumed)
		if attempt > 0 {
			var reqClone *http.Request
			if req.Body != nil && req.GetBody != nil {
				reqBody, err := req.GetBody()
				if err != nil {
					return &HTTPResponse{
						Error:   fmt.Errorf("failed to clone request body: %w", err),
						Attempt: attempt,
					}, err
				}
				reqClone, err = http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), reqBody)
				if err != nil {
					return &HTTPResponse{
						Error:   fmt.Errorf("failed to clone request: %w", err),
						Attempt: attempt,
					}, err
				}
				// Copy headers
				for key, values := range req.Header {
					for _, value := range values {
						reqClone.Header.Add(key, value)
					}
				}
			} else {
				// Simple clone for requests without bodies
				var err error
				reqClone, err = http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), nil)
				if err != nil {
					return &HTTPResponse{
						Error:   fmt.Errorf("failed to clone request: %w", err),
						Attempt: attempt,
					}, err
				}
				// Copy headers
				for key, values := range req.Header {
					for _, value := range values {
						reqClone.Header.Add(key, value)
					}
				}
			}
			req = reqClone
		}

		// Execute the request
		resp, respErr = client.Do(req)

		// Create the response structure
		httpResp := &HTTPResponse{
			Response: resp,
			Error:    respErr,
			Attempt:  attempt,
		}

		// Call the pre-retry hook if provided
		if attempt < options.MaxRetries && options.PreRetryHook != nil {
			if hookErr := options.PreRetryHook(ctx, req, httpResp); hookErr != nil {
				// If the hook returns an error, don't retry
				return httpResp, hookErr
			}
		}

		// Handle connection errors
		if respErr != nil {
			lastErr = respErr
			// Check if the error is retryable
			if isNetworkErrorRetryable(respErr, options) {
				// Wait for backoff duration or until context is cancelled
				if attempt < options.MaxRetries {
					delay := calculateBackoff(attempt, &Options{
						InitialDelay:  options.InitialDelay,
						MaxDelay:      options.MaxDelay,
						BackoffFactor: options.BackoffFactor,
					})
					delay = addJitter(delay, options.JitterFactor)

					select {
					case <-ctx.Done():
						return httpResp, fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
					case <-time.After(delay):
						// Continue with retry
						continue
					}
				}
			}

			// Not retryable or no more retries
			return httpResp, fmt.Errorf("HTTP request failed: %w", respErr)
		}

		// At this point we have a response
		statusCode = resp.StatusCode
		lastStatusCode = statusCode

		// Read the response body (even for error responses)
		if resp.Body != nil {
			var readErr error
			respBody, readErr = io.ReadAll(resp.Body)
			resp.Body.Close() // Always close the body

			if readErr != nil {
				httpResp.Error = readErr
				return httpResp, fmt.Errorf("failed to read response body: %w", readErr)
			}

			httpResp.Body = respBody
		}

		// Check if the status code indicates success
		if statusCode < 400 {
			return httpResp, nil
		}

		// Check if the status code is retryable and we have retries left
		if isStatusCodeRetryable(statusCode, options) && attempt < options.MaxRetries {
			// Wait for backoff duration or until context is cancelled
			delay := calculateBackoff(attempt, &Options{
				InitialDelay:  options.InitialDelay,
				MaxDelay:      options.MaxDelay,
				BackoffFactor: options.BackoffFactor,
			})
			delay = addJitter(delay, options.JitterFactor)

			select {
			case <-ctx.Done():
				return httpResp, fmt.Errorf("operation cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue with retry
				continue
			}
		}

		// Not retryable or no more retries
		httpResp.Error = fmt.Errorf("HTTP request failed with status %d", statusCode)
		return httpResp, httpResp.Error
	}

	// If we got here, all retries failed
	return &HTTPResponse{
		Response: resp,
		Body:     respBody,
		Error:    fmt.Errorf("operation failed after %d retries, last status: %d, last error: %v", options.MaxRetries, lastStatusCode, lastErr),
		Attempt:  options.MaxRetries,
	}, fmt.Errorf("operation failed after %d retries", options.MaxRetries)
}

// DoHTTP is a simpler version of DoHTTPRequest that handles creating the request.
//
// Example:
//
//	resp, err := retry.DoHTTP(ctx, client, "GET", "https://api.example.com", nil,
//	    retry.WithHTTPMaxRetries(3),
//	    retry.WithHTTPInitialDelay(100*time.Millisecond))
func DoHTTP(ctx context.Context, client *http.Client, method, url string, body io.Reader, opts ...HTTPOption) (*HTTPResponse, error) {
	// Create the request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return DoHTTPRequest(ctx, client, req, opts...)
}

// isStatusCodeRetryable checks if a status code should trigger a retry.
func isStatusCodeRetryable(statusCode int, options *HTTPOptions) bool {
	// Check for 5xx errors if RetryAllServerErrors is true
	if options.RetryAllServerErrors && statusCode >= 500 && statusCode < 600 {
		return true
	}

	// Check the specific retryable HTTP codes
	for _, code := range options.RetryableHTTPCodes {
		if statusCode == code {
			return true
		}
	}

	// Check for specific 4xx errors that should be retried
	for _, code := range options.RetryOn4xx {
		if statusCode == code {
			return true
		}
	}

	return false
}

// isNetworkErrorRetryable checks if a network error should trigger a retry.
func isNetworkErrorRetryable(err error, options *HTTPOptions) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Check for retryable error strings
	errMsg := err.Error()
	for _, retryableErr := range options.RetryableNetworkErrors {
		if retryableErr != "" && errMatchesPattern(errMsg, retryableErr) {
			return true
		}
	}

	return false
}

// WithHTTPOptionsContext returns a new context with the HTTP retry options set.
func WithHTTPOptionsContext(ctx context.Context, options *HTTPOptions) context.Context {
	return context.WithValue(ctx, contextKey("http-retry-options"), options)
}

// GetHTTPOptionsFromContext gets the HTTP retry options from the context.
func GetHTTPOptionsFromContext(ctx context.Context) *HTTPOptions {
	if options, ok := ctx.Value(contextKey("http-retry-options")).(*HTTPOptions); ok {
		return options
	}
	return DefaultHTTPOptions()
}
