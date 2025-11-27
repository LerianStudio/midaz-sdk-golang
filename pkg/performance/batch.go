package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
)

// BatchRequest represents a single request in a batch.
type BatchRequest struct {
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

// BatchResponse represents a single response in a batch.
type BatchResponse struct {
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

// BatchResult contains the results of a batch operation.
type BatchResult struct {
	// Responses are the responses for each request
	Responses []BatchResponse `json:"responses"`

	// Error is the error that occurred during the batch operation
	Error error `json:"-"`
}

// BatchOptions configures the behavior of batch requests.
type BatchOptions struct {
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
}

// BatchOption defines a function that configures BatchOptions
type BatchOption func(*BatchOptions) error

// WithBatchTimeout sets the maximum time to wait for a batch request to complete
func WithBatchTimeout(timeout time.Duration) BatchOption {
	return func(o *BatchOptions) error {
		if timeout <= 0 {
			return fmt.Errorf("batch timeout must be positive, got %v", timeout)
		}

		o.Timeout = timeout

		return nil
	}
}

// WithMaxBatchSize sets the maximum number of requests in a single batch
func WithMaxBatchSize(size int) BatchOption {
	return func(o *BatchOptions) error {
		if size <= 0 {
			return fmt.Errorf("max batch size must be positive, got %d", size)
		}

		o.MaxBatchSize = size

		return nil
	}
}

// WithRetryCount sets the number of times to retry a failed batch request
func WithRetryCount(count int) BatchOption {
	return func(o *BatchOptions) error {
		if count < 0 {
			return fmt.Errorf("retry count must be non-negative, got %d", count)
		}

		o.RetryCount = count

		return nil
	}
}

// WithRetryBackoff sets the backoff duration between retries
func WithRetryBackoff(backoff time.Duration) BatchOption {
	return func(o *BatchOptions) error {
		if backoff < 0 {
			return fmt.Errorf("retry backoff must be non-negative, got %v", backoff)
		}

		o.RetryBackoff = backoff

		return nil
	}
}

// WithContinueOnError sets whether to continue processing if one request fails
func WithContinueOnError(continueOnError bool) BatchOption {
	return func(o *BatchOptions) error {
		o.ContinueOnError = continueOnError
		return nil
	}
}

// WithHighThroughputBatching configures batch options for high throughput
func WithHighThroughputBatching() BatchOption {
	return func(o *BatchOptions) error {
		o.MaxBatchSize = 200
		o.RetryCount = 5
		o.RetryBackoff = 100 * time.Millisecond

		return nil
	}
}

// WithReliableBatching configures batch options for high reliability
func WithReliableBatching() BatchOption {
	return func(o *BatchOptions) error {
		o.RetryCount = 10
		o.RetryBackoff = 1 * time.Second
		o.ContinueOnError = true

		return nil
	}
}

// DefaultBatchOptions returns the default batch options.
func DefaultBatchOptions() *BatchOptions {
	return &BatchOptions{
		Timeout:         60 * time.Second,
		MaxBatchSize:    100,
		RetryCount:      3,
		RetryBackoff:    500 * time.Millisecond,
		ContinueOnError: false,
	}
}

// NewBatchOptions creates a new BatchOptions with the given options.
func NewBatchOptions(opts ...BatchOption) (*BatchOptions, error) {
	// Start with default options
	options := DefaultBatchOptions()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, fmt.Errorf("failed to apply batch option: %w", err)
		}
	}

	return options, nil
}

// BatchProcessor handles batching of HTTP requests.
type BatchProcessor struct {
	// httpClient is the HTTP client to use for requests
	httpClient *http.Client

	// baseURL is the base URL for all requests
	baseURL string

	// defaultHeaders are headers to include in all requests
	defaultHeaders map[string]string

	// options are the batch options
	options *BatchOptions

	// jsonPool is the JSON encoder/decoder pool
	jsonPool *JSONPool
}

// BatchProcessorOption defines a function that configures a BatchProcessor
type BatchProcessorOption func(*BatchProcessor) error

