package concurrent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
)

// HTTPBatchProcessorWithRetry is an HTTPBatchProcessor that uses the enhanced retry package.
type HTTPBatchProcessorWithRetry struct {
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

	// retryOptions are the options to use for HTTP retries
	retryOptions retry.HTTPOptions
}

// NewHTTPBatchProcessorWithRetry creates a new HTTPBatchProcessorWithRetry.
// It uses the enhanced retry package for improved resilience.
//
// Parameters:
//   - client: The HTTP client to use for requests. If nil, a default client will be created.
//   - baseURL: The base URL for all requests.
//   - opts: Optional functional options to configure the processor.
//
// Example:
//
//	// Create with default options
//	processor := concurrent.NewHTTPBatchProcessorWithRetry(client, "https://api.example.com")
//
//	// Create with custom options
//	processor := concurrent.NewHTTPBatchProcessorWithRetry(client, "https://api.example.com",
//	    concurrent.WithBatchRetryCount(5),
//	    concurrent.WithBatchContinueOnError(true))
func NewHTTPBatchProcessorWithRetry(client *http.Client, baseURL string, opts ...HTTPBatchOption) (*HTTPBatchProcessorWithRetry, error) {
	// Start with default options
	options := DefaultHTTPBatchOptions()

	// Apply all provided options, continuing with defaults on error
	for _, opt := range opts {
		opt(options) //nolint:errcheck // option errors are non-fatal, continue with defaults
	}

	if client == nil {
		client = &http.Client{}
	}

	// Convert batch options to retry options
	retryOptions := retry.DefaultHTTPOptions()
	// Apply retry options from batch options
	if err := retry.WithHTTPMaxRetries(options.RetryCount)(retryOptions); err != nil {
		return nil, fmt.Errorf("failed to set max retries: %w", err)
	}

	if err := retry.WithHTTPInitialDelay(options.RetryBackoff)(retryOptions); err != nil {
		return nil, fmt.Errorf("failed to set initial delay: %w", err)
	}

	if err := retry.WithHTTPMaxDelay(options.RetryBackoff * 10)(retryOptions); err != nil { // Scale up max delay
		return nil, fmt.Errorf("failed to set max delay: %w", err)
	}

	if err := retry.WithHTTPBackoffFactor(2.0)(retryOptions); err != nil {
		return nil, fmt.Errorf("failed to set backoff factor: %w", err)
	}

	if err := retry.WithHTTPRetryAllServerErrors(true)(retryOptions); err != nil {
		return nil, fmt.Errorf("failed to set retry all server errors: %w", err)
	}

	if err := retry.WithHTTPRetryOn4xx([]int{429})(retryOptions); err != nil { // Too Many Requests
		return nil, fmt.Errorf("failed to set retry on 4xx: %w", err)
	}

	return &HTTPBatchProcessorWithRetry{
		httpClient:     client,
		baseURL:        baseURL,
		defaultHeaders: make(map[string]string),
		options:        options,
		jsonMarshaler:  &DefaultJSONMarshaler{},
		retryOptions:   *retryOptions,
	}, nil
}

// SetJSONMarshaler sets a custom JSON marshaler implementation.
func (b *HTTPBatchProcessorWithRetry) SetJSONMarshaler(marshaler JSONMarshaler) {
	b.jsonMarshaler = marshaler
}

// SetDefaultHeader sets a default header for all requests.
func (b *HTTPBatchProcessorWithRetry) SetDefaultHeader(key, value string) {
	b.defaultHeaders[key] = value
}

// SetDefaultHeaders sets multiple default headers for all requests.
func (b *HTTPBatchProcessorWithRetry) SetDefaultHeaders(headers map[string]string) {
	for k, v := range headers {
		b.defaultHeaders[k] = v
	}
}

// ExecuteBatch executes a batch of requests and returns the results.
func (b *HTTPBatchProcessorWithRetry) ExecuteBatch(ctx context.Context, requests []HTTPBatchRequest) (*HTTPBatchResult, error) {
	// Handle empty request case
	if len(requests) == 0 {
		return &HTTPBatchResult{Responses: []HTTPBatchResponse{}}, nil
	}

	// Prepare context and check for large batches
	ctx = b.prepareExecutionContext(ctx)
	if len(requests) > b.options.MaxBatchSize {
		return b.executeBatches(ctx, requests)
	}

	// Execute single batch
	return b.executeSingleBatch(ctx, requests)
}

