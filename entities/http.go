// Package entities provides high-level encapsulation for Midaz API interaction.
// It provides domain-specific entities like accounts, assets, organizations, etc.
package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/security"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/version"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/singleflight"
)

var (
	errEmptyResponseBody = errors.New("empty response body")
	errNullResponseBody  = errors.New("null response body")
)

const (
	maxHTTPResponseBodyBytes = int64(10 << 20)
	maxHTTPRequestBodyBytes  = int64(10 << 20)
)

const (
	internalCallerIdempotencyHeader = "X-Midaz-Caller-Idempotency"
	internalAutoIdempotencyHeader   = "X-Midaz-Auto-Idempotency"
)

// defaultUserAgent returns the SDK's centralized user-agent string.
// The configured value flows in via the WithUserAgent option (driven from
// pkg/config.Config.UserAgent, which itself can be populated from the
// MIDAZ_USER_AGENT env var when the caller opts in via FromEnvironment).
func defaultUserAgent() string {
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
	mu                sync.RWMutex
	client            *http.Client
	authToken         string
	userAgent         string
	tenantID          string
	debug             bool
	retryOptions      *retry.Options        // Retry options for the client
	jsonPool          *performance.JSONPool // Pool for JSON encoding/decoding
	metrics           *observability.MetricsCollector
	observability     observability.Provider
	enableIdempotency bool
	customRetryPolicy func(*http.Response, error) bool
	tokenProvider     func(context.Context) (string, error)
	tokenInvalidator  func()
	// logger is the canonical *slog.Logger for retry/slow-call/internal
	// warnings. Always non-nil after midaz.New() wires the parent client's
	// logger via SetLogger. Per-service HTTP clients inherit it through
	// applyConfigurationFrom.
	logger *slog.Logger
	// slowCallThreshold is the duration above which a successful call
	// emits a Warn-level structured log. Zero disables the warning.
	slowCallThreshold time.Duration
	// tokenRefreshGroup serializes concurrent 401-driven token refreshes per
	// cache key. A burst of in-flight requests that all hit a 401 at the
	// same time funnel through one underlying call to tokenProvider, which
	// matters when tokenProvider is the network-bound access-manager
	// exchange.
	tokenRefreshGroup singleflight.Group
}

type httpClientConfigSnapshot struct {
	authToken         string
	userAgent         string
	tenantID          string
	debug             bool
	retryOptions      *retry.Options
	metrics           *observability.MetricsCollector
	observability     observability.Provider
	enableIdempotency bool
	customRetryPolicy func(*http.Response, error) bool
	tokenProvider     func(context.Context) (string, error)
	tokenInvalidator  func()
	logger            *slog.Logger
	slowCallThreshold time.Duration
}

// NewHTTPClient creates a new HTTP client with safe v3 defaults.
//
// v3 invariant (Track 3): no environment variables are read here. Configuration
// flows in exclusively through pkg/config.Config and the WithDebug / WithUserAgent
// / WithObservability / WithRetryOptions / SetEnableIdempotency setters. Callers
// who want env-driven configuration must opt in via config.NewConfig(config.FromEnvironment()).
//
// Defaults: debug=false, userAgent=version.UserAgent(), enableIdempotency=true,
// retryOptions=retry.DefaultOptions().
//
// Parameters:
//   - client: The underlying *http.Client. If nil, the SDK's package-level default is used.
//   - authToken: The authentication token for API authorization.
//   - provider: The observability provider for tracing, metrics, and logging (can be nil).
func NewHTTPClient(client *http.Client, authToken string, provider observability.Provider) *HTTPClient {
	if client == nil {
		client = defaultHTTPClient()
	}

	return &HTTPClient{
		client:            client,
		authToken:         authToken,
		userAgent:         defaultUserAgent(),
		debug:             false,
		retryOptions:      retry.DefaultOptions(),
		jsonPool:          performance.NewJSONPool(),
		metrics:           initMetricsCollector(provider),
		observability:     provider,
		enableIdempotency: true,
	}
}

// defaultHTTPClient returns the SDK's package-level default *http.Client.
// The instance is initialized lazily exactly once via sync.OnceValue so that
// the (relatively expensive) http.Transport with connection pool is shared
// across every nil-fallback site. The returned client is safe for
// concurrent use — net/http guarantees this for the underlying Transport.
//
// Tests that need a fresh transport should construct one explicitly via
// http.Client{} rather than mutating this shared instance.
var defaultHTTPClient = sync.OnceValue(func() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return security.ValidateOutboundRequest(req)
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
})

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

// WithRetryOptions sets custom retry options for the HTTP client.
func (c *HTTPClient) WithRetryOptions(options ...retry.Option) *HTTPClient {
	if c == nil {
		return nil
	}

	// Create a new options struct with defaults
	retryOpts := retry.DefaultOptions()

	// Apply all options
	for _, opt := range options {
		if opt == nil {
			c.debugLog("Error applying retry option: retry option cannot be nil")
			continue
		}

		// Apply the option and log errors, but continue
		if err := opt(retryOpts); err != nil {
			c.debugLog("Error applying retry option: %v", err)
		}
	}

	c.mu.Lock()
	c.retryOptions = retryOpts
	c.mu.Unlock()

	return c
}

// WithRetryOption applies a retry option to the HTTP client.
func (c *HTTPClient) WithRetryOption(option retry.Option) *HTTPClient {
	if c == nil {
		return nil
	}

	if option == nil {
		c.debugLog("Error applying retry option: retry option cannot be nil")
		return c
	}

	c.mu.Lock()

	if c.retryOptions == nil {
		c.retryOptions = retry.DefaultOptions()
	}

	if err := option(c.retryOptions); err != nil {
		c.mu.Unlock()
		c.debugLog("Error applying retry option: %v", err)

		return c
	}
	c.mu.Unlock()

	return c
}

// SetLogger installs the *slog.Logger used for retry/slow-call/internal
// warnings. Passing nil reverts to a discard handler. The logger is shared
// with all per-service HTTP clients via propagateHTTPClientConfiguration.
func (c *HTTPClient) SetLogger(logger *slog.Logger) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger = logger
}