// WithHTTPClient sets the HTTP client for the batch processor
func WithHTTPClient(client *http.Client) BatchProcessorOption {
	return func(p *BatchProcessor) error {
		if client == nil {
			return fmt.Errorf("HTTP client cannot be nil")
		}

		p.httpClient = client

		return nil
	}
}

// WithBaseURL sets the base URL for the batch processor
func WithBaseURL(url string) BatchProcessorOption {
	return func(p *BatchProcessor) error {
		if url == "" {
			return fmt.Errorf("base URL cannot be empty")
		}

		p.baseURL = url

		return nil
	}
}

// WithBatchOptions sets the batch options for the batch processor
func WithBatchOptions(options *BatchOptions) BatchProcessorOption {
	return func(p *BatchProcessor) error {
		if options == nil {
			return fmt.Errorf("batch options cannot be nil")
		}

		p.options = options

		return nil
	}
}

// WithDefaultHeader sets a default header for all requests in the batch
func WithDefaultHeader(key, value string) BatchProcessorOption {
	return func(p *BatchProcessor) error {
		if key == "" {
			return fmt.Errorf("header key cannot be empty")
		}

		p.defaultHeaders[key] = value

		return nil
	}
}

// WithDefaultHeaders sets multiple default headers for all requests in the batch
func WithDefaultHeaders(headers map[string]string) BatchProcessorOption {
	return func(p *BatchProcessor) error {
		if headers == nil {
			return fmt.Errorf("headers map cannot be nil")
		}

		for k, v := range headers {
			p.defaultHeaders[k] = v
		}

		return nil
	}
}

// WithJSONPool sets the JSON pool for the batch processor
func WithJSONPool(pool *JSONPool) BatchProcessorOption {
	return func(p *BatchProcessor) error {
		if pool == nil {
			return fmt.Errorf("JSON pool cannot be nil")
		}

		p.jsonPool = pool

		return nil
	}
}

// NewBatchProcessor creates a new BatchProcessor with the given options.
func NewBatchProcessor(baseURL string, opts ...BatchProcessorOption) (*BatchProcessor, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}

	// Start with default values
	options := DefaultBatchOptions()

	// Create a default processor
	processor := &BatchProcessor{
		baseURL:        baseURL,
		defaultHeaders: make(map[string]string),
		options:        options,
		jsonPool:       NewJSONPool(),
	}

	// Create a default HTTP client if needed
	transport, err := NewTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	processor.httpClient = &http.Client{
		Transport: transport,
		Timeout:   options.Timeout,
	}

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(processor); err != nil {
			return nil, fmt.Errorf("failed to apply batch processor option: %w", err)
		}
	}

	return processor, nil
}

// NewBatchProcessorWithDefaults creates a new BatchProcessor with the given client, baseURL, and options
// for backward compatibility.
func NewBatchProcessorWithDefaults(client *http.Client, baseURL string, options *BatchOptions) *BatchProcessor {
	var opts []BatchProcessorOption

	if client != nil {
		opts = append(opts, WithHTTPClient(client))
	}

	if options != nil {
		opts = append(opts, WithBatchOptions(options))
	}

	// NewBatchProcessor may fail if baseURL is empty, but this function maintains
	// backward compatibility and returns nil in that case
	processor, _ := NewBatchProcessor(baseURL, opts...) //nolint:errcheck // backward compat: nil on error

	return processor
}

// SetDefaultHeader sets a default header for all requests.
// This method is maintained for backward compatibility.
func (b *BatchProcessor) SetDefaultHeader(key, value string) {
	b.defaultHeaders[key] = value
}

// SetDefaultHeaders sets multiple default headers for all requests.
// This method is maintained for backward compatibility.
func (b *BatchProcessor) SetDefaultHeaders(headers map[string]string) {
	for k, v := range headers {
		b.defaultHeaders[k] = v
	}
}

// ExecuteBatch executes a batch of requests and returns the results.
func (b *BatchProcessor) ExecuteBatch(ctx context.Context, requests []BatchRequest) (*BatchResult, error) {
	// Early return for empty requests
	if len(requests) == 0 {
		return &BatchResult{Responses: []BatchResponse{}}, nil
	}

	// Setup context with timeout
	ctx = b.setupContextTimeout(ctx)

	// Handle large batches by splitting
	if len(requests) > b.options.MaxBatchSize {
		return b.executeBatches(ctx, requests)
	}

	// Prepare and execute the single batch
	return b.executeSingleBatch(ctx, requests)
}

