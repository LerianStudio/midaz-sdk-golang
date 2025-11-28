// Package concurrent provides utilities for parallel processing and batch operations.
package concurrent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// HTTPBatchRequest represents a single request in a batch.
type HTTPBatchRequest struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, etc.)
	Method string `json:"method"`

	// Path is the relative path for the request, without the base URL
	Path string `json:"path"`

	// Headers are optional HTTP headers for this request
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the request body (for POST, PUT, PATCH)
	Body any `json:"body,omitempty"`

	// ID is a client-generated ID for matching requests with responses
	ID string `json:"id"`
}

// HTTPBatchResponse represents a single response in a batch.
type HTTPBatchResponse struct {
	// StatusCode is the HTTP status code
	StatusCode int `json:"statusCode"`

	// Headers are the response headers
	Headers map[string]string `json:"headers,omitempty"`

	// Body is the response body
	Body json.RawMessage `json:"body,omitempty"`

	// Error is the error message if the request failed
	Error string `json:"error,omitempty"`

	// ID is the client-generated ID from the request
	ID string `json:"id"`
}

// HTTPBatchResult contains the results of a batch operation.
type HTTPBatchResult struct {
	// Responses are the responses for each request
	Responses []HTTPBatchResponse `json:"responses"`

	// Error is the error that occurred during the batch operation
	Error error `json:"-"`
}

// HTTPBatchOptions configures the behavior of HTTP batch requests.
type HTTPBatchOptions struct {
	// Timeout is the maximum time to wait for a batch request to complete
	Timeout time.Duration

	// MaxBatchSize is the maximum number of requests in a single batch
	MaxBatchSize int

	// RetryCount is the number of times to retry a failed batch request
	RetryCount int

	// RetryBackoff is the backoff duration between retries
	RetryBackoff time.Duration

	// ContinueOnError determines whether to continue processing if one request fails
	ContinueOnError bool

	// Workers is the number of concurrent workers for processing multiple batches
	Workers int
}

// HTTPBatchOption is a function that configures an HTTPBatchOptions object.
type HTTPBatchOption func(*HTTPBatchOptions) error

// DefaultHTTPBatchOptions returns the default HTTP batch options.
func DefaultHTTPBatchOptions() *HTTPBatchOptions {
	return &HTTPBatchOptions{
		Timeout:         60 * time.Second,
		MaxBatchSize:    100,
		RetryCount:      3,
		RetryBackoff:    500 * time.Millisecond,
		ContinueOnError: false,
		Workers:         5,
	}
}

// WithBatchTimeout returns an option that sets the timeout for batch operations.
// The value must be positive.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithBatchTimeout(30 * time.Second))
func WithBatchTimeout(timeout time.Duration) HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive, got %v", timeout)
		}

		o.Timeout = timeout

		return nil
	}
}

// WithMaxBatchSize returns an option that sets the maximum number of requests in a single batch.
// The value must be positive.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithMaxBatchSize(50))
func WithMaxBatchSize(size int) HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		if size <= 0 {
			return fmt.Errorf("maxBatchSize must be positive, got %d", size)
		}

		o.MaxBatchSize = size

		return nil
	}
}

// WithBatchRetryCount returns an option that sets the number of times to retry a failed batch request.
// The value must be non-negative.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithBatchRetryCount(5))
func WithBatchRetryCount(count int) HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		if count < 0 {
			return fmt.Errorf("retryCount must be non-negative, got %d", count)
		}

		o.RetryCount = count

		return nil
	}
}

// WithBatchRetryBackoff returns an option that sets the backoff duration between retries.
// The value must be positive.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithBatchRetryBackoff(200 * time.Millisecond))
func WithBatchRetryBackoff(backoff time.Duration) HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		if backoff <= 0 {
			return fmt.Errorf("retryBackoff must be positive, got %v", backoff)
		}

		o.RetryBackoff = backoff

		return nil
	}
}

// WithBatchContinueOnError returns an option that determines whether to continue processing if one request fails.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithBatchContinueOnError(true))
func WithBatchContinueOnError(continueOnError bool) HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		o.ContinueOnError = continueOnError
		return nil
	}
}