// Logger returns the configured *slog.Logger or a discard logger if none was
// installed. Always non-nil so callers can write log lines unconditionally.
func (c *HTTPClient) Logger() *slog.Logger {
	if c == nil {
		return slog.New(slog.DiscardHandler)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.logger == nil {
		return slog.New(slog.DiscardHandler)
	}

	return c.logger
}

// SetSlowCallThreshold installs the duration above which a successful call
// emits a Warn-level log line. Zero or negative values disable the warning.
func (c *HTTPClient) SetSlowCallThreshold(threshold time.Duration) {
	if c == nil {
		return
	}

	if threshold < 0 {
		threshold = 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.slowCallThreshold = threshold
}

// SetEnableIdempotency enables or disables automatic idempotency header
// generation at the HTTP client level.
//
// When enabled (the default unless MIDAZ_IDEMPOTENCY=false), every unsafe
// request (POST/PUT/PATCH/DELETE) gets an automatic UUIDv4 idempotency key
// in the X-Idempotency header — UNLESS the caller already supplied one via
// [sdkctx.WithIdempotencyKey], in which case the explicit key wins.
//
// To opt OUT for a single call without disabling client-level idempotency
// entirely, attach the request context with [sdkctx.WithoutAutoIdempotency].
// The ordering rule is: explicit caller key > suppression > auto-generation.
func (c *HTTPClient) SetEnableIdempotency(enabled bool) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.enableIdempotency = enabled
}

// SetCustomRetryPolicy sets the predicate used to decide whether retries should continue.
func (c *HTTPClient) SetCustomRetryPolicy(shouldRetry func(*http.Response, error) bool) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.customRetryPolicy = shouldRetry
}

func (c *HTTPClient) setAuthTokenProvider(provider func(context.Context) (string, error), invalidator func()) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.tokenProvider = provider
	c.tokenInvalidator = invalidator
}

func (c *HTTPClient) applyConfigurationFrom(source *HTTPClient) {
	if source == nil {
		return
	}

	c.applyConfigurationSnapshot(source.cloneConfiguration())
}

func (c *HTTPClient) cloneConfiguration() httpClientConfigSnapshot {
	if c == nil {
		return httpClientConfigSnapshot{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return httpClientConfigSnapshot{
		authToken:         c.authToken,
		userAgent:         c.userAgent,
		tenantID:          c.tenantID,
		debug:             c.debug,
		retryOptions:      cloneRetryOptions(c.retryOptions),
		metrics:           c.metrics,
		observability:     c.observability,
		enableIdempotency: c.enableIdempotency,
		customRetryPolicy: c.customRetryPolicy,
		tokenProvider:     c.tokenProvider,
		tokenInvalidator:  c.tokenInvalidator,
		logger:            c.logger,
		slowCallThreshold: c.slowCallThreshold,
	}
}

func (c *HTTPClient) applyConfigurationSnapshot(snapshot httpClientConfigSnapshot) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.authToken = snapshot.authToken
	c.userAgent = snapshot.userAgent
	c.tenantID = snapshot.tenantID
	c.debug = snapshot.debug
	c.retryOptions = cloneRetryOptions(snapshot.retryOptions)
	c.metrics = snapshot.metrics
	c.observability = snapshot.observability
	c.enableIdempotency = snapshot.enableIdempotency
	c.customRetryPolicy = snapshot.customRetryPolicy
	c.tokenProvider = snapshot.tokenProvider
	c.tokenInvalidator = snapshot.tokenInvalidator
	c.logger = snapshot.logger
	c.slowCallThreshold = snapshot.slowCallThreshold
}

// WithUserAgent sets a custom user agent string for the HTTP client.
func (c *HTTPClient) WithUserAgent(userAgent string) *HTTPClient {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.userAgent = userAgent

	return c
}

// WithDebug enables or disables debug mode for the HTTP client.
func (c *HTTPClient) WithDebug(debug bool) *HTTPClient {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.debug = debug

	return c
}

// SetTenantID sets the default tenant ID for all requests made by this HTTP client.
// When a request is made, the tenant ID from the request context takes precedence
// over this client-level default. If neither is set, no X-Tenant-ID header is sent.
// This header is best-effort compatibility metadata and is not the sole tenant
// authority for the reference Midaz path.
//
// SetTenantID is safe for concurrent use; it acquires the client's write lock.
// All other field-mutating helpers in this file (setObservabilityLocked,
// setDebugLocked, setUserAgentLocked, setMetricsLocked, setTenantIDLocked)
// follow the same convention so that constructors and Option callbacks
// cannot bypass the mutex.
func (c *HTTPClient) SetTenantID(tenantID string) {
	if c == nil {
		return
	}

	c.setTenantIDLocked(strings.TrimSpace(tenantID))
}

// setDebugLocked sets the debug flag under the write lock. Internal callers
// (constructors that observe MIDAZ_DEBUG, Option setters) must use this
// helper instead of writing c.debug directly so that race detector runs
// stay clean.
func (c *HTTPClient) setDebugLocked(debug bool) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.debug = debug
}

// setUserAgentLocked sets the user agent under the write lock.
func (c *HTTPClient) setUserAgentLocked(userAgent string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.userAgent = userAgent
}

// setObservabilityLocked replaces the observability provider AND the metrics
// collector under a single write lock. Doing both in one critical section
// prevents observers from briefly seeing a new provider with the old
// metrics collector (or vice versa) during reconfiguration.
func (c *HTTPClient) setObservabilityLocked(provider observability.Provider, metrics *observability.MetricsCollector) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.observability = provider
	c.metrics = metrics
}

// setTenantIDLocked sets the tenant ID under the write lock.
func (c *HTTPClient) setTenantIDLocked(tenantID string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.tenantID = tenantID
}

// setAuthTokenLocked sets the auth token under the write lock. Used by
// Entity.SetAuthToken (plugin-auth refresh path) and the in-request 401
// refresh path. The auth token is read on every request to populate the
// Authorization header, so concurrent mutation absolutely must go through
// the same lock the readers use.
func (c *HTTPClient) setAuthTokenLocked(token string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.authToken = token
}

// GetTenantID returns the current default tenant ID configured on this HTTP client.
func (c *HTTPClient) GetTenantID() string {
	if c == nil {
		return ""
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.tenantID
}

// injectContextHeaders adds context-based headers (idempotency key, tenant ID) to the provided
// headers map. If headers is nil and there are headers to inject, a new map is created and returned.
func (c *HTTPClient) injectContextHeaders(ctx context.Context, method string, headers map[string]string) map[string]string {
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot := c.cloneConfiguration()
	// Inject idempotency header from context only for unsafe methods. Safe GET/HEAD
	// list/count requests are not idempotency participants and must not leak keys.
	if key := getIdempotencyKeyFromContext(ctx); isUnsafeMethod(method) && key != "" {
		if headers == nil {
			headers = map[string]string{}
		}

		headers["X-Idempotency"] = key
		headers[internalCallerIdempotencyHeader] = BoolTrue
	}

	// Inject tenant ID header from context or client-level default.
	// Context value takes precedence over the client-level default.
	if tid := sdkctx.TenantIDFromContext(ctx); tid != "" {
		if headers == nil {
			headers = map[string]string{}
		}

		headers[HeaderTenantID] = tid
	} else if snapshot.tenantID != "" {
		if headers == nil {
			headers = map[string]string{}
		}

		headers[HeaderTenantID] = snapshot.tenantID
	}

	return headers
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
	if ctx == nil {
		ctx = context.Background()
	}

	// Create observability context and span
	ctx, endSpan := c.setupObservabilityContext(ctx, method, requestURL)
	defer endSpan()

	// Build HTTP request
	req, _, err := c.buildHTTPRequest(ctx, method, requestURL, body)
	if err != nil {
		c.recordSDKFailure(ctx, method, requestURL, 0, err)

		return err
	}

	// Inject context-based headers (idempotency key, tenant ID)
	headers = c.injectContextHeaders(ctx, method, headers)
	headers = c.ensureIdempotencyHeader(ctx, method, headers)

	// Setup headers
	c.setupRequestHeaders(req, headers, body != nil)

	c.injectTraceContext(ctx, req)

	// Execute request with retry logic and capture elapsed time
	start := time.Now()
	resp, responseBody, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		c.recordRequestFailure(ctx, method, requestURL, resp, elapsed, err)
		return err
	}
	// Ensure response body is closed after we're done with it
	defer func() {
		if resp != nil && resp.Body != nil {
			c.closeResponseBody(resp)
		}
	}()

	c.logResponseDetails(method, requestURL, resp, responseBody)

	// Process response
	if err := c.processResponse(result, responseBody); err != nil {
		if errors.Is(err, errEmptyResponseBody) || errors.Is(err, errNullResponseBody) {
			c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

			return err
		}

		c.recordSDKFailure(ctx, method, requestURL, elapsed, err)

		return err
	}

	c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

	return nil
}

