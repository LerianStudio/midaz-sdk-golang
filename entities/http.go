// Package entities provides high-level encapsulation for Midaz API interaction.
// It provides domain-specific entities like accounts, assets, organizations, etc.
package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/version"
)

// getUserAgent retrieves the user agent string from environment variable or uses default
func getUserAgent() string {
	// Check for environment variable
	if userAgent := os.Getenv("MIDAZ_USER_AGENT"); userAgent != "" {
		return userAgent
	}
	// Fall back to centralized version
	return version.UserAgent()
}

// HTTPClient is a wrapper around http.Client with additional functionality:
// - Authentication with API tokens
// - JSON request and response handling
// - Error handling and mapping API errors to SDK errors
// - Debug logging of requests and responses when enabled
// - Automatic retries with exponential backoff
// - Optimized performance with connection pooling and JSON handling
// - Observability with tracing, metrics, and logging
type HTTPClient struct {
	client        *http.Client
	authToken     string
	userAgent     string
	debug         bool
	retryOptions  *retry.Options        // Retry options for the client
	jsonPool      *performance.JSONPool // Pool for JSON encoding/decoding
	metrics       *observability.MetricsCollector
	observability observability.Provider
}

// NewHTTPClient creates a new HTTP client with the provided configuration.
// The debug flag is set to false by default and can be overridden using the WithDebug option.
//
// Parameters:
//   - client: The HTTP client to use for requests.
//   - authToken: The authentication token for API authorization.
//   - provider: The observability provider for tracing, metrics, and logging (can be nil).
func NewHTTPClient(client *http.Client, authToken string, provider observability.Provider) *HTTPClient {
	debug := os.Getenv("MIDAZ_DEBUG") == "true"
	retryOptions := initRetryOptionsFromEnv(provider)
	metrics := initMetricsCollector(provider)

	// Use the default client if none is provided
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &HTTPClient{
		client:        client,
		authToken:     authToken,
		userAgent:     getUserAgent(),
		debug:         debug,
		retryOptions:  retryOptions,
		jsonPool:      performance.NewJSONPool(),
		metrics:       metrics,
		observability: provider,
	}
}

// initRetryOptionsFromEnv initializes retry options from environment variables.
func initRetryOptionsFromEnv(provider observability.Provider) *retry.Options {
	retryOptions := retry.DefaultOptions()

	// Check for retry configuration in environment variables
	if maxRetries := os.Getenv("MIDAZ_MAX_RETRIES"); maxRetries != "" {
		if val, err := strconv.Atoi(maxRetries); err == nil && val >= 0 {
			if err := retry.WithMaxRetries(val)(retryOptions); err != nil {
				logRetryError(provider, "Failed to set max retries: %v", err)
			}
		}
	}

	// Check if retries are disabled
	if retryEnv := os.Getenv("MIDAZ_ENABLE_RETRIES"); retryEnv == "false" {
		if err := retry.WithMaxRetries(0)(retryOptions); err != nil {
			logRetryError(provider, "Failed to disable retries: %v", err)
		}
	}

	return retryOptions
}

// initMetricsCollector initializes the metrics collector if observability is enabled.
func initMetricsCollector(provider observability.Provider) *observability.MetricsCollector {
	if provider == nil || !provider.IsEnabled() {
		return nil
	}

	metrics, err := observability.NewMetricsCollector(provider)
	if err != nil && provider.Logger() != nil {
		provider.Logger().Warnf("Failed to create metrics collector: %v", err)
	}

	return metrics
}

// logRetryError logs a retry configuration error if observability is enabled.
func logRetryError(provider observability.Provider, format string, args ...any) {
	if provider != nil && provider.IsEnabled() {
		provider.Logger().Errorf(format, args...)
	}
}

// WithRetryOptions sets custom retry options for the HTTP client.
func (c *HTTPClient) WithRetryOptions(options ...retry.Option) *HTTPClient {
	// Create a new options struct with defaults
	retryOpts := &retry.Options{}

	// Apply all options
	for _, opt := range options {
		// Apply the option and log errors, but continue
		if err := opt(retryOpts); err != nil {
			c.debugLog("Error applying retry option: %v", err)
		}
	}

	c.retryOptions = retryOpts

	return c
}