// setupContextTimeout applies timeout to context if not already set
func (b *BatchProcessor) setupContextTimeout(ctx context.Context) context.Context {
	if _, ok := ctx.Deadline(); !ok && b.options.Timeout > 0 {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, b.options.Timeout)
		// Note: caller is responsible for calling cancel when done
		_ = cancel

		return ctxWithTimeout
	}

	return ctx
}

// executeSingleBatch processes a single batch of requests
func (b *BatchProcessor) executeSingleBatch(ctx context.Context, requests []BatchRequest) (*BatchResult, error) {
	// Prepare requests
	requests = b.assignRequestIDs(requests)

	// Build and send HTTP request
	resp, err := b.sendBatchRequest(ctx, requests)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse and validate response
	result, err := b.processBatchResponse(resp)
	if err != nil {
		return nil, err
	}

	// Return error if batch result contains error and should not continue on error
	if result.Error != nil {
		return result, result.Error
	}

	return result, nil
}

// assignRequestIDs ensures all requests have unique IDs
func (b *BatchProcessor) assignRequestIDs(requests []BatchRequest) []BatchRequest {
	for i := range requests {
		if requests[i].ID == "" {
			requests[i].ID = fmt.Sprintf("req_%d", i)
		}
	}

	return requests
}

// sendBatchRequest creates and sends the HTTP request with retry logic
func (b *BatchProcessor) sendBatchRequest(ctx context.Context, requests []BatchRequest) (*http.Response, error) {
	req, err := b.buildHTTPRequest(ctx, requests)
	if err != nil {
		return nil, err
	}

	return b.executeWithRetry(ctx, req)
}

// buildHTTPRequest creates the HTTP request with proper headers and body
func (b *BatchProcessor) buildHTTPRequest(ctx context.Context, requests []BatchRequest) (*http.Request, error) {
	reqBody, err := b.jsonPool.Marshal(requests)
	if err != nil {
		return nil, errors.NewInternalError("BatchRequest", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/batch", bytes.NewReader(reqBody))
	if err != nil {
		return nil, errors.NewInternalError("BatchRequest", err)
	}

	b.setRequestHeaders(req)

	return req, nil
}

// setRequestHeaders adds all necessary headers to the request
func (b *BatchProcessor) setRequestHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	for k, v := range b.defaultHeaders {
		req.Header.Set(k, v)
	}
}

// executeWithRetry executes the request with retry logic
func (b *BatchProcessor) executeWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response

	var respErr error

	for retry := 0; retry <= b.options.RetryCount; retry++ {
		resp, respErr = b.httpClient.Do(req)
		if respErr == nil {
			return resp, nil
		}

		if !b.shouldRetry(ctx, retry) {
			break
		}

		if err := b.waitForRetry(ctx); err != nil {
			return nil, err
		}
	}

	if respErr != nil {
		return nil, errors.NewNetworkError("BatchRequest", respErr)
	}

	return resp, nil
}

// shouldRetry determines if another retry attempt should be made
func (b *BatchProcessor) shouldRetry(ctx context.Context, retry int) bool {
	return retry < b.options.RetryCount && ctx.Err() == nil
}

// waitForRetry waits for the retry backoff period or context cancellation
func (b *BatchProcessor) waitForRetry(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return errors.NewCancellationError("BatchRequest", ctx.Err())
	case <-time.After(b.options.RetryBackoff):
		return nil
	}
}

// processBatchResponse processes the HTTP response and creates BatchResult
func (b *BatchProcessor) processBatchResponse(resp *http.Response) (*BatchResult, error) {
	if resp.StatusCode >= 400 {
		return nil, b.handleErrorResponse(resp)
	}

	responses, err := b.parseResponseBody(resp)
	if err != nil {
		return nil, err
	}

	return b.createBatchResult(responses), nil
}