// prepareExecutionContext prepares the context for batch execution
func (b *HTTPBatchProcessorWithRetry) prepareExecutionContext(ctx context.Context) context.Context {
	// Apply context timeout if one isn't already set
	if _, ok := ctx.Deadline(); !ok && b.options.Timeout > 0 {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, b.options.Timeout)
		// Note: We can't defer cancel here as the context is returned
		// The caller must handle the cancellation
		_ = cancel // Avoid unused variable warning
	}

	// Add retry options to the context
	return retry.WithHTTPOptionsContext(ctx, &b.retryOptions)
}

// executeSingleBatch executes a single batch of requests
func (b *HTTPBatchProcessorWithRetry) executeSingleBatch(ctx context.Context, requests []HTTPBatchRequest) (*HTTPBatchResult, error) {
	// Ensure each request has an ID
	b.ensureRequestIDs(requests)

	// Create and configure the HTTP request
	req, err := b.createHTTPRequest(ctx, requests)
	if err != nil {
		return nil, err
	}

	// Execute the request with enhanced retry
	httpResp, err := retry.DoHTTPRequestWithContext(ctx, b.httpClient, req)
	if err != nil {
		return nil, pkgerrors.NewNetworkError("HTTPBatchRequest", err)
	}

	// Parse and process the response
	return b.parseAndProcessResponse(httpResp)
}

// ensureRequestIDs ensures each request has an ID
func (*HTTPBatchProcessorWithRetry) ensureRequestIDs(requests []HTTPBatchRequest) {
	for i := range requests {
		if requests[i].ID == "" {
			requests[i].ID = fmt.Sprintf("req_%d", i)
		}
	}
}

// createHTTPRequest creates and configures the HTTP request
func (b *HTTPBatchProcessorWithRetry) createHTTPRequest(ctx context.Context, requests []HTTPBatchRequest) (*http.Request, error) {
	// Create the request body
	reqBody, err := b.jsonMarshaler.Marshal(requests)
	if err != nil {
		return nil, pkgerrors.NewInternalError("HTTPBatchRequest", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+"/batch", bytes.NewReader(reqBody))
	if err != nil {
		return nil, pkgerrors.NewInternalError("HTTPBatchRequest", err)
	}

	// Set GetBody function to allow the request to be retried
	bodyBytes := reqBody // Copy for closure
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	// Add headers
	b.setRequestHeaders(req)

	return req, nil
}

// setRequestHeaders sets the appropriate headers on the HTTP request
func (b *HTTPBatchProcessorWithRetry) setRequestHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")

	for k, v := range b.defaultHeaders {
		req.Header.Set(k, v)
	}
}

// parseAndProcessResponse parses the HTTP response and processes the results
func (b *HTTPBatchProcessorWithRetry) parseAndProcessResponse(httpResp *retry.HTTPResponse) (*HTTPBatchResult, error) {
	// Check for HTTP errors
	if httpResp.Error != nil {
		return nil, pkgerrors.NewNetworkError("HTTPBatchRequest", httpResp.Error)
	}

	// Parse the response
	var responses []HTTPBatchResponse
	if err := b.jsonMarshaler.Unmarshal(httpResp.Body, &responses); err != nil {
		return nil, pkgerrors.NewInternalError("HTTPBatchRequest", fmt.Errorf("failed to decode batch response: %w", err))
	}

	// Create result and check for errors
	result := &HTTPBatchResult{Responses: responses}

	if b.hasResponseErrors(responses) && !b.options.ContinueOnError {
		result.Error = pkgerrors.NewInternalError("HTTPBatchRequest", errors.New("one or more batch requests failed"))
	}

	return result, result.Error
}

// hasResponseErrors checks if any responses have errors
func (*HTTPBatchProcessorWithRetry) hasResponseErrors(responses []HTTPBatchResponse) bool {
	for _, resp := range responses {
		if resp.StatusCode >= 400 || resp.Error != "" {
			return true
		}
	}

	return false
}

// executeBatches splits a large batch into smaller batches and executes them using the worker pool.
func (b *HTTPBatchProcessorWithRetry) executeBatches(ctx context.Context, requests []HTTPBatchRequest) (*HTTPBatchResult, error) {
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
func (b *HTTPBatchProcessorWithRetry) ParseResponse(result *HTTPBatchResult, requestID string, target any) error {
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
func (*HTTPBatchProcessorWithRetry) findResponseByID(responses []HTTPBatchResponse, requestID string) *HTTPBatchResponse {
	for i := range responses {
		if responses[i].ID == requestID {
			return &responses[i]
		}
	}

	return nil
}

// parseResponseBody parses the response body into the target after validating the response.
func (b *HTTPBatchProcessorWithRetry) parseResponseBody(resp *HTTPBatchResponse, requestID string, target any) error {
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