// WithRetryOption applies a retry option to the HTTP client.
func (c *HTTPClient) WithRetryOption(option retry.Option) *HTTPClient {
	if err := option(c.retryOptions); err != nil {
		// Log the error but continue with existing options
		c.debugLog("Error applying retry option: %v", err)
	}

	return c
}

// WithUserAgent sets a custom user agent string for the HTTP client.
func (c *HTTPClient) WithUserAgent(userAgent string) *HTTPClient {
	c.userAgent = userAgent
	return c
}

// WithDebug enables or disables debug mode for the HTTP client.
func (c *HTTPClient) WithDebug(debug bool) *HTTPClient {
	c.debug = debug
	return c
}

// doRequest performs an HTTP request with the given method, URL, headers, and body.
// It handles JSON encoding and decoding, authentication, error handling, and retries.
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation and timeouts.
//   - method: The HTTP method (GET, POST, PUT, DELETE, etc.).
//   - requestURL: The URL to send the request to.
//   - headers: Additional headers to include in the request.
//   - body: The request body (will be JSON encoded).
//   - result: A pointer to the result object (will be JSON decoded from the response).
//
// Returns:
//   - error: An error if the request failed.
func (c *HTTPClient) doRequest(ctx context.Context, method, requestURL string, headers map[string]string, body, result any) error {
	// Create observability context and span
	ctx, endSpan := c.setupObservabilityContext(ctx, method, requestURL)
	defer endSpan()

	// Build HTTP request
	req, _, err := c.buildHTTPRequest(ctx, method, requestURL, body)
	if err != nil {
		return err
	}

	// Inject idempotency header from context if present.
	// If key is empty, no header is set (which is the expected behavior for non-idempotent requests).
	if key := getIdempotencyKeyFromContext(ctx); key != "" {
		if headers == nil {
			headers = map[string]string{}
		}

		headers["X-Idempotency"] = key
	}

	// Setup headers
	c.setupRequestHeaders(req, headers, body != nil)

	// Execute request with retry logic and capture elapsed time
	start := time.Now()
	resp, responseBody, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		return err
	}
	// Ensure response body is closed after we're done with it
	defer func() {
		if resp != nil && resp.Body != nil {
			c.closeResponseBody(resp)
		}
	}()

	// Record metrics and debug logging
	c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)
	c.logResponseDetails(method, requestURL, resp, responseBody)

	// Process response
	return c.processResponse(result, responseBody)
}

// doRawRequest performs an HTTP request using a pre-built byte payload without JSON encoding.
func (c *HTTPClient) doRawRequest(ctx context.Context, method, requestURL string, headers map[string]string, body []byte, result any) error {
	ctx, endSpan := c.setupObservabilityContext(ctx, method, requestURL)
	defer endSpan()

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	if len(body) > 0 {
		if headers == nil {
			headers = map[string]string{}
		}

		if strings.TrimSpace(headers["Content-Type"]) == "" {
			return fmt.Errorf("content-type header required for non-empty request body")
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set GetBody for retry support - allows body to be recreated on retries
	if len(body) > 0 {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}

	if key := getIdempotencyKeyFromContext(ctx); key != "" {
		if headers == nil {
			headers = map[string]string{}
		}

		headers["X-Idempotency"] = key
	}

	c.setupRequestHeaders(req, headers, len(body) > 0)

	start := time.Now()
	resp, responseBody, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		return err
	}
	// Ensure response body is closed after we're done with it
	defer func() {
		if resp != nil && resp.Body != nil {
			c.closeResponseBody(resp)
		}
	}()

	c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)
	c.logResponseDetails(method, requestURL, resp, responseBody)

	return c.processResponse(result, responseBody)
}

// setupObservabilityContext creates tracing span if observability is enabled
func (c *HTTPClient) setupObservabilityContext(ctx context.Context, method, requestURL string) (context.Context, func()) {
	if c.observability == nil || !c.observability.IsEnabled() {
		return ctx, func() {}
	}

	spanCtx, span := c.observability.Tracer().Start(ctx, fmt.Sprintf("HTTP %s %s", method, requestURL))

	return spanCtx, func() { span.End() }
}