// doRawRequest performs an HTTP request using a pre-built byte payload without JSON encoding.
func (c *HTTPClient) doRawRequest(ctx context.Context, method, requestURL string, headers map[string]string, body []byte, result any) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, endSpan := c.setupObservabilityContext(ctx, method, requestURL)
	defer endSpan()

	req, headers, err := buildRawHTTPRequest(ctx, method, requestURL, headers, body)
	if err != nil {
		c.recordSDKFailure(ctx, method, requestURL, 0, err)

		return err
	}

	// Set GetBody for retry support - allows body to be recreated on retries
	if len(body) > 0 {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}

	// Inject context-based headers (idempotency key, tenant ID)
	headers = c.injectContextHeaders(ctx, method, headers)
	headers = c.ensureIdempotencyHeader(ctx, method, headers)

	c.setupRequestHeaders(req, headers, len(body) > 0)

	c.injectTraceContext(ctx, req)

	start := time.Now()
	resp, responseBody, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		c.recordRequestFailure(ctx, method, requestURL, resp, elapsed, err)
		return err
	}
	// Ensure response body is closed after we're done with it
	defer func() {
		if resp != nil && resp.Body != nil {
			c.closeResponseBody(resp)
		}
	}()

	c.logResponseDetails(method, requestURL, resp, responseBody)

	if err := c.processResponse(result, responseBody); err != nil {
		if errors.Is(err, errEmptyResponseBody) || errors.Is(err, errNullResponseBody) {
			c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

			return err
		}

		c.recordSDKFailure(ctx, method, requestURL, elapsed, err)

		return err
	}

	c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

	return nil
}

func prepareRawRequestBody(headers map[string]string, body []byte) (io.Reader, map[string]string, error) {
	if len(body) == 0 {
		return nil, headers, nil
	}

	if int64(len(body)) > maxHTTPRequestBodyBytes {
		return nil, nil, fmt.Errorf("request body exceeds maximum size of %d bytes", maxHTTPRequestBodyBytes)
	}

	if headers == nil {
		headers = map[string]string{}
	}

	if strings.TrimSpace(headers["Content-Type"]) == "" {
		return nil, nil, errors.New("content-type header required for non-empty request body")
	}

	return bytes.NewReader(body), headers, nil
}

func buildRawHTTPRequest(ctx context.Context, method, requestURL string, headers map[string]string, body []byte) (*http.Request, map[string]string, error) {
	reader, headers, err := prepareRawRequestBody(headers, body)
	if err != nil {
		return nil, nil, err
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse request URL: %w", err)
	}

	validationReq := &http.Request{URL: parsedURL}
	if err := security.ValidateOutboundRequest(validationReq); err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), reader) // #nosec G704 -- request URL validated via security.ValidateOutboundRequest using parsed URL
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	return req, headers, nil
}

func (c *HTTPClient) doCountRequest(ctx context.Context, method, requestURL string, headers map[string]string) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, endSpan := c.setupObservabilityContext(ctx, method, requestURL)
	defer endSpan()

	req, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	if err := security.ValidateOutboundRequest(req); err != nil {
		return 0, err
	}

	headers = c.injectContextHeaders(ctx, method, headers)
	c.setupRequestHeaders(req, headers, false)

	c.injectTraceContext(ctx, req)

	start := time.Now()
	resp, responseBody, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		c.recordRequestFailure(ctx, method, requestURL, resp, elapsed, err)
		return 0, err
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			c.closeResponseBody(resp)
		}
	}()

	c.logResponseDetails(method, requestURL, resp, responseBody)

	count, err := parseTotalCountHeader(resp.Header)
	if err != nil {
		countErr := sdkerrors.NewInternalError("CountRequest", err)
		c.recordSDKFailure(ctx, method, requestURL, elapsed, countErr)

		return 0, countErr
	}

	c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

	return count, nil
}

func parseTotalCountHeader(headers http.Header) (int, error) {
	raw := headers.Get(HeaderTotalCount)

	totalCount := strings.TrimSpace(raw)
	if totalCount == "" {
		return 0, fmt.Errorf("missing %s header", HeaderTotalCount)
	}

	count, err := strconv.ParseInt(totalCount, 10, 0)
	if err != nil || count < 0 {
		return 0, fmt.Errorf("invalid %s header", HeaderTotalCount)
	}

	return int(count), nil
}

func countRequestMethod() string {
	return http.MethodHead
}

func countRequestHeaders() map[string]string {
	return nil
}

// setupObservabilityContext creates tracing span if observability is enabled
func (c *HTTPClient) setupObservabilityContext(ctx context.Context, method, requestURL string) (context.Context, func()) {
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot := c.cloneConfiguration()
	if snapshot.observability == nil || !snapshot.observability.IsEnabled() {
		return ctx, func() {}
	}

	spanCtx, span := snapshot.observability.Tracer().Start(
		ctx,
		fmt.Sprintf("HTTP %s %s", method, normalizeTelemetryURL(requestURL)),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	spanCtx = observability.WithProvider(spanCtx, snapshot.observability)
	enrichHTTPSpan(spanCtx, method, requestURL, nil, nil)

	return spanCtx, func() { span.End() }
}

func (c *HTTPClient) injectTraceContext(ctx context.Context, req *http.Request) {
	if req == nil {
		return
	}

	snapshot := c.cloneConfiguration()
	if snapshot.observability == nil || !snapshot.observability.IsEnabled() {
		return
	}

	if req.Header == nil {
		req.Header = make(http.Header)
	}

	observability.InjectHTTPContext(ctx, req.Header)
}

func enrichHTTPSpan(ctx context.Context, method, requestURL string, resp *http.Response, err error) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	attrs := httpSpanAttributes(method, requestURL)
	if resp != nil {
		attrs = append(attrs, attribute.Int(observability.KeyHTTPResponseStatusCode, resp.StatusCode))
		if resp.ProtoMajor > 0 {
			attrs = append(attrs, attribute.String(observability.KeyNetworkProtocolVersion, httpProtocolVersion(resp.ProtoMajor, resp.ProtoMinor)))
		}
		if requestID := strings.TrimSpace(resp.Header.Get("X-Request-ID")); requestID != "" {
			attrs = append(attrs, attribute.StringSlice("http.response.header.x-request-id", []string{requestID}))
		}
	}

	span.SetAttributes(attrs...)

	if err != nil {
		sanitizedErr := sanitizeTelemetryError(err)
		span.SetStatus(codes.Error, sanitizedErr)
		span.SetAttributes(attribute.String(observability.KeyErrorType, telemetryErrorType(err)))
		span.RecordError(errors.New(sanitizedErr))

		return
	}

	if resp != nil && resp.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP status code: %d", resp.StatusCode))
		span.SetAttributes(attribute.String(observability.KeyErrorType, strconv.Itoa(resp.StatusCode)))

		return
	}

	if resp != nil {
		span.SetStatus(codes.Ok, "")
	}
}

