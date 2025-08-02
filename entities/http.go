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
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/retry"
	"go.opentelemetry.io/otel/trace"
)

// getUserAgent retrieves the user agent string from environment variable or uses default
func getUserAgent() string {
	// Check for environment variable
	if userAgent := os.Getenv("MIDAZ_USER_AGENT"); userAgent != "" {
		return userAgent
	}
	// Fall back to config default
	return "Midaz-Go-SDK/1.0.0"
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
	// Check if we're using the debug flag from the environment
	debug := false

	if debugEnv := os.Getenv("MIDAZ_DEBUG"); debugEnv == "true" {
		debug = true
	}

	// Initialize retry options with defaults
	retryOptions := retry.DefaultOptions()

	// Check for retry configuration in environment variables
	if maxRetries := os.Getenv("MIDAZ_MAX_RETRIES"); maxRetries != "" {
		if val, err := parseInt(maxRetries); err == nil && val >= 0 {
			if err := retry.WithMaxRetries(val)(retryOptions); err != nil {
				// Log the error if observability is enabled
				if provider != nil && provider.IsEnabled() {
					provider.Logger().Errorf("Failed to set max retries: %v", err)
				}
			}
		}
	}

	// Check if retries are disabled
	if retryEnv := os.Getenv("MIDAZ_ENABLE_RETRIES"); retryEnv == "false" {
		if err := retry.WithMaxRetries(0)(retryOptions); err != nil {
			// Log the error if observability is enabled
			if provider != nil && provider.IsEnabled() {
				provider.Logger().Errorf("Failed to disable retries: %v", err)
			}
		}
	}

	// Initialize metrics collector if observability is provided
	var metrics *observability.MetricsCollector
	if provider != nil && provider.IsEnabled() {
		metrics, _ = observability.NewMetricsCollector(provider)
	}

	// Use the default client if none is provided
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	// Create the HTTP client
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
	// Create a span for the HTTP request if observability is enabled
	var spanCtx context.Context
	if c.observability != nil && c.observability.IsEnabled() {
		var span trace.Span
		spanCtx, span = c.observability.Tracer().Start(ctx, fmt.Sprintf("HTTP %s %s", method, requestURL))
		ctx = spanCtx

		defer span.End()
	}

	// Create the HTTP request
	var reqBody io.Reader

	// Log the request URL if debug mode is enabled
	if c.debug {
		c.debugLog("Request URL: %s %s", method, requestURL)
	}

	if body != nil {
		// Serialize the request body to JSON
		bodyBytes, err := c.jsonPool.Marshal(body)

		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)

		// Log the request body if debug mode is enabled
		if c.debug {
			c.debugLog("Request body: %s", string(bodyBytes))
		}
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
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
		req.Header.Set("Authorization", c.authToken)
	}

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Define a function to execute the request with retry logic
	var resp *http.Response

	var responseBody []byte

	// Set context with retry options
	retryCtx := retry.WithOptionsContext(ctx, c.retryOptions)

	// Use retry.Do to execute the request with retries
	err = retry.DoWithContext(
		retryCtx,
		func() error {
			// Do the actual HTTP request
			resp, err = c.client.Do(req)
			if err != nil {
				if c.debug {
					c.debugLog("HTTP request failed: %s %s - %v", method, requestURL, err)
				}
				return fmt.Errorf("HTTP request failed: %w", err)
			}

			// Read the response body
			responseBody, err = io.ReadAll(resp.Body)
			if closeErr := resp.Body.Close(); closeErr != nil { // Always close the body
				// Log the error but continue with the response
				if c.debug {
					c.debugLog("Failed to close response body: %v", closeErr)
				}
			}

			// Return an error if the status code indicates a problem
			if resp.StatusCode >= 400 {
				apiErr := c.parseErrorResponse(resp.StatusCode, responseBody)

				// Log error details in debug mode
				if c.debug {
					c.debugLog("HTTP Error response from: %s %s", method, requestURL)
					c.debugLog("Error status: %d", resp.StatusCode)
					c.debugLog("Error headers: %v", resp.Header)
					c.debugLog("Error body: %s", string(responseBody))
					c.debugLog("Parsed error: %v", apiErr)
				}

				return apiErr
			}

			return err
		},
	)

	// If the request failed after retries, return the error
	if err != nil {
		return err
	}

	// Record metrics if observability is enabled
	if c.metrics != nil {
		c.metrics.RecordRequest(ctx, method, requestURL, resp.StatusCode, time.Duration(len(responseBody))*time.Millisecond)
	}

	// Log the response details if debug mode is enabled
	if c.debug {
		c.debugLog("Response from: %s %s", method, requestURL)
		c.debugLog("Response status: %d", resp.StatusCode)
		c.debugLog("Response headers: %v", resp.Header)
		c.debugLog("Response body: %s", string(responseBody))
	}

	// Unmarshal the response if there's a result pointer
	if result != nil && len(responseBody) > 0 {
		if err := c.jsonPool.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Legacy sendRequest method to maintain backward compatibility
func (c *HTTPClient) sendRequest(req *http.Request, v any) error {
	// Extract method and URL from the request
	method := req.Method
	url := req.URL.String()

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
	return c.doRequest(ctx, method, url, headers, body, v)
}

// debugLog logs a debug message if debug mode is enabled.
func (c *HTTPClient) debugLog(format string, args ...any) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[Midaz SDK Debug] "+format+"\n", args...)
	}
}

// parseErrorResponse parses an error response from the API and converts it to an SDK error.
func (c *HTTPClient) parseErrorResponse(statusCode int, body []byte) error {
	// If there's no body, create a generic error
	if len(body) == 0 {
		return sdkerrors.ErrorFromHTTPResponse(statusCode, "", "Empty response from server", "", "", "")
	}

	// Try to parse the error body as a JSON object
	var apiError struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}

	if err := json.Unmarshal(body, &apiError); err != nil {
		// If we can't parse the JSON, return the raw body as the error message
		return sdkerrors.ErrorFromHTTPResponse(statusCode, "", string(body), "", "", "")
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
	return sdkerrors.ErrorFromHTTPResponse(statusCode, "", message, apiError.Code, "", "")
}

// parseInt converts a string to an integer with error handling.
func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
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
func (c *HTTPClient) NewRequest(method, url string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		// Serialize the request body to JSON
		bodyBytes, err := c.jsonPool.Marshal(body)

		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, bodyReader)
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
