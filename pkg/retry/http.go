// Package retry provides utilities for implementing retry logic with exponential backoff,
// including specialized support for HTTP requests.
package retry

import (
	"context"
	"errors"
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
func WithHTTPRetryableNetworkErrors(networkErrors []string) HTTPOption {
	return func(o *HTTPOptions) error {
		o.RetryableNetworkErrors = networkErrors
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

// doHTTPRequestWithOptions performs an HTTP request with retries.
func doHTTPRequestWithOptions(ctx context.Context, client *http.Client, req *http.Request, options *HTTPOptions) (*HTTPResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}

	retryState := &httpRetryState{
		ctx:     ctx,
		client:  client,
		options: options,
	}

	return retryState.executeWithRetries(req)
}

// httpRetryState holds state for HTTP retry operations.
type httpRetryState struct {
	ctx            context.Context
	client         *http.Client
	options        *HTTPOptions
	lastErr        error
	lastStatusCode int
	resp           *http.Response
	respBody       []byte
}

// executeWithRetries performs the main retry loop.
func (r *httpRetryState) executeWithRetries(req *http.Request) (*HTTPResponse, error) {
	for attempt := 0; attempt <= r.options.MaxRetries; attempt++ {
		if err := r.checkContextCancellation(attempt); err != nil {
			return r.createErrorResponse(attempt, err), err
		}

		currentReq, err := r.prepareRequest(req, attempt)
		if err != nil {
			return r.createErrorResponse(attempt, err), err
		}

		httpResp, shouldContinue, err := r.executeAttempt(currentReq, attempt)
		if err != nil {
			return httpResp, err
		}

		if !shouldContinue {
			return httpResp, nil
		}
	}

	return r.createFinalErrorResponse(), fmt.Errorf("operation failed after %d retries", r.options.MaxRetries)
}

// checkContextCancellation checks if the context is cancelled.
func (r *httpRetryState) checkContextCancellation(attempt int) error {
	_ = attempt // Parameter reserved for future retry attempt logging

	if r.ctx.Err() != nil {
		return fmt.Errorf("operation cancelled: %w", r.ctx.Err())
	}

	return nil
}

// prepareRequest clones the request for retry attempts.
func (r *httpRetryState) prepareRequest(req *http.Request, attempt int) (*http.Request, error) {
	if attempt == 0 {
		return req, nil
	}

	return r.cloneRequest(req)
}

// cloneRequest creates a clone of the HTTP request for retry.
func (r *httpRetryState) cloneRequest(req *http.Request) (*http.Request, error) {
	if req.Body != nil && req.GetBody != nil {
		return r.cloneRequestWithBody(req)
	}

	return r.cloneRequestWithoutBody(req)
}

// cloneRequestWithBody clones a request that has a body.
func (r *httpRetryState) cloneRequestWithBody(req *http.Request) (*http.Request, error) {
	reqBody, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("failed to clone request body: %w", err)
	}

	reqClone, err := http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to clone request: %w", err)
	}

	r.copyHeaders(req, reqClone)

	return reqClone, nil
}

// cloneRequestWithoutBody clones a request that doesn't have a body.
func (r *httpRetryState) cloneRequestWithoutBody(req *http.Request) (*http.Request, error) {
	reqClone, err := http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to clone request: %w", err)
	}

	r.copyHeaders(req, reqClone)

	return reqClone, nil
}

// copyHeaders copies headers from source to destination request.
func (*httpRetryState) copyHeaders(src, dst *http.Request) {
	for key, values := range src.Header {
		for _, value := range values {
			dst.Header.Add(key, value)
		}
	}
}

// executeAttempt executes a single HTTP request attempt.
func (r *httpRetryState) executeAttempt(req *http.Request, attempt int) (*HTTPResponse, bool, error) {
	resp, respErr := r.client.Do(req)
	r.resp = resp

	// Ensure response body is properly closed according to Go HTTP best practices
	// Use a flag to track if body was consumed by readResponseBody to avoid double closing
	var bodyConsumed bool

	defer func() {
		if !bodyConsumed && resp != nil && resp.Body != nil {
			// Silently close the body - close errors are non-actionable in library code
			_ = resp.Body.Close()
		}
	}()

	httpResp := &HTTPResponse{
		Response: resp,
		Error:    respErr,
		Attempt:  attempt,
	}

	if err := r.callPreRetryHook(req, httpResp, attempt); err != nil {
		return httpResp, false, err
	}

	if respErr != nil {
		return r.handleConnectionError(httpResp, respErr, attempt)
	}

	result, shouldRetry, err := r.handleResponse(httpResp, attempt)
	if result != nil && err == nil && result.Body != nil {
		// Body was consumed by readResponseBody, mark it as such
		bodyConsumed = true
	}

	return result, shouldRetry, err
}