func sanitizeTelemetryError(err error) string {
	if err == nil {
		return ""
	}

	return sanitizeLogInput(sdkerrors.RedactSensitiveString(err.Error()))
}

func httpSpanAttributes(method, requestURL string) []attribute.KeyValue {
	normalizedURL := normalizeTelemetryURL(requestURL)
	attrs := make([]attribute.KeyValue, 0, 10)
	attrs = append(attrs,
		attribute.String(observability.KeyHTTPRequestMethod, semconvHTTPMethod(method)),
		attribute.String(observability.KeyURLFull, normalizedURL),
		attribute.String(observability.KeyOperationName, fmt.Sprintf("HTTP %s %s", method, normalizedURL)),
		attribute.String(observability.KeyOperationType, "http.request"),
	)
	if original := semconvHTTPMethodOriginal(method); original != "" {
		attrs = append(attrs, attribute.String("http.request.method_original", original))
	}

	parsedURL, err := url.Parse(normalizedURL)
	if err != nil {
		return attrs
	}

	attrs = append(attrs,
		attribute.String(observability.KeyURLPath, parsedURL.EscapedPath()),
		attribute.String(observability.KeyURLScheme, parsedURL.Scheme),
	)
	if host := parsedURL.Hostname(); host != "" {
		attrs = append(attrs, attribute.String(observability.KeyServerAddress, host))
	}
	if port, ok := telemetryURLPort(parsedURL); ok {
		attrs = append(attrs, attribute.Int(observability.KeyServerPort, port))
	}

	return attrs
}

func telemetryErrorType(err error) string {
	if err == nil {
		return "_OTHER"
	}

	typeName := fmt.Sprintf("%T", err)
	if typeName == "" || typeName == "<nil>" {
		return "_OTHER"
	}

	return typeName
}

func semconvHTTPMethod(method string) string {
	upper := strings.ToUpper(method)
	switch upper {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return upper
	default:
		return "_OTHER"
	}
}

func semconvHTTPMethodOriginal(method string) string {
	if semconvHTTPMethod(method) == "_OTHER" {
		return method
	}

	return ""
}

func telemetryURLPort(parsedURL *url.URL) (int, bool) {
	if parsedURL == nil {
		return 0, false
	}
	if portString := parsedURL.Port(); portString != "" {
		port, err := strconv.Atoi(portString)
		return port, err == nil
	}
	switch strings.ToLower(parsedURL.Scheme) {
	case "http":
		return 80, true
	case "https":
		return 443, true
	default:
		return 0, false
	}
}

func httpProtocolVersion(major, minor int) string {
	if major <= 0 {
		return ""
	}

	return fmt.Sprintf("%d.%d", major, minor)
}

// buildHTTPRequest creates the HTTP request with body handling
func (c *HTTPClient) buildHTTPRequest(ctx context.Context, method, requestURL string, body any) (*http.Request, []byte, error) {
	c.debugLog("Request URL: %s %s", method, redactDebugURL(requestURL))

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse request URL: %w", err)
	}

	if err := security.ValidateOutboundRequest(&http.Request{URL: parsedURL}); err != nil {
		return nil, nil, fmt.Errorf("invalid request URL: %w", err)
	}

	reqBody, bodyBytes, err := c.prepareRequestBody(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare request body: %w", err)
	}

	validatedURL := parsedURL.String()

	req, err := http.NewRequestWithContext(ctx, method, validatedURL, reqBody) // #nosec G704 -- URL is parsed and validated with security.ValidateOutboundRequest
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

	if int64(len(bodyBytes)) > maxHTTPRequestBodyBytes {
		return nil, nil, fmt.Errorf("request body exceeds maximum size of %d bytes", maxHTTPRequestBodyBytes)
	}

	c.debugLogRequestBody(bodyBytes)

	return bytes.NewReader(bodyBytes), bodyBytes, nil
}

// debugLogRequestBody logs request body if debug mode is enabled
func (c *HTTPClient) debugLogRequestBody(bodyBytes []byte) {
	if c.cloneConfiguration().debug {
		c.debugLog("Request body: %s", redactDebugBody(bodyBytes))
	}
}

// setupRequestHeaders configures all necessary request headers
func (c *HTTPClient) setupRequestHeaders(req *http.Request, headers map[string]string, hasBody bool) {
	snapshot := c.cloneConfiguration()
	// Standard headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", snapshot.userAgent)

	// Custom headers first (allows overriding Content-Type)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Content type for requests with body (only if not already set by custom headers)
	if hasBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Authorization header
	if snapshot.authToken != "" {
		req.Header.Set("Authorization", formatAuthorizationHeader(snapshot.authToken))
	}
}