// WithBatchWorkers returns an option that sets the number of concurrent workers for processing multiple batches.
// The value must be positive.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithBatchWorkers(10))
func WithBatchWorkers(workers int) HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		if workers <= 0 {
			return fmt.Errorf("workers must be positive, got %d", workers)
		}

		o.Workers = workers

		return nil
	}
}

// WithHighThroughputBatch returns an option that configures the batch processor for high throughput.
// This increases concurrency, batch size, and retries for better performance under heavy load.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithHighThroughputBatch())
func WithHighThroughputBatch() HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		o.MaxBatchSize = 200
		o.RetryCount = 5
		o.Workers = 10
		o.Timeout = 120 * time.Second

		return nil
	}
}

// WithLowLatencyBatch returns an option that configures the batch processor for low latency.
// This decreases batch size and increases workers for faster processing of smaller batches.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithLowLatencyBatch())
func WithLowLatencyBatch() HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		o.MaxBatchSize = 25
		o.Workers = 8
		o.Timeout = 30 * time.Second
		o.RetryBackoff = 100 * time.Millisecond

		return nil
	}
}

// WithHighReliabilityBatch returns an option that configures the batch processor for high reliability.
// This increases timeout, retries, and enables continuing on error for maximum resilience.
//
// Example:
//
//	processor := concurrent.NewHTTPBatchProcessor(client, baseURL,
//	    concurrent.WithHighReliabilityBatch())
func WithHighReliabilityBatch() HTTPBatchOption {
	return func(o *HTTPBatchOptions) error {
		o.RetryCount = 7
		o.RetryBackoff = 750 * time.Millisecond
		o.ContinueOnError = true
		o.Timeout = 180 * time.Second

		return nil
	}
}

// JSONMarshaler is an interface for JSON marshaling and unmarshaling.
// This allows for different implementations (like jsoniter) to be used for performance.
type JSONMarshaler interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// DefaultJSONMarshaler uses the standard encoding/json package.
type DefaultJSONMarshaler struct{}

// Marshal implements JSONMarshaler.Marshal using the standard encoding/json package.
func (*DefaultJSONMarshaler) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal implements JSONMarshaler.Unmarshal using the standard encoding/json package.
func (*DefaultJSONMarshaler) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// HTTPBatchProcessor handles batching of HTTP requests with efficient concurrency.
type HTTPBatchProcessor struct {
	// httpClient is the HTTP client to use for requests
	httpClient *http.Client

	// baseURL is the base URL for all requests
	baseURL string

	// defaultHeaders are headers to include in all requests
	defaultHeaders map[string]string

	// options are the batch options
	options *HTTPBatchOptions

	// jsonMarshaler is the JSON marshaler to use
	jsonMarshaler JSONMarshaler
}

// NewHTTPBatchProcessor creates a new HTTPBatchProcessor with the given client and baseURL.
// It accepts optional functional options to customize the processor's behavior.
//
// Parameters:
//   - client: The HTTP client to use for requests. If nil, a default client will be created.
//   - baseURL: The base URL for all requests.
//   - opts: Optional functional options to configure the processor.
//
// Example:
//
//	// Create with default options
//	processor := concurrent.NewHTTPBatchProcessor(client, "https://api.example.com")
//
//	// Create with custom options
//	processor := concurrent.NewHTTPBatchProcessor(client, "https://api.example.com",
//	    concurrent.WithBatchTimeout(30 * time.Second),
//	    concurrent.WithMaxBatchSize(50),
//	    concurrent.WithBatchRetryCount(5))
func NewHTTPBatchProcessor(client *http.Client, baseURL string, opts ...HTTPBatchOption) *HTTPBatchProcessor {
	// Start with default options
	options := DefaultHTTPBatchOptions()

	// Apply all provided options, continuing with defaults on error
	for _, opt := range opts {
		_ = opt(options) //nolint:errcheck // option errors are non-fatal, continue with defaults
	}

	if client == nil {
		client = &http.Client{
			Timeout: options.Timeout,
		}
	}

	return &HTTPBatchProcessor{
		httpClient:     client,
		baseURL:        baseURL,
		defaultHeaders: make(map[string]string),
		options:        options,
		jsonMarshaler:  &DefaultJSONMarshaler{},
	}
}