// callPreRetryHook calls the pre-retry hook if configured.
func (r *httpRetryState) callPreRetryHook(req *http.Request, httpResp *HTTPResponse, attempt int) error {
	if attempt < r.options.MaxRetries && r.options.PreRetryHook != nil {
		return r.options.PreRetryHook(r.ctx, req, httpResp)
	}

	return nil
}

// handleConnectionError handles connection errors during HTTP requests.
func (r *httpRetryState) handleConnectionError(httpResp *HTTPResponse, respErr error, attempt int) (*HTTPResponse, bool, error) {
	r.lastErr = respErr

	if !isNetworkErrorRetryable(respErr, r.options) {
		return httpResp, false, fmt.Errorf("HTTP request failed: %w", respErr)
	}

	if attempt >= r.options.MaxRetries {
		return httpResp, false, fmt.Errorf("HTTP request failed: %w", respErr)
	}

	if err := r.waitForRetry(attempt); err != nil {
		return httpResp, false, err
	}

	return nil, true, nil // Continue with retry
}

// handleResponse processes successful HTTP responses.
func (r *httpRetryState) handleResponse(httpResp *HTTPResponse, attempt int) (*HTTPResponse, bool, error) {
	statusCode := r.resp.StatusCode
	r.lastStatusCode = statusCode

	if err := r.readResponseBody(httpResp); err != nil {
		return httpResp, false, err
	}

	if statusCode < 400 {
		return httpResp, false, nil // Success
	}

	return r.handleErrorResponse(httpResp, statusCode, attempt)
}

// readResponseBody reads and stores the response body.
func (r *httpRetryState) readResponseBody(httpResp *HTTPResponse) error {
	if r.resp.Body == nil {
		return nil
	}

	respBody, readErr := io.ReadAll(r.resp.Body)

	// Silently close the body - close errors are non-actionable in library code
	_ = r.resp.Body.Close()

	if readErr != nil {
		httpResp.Error = readErr
		return fmt.Errorf("failed to read response body: %w", readErr)
	}

	r.respBody = respBody
	httpResp.Body = respBody

	return nil
}

// handleErrorResponse handles HTTP error responses.
func (r *httpRetryState) handleErrorResponse(httpResp *HTTPResponse, statusCode, attempt int) (*HTTPResponse, bool, error) {
	if !isStatusCodeRetryable(statusCode, r.options) || attempt >= r.options.MaxRetries {
		httpResp.Error = fmt.Errorf("HTTP request failed with status %d", statusCode)
		return httpResp, false, httpResp.Error
	}

	if err := r.waitForRetry(attempt); err != nil {
		return httpResp, false, err
	}

	return nil, true, nil // Continue with retry
}

// waitForRetry waits for the calculated backoff duration.
func (r *httpRetryState) waitForRetry(attempt int) error {
	delay := calculateBackoff(attempt, &Options{
		InitialDelay:  r.options.InitialDelay,
		MaxDelay:      r.options.MaxDelay,
		BackoffFactor: r.options.BackoffFactor,
	})
	delay = addJitter(delay, r.options.JitterFactor)

	// Use time.NewTimer instead of time.After to allow proper cleanup
	// and avoid potential timer leaks when context is cancelled
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-r.ctx.Done():
		return fmt.Errorf("operation cancelled during retry: %w", r.ctx.Err())
	case <-timer.C:
		return nil
	}
}

// createErrorResponse creates an HTTPResponse for errors.
func (*httpRetryState) createErrorResponse(attempt int, err error) *HTTPResponse {
	return &HTTPResponse{
		Error:   err,
		Attempt: attempt,
	}
}

// createFinalErrorResponse creates the final error response after all retries failed.
func (r *httpRetryState) createFinalErrorResponse() *HTTPResponse {
	return &HTTPResponse{
		Response: r.resp,
		Body:     r.respBody,
		Error:    fmt.Errorf("operation failed after %d retries, last status: %d, last error: %w", r.options.MaxRetries, r.lastStatusCode, r.lastErr),
		Attempt:  r.options.MaxRetries,
	}
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
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
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