// executeRequestWithRetry handles the request execution with retry logic.
//
// Retries are enabled for unsafe methods (POST/PUT/PATCH/DELETE) when an
// idempotency key is present — either explicitly supplied by the caller
// via [sdkctx.WithIdempotencyKey] OR auto-generated by ensureIdempotencyHeader when
// client-level idempotency is enabled. The previous implementation only
// honored the caller-supplied form, which silently disabled retries for
// every auto-keyed unsafe request — exactly the workloads where retries
// matter most.
func (c *HTTPClient) executeRequestWithRetry(ctx context.Context, req *http.Request, method, requestURL string) (*http.Response, []byte, error) {
	snapshot := c.cloneConfiguration()

	callerProvidedIdempotency := req.Header.Get(internalCallerIdempotencyHeader) == BoolTrue
	autoIdempotency := req.Header.Get(internalAutoIdempotencyHeader) == BoolTrue
	hasIdempotencyKey := callerProvidedIdempotency || autoIdempotency

	// Strip the internal markers BEFORE the request goes on the wire — the
	// server must never see these synthetic headers.
	req.Header.Del(internalCallerIdempotencyHeader)
	req.Header.Del(internalAutoIdempotencyHeader)

	effectiveRetryOptions := cloneRetryOptions(snapshot.retryOptions)
	if effectiveRetryOptions == nil {
		effectiveRetryOptions = retry.DefaultOptions()
	}

	// Replace the previous "append a magic 'custom retryable' substring to
	// RetryableErrors" workaround with a typed predicate. The predicate
	// uses errors.As to recognise our internal sentinels — drastically
	// less brittle than substring matching against err.Error().
	effectiveRetryOptions.ErrorPredicate = isInternalRetryableError

	if isUnsafeMethod(req.Method) && !hasIdempotencyKey {
		effectiveRetryOptions.MaxRetries = 0
	}

	retryCtx := retry.WithOptionsContext(ctx, effectiveRetryOptions)

	// Install the per-attempt observability hook on the retry context.
	// The hook emits a structured slog line and records a metric for
	// every retry attempt — the pieces that used to be a // TODO.
	logger := snapshot.logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	metrics := snapshot.metrics

	hook := func(hookCtx context.Context, attempt int, cause error, delay time.Duration) {
		urlPath := normalizeTelemetryURLForLog(requestURL)

		causeMsg := ""
		statusCode := 0
		var statusErr interface{ StatusCode() int }
		if cause != nil {
			causeMsg = sdkerrors.RedactSensitiveString(cause.Error())
			if errors.As(cause, &statusErr) {
				statusCode = statusErr.StatusCode()
			}
		}

		attrs := []slog.Attr{
			slog.String("sdk.name", sdkLoggerName),
			slog.String("sdk.component", "retry"),
			slog.String("operation", method),
			slog.String("http.method", method),
			slog.String("url.path", urlPath),
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", effectiveRetryOptions.MaxRetries),
			slog.Int64("delay_ms", delay.Milliseconds()),
			slog.String("cause", causeMsg),
		}

		if statusCode > 0 {
			attrs = append(attrs, slog.Int("http.status_code", statusCode))
		}

		level := slog.LevelDebug
		if attempt >= effectiveRetryOptions.MaxRetries {
			// Final attempt before exhaustion → warn level so it shows up
			// in production log filters that suppress debug.
			level = slog.LevelWarn
		}

		logger.LogAttrs(hookCtx, level, "retrying request", attrs...)

		if metrics != nil {
			metrics.RecordRetry(hookCtx, method, "http", attempt)
		}
	}

	retryCtx = retry.WithAttemptHook(retryCtx, hook)

	execution := &retryExecution{}

	err := retry.DoWithContext(retryCtx, func() error {
		return c.executeRetryAttempt(req, method, requestURL, execution)
	})

	return execution.resp, execution.responseBody, err
}

// sdkLoggerName is the value emitted under the sdk.name structured log
// field. Constant so callers can rely on a stable string.
const sdkLoggerName = "midaz-go-sdk"

// normalizeTelemetryURLForLog strips dynamic IDs from a request URL so
// structured log lines have stable cardinality (good for log search and
// metric labels). Falls back to the raw URL on parse error.
func normalizeTelemetryURLForLog(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Replace path segments that look like UUIDs / numeric IDs with
	// placeholders. This matches the entity-id pattern used elsewhere
	// in the SDK's observability tooling.
	parts := strings.Split(parsed.Path, "/")
	for i, p := range parts {
		if isLikelyID(p) {
			parts[i] = ":id"
		}
	}

	parsed.Path = strings.Join(parts, "/")
	parsed.RawQuery = "" // never include query strings in log fields

	return parsed.String()
}

// isLikelyID returns true if the segment looks like an entity identifier
// (UUID-shaped or all digits).
func isLikelyID(segment string) bool {
	if segment == "" {
		return false
	}

	// UUID-shaped (36 chars with 4 hyphens)
	if len(segment) == 36 && strings.Count(segment, "-") == 4 {
		return true
	}

	// All digits
	for _, r := range segment {
		if r < '0' || r > '9' {
			return false
		}
	}

	return len(segment) > 0
}

// isInternalRetryableError matches the SDK's internal "always retryable"
// sentinel — currently only retryableCustomPolicyError, returned when the
// caller-supplied custom retry policy says so or when a 401 triggered a
// token refresh.
//
// retryableHTTPError is intentionally NOT matched here: that wrapper just
// carries an embedded HTTP status code, and whether to retry on a given
// status is driven by RetryableHTTPCodes via the StatusCode() interface
// path inside IsRetryableError. The retry layer already sees through the
// wrapper because retryableHTTPError exposes StatusCode().
func isInternalRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var customPolicy retryableCustomPolicyError
	return errors.As(err, &customPolicy)
}

type retryExecution struct {
	resp          *http.Response
	responseBody  []byte
	refreshedAuth bool
}

func (c *HTTPClient) executeRetryAttempt(req *http.Request, method, requestURL string, execution *retryExecution) error {
	if err := resetRequestBody(req); err != nil {
		return err
	}

	if err := security.ValidateOutboundRequest(req); err != nil {
		return fmt.Errorf("invalid request URL: %w", err)
	}

	snapshot := c.cloneConfiguration()
	client := c.snapshotHTTPClient()

	resp, err := client.Do(req) // #nosec G704 -- request URL validated via security.ValidateOutboundRequest
	if err != nil {
		return c.handleRequestExecutionError(method, requestURL, err)
	}

	// Drain-and-close on every code path so http.Transport can reuse the
	// underlying TCP connection. The deferred close alone is not enough —
	// when the body exceeds maxHTTPResponseBodyBytes the LimitReader
	// returns short and the unread tail starves the pool.
	defer drainAndCloseResponseBody(c, resp)

	execution.resp = resp

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBodyBytes+1))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(responseBody)) > maxHTTPResponseBodyBytes {
		return fmt.Errorf("response body exceeds %d bytes", maxHTTPResponseBodyBytes)
	}

	execution.responseBody = responseBody

	if resp.StatusCode == http.StatusUnauthorized && !execution.refreshedAuth && snapshot.tokenProvider != nil {
		token, refreshed := c.refreshAuthToken(req.Context(), snapshot)
		if refreshed {
			req.Header.Set("Authorization", formatAuthorizationHeader(token))
			execution.refreshedAuth = true

			return retryableCustomPolicyError{err: errors.New("access manager token refreshed after unauthorized response")}
		}
	}

	return c.handleRetryAttemptResponse(resp, responseBody, method, requestURL)
}