// SetJSONMarshaler sets a custom JSON marshaler implementation.
func (b *HTTPBatchProcessor) SetJSONMarshaler(marshaler JSONMarshaler) {
	b.jsonMarshaler = marshaler
}

// SetDefaultHeader sets a default header for all requests.
func (b *HTTPBatchProcessor) SetDefaultHeader(key, value string) {
	b.defaultHeaders[key] = value
}

// SetDefaultHeaders sets multiple default headers for all requests.
func (b *HTTPBatchProcessor) SetDefaultHeaders(headers map[string]string) {
	for k, v := range headers {
		b.defaultHeaders[k] = v
	}
}

// ExecuteBatch executes a batch of requests and returns the results.
func (b *HTTPBatchProcessor) ExecuteBatch(ctx context.Context, requests []HTTPBatchRequest) (*HTTPBatchResult, error) {
	if len(requests) == 0 {
		return &HTTPBatchResult{Responses: []HTTPBatchResponse{}}, nil
	}

	executor := &batchExecutor{
		processor: b,
		ctx:       b.applyContextTimeout(ctx),
	}

	return executor.execute(requests)
}

// batchExecutor handles the execution of HTTP batch requests.
type batchExecutor struct {
	processor *HTTPBatchProcessor
	ctx       context.Context
}

// execute runs the batch execution logic.
func (e *batchExecutor) execute(requests []HTTPBatchRequest) (*HTTPBatchResult, error) {
	if len(requests) > e.processor.options.MaxBatchSize {
		return e.processor.executeBatches(e.ctx, requests)
	}

	e.ensureRequestIDs(requests)

	req, err := e.createHTTPRequest(requests)
	if err != nil {
		return nil, err
	}

	respBody, statusCode, err := e.executeWithRetries(req)
	if err != nil {
		return nil, err
	}

	if statusCode >= 400 {
		return nil, e.handleErrorResponse(respBody, statusCode)
	}

	return e.parseSuccessResponse(respBody)
}

// applyContextTimeout applies timeout if not already set.
func (b *HTTPBatchProcessor) applyContextTimeout(ctx context.Context) context.Context {
	if _, ok := ctx.Deadline(); !ok && b.options.Timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, b.options.Timeout)
		// Note: We need to store the cancel function somewhere to avoid goroutine leak
		// This is a design issue in the original code that should be addressed
		_ = cancel

		return timeoutCtx
	}

	return ctx
}

// ensureRequestIDs ensures each request has a unique ID.
func (*batchExecutor) ensureRequestIDs(requests []HTTPBatchRequest) {
	for i := range requests {
		if requests[i].ID == "" {
			requests[i].ID = fmt.Sprintf("req_%d", i)
		}
	}
}

// createHTTPRequest creates the HTTP request for the batch.
func (e *batchExecutor) createHTTPRequest(requests []HTTPBatchRequest) (*http.Request, error) {
	reqBody, err := e.processor.jsonMarshaler.Marshal(requests)
	if err != nil {
		return nil, pkgerrors.NewInternalError("HTTPBatchRequest", err)
	}

	req, err := http.NewRequestWithContext(e.ctx, http.MethodPost, e.processor.baseURL+"/batch", bytes.NewReader(reqBody))
	if err != nil {
		return nil, pkgerrors.NewInternalError("HTTPBatchRequest", err)
	}

	e.setRequestHeaders(req)

	return req, nil
}

// setRequestHeaders sets the required headers for the batch request.
func (e *batchExecutor) setRequestHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	for k, v := range e.processor.defaultHeaders {
		req.Header.Set(k, v)
	}
}