// buildHTTPRequest creates the HTTP request with body handling
func (c *HTTPClient) buildHTTPRequest(ctx context.Context, method, requestURL string, body any) (*http.Request, []byte, error) {
	c.debugLog("Request URL: %s %s", method, requestURL)

	reqBody, bodyBytes, err := c.prepareRequestBody(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set GetBody for retry support - allows body to be recreated on retries
	if len(bodyBytes) > 0 {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	return req, bodyBytes, nil
}

// prepareRequestBody handles JSON marshaling and logging for request body
func (c *HTTPClient) prepareRequestBody(body any) (io.Reader, []byte, error) {
	if body == nil {
		return nil, nil, nil
	}

	bodyBytes, err := c.jsonPool.Marshal(body)
	if err != nil {
		return nil, nil, err
	}

	c.debugLogRequestBody(bodyBytes)

	return bytes.NewReader(bodyBytes), bodyBytes, nil
}

// debugLogRequestBody logs request body if debug mode is enabled
func (c *HTTPClient) debugLogRequestBody(bodyBytes []byte) {
	if c.debug {
		c.debugLog("Request body: %s", string(bodyBytes))
	}
}

// setupRequestHeaders configures all necessary request headers
func (c *HTTPClient) setupRequestHeaders(req *http.Request, headers map[string]string, hasBody bool) {
	// Standard headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	// Custom headers first (allows overriding Content-Type)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Content type for requests with body (only if not already set by custom headers)
	if hasBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Authorization header
	if c.authToken != "" {
		req.Header.Set("Authorization", c.authToken)
	}
}

// executeRequestWithRetry handles the request execution with retry logic
func (c *HTTPClient) executeRequestWithRetry(ctx context.Context, req *http.Request, method, requestURL string) (*http.Response, []byte, error) {
	var resp *http.Response

	var responseBody []byte

	retryCtx := retry.WithOptionsContext(ctx, c.retryOptions)

	err := retry.DoWithContext(retryCtx, func() error {
		var err error

		// Reset request body for retry if GetBody is available
		if req.GetBody != nil {
			req.Body, err = req.GetBody()
			if err != nil {
				return fmt.Errorf("failed to reset request body for retry: %w", err)
			}
		}

		resp, err = c.client.Do(req)
		if err != nil {
			c.debugLogRequestError(method, requestURL, err)
			return fmt.Errorf("HTTP request failed: %w", err)
		}

		// Ensure response body is always closed, even on error paths
		defer func() {
			if resp != nil && resp.Body != nil {
				c.closeResponseBody(resp)
			}
		}()

		responseBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode >= 400 {
			// Extract request ID from response headers for error context
			requestID := resp.Header.Get("X-Request-ID")
			return c.handleErrorResponse(resp.StatusCode, responseBody, method, requestURL, requestID)
		}

		return nil
	})

	return resp, responseBody, err
}

// closeResponseBody safely closes response body with debug logging
func (c *HTTPClient) closeResponseBody(resp *http.Response) {
	if closeErr := resp.Body.Close(); closeErr != nil && c.debug {
		c.debugLog("Failed to close response body: %v", closeErr)
	}
}

// handleErrorResponse processes API error responses
func (c *HTTPClient) handleErrorResponse(statusCode int, responseBody []byte, method, requestURL, requestID string) error {
	apiErr := c.parseErrorResponse(statusCode, responseBody, requestID)

	if c.debug {
		c.debugLog("HTTP Error response from: %s %s", method, requestURL)
		c.debugLog("Error status: %d", statusCode)

		if requestID != "" {
			c.debugLog("Request ID: %s", requestID)
		}
		c.debugLog("Error body: %s", string(responseBody))
		c.debugLog("Parsed error: %v", apiErr)
	}

	return apiErr
}

// debugLogRequestError logs request failures in debug mode
func (c *HTTPClient) debugLogRequestError(method, requestURL string, err error) {
	if c.debug {
		c.debugLog("HTTP request failed: %s %s - %v", method, requestURL, err)
	}
}

// recordRequestMetrics records performance metrics if enabled
func (c *HTTPClient) recordRequestMetrics(ctx context.Context, method, requestURL string, resp *http.Response, elapsed time.Duration) {
	if c.metrics != nil {
		c.metrics.RecordRequest(ctx, method, requestURL, resp.StatusCode, elapsed)
	}
}

// logResponseDetails logs response information in debug mode
func (c *HTTPClient) logResponseDetails(method, requestURL string, resp *http.Response, responseBody []byte) {
	if !c.debug {
		return
	}

	c.debugLog("Response from: %s %s", method, requestURL)
	c.debugLog("Response status: %d", resp.StatusCode)
	c.debugLog("Response headers: %v", resp.Header)
	c.debugLog("Response body: %s", string(responseBody))
}

// processResponse handles JSON unmarshaling of the response
func (c *HTTPClient) processResponse(result any, responseBody []byte) error {
	if result == nil || len(responseBody) == 0 {
		return nil
	}

	if err := c.jsonPool.Unmarshal(responseBody, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

// Legacy sendRequest method to maintain backward compatibility
func (c *HTTPClient) sendRequest(req *http.Request, v any) error {
	// Extract method and URL from the request
	method := req.Method
	requestURL := req.URL.String()

	// Extract headers from the request
	headers := make(map[string]string)

	for key, values := range req.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Extract body from the request
	var body any

	if req.Body != nil {
		// Read the body
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return err
		}

		// Close the body
		if closeErr := req.Body.Close(); closeErr != nil {
			// Log the error but continue with the request
			if c.debug {
				c.debugLog("Failed to close request body: %v", closeErr)
			}
		}

		// Unmarshal the body if not empty
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				return err
			}
		}
	}

	// Use the context from the request
	ctx := req.Context()

	// Call the new doRequest method
	return c.doRequest(ctx, method, requestURL, headers, body, v)
}

// debugLog logs a debug message if debug mode is enabled.
// Uses observability logger when available, otherwise falls back to stderr.
func (c *HTTPClient) debugLog(format string, args ...any) {
	if !c.debug {
		return
	}

	// Use observability logger if available
	if c.observability != nil && c.observability.IsEnabled() && c.observability.Logger() != nil {
		c.observability.Logger().Debugf(format, args...)
		return
	}

	// Fall back to stderr for debug output
	fmt.Fprintf(os.Stderr, "[Midaz SDK Debug] "+format+"\n", args...)
}

// idempotency context helpers
type contextKeyIdempotency struct{}

// WithIdempotencyKey attaches an idempotency key to the request context.
// The HTTP client will add it as an 'X-Idempotency' header.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}

	return context.WithValue(ctx, contextKeyIdempotency{}, key)
}