// refreshAuthToken obtains a fresh token via the configured tokenProvider.
// Concurrent callers that hit a 401 at roughly the same time funnel through
// a singleflight so the underlying provider call runs once. The token, once
// fetched, is written through setAuthTokenLocked so other in-flight
// requests will see it on their next header build.
//
// Returns the token and true on success; empty string and false when no
// fresh token could be obtained.
func (c *HTTPClient) refreshAuthToken(ctx context.Context, snapshot httpClientConfigSnapshot) (string, bool) {
	if snapshot.tokenProvider == nil {
		return "", false
	}

	// Use a stable singleflight key so all concurrent refreshers share one
	// underlying call. The key is intentionally derived from the auth
	// invalidator identity and the existing token so that two clients
	// pointing at different tenants don't collapse onto a single refresh.
	groupKey := "tokenrefresh"
	if snapshot.tokenInvalidator != nil {
		groupKey = fmt.Sprintf("tokenrefresh|%p", snapshot.tokenInvalidator)
	}

	result, err, _ := c.tokenRefreshGroup.Do(groupKey, func() (any, error) {
		if snapshot.tokenInvalidator != nil {
			snapshot.tokenInvalidator()
		}

		token, tokenErr := snapshot.tokenProvider(ctx)
		if tokenErr != nil {
			return "", tokenErr
		}

		return strings.TrimSpace(token), nil
	})
	if err != nil {
		return "", false
	}

	token, ok := result.(string)
	if !ok || token == "" {
		return "", false
	}

	c.setAuthTokenLocked(token)

	return token, true
}

// drainAndCloseResponseBody is the defense-in-depth body cleanup used by
// the retry path. It drains any unread tail (so the connection can return
// to the pool) and then closes the body. We surface the close error to
// the SDK debug log via the supplied client; non-debug callers still get
// the standard "best-effort cleanup" semantics.
func drainAndCloseResponseBody(c *HTTPClient, resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	// Discard whatever is left so http.Transport can reuse the keep-alive.
	// We bound the discard with the same response cap to avoid a hostile
	// server tying us up indefinitely.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxHTTPResponseBodyBytes))

	if c != nil {
		c.closeResponseBody(resp)
		return
	}

	_ = resp.Body.Close()
}

func (c *HTTPClient) snapshotHTTPClient() *http.Client {
	if c == nil {
		return defaultHTTPClient()
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client == nil {
		return defaultHTTPClient()
	}

	return c.client
}

func resetRequestBody(req *http.Request) error {
	if req.GetBody == nil {
		return nil
	}

	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("failed to reset request body for retry: %w", err)
	}

	req.Body = body

	return nil
}

func (c *HTTPClient) handleRequestExecutionError(method, requestURL string, err error) error {
	c.debugLogRequestError(method, requestURL, err)

	requestErr := fmt.Errorf("HTTP request failed: %w", err)

	snapshot := c.cloneConfiguration()
	if snapshot.customRetryPolicy != nil {
		if snapshot.customRetryPolicy(nil, requestErr) {
			return retryableCustomPolicyError{err: requestErr}
		}

		return retry.AsNonRetryable(requestErr)
	}

	return requestErr
}

func (c *HTTPClient) handleRetryAttemptResponse(resp *http.Response, responseBody []byte, method, requestURL string) error {
	if resp.StatusCode < 400 {
		return nil
	}

	requestID := resp.Header.Get("X-Request-ID")

	apiErr := c.handleErrorResponse(resp.StatusCode, responseBody, method, requestURL, requestID)

	snapshot := c.cloneConfiguration()
	if snapshot.customRetryPolicy != nil {
		if snapshot.customRetryPolicy(resp, apiErr) {
			return retryableCustomPolicyError{err: apiErr}
		}

		return retry.AsNonRetryable(apiErr)
	}

	return retryableHTTPError{err: apiErr, statusCode: resp.StatusCode}
}

type retryableHTTPError struct {
	err        error
	statusCode int
}

func (e retryableHTTPError) Error() string {
	return e.err.Error()
}

func (e retryableHTTPError) Unwrap() error {
	return e.err
}

// StatusCode returns the HTTP status code that made the error retryable.
func (e retryableHTTPError) StatusCode() int {
	return e.statusCode
}

type retryableCustomPolicyError struct {
	err error
}

func (e retryableCustomPolicyError) Error() string {
	return "custom retryable: " + e.err.Error()
}

func (e retryableCustomPolicyError) Unwrap() error {
	return e.err
}

// closeResponseBody safely closes response body with debug logging
func (c *HTTPClient) closeResponseBody(resp *http.Response) {
	if closeErr := resp.Body.Close(); closeErr != nil && c.cloneConfiguration().debug {
		c.debugLog("Failed to close response body: %v", closeErr)
	}
}

// handleErrorResponse processes API error responses
func (c *HTTPClient) handleErrorResponse(statusCode int, responseBody []byte, method, requestURL, requestID string) error {
	apiErr := c.parseErrorResponse(statusCode, responseBody, requestID)

	if c.cloneConfiguration().debug {
		c.debugLog("HTTP Error response from: %s %s", method, redactDebugURL(requestURL))
		c.debugLog("Error status: %d", statusCode)

		if requestID != "" {
			c.debugLog("Request ID: %s", requestID)
		}

		c.debugLog("Error body: %s", redactDebugBody(responseBody))
		c.debugLog("Parsed error: %v", apiErr)
	}

	return apiErr
}

// debugLogRequestError logs request failures in debug mode
func (c *HTTPClient) debugLogRequestError(method, requestURL string, err error) {
	if c.cloneConfiguration().debug {
		c.debugLog("HTTP request failed: %s %s - %v", method, redactDebugURL(requestURL), err)
	}
}

// recordRequestMetrics records performance metrics if enabled
func (c *HTTPClient) recordRequestMetrics(ctx context.Context, method, requestURL string, resp *http.Response, elapsed time.Duration) {
	enrichHTTPSpan(ctx, method, requestURL, resp, nil)

	snapshot := c.cloneConfiguration()
	if snapshot.metrics != nil && resp != nil {
		snapshot.metrics.RecordRequest(ctx, method, normalizeTelemetryURL(requestURL), resp.StatusCode, elapsed)
	}

	c.maybeLogSlowCall(ctx, snapshot, method, requestURL, resp, elapsed)
}

// maybeLogSlowCall emits a Warn-level structured log line when the call
// duration exceeds the configured WithSlowCallThreshold. Zero (default) or
// negative thresholds disable the warning. The line uses the same field
// schema as retry logs for consistent dashboards.
//
// Receiver is unused (intentionally a method for symmetry with the rest of
// the recordRequest* family) — silenced via _ to satisfy revive.
func (*HTTPClient) maybeLogSlowCall(ctx context.Context, snapshot httpClientConfigSnapshot, method, requestURL string, resp *http.Response, elapsed time.Duration) {
	threshold := snapshot.slowCallThreshold
	if threshold <= 0 || elapsed < threshold {
		return
	}

	logger := snapshot.logger
	if logger == nil {
		// No logger means nowhere to emit. Threshold without a logger
		// is a harmless no-op (covered by the WithSlowCallThreshold
		// godoc note).
		return
	}

	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}

	requestID := ""
	if resp != nil {
		requestID = resp.Header.Get("X-Request-ID")
	}

	logger.LogAttrs(ctx, slog.LevelWarn, "slow API call",
		slog.String("sdk.name", sdkLoggerName),
		slog.String("sdk.component", "http"),
		slog.String("operation", method),
		slog.String("http.method", method),
		slog.String("url.path", normalizeTelemetryURLForLog(requestURL)),
		slog.Int("http.status_code", statusCode),
		slog.Int64("duration_ms", elapsed.Milliseconds()),
		slog.Int64("threshold_ms", threshold.Milliseconds()),
		slog.String("request_id", requestID),
	)
}