// executeWithRetries executes the HTTP request with retry logic.
func (e *batchExecutor) executeWithRetries(req *http.Request) ([]byte, int, error) {
	for retryCount := 0; retryCount <= e.processor.options.RetryCount; retryCount++ {
		resp, err := e.processor.httpClient.Do(req)
		if err != nil {
			if shouldRetryConnectionError(retryCount, e.processor.options.RetryCount, e.ctx.Err()) {
				if waitErr := e.waitForRetry(); waitErr != nil {
					return nil, 0, waitErr
				}

				continue
			}

			return nil, 0, pkgerrors.NewNetworkError("HTTPBatchRequest", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close() // #nosec G104 - best effort close after read; error doesn't affect already-read data

		if readErr != nil {
			return nil, 0, pkgerrors.NewInternalError("HTTPBatchRequest", readErr)
		}

		if !shouldRetryStatus(resp.StatusCode, retryCount, e.processor.options.RetryCount) {
			return respBody, resp.StatusCode, nil
		}

		if waitErr := e.waitForRetry(); waitErr != nil {
			return nil, 0, waitErr
		}
	}

	return nil, 0, pkgerrors.NewInternalError("HTTPBatchRequest", errors.New("max retries exceeded"))
}

// shouldRetryConnectionError determines if a connection error should trigger a retry.
func shouldRetryConnectionError(retryCount, maxRetries int, ctxErr error) bool {
	return retryCount < maxRetries && ctxErr == nil
}

// shouldRetryStatus determines if an HTTP status code should trigger a retry.
func shouldRetryStatus(statusCode, retryCount, maxRetries int) bool {
	if statusCode < 400 {
		return false // Success
	}

	if statusCode < 500 {
		return false // Client error, don't retry
	}

	return retryCount < maxRetries // Server error, retry if possible
}

// waitForRetry waits for the retry backoff duration.
func (e *batchExecutor) waitForRetry() error {
	select {
	case <-e.ctx.Done():
		return pkgerrors.NewCancellationError("HTTPBatchRequest", e.ctx.Err())
	case <-time.After(e.processor.options.RetryBackoff):
		return nil
	}
}

// handleErrorResponse handles error responses from the batch API.
func (*batchExecutor) handleErrorResponse(respBody []byte, statusCode int) error {
	var errResp struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
		return pkgerrors.NewInternalError("HTTPBatchRequest", fmt.Errorf("batch request failed: %s", errResp.Error))
	}

	return pkgerrors.NewInternalError("HTTPBatchRequest", fmt.Errorf("batch request failed with status %d", statusCode))
}

// parseSuccessResponse parses a successful response and checks for individual errors.
func (e *batchExecutor) parseSuccessResponse(respBody []byte) (*HTTPBatchResult, error) {
	var responses []HTTPBatchResponse
	if err := json.Unmarshal(respBody, &responses); err != nil {
		return nil, pkgerrors.NewInternalError("HTTPBatchRequest", fmt.Errorf("failed to decode batch response: %w", err))
	}

	result := &HTTPBatchResult{Responses: responses}

	if e.hasIndividualErrors(responses) && !e.processor.options.ContinueOnError {
		result.Error = pkgerrors.NewInternalError("HTTPBatchRequest", errors.New("one or more batch requests failed"))
	}

	return result, result.Error
}

// hasIndividualErrors checks if any individual requests in the batch failed.
func (*batchExecutor) hasIndividualErrors(responses []HTTPBatchResponse) bool {
	for _, resp := range responses {
		if resp.StatusCode >= 400 || resp.Error != "" {
			return true
		}
	}

	return false
}

// executeBatches splits a large batch into smaller batches and executes them using the worker pool.
func (b *HTTPBatchProcessor) executeBatches(ctx context.Context, requests []HTTPBatchRequest) (*HTTPBatchResult, error) {
	// Create batches
	var batches [][]HTTPBatchRequest

	for i := 0; i < len(requests); i += b.options.MaxBatchSize {
		end := i + b.options.MaxBatchSize
		if end > len(requests) {
			end = len(requests)
		}

		batches = append(batches, requests[i:end])
	}

	// Process batches concurrently using worker pool
	results := WorkerPool(ctx, batches, func(ctx context.Context, batch []HTTPBatchRequest) (*HTTPBatchResult, error) {
		return b.ExecuteBatch(ctx, batch)
	}, WithWorkers(b.options.Workers))

	// Combine results
	var allResponses []HTTPBatchResponse

	var firstError error

	for _, r := range results {
		if r.Error != nil {
			if firstError == nil {
				firstError = r.Error
			}

			if !b.options.ContinueOnError {
				break
			}
		}

		if r.Value != nil {
			allResponses = append(allResponses, r.Value.Responses...)
		}
	}

	return &HTTPBatchResult{
		Responses: allResponses,
		Error:     firstError,
	}, firstError
}

// ParseResponse parses a batch response for a specific request ID into the target.
func (b *HTTPBatchProcessor) ParseResponse(result *HTTPBatchResult, requestID string, target any) error {
	if result == nil {
		return pkgerrors.NewInternalError("ParseHTTPBatchResponse", errors.New("batch result is nil"))
	}

	resp := b.findResponseByID(result.Responses, requestID)
	if resp == nil {
		return pkgerrors.NewInternalError("ParseHTTPBatchResponse", fmt.Errorf("no response found for request %s", requestID))
	}

	return b.parseResponseBody(resp, requestID, target)
}

// findResponseByID searches for a response with the given request ID.
func (*HTTPBatchProcessor) findResponseByID(responses []HTTPBatchResponse, requestID string) *HTTPBatchResponse {
	for i := range responses {
		if responses[i].ID == requestID {
			return &responses[i]
		}
	}

	return nil
}

// parseResponseBody parses the response body into the target after validating the response.
func (b *HTTPBatchProcessor) parseResponseBody(resp *HTTPBatchResponse, requestID string, target any) error {
	if resp.Error != "" {
		return pkgerrors.NewInternalError("ParseHTTPBatchResponse", fmt.Errorf("request %s failed: %s", requestID, resp.Error))
	}

	if resp.StatusCode >= 400 {
		return pkgerrors.NewInternalError("ParseHTTPBatchResponse", fmt.Errorf("request %s failed with status %d", requestID, resp.StatusCode))
	}

	if target == nil || len(resp.Body) == 0 {
		return nil
	}

	if err := b.jsonMarshaler.Unmarshal(resp.Body, target); err != nil {
		return pkgerrors.NewInternalError("ParseHTTPBatchResponse", fmt.Errorf("failed to parse response for request %s: %w", requestID, err))
	}

	return nil
}

// ExecuteBatchWithPoolOptions executes a batch of requests with custom worker pool options.
func (b *HTTPBatchProcessor) ExecuteBatchWithPoolOptions(ctx context.Context, requests []HTTPBatchRequest, opts ...PoolOption) (*HTTPBatchResult, error) {
	if len(requests) == 0 {
		return &HTTPBatchResult{
			Responses: []HTTPBatchResponse{},
		}, nil
	}

	// Apply context timeout if one isn't already set
	if _, ok := ctx.Deadline(); !ok && b.options.Timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, b.options.Timeout)
		defer cancel()
	}

	// Create batches
	var batches [][]HTTPBatchRequest

	for i := 0; i < len(requests); i += b.options.MaxBatchSize {
		end := i + b.options.MaxBatchSize
		if end > len(requests) {
			end = len(requests)
		}

		batches = append(batches, requests[i:end])
	}

	// Process batches concurrently using worker pool with custom options
	results := WorkerPool(ctx, batches, func(ctx context.Context, batch []HTTPBatchRequest) (*HTTPBatchResult, error) {
		return b.ExecuteBatch(ctx, batch)
	}, opts...)

	// Combine results
	var allResponses []HTTPBatchResponse

	var firstError error

	for _, r := range results {
		if r.Error != nil {
			if firstError == nil {
				firstError = r.Error
			}

			if !b.options.ContinueOnError {
				break
			}
		}

		if r.Value != nil {
			allResponses = append(allResponses, r.Value.Responses...)
		}
	}

	return &HTTPBatchResult{
		Responses: allResponses,
		Error:     firstError,
	}, firstError
}