func getIdempotencyKeyFromContext(ctx context.Context) string {
	if v := ctx.Value(contextKeyIdempotency{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// parseErrorResponse parses an error response from the API and converts it to an SDK error.
func (c *HTTPClient) parseErrorResponse(statusCode int, body []byte, requestID string) error {
	// If there's no body, create a generic error
	if len(body) == 0 {
		return sdkerrors.ErrorFromHTTPResponse(statusCode, requestID, "Empty response from server", "", "", "")
	}

	// Try to parse the error body as a JSON object
	var apiError struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}

	if err := json.Unmarshal(body, &apiError); err != nil {
		// If we can't parse the JSON, return the raw body as the error message
		return sdkerrors.ErrorFromHTTPResponse(statusCode, requestID, string(body), "", "", "")
	}

	// Use the message if available, otherwise use the error field
	message := apiError.Message
	if message == "" {
		message = apiError.Error
	}

	// If there's still no message, use a default one
	if message == "" {
		message = fmt.Sprintf("API error with status code %d", statusCode)
	}

	// Create the appropriate error type based on the status code
	return sdkerrors.ErrorFromHTTPResponse(statusCode, requestID, message, apiError.Code, "", "")
}

// AddURLParams adds query parameters to a URL.
func AddURLParams(baseURL string, params map[string]string) string {
	if len(params) == 0 {
		return baseURL
	}

	// Parse the existing URL
	u, err := url.Parse(baseURL)
	if err != nil {
		// If we can't parse the URL, just return it as-is
		return baseURL
	}

	// Get existing query values
	q := u.Query()

	// Add new parameters
	for key, value := range params {
		q.Set(key, value)
	}

	// Update the URL with the new query string
	u.RawQuery = q.Encode()

	return u.String()
}

// NewRequest creates a new HTTP request with the given method, URL, and body.
// It's a convenient wrapper around http.NewRequest for backward compatibility.
func (c *HTTPClient) NewRequest(method, requestURL string, body any) (*http.Request, error) {
	var bodyReader io.Reader

	if body != nil {
		// Serialize the request body to JSON
		bodyBytes, err := c.jsonPool.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, requestURL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add standard headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	// Add content type if there's a body
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add authorization if there's a token
	if c.authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	}

	return req, nil
}