func (c *HTTPClient) recordRequestFailure(ctx context.Context, method, requestURL string, resp *http.Response, elapsed time.Duration, err error) {
	enrichHTTPSpan(ctx, method, requestURL, resp, err)

	statusCode := http.StatusInternalServerError
	if resp != nil {
		statusCode = resp.StatusCode
	}

	c.recordFailure(ctx, method, requestURL, statusCode, elapsed, err)
}

func (c *HTTPClient) recordSDKFailure(ctx context.Context, method, requestURL string, elapsed time.Duration, err error) {
	enrichHTTPSpan(ctx, method, requestURL, nil, err)

	c.recordFailure(ctx, method, requestURL, http.StatusInternalServerError, elapsed, err)
}

func (c *HTTPClient) recordFailure(ctx context.Context, method, requestURL string, statusCode int, elapsed time.Duration, err error) {
	snapshot := c.cloneConfiguration()
	if snapshot.metrics != nil {
		snapshot.metrics.RecordRequest(ctx, method, normalizeTelemetryURL(requestURL), statusCode, elapsed)
	}

	if snapshot.observability != nil && snapshot.observability.IsEnabled() {
		observability.RecordError(ctx, err, "http_request_failed")
	}
}

// logResponseDetails logs response information in debug mode
func (c *HTTPClient) logResponseDetails(method, requestURL string, resp *http.Response, responseBody []byte) {
	if !c.cloneConfiguration().debug {
		return
	}

	c.debugLog("Response from: %s %s", method, redactDebugURL(requestURL))
	c.debugLog("Response status: %d", resp.StatusCode)
	c.debugLog("Response headers: %v", redactHeaders(resp.Header))
	c.debugLog("Response body: %s", redactDebugBody(responseBody))
}

// processResponse handles JSON unmarshaling of the response
func (c *HTTPClient) processResponse(result any, responseBody []byte) error {
	if result == nil {
		return nil
	}

	if len(responseBody) == 0 {
		return errEmptyResponseBody
	}

	if bytes.Equal(bytes.TrimSpace(responseBody), []byte("null")) {
		return errNullResponseBody
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

	// Extract the raw body from the request so we can preserve the original payload
	// without forcing a JSON decode/re-encode cycle.
	body, err := c.extractRequestBody(req)
	if err != nil {
		return err
	}

	if len(body) > 0 && strings.TrimSpace(headers["Content-Type"]) == "" {
		headers["Content-Type"] = "application/json"
	}

	// Use the context from the request
	ctx := req.Context()

	// Reuse the raw-request path to preserve the original body bytes.
	return c.doRawRequest(ctx, method, requestURL, headers, body, v)
}

func cloneRetryOptions(options *retry.Options) *retry.Options {
	if options == nil {
		return nil
	}

	cloned := *options
	cloned.RetryableErrors = append([]string(nil), options.RetryableErrors...)
	cloned.RetryableHTTPCodes = append([]int(nil), options.RetryableHTTPCodes...)

	return &cloned
}

func formatAuthorizationHeader(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return token
	}

	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return token
	}

	return "Bearer " + token
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// ensureIdempotencyHeader injects an auto-generated X-Idempotency key when
// client-level idempotency is enabled and the caller hasn't already
// supplied one. The per-call suppression escape hatch
// ([sdkctx.WithoutAutoIdempotency]) is honored here — when the context
// carries the suppression flag we leave the headers unchanged so the
// request goes out without an idempotency key.
//
// Ordering: explicit caller key > context-level suppression > auto-gen.
func (c *HTTPClient) ensureIdempotencyHeader(ctx context.Context, method string, headers map[string]string) map[string]string {
	if !c.cloneConfiguration().enableIdempotency || !isUnsafeMethod(method) {
		return headers
	}

	if headers != nil && strings.TrimSpace(headers["X-Idempotency"]) != "" {
		// Caller-provided key wins regardless of suppression.
		return headers
	}

	if autoIdempotencySuppressed(ctx) {
		// Caller asked us not to auto-generate for this call. Leave the
		// request alone.
		return headers
	}

	if headers == nil {
		headers = map[string]string{}
	}

	headers["X-Idempotency"] = uuid.NewString()
	headers[internalAutoIdempotencyHeader] = BoolTrue

	return headers
}

// extractRequestBody reads and returns the raw request body bytes.
//
// The reader is wrapped in an io.LimitReader bounded to
// maxHTTPRequestBodyBytes+1 so an oversized body cannot pin arbitrary
// memory before we get a chance to enforce the limit. After ReadAll
// returns we compare the actual length against the cap; anything past it
// surfaces the same "request body too large" error as the post-buffered
// check did before, but without ever buffering the offending payload.
func (c *HTTPClient) extractRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	limited := io.LimitReader(req.Body, maxHTTPRequestBodyBytes+1)

	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}

	if int64(len(bodyBytes)) > maxHTTPRequestBodyBytes {
		return nil, fmt.Errorf("request body exceeds maximum size of %d bytes", maxHTTPRequestBodyBytes)
	}

	if closeErr := req.Body.Close(); closeErr != nil && c.cloneConfiguration().debug {
		c.debugLog("Failed to close request body: %v", closeErr)
	}

	if len(bodyBytes) == 0 {
		return nil, nil
	}

	return bodyBytes, nil
}

// sanitizeLogInput removes control characters from strings to prevent log injection attacks.
// This sanitizes newlines, carriage returns, and other control characters that could
// allow attackers to forge log entries. Uses strconv.Quote for proper escaping of all
// control characters, then strips the surrounding quotes for cleaner log output.
func sanitizeLogInput(input string) string {
	// Use strconv.Quote which properly escapes all control characters
	// This is a standard library function recognized by security scanners
	quoted := strconv.Quote(input)
	// Remove the surrounding quotes added by Quote
	if len(quoted) >= 2 {
		return quoted[1 : len(quoted)-1]
	}

	return input
}

func redactHeaders(headers http.Header) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for key, values := range headers {
		switch strings.ToLower(key) {
		case "authorization", "cookie", "set-cookie", "x-idempotency", strings.ToLower(HeaderTenantID):
			redacted[key] = []string{"[REDACTED]"}
		default:
			copied := make([]string, len(values))
			copy(copied, values)
			redacted[key] = copied
		}
	}

	return redacted
}

func redactDebugBody(body []byte) string {
	if len(body) == 0 {
		return "[empty]"
	}

	return fmt.Sprintf("[REDACTED len=%d]", len(body))
}

func normalizeTelemetryURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	segments := strings.Split(parsedURL.Path, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}

		if isLikelyTelemetryIdentifier(segment) {
			segments[i] = ":id"
		}
	}

	parsedURL.Path = strings.Join(segments, "/")

	return parsedURL.String()
}

func redactDebugURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	for key := range query {
		if isSensitiveQueryKey(key) {
			query.Set(key, "[REDACTED]")
		}
	}

	parsedURL.RawQuery = query.Encode()
	parsedURL.Fragment = ""

	return parsedURL.String()
}

func isSensitiveQueryKey(key string) bool {
	normalized := strings.ToLower(key)
	if strings.Contains(normalized, "document") || strings.Contains(normalized, "metadata") {
		return true
	}

	switch normalized {
	case "banking_details_account", "banking_details_iban", "external_id":
		return true
	default:
		return false
	}
}

func isLikelyTelemetryIdentifier(segment string) bool {
	if len(segment) == 36 && strings.Count(segment, "-") == 4 {
		return true
	}

	allDigits := true
	hasDigit := false

	for _, r := range segment {
		if r < '0' || r > '9' {
			allDigits = false
		} else {
			hasDigit = true
		}
	}

	if allDigits && len(segment) > 3 {
		return true
	}

	if len(segment) >= 16 && hasDigit {
		return true
	}

	return false
}

// sanitizeLogArgs sanitizes all string arguments to prevent log injection.
func sanitizeLogArgs(args []any) []any {
	sanitized := make([]any, len(args))
	for i, arg := range args {
		if s, ok := arg.(string); ok {
			sanitized[i] = sanitizeLogInput(s)
		} else {
			sanitized[i] = arg
		}
	}

	return sanitized
}

// debugLog routes a debug-level message through the configured *slog.Logger
// when WithDebug(true) is enabled. v3 contract: there is exactly one log
// path. The MIDAZ_DEBUG-bypass that wrote raw text to stderr in v2 is gone.
//
// All string arguments are sanitized to prevent log injection attacks; the
// pre-formatted message is wrapped as a single 'message' attribute so the
// underlying handler can structure it appropriately (JSON keeps the literal
// message; text handlers emit it as the msg field).
func (c *HTTPClient) debugLog(format string, args ...any) {
	snapshot := c.cloneConfiguration()
	if !snapshot.debug {
		return
	}

	logger := snapshot.logger
	if logger == nil {
		// HTTPClient created outside of midaz.New (e.g., test or
		// access-manager bootstrap). v3 silent default.
		return
	}

	// Sanitize arguments and pre-format message to prevent log injection.
	// Pre-formatting breaks the taint chain by creating a new string value
	// that is not directly derived from user input in the eyes of static analysis.
	sanitizedArgs := sanitizeLogArgs(args)
	message := fmt.Sprintf(format, sanitizedArgs...)

	// Log injection mitigated: all arguments are sanitized via strconv.Quote in sanitizeLogArgs()
	// which escapes all control characters including \n, \r, \t, and non-printable chars.
	logger.Debug(message,
		slog.String("sdk.name", sdkLoggerName),
		slog.String("sdk.component", "http"),
	)
}

// parseErrorResponse parses an error response from the API and converts it to an SDK error.
func (*HTTPClient) parseErrorResponse(statusCode int, body []byte, requestID string) error {
	// If there's no body, create a generic error
	if len(body) == 0 {
		return sdkerrors.ErrorFromHTTPResponse(statusCode, requestID, "Empty response from server", "", "", "")
	}

	// Try to parse the error body as a JSON object
	var apiError struct {
		Error      string         `json:"error"`
		Message    string         `json:"message"`
		Code       string         `json:"code"`
		Title      string         `json:"title"`
		EntityType string         `json:"entityType"`
		Fields     any            `json:"fields"`
		Details    map[string]any `json:"details"`
		Err        any            `json:"err"`
	}

	if err := json.Unmarshal(body, &apiError); err != nil {
		message := fmt.Sprintf("API returned non-JSON error response with status code %d and body length %d", statusCode, len(body))
		return sdkerrors.ErrorFromHTTPResponse(statusCode, requestID, message, "", "", "")
	}

	details := apiError.Details
	if details == nil {
		details = map[string]any{}
	}

	if apiError.Err != nil {
		details["err"] = apiError.Err
	}

	details = sdkerrors.RedactSensitiveDetails(details)

	// Use the message if available, otherwise use the error field
	message := apiError.Message
	if message == "" {
		message = apiError.Error
	}

	// If there's still no message, use a default one
	if message == "" {
		message = fmt.Sprintf("API error with status code %d", statusCode)
	}

	message = sdkerrors.RedactSensitiveString(message)
	title := sdkerrors.RedactSensitiveString(apiError.Title)

	fields := normalizeAPIErrorFields(apiError.Fields)
	if len(fields) > 0 {
		details["fields"] = apiError.Fields
		details = sdkerrors.RedactSensitiveDetails(details)
	}

	// Create the appropriate error type based on the status code
	return sdkerrors.ErrorFromHTTPResponseWithDetails(
		statusCode,
		requestID,
		message,
		apiError.Code,
		apiError.EntityType,
		"",
		title,
		fields,
		details,
	)
}

func normalizeAPIErrorFields(fields any) []string {
	switch typed := fields.(type) {
	case nil:
		return nil
	case []string:
		return sdkerrors.RedactSensitiveStringSlice(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, field := range typed {
			if text, ok := field.(string); ok {
				out = append(out, text)
			}
		}

		return sdkerrors.RedactSensitiveStringSlice(out)
	case map[string]any:
		out := make([]string, 0, len(typed))
		for field := range typed {
			out = append(out, field)
		}

		return sdkerrors.RedactSensitiveStringSlice(out)
	default:
		return nil
	}
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
// It uses context.Background for backward compatibility. Use NewRequestWithContext
// when cancellation, deadlines, or request-scoped values are required.
func (c *HTTPClient) NewRequest(method, requestURL string, body any) (*http.Request, error) {
	return c.NewRequestWithContext(context.Background(), method, requestURL, body)
}

// NewRequestWithContext creates a new HTTP request with the given context, method, URL, and body.
func (c *HTTPClient) NewRequestWithContext(ctx context.Context, method, requestURL string, body any) (*http.Request, error) {
	if c == nil {
		return nil, errors.New("HTTP client cannot be nil")
	}

	if ctx == nil {
		return nil, errors.New("request context cannot be nil")
	}

	var bodyReader io.Reader

	if body != nil {
		// Serialize the request body to JSON
		bodyBytes, err := c.jsonPool.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}

		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add standard headers
	req.Header.Set("Accept", "application/json")

	snapshot := c.cloneConfiguration()
	req.Header.Set("User-Agent", snapshot.userAgent)

	// Add content type if there's a body
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add authorization if there's a token
	if snapshot.authToken != "" {
		req.Header.Set("Authorization", formatAuthorizationHeader(snapshot.authToken))
	}

	return req, nil
}