// handleErrorResponse processes error responses from the server
func (b *BatchProcessor) handleErrorResponse(resp *http.Response) error {
	var errResp struct {
		Error string `json:"error"`
	}

	if err := b.jsonPool.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
		return errors.NewInternalError("BatchRequest", fmt.Errorf("batch request failed: %s", errResp.Error))
	}

	return errors.NewInternalError("BatchRequest", fmt.Errorf("batch request failed with status %d", resp.StatusCode))
}

// parseResponseBody decodes the response body into BatchResponse slice
func (b *BatchProcessor) parseResponseBody(resp *http.Response) ([]BatchResponse, error) {
	var responses []BatchResponse
	if err := b.jsonPool.NewDecoder(resp.Body).Decode(&responses); err != nil {
		return nil, errors.NewInternalError("BatchRequest", fmt.Errorf("failed to decode batch response: %w", err))
	}

	return responses, nil
}

// createBatchResult creates the final BatchResult with error handling
func (b *BatchProcessor) createBatchResult(responses []BatchResponse) *BatchResult {
	result := &BatchResult{Responses: responses}

	if b.hasResponseErrors(responses) && !b.options.ContinueOnError {
		result.Error = errors.NewInternalError("BatchRequest", fmt.Errorf("one or more batch requests failed"))
	}

	return result
}

// hasResponseErrors checks if any individual responses contain errors
func (b *BatchProcessor) hasResponseErrors(responses []BatchResponse) bool {
	for _, resp := range responses {
		if resp.StatusCode >= 400 || resp.Error != "" {
			return true
		}
	}

	return false
}

// executeBatches splits a large batch into smaller batches and executes them.
func (b *BatchProcessor) executeBatches(ctx context.Context, requests []BatchRequest) (*BatchResult, error) {
	// Calculate the number of batches needed
	batchCount := (len(requests) + b.options.MaxBatchSize - 1) / b.options.MaxBatchSize

	// Create channels for results and errors
	resultsChan := make(chan []BatchResponse, batchCount)
	errorsChan := make(chan error, batchCount)

	var wg sync.WaitGroup

	wg.Add(batchCount)

	// Process each batch concurrently
	for i := 0; i < batchCount; i++ {
		start := i * b.options.MaxBatchSize
		end := start + b.options.MaxBatchSize

		if end > len(requests) {
			end = len(requests)
		}

		batch := requests[start:end]

		go func(batch []BatchRequest) {
			defer wg.Done()

			result, err := b.executeSingleBatch(ctx, batch)
			if err != nil {
				errorsChan <- err
				return
			}

			resultsChan <- result.Responses
		}(batch)
	}

	// Wait for all batches to complete
	go func() {
		wg.Wait()
		close(resultsChan)
		close(errorsChan)
	}()

	// Collect results and errors
	var responses []BatchResponse

	var firstError error

	for resp := range resultsChan {
		responses = append(responses, resp...)
	}

	// Check for errors
	for err := range errorsChan {
		if firstError == nil {
			firstError = err
		}
	}

	result := &BatchResult{
		Responses: responses,
		Error:     firstError,
	}

	return result, firstError
}

// ParseBatchResponse parses a batch response for a specific request ID into the target.
func (b *BatchProcessor) ParseBatchResponse(result *BatchResult, requestID string, target any) error {
	if result == nil {
		return errors.NewInternalError("ParseBatchResponse", fmt.Errorf("batch result is nil"))
	}

	// Find the response for the given request ID
	for _, resp := range result.Responses {
		if resp.ID == requestID {
			if resp.Error != "" {
				return errors.NewInternalError("ParseBatchResponse", fmt.Errorf("request %s failed: %s", requestID, resp.Error))
			}

			if resp.StatusCode >= 400 {
				return errors.NewInternalError("ParseBatchResponse", fmt.Errorf("request %s failed with status %d", requestID, resp.StatusCode))
			}

			// Parse the response body
			if target != nil && len(resp.Body) > 0 {
				if err := b.jsonPool.Unmarshal(resp.Body, target); err != nil {
					return errors.NewInternalError("ParseBatchResponse", fmt.Errorf("failed to parse response for request %s: %w", requestID, err))
				}
			}

			return nil
		}
	}

	return errors.NewInternalError("ParseBatchResponse", fmt.Errorf("no response found for request %s", requestID))
}
