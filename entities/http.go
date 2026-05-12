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
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/retry"
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
	httpPhaseRequestBuild    = "request_build"
	httpPhaseRequestValidate = "request_validate"
	httpPhaseRequestMarshal  = "request_marshal"
	httpPhaseRequestSend     = "request_send"
	httpPhaseResponseRead    = "response_read"
	httpPhaseResponseDecode  = "response_decode"
)

const (
	maxHTTPResponseBodyBytes = int64(10 << 20)
	maxHTTPRequestBodyBytes  = int64(10 << 20)
	quotedStringMinLength    = 2
)

const (
	internalCallerIdempotencyHeader = "X-Midaz-Caller-Idempotency"
	internalAutoIdempotencyHeader   = "X-Midaz-Auto-Idempotency"
)

// defaultUserAgent returns the SDK's centralized user-agent string.
// The configured value flows in via (*HTTPClient).SetUserAgent (driven
// from pkg/config.Config.UserAgent, which itself can be populated from
// the MIDAZ_USER_AGENT env var when the caller opts in via FromEnvironment).
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
	mu        sync.RWMutex
	client    *http.Client
	authToken string
	// Atomic primitives for the hot-path scalar knobs. These three fields
	// are read on every request (header build, debug gating, idempotency
	// gating) but mutated rarely (only via setters called from midaz.New
	// or test plumbing). Going atomic eliminates the RWMutex Lock/Unlock
	// pair from the per-request read path — a measurable win under retry
	// fan-out workloads. The atomic.Pointer[string] for userAgent stores a
	// pointer-to-string so the read path doesn't have to copy the bytes;
	// a nil pointer means "fall back to defaultUserAgent()".
	userAgent         atomic.Pointer[string]
	debug             atomic.Bool
	enableIdempotency atomic.Bool
	retryOptions      *retry.Options        // Retry options for the client
	jsonPool          *performance.JSONPool // Pool for JSON encoding/decoding
	metrics           *observability.MetricsCollector
	observability     observability.Provider
	customRetryPolicy func(*http.Response, error) bool
	tokenProvider     func(context.Context) (string, error)
	tokenInvalidator  func()
	// logger is the canonical *slog.Logger for retry/slow-call/internal
	// warnings. Always non-nil after midaz.New() wires the parent client's
	// logger via SetLogger. Every entity service shares the same *HTTPClient,
	// so SetLogger calls take effect on every service's next request.
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
// flows in exclusively through pkg/config.Config and the SetDebug / SetUserAgent /
// SetEnableIdempotency / WithRetryOptions setters (plus (*Entity).SetObservability
// for observability). Callers who want env-driven configuration must opt in via
// config.NewConfig(config.FromEnvironment()).
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

	c := &HTTPClient{
		client:        client,
		authToken:     authToken,
		retryOptions:  retry.DefaultOptions(),
		jsonPool:      performance.NewJSONPool(),
		metrics:       initMetricsCollector(provider),
		observability: provider,
	}

	defaultUA := defaultUserAgent()
	c.userAgent.Store(&defaultUA)
	c.enableIdempotency.Store(true)

	return c
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
			MaxConnsPerHost:       100,
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
//
// Each provided option is applied on top of the *currently configured*
// policy (cloned under the mutex), so callers can tune one knob —
// e.g. WithMaxRetries — without losing prior settings loaded from
// config or set by earlier calls. If no policy has been configured
// yet, retry.DefaultOptions() is used as the base.
func (c *HTTPClient) WithRetryOptions(options ...retry.Option) error {
	if c == nil {
		return errors.New("HTTP client cannot be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Start from the current policy so client-level overrides do not
	// discard retry settings already loaded from config.
	retryOpts := cloneRetryOptions(c.retryOptions)
	if retryOpts == nil {
		retryOpts = retry.DefaultOptions()
	}

	// Apply all options
	for i, opt := range options {
		if opt == nil {
			return fmt.Errorf("retry option at index %d cannot be nil", i)
		}

		if err := opt(retryOpts); err != nil {
			return fmt.Errorf("retry option at index %d failed: %w", i, err)
		}
	}

	c.retryOptions = retryOpts

	return nil
}

// SetLogger installs the *slog.Logger used for retry/slow-call/internal
// warnings. Passing nil reverts to a discard handler. Because every entity
// service shares the same *HTTPClient, calling SetLogger here updates the
// logger seen by all 16 services on their next request.
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
//
// Lock-free: backed by an atomic.Bool so the request path can read this
// flag without contending with the HTTPClient mutex.
func (c *HTTPClient) SetEnableIdempotency(enabled bool) {
	if c == nil {
		return
	}

	c.enableIdempotency.Store(enabled)
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

// cloneConfiguration captures a value-copy of the HTTPClient's read-mostly
// state for a single request scope. The hot-path scalars (debug, userAgent,
// enableIdempotency) come from atomic loads; the remaining pointer fields
// are read under the HTTPClient RLock.
//
// retryOptions is captured BY POINTER (no per-snapshot slice clone). The
// per-request mutation site in [executeRequestWithRetry] does its own
// clone-then-mutate before passing the policy to the retry layer; everything
// else only reads retryOptions, so a shared pointer is safe.
func (c *HTTPClient) cloneConfiguration() httpClientConfigSnapshot {
	if c == nil {
		return httpClientConfigSnapshot{}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return httpClientConfigSnapshot{
		authToken:         c.authToken,
		userAgent:         c.loadUserAgent(),
		debug:             c.debug.Load(),
		retryOptions:      c.retryOptions,
		metrics:           c.metrics,
		observability:     c.observability,
		enableIdempotency: c.enableIdempotency.Load(),
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
	c.authToken = snapshot.authToken
	c.retryOptions = cloneRetryOptions(snapshot.retryOptions)
	c.metrics = snapshot.metrics
	c.observability = snapshot.observability
	c.customRetryPolicy = snapshot.customRetryPolicy
	c.tokenProvider = snapshot.tokenProvider
	c.tokenInvalidator = snapshot.tokenInvalidator
	c.logger = snapshot.logger
	c.slowCallThreshold = snapshot.slowCallThreshold
	c.mu.Unlock()

	// Atomic fields live outside the mutex; copy them through their typed
	// stores so concurrent readers always see a consistent value.
	c.debug.Store(snapshot.debug)
	c.enableIdempotency.Store(snapshot.enableIdempotency)
	ua := snapshot.userAgent
	c.userAgent.Store(&ua)
}

// SetDebug enables or disables debug-mode logging for the HTTP client.
//
// Lock-free: backed by an atomic.Bool so the request path can flip this
// flag without contending with the HTTPClient mutex.
func (c *HTTPClient) SetDebug(debug bool) {
	if c == nil {
		return
	}

	c.debug.Store(debug)
}

// SetUserAgent sets the User-Agent header value sent on every outbound
// request issued by this HTTPClient.
//
// Lock-free: backed by an atomic.Pointer[string]. Safe to call
// concurrently with in-flight requests.
func (c *HTTPClient) SetUserAgent(userAgent string) {
	if c == nil {
		return
	}

	// Snapshot the value into a fresh allocation so subsequent mutations
	// by the caller cannot race with readers holding the previous pointer.
	ua := userAgent
	c.userAgent.Store(&ua)
}

// loadUserAgent returns the configured user-agent string, falling back to
// the SDK default when none has been set.
func (c *HTTPClient) loadUserAgent() string {
	if c == nil {
		return defaultUserAgent()
	}

	if ptr := c.userAgent.Load(); ptr != nil {
		return *ptr
	}

	return defaultUserAgent()
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

// setAuthTokenLocked sets the auth token under the write lock. Used by
// the access-manager token-refresh path (refreshAuthToken) and the initial
// token seeding inside NewEntityWithConfig. The auth token is read on every
// request to populate the Authorization header, so concurrent mutation
// absolutely must go through the same lock the readers use.
func (c *HTTPClient) setAuthTokenLocked(token string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.authToken = token
}

// injectContextHeaders adds context-based headers (idempotency key) to the provided
// headers map. If headers is nil and there are headers to inject, a new map is created and returned.
//
// Idempotency key precedence (first non-empty source wins):
//  1. Caller-supplied input field — service methods that accept an
//     IdempotencyKey on their input struct (e.g., CreateTransactionInput)
//     write it directly into headers along with the
//     [internalCallerIdempotencyHeader] marker. That marker tells this
//     function the caller has spoken; we MUST NOT overwrite the value here.
//  2. ctx-supplied via [sdkctx.WithIdempotencyKey] — request-scoped override,
//     used when the input struct doesn't carry the field or the caller wants
//     to propagate a key across a chain of calls.
//  3. Auto-generated UUID — applied later in [ensureIdempotencyHeader] when
//     client-level idempotency is enabled and neither (1) nor (2) supplied a
//     key.
//
// For a ledger SDK, getting this ordering wrong is the difference between
// proper dedup and double-bookkeeping under retries — the input-level key
// is the caller's most explicit assertion of "this transaction has key X"
// and must not be silently replaced.
func (*HTTPClient) injectContextHeaders(ctx context.Context, method string, headers map[string]string) map[string]string {
	if ctx == nil {
		ctx = context.Background()
	}

	// Inject idempotency header from context only for unsafe methods. Safe GET/HEAD
	// list/count requests are not idempotency participants and must not leak keys.
	if key := getIdempotencyKeyFromContext(ctx); isUnsafeMethod(method) && key != "" {
		if headers == nil {
			headers = map[string]string{}
		}

		// Caller-supplied X-Idempotency wins over ctx-supplied key. Any
		// non-empty header value is treated as caller-supplied — the
		// internal marker is no longer required, so callers that set the
		// header directly via the headers map are honored too.
		callerSupplied := strings.TrimSpace(headers["X-Idempotency"]) != ""
		if !callerSupplied {
			headers["X-Idempotency"] = key
			headers[internalCallerIdempotencyHeader] = boolTrue
		}
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
		c.logHTTPPhaseFailure(ctx, method, requestURL, nil, err)
		c.recordSDKFailure(ctx, method, requestURL, 0, err)

		return err
	}

	// Inject context-based headers (idempotency key)
	headers = c.injectContextHeaders(ctx, method, headers)
	headers = c.ensureIdempotencyHeader(ctx, method, headers)

	// Setup headers
	c.setupRequestHeaders(req, headers, body != nil)

	c.injectTraceContext(ctx, req)

	// Execute request with retry logic and capture elapsed time
	start := time.Now()
	resp, responseBody, maxRetries, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		c.logHTTPTerminalFailure(ctx, method, requestURL, resp, err, maxRetries)
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
		decodeErr := wrapHTTPPhaseError(httpPhaseResponseDecode, true, err)
		c.logHTTPPhaseFailure(ctx, method, requestURL, resp, decodeErr)

		if errors.Is(err, errEmptyResponseBody) || errors.Is(err, errNullResponseBody) {
			c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

			return decodeErr
		}

		c.recordRequestFailure(ctx, method, requestURL, resp, elapsed, decodeErr)

		return decodeErr
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
		c.logHTTPPhaseFailure(ctx, method, requestURL, nil, err)
		c.recordSDKFailure(ctx, method, requestURL, 0, err)

		return err
	}

	// Set GetBody for retry support - allows body to be recreated on retries
	if len(body) > 0 {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}

	// Inject context-based headers (idempotency key)
	headers = c.injectContextHeaders(ctx, method, headers)
	headers = c.ensureIdempotencyHeader(ctx, method, headers)

	c.setupRequestHeaders(req, headers, len(body) > 0)

	c.injectTraceContext(ctx, req)

	start := time.Now()
	resp, responseBody, maxRetries, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		c.logHTTPTerminalFailure(ctx, method, requestURL, resp, err, maxRetries)
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
		decodeErr := wrapHTTPPhaseError(httpPhaseResponseDecode, true, err)
		c.logHTTPPhaseFailure(ctx, method, requestURL, resp, decodeErr)

		if errors.Is(err, errEmptyResponseBody) || errors.Is(err, errNullResponseBody) {
			c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

			return decodeErr
		}

		c.recordRequestFailure(ctx, method, requestURL, resp, elapsed, decodeErr)

		return decodeErr
	}

	c.recordRequestMetrics(ctx, method, requestURL, resp, elapsed)

	return nil
}

func prepareRawRequestBody(headers map[string]string, body []byte) (io.Reader, map[string]string, error) {
	if len(body) == 0 {
		return nil, headers, nil
	}

	if int64(len(body)) > maxHTTPRequestBodyBytes {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("request body exceeds maximum size of %d bytes", maxHTTPRequestBodyBytes))
	}

	if headers == nil {
		headers = map[string]string{}
	}

	if strings.TrimSpace(headers["Content-Type"]) == "" {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, errors.New("content-type header required for non-empty request body"))
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
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("failed to parse request URL: %w", err))
	}

	validationReq := &http.Request{URL: parsedURL}
	if err := security.ValidateOutboundRequest(validationReq); err != nil {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestValidate, false, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), reader) // #nosec G704 -- request URL validated via security.ValidateOutboundRequest using parsed URL
	if err != nil {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("failed to create request: %w", err))
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
		buildErr := wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("failed to create request: %w", err))
		c.logHTTPPhaseFailure(ctx, method, requestURL, nil, buildErr)
		c.recordSDKFailure(ctx, method, requestURL, 0, buildErr)

		return 0, buildErr
	}

	if err := security.ValidateOutboundRequest(req); err != nil {
		validateErr := wrapHTTPPhaseError(httpPhaseRequestValidate, false, err)
		c.logHTTPPhaseFailure(ctx, method, requestURL, nil, validateErr)
		c.recordSDKFailure(ctx, method, requestURL, 0, validateErr)

		return 0, validateErr
	}

	headers = c.injectContextHeaders(ctx, method, headers)
	c.setupRequestHeaders(req, headers, false)

	c.injectTraceContext(ctx, req)

	start := time.Now()
	resp, responseBody, maxRetries, err := c.executeRequestWithRetry(ctx, req, method, requestURL)
	elapsed := time.Since(start)

	if err != nil {
		c.logHTTPTerminalFailure(ctx, method, requestURL, resp, err, maxRetries)
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
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("failed to parse request URL: %w", err))
	}

	if err := security.ValidateOutboundRequest(&http.Request{URL: parsedURL}); err != nil {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestValidate, false, fmt.Errorf("invalid request URL: %w", err))
	}

	reqBody, bodyBytes, err := c.prepareRequestBody(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare request body: %w", err)
	}

	validatedURL := parsedURL.String()

	req, err := http.NewRequestWithContext(ctx, method, validatedURL, reqBody) // #nosec G704 -- URL is parsed and validated with security.ValidateOutboundRequest
	if err != nil {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("failed to create request: %w", err))
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
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestMarshal, false, err)
	}

	if int64(len(bodyBytes)) > maxHTTPRequestBodyBytes {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestMarshal, false, fmt.Errorf("request body exceeds maximum size of %d bytes", maxHTTPRequestBodyBytes))
	}

	c.debugLogRequestBody(bodyBytes)

	return bytes.NewReader(bodyBytes), bodyBytes, nil
}

// debugLogRequestBody logs request body if debug mode is enabled
func (c *HTTPClient) debugLogRequestBody(bodyBytes []byte) {
	if c != nil && c.debug.Load() {
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
func (c *HTTPClient) executeRequestWithRetry(ctx context.Context, req *http.Request, method, requestURL string) (*http.Response, []byte, int, error) {
	snapshot := c.cloneConfiguration()

	autoIdempotency := req.Header.Get(internalAutoIdempotencyHeader) == boolTrue
	hasIdempotencyKey := strings.TrimSpace(req.Header.Get("X-Idempotency")) != "" || autoIdempotency

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
		causeMsg := ""
		var statusErr interface{ StatusCode() int }
		if cause != nil {
			causeMsg = safeLogError(cause)
		}

		attrs := []slog.Attr{
			slog.String("sdk.name", sdkLoggerName),
			slog.String("sdk.component", "retry"),
		}
		attrs = append(attrs, safeHTTPLogAttrs(method, requestURL, nil, true)...)
		attrs = append(attrs,
			slog.Int("attempt", attempt),
			slog.Int("max_retries", effectiveRetryOptions.MaxRetries),
			slog.Int64("delay_ms", delay.Milliseconds()),
			slog.String("cause", causeMsg),
		)

		if errors.As(cause, &statusErr) && statusErr.StatusCode() > 0 {
			attrs = append(attrs, slog.Int("http.status_code", statusErr.StatusCode()))
		}
		if requestID := requestIDFromError(cause); requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}
		if category := errorCategory(cause); category != "" {
			attrs = append(attrs, slog.String("error.category", category))
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

	execution := &retryExecution{maxRetries: effectiveRetryOptions.MaxRetries}

	err := retry.DoWithContext(retryCtx, func() error {
		return c.executeRetryAttempt(req, method, requestURL, execution)
	})
	// Terminal logging is centralised in [doRequest] / [doRawRequest] /
	// [doCountRequest] via [logHTTPTerminalFailure]. This function only
	// ferries the (resp, body, max_retries, err) tuple back; the caller
	// decides which terminal log applies based on error shape.

	return execution.resp, execution.responseBody, execution.maxRetries, err
}

// sdkLoggerName is the value emitted under the sdk.name structured log
// field. Constant so callers can rely on a stable string.
const sdkLoggerName = "midaz-go-sdk"

// isInternalRetryableError matches the SDK's internal "always retryable"
// sentinel — currently only retryableCustomPolicyError, returned when the
// caller-supplied custom retry policy says so.
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
	resp         *http.Response
	responseBody []byte
	// maxRetries is the effective retry budget (after the unsafe-no-key
	// coercion to 0). Stored here so [logHTTPTerminalFailure] in the
	// caller can emit "max_retries"/"attempts" attributes without
	// re-deriving the value.
	maxRetries int
	// refreshedAuth latches true once a 401 has triggered a successful token
	// refresh for this request. It prevents a second refresh attempt within
	// the same request even across the inline auth-refresh loop.
	refreshedAuth bool
	// justRefreshedAuth is the per-iteration signal from performSingleAttempt
	// to executeRetryAttempt: "I just refreshed the token, loop once more
	// with the fresh credentials." It is reset between iterations.
	justRefreshedAuth bool
}

type httpPhaseError struct {
	phase       string
	requestSent bool
	err         error
}

func wrapHTTPPhaseError(phase string, requestSent bool, err error) error {
	if err == nil {
		return nil
	}

	var phaseErr *httpPhaseError
	if errors.As(err, &phaseErr) && phaseErr != nil {
		return err
	}

	return &httpPhaseError{phase: phase, requestSent: requestSent, err: err}
}

func (e *httpPhaseError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}

	return e.err.Error()
}

func (e *httpPhaseError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

func phaseFromError(err error) (phase string, requestSent bool, ok bool) {
	var phaseErr *httpPhaseError
	if errors.As(err, &phaseErr) && phaseErr != nil {
		return phaseErr.phase, phaseErr.requestSent, true
	}

	return "", false, false
}

// HTTPPhase reports the SDK HTTP-phase tag carried by err, when err
// originates from the transport layer. Returns one of:
//
//   - "request_build"    — failure constructing the request (URL parse,
//     marshal, content-type guard).
//   - "request_validate" — failure in [security.ValidateOutboundRequest].
//   - "request_marshal"  — JSON marshalling failure.
//   - "request_send"     — *http.Client.Do returned an error.
//   - "response_read"    — io.ReadAll on the response body failed.
//   - "response_decode"  — JSON unmarshal failure on a 2xx response.
//
// Returns "" when err is not phase-tagged (most commonly: it originates
// from an upstream HTTP response, in which case [errors.As] for
// *errors.Error yields the typed shape).
//
// The boolean second return value reports whether the request reached
// the wire before failing (false for build/validate/marshal, true for
// send/read/decode).
func HTTPPhase(err error) (string, bool) {
	phase, requestSent, ok := phaseFromError(err)
	if !ok {
		return "", false
	}

	return phase, requestSent
}

func (c *HTTPClient) executeRetryAttempt(req *http.Request, method, requestURL string, execution *retryExecution) error {
	// Inner loop runs at most twice: the original attempt, plus one
	// re-execution if a 401 triggers a successful token refresh.
	//
	// Auth-refresh-retry is request-scoped, not retry-loop-scoped. It must
	// fire whenever a tokenProvider is wired and refresh succeeds, even when
	// the caller has disabled the generic retry loop via WithoutRetries
	// (MaxRetries=0). The execution.refreshedAuth flag bounds this to a
	// single re-execution — a second 401 cannot re-enter this branch.
	for {
		err := c.performSingleAttempt(req, method, requestURL, execution)
		if !execution.justRefreshedAuth {
			return err
		}

		execution.justRefreshedAuth = false
	}
}

// performSingleAttempt runs one HTTP round trip and post-processes the
// response. If a 401 fires AND a tokenProvider is wired AND refresh
// succeeds, it sets execution.justRefreshedAuth so executeRetryAttempt
// knows to loop once more with the fresh token. The execution.refreshedAuth
// flag prevents a second refresh from being attempted within the same
// request.
func (c *HTTPClient) performSingleAttempt(req *http.Request, method, requestURL string, execution *retryExecution) error {
	if err := resetRequestBody(req); err != nil {
		return wrapHTTPPhaseError(httpPhaseRequestBuild, false, err)
	}

	if err := security.ValidateOutboundRequest(req); err != nil {
		return wrapHTTPPhaseError(httpPhaseRequestValidate, false, fmt.Errorf("invalid request URL: %w", err))
	}

	client := c.snapshotHTTPClient()

	resp, err := client.Do(req) // #nosec G704 -- request URL validated via security.ValidateOutboundRequest
	if err != nil {
		return wrapHTTPPhaseError(httpPhaseRequestSend, true, c.handleRequestExecutionError(method, requestURL, err))
	}

	// Drain-and-close on every code path so http.Transport can reuse the
	// underlying TCP connection. The deferred close alone is not enough —
	// when the body exceeds maxHTTPResponseBodyBytes the LimitReader
	// returns short and the unread tail starves the pool.
	defer drainAndCloseResponseBody(c, resp)

	execution.resp = resp

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBodyBytes+1))
	if err != nil {
		return wrapHTTPPhaseError(httpPhaseResponseRead, true, fmt.Errorf("failed to read response body: %w", err))
	}

	if int64(len(responseBody)) > maxHTTPResponseBodyBytes {
		return wrapHTTPPhaseError(httpPhaseResponseRead, true, fmt.Errorf("response body exceeds %d bytes", maxHTTPResponseBodyBytes))
	}

	execution.responseBody = responseBody

	// Snapshot only when we may need tokenProvider/tokenInvalidator — the
	// 401-and-refresh-eligible branch. Successful 2xx and most non-401
	// failures bypass this allocation entirely.
	if resp.StatusCode == http.StatusUnauthorized && !execution.refreshedAuth {
		snapshot := c.cloneConfiguration()
		if snapshot.tokenProvider == nil {
			return c.handleRetryAttemptResponse(req.Context(), resp, responseBody, method, requestURL)
		}

		c.logAuthRefresh(req.Context(), "started", method, requestURL, resp, nil)

		token, refreshed, refreshErr := c.refreshAuthToken(req.Context(), snapshot)
		if refreshed {
			c.logAuthRefresh(req.Context(), "succeeded", method, requestURL, resp, nil)
			req.Header.Set("Authorization", formatAuthorizationHeader(token))
			execution.refreshedAuth = true
			execution.justRefreshedAuth = true

			// Returned error is consumed inside executeRetryAttempt's loop
			// when justRefreshedAuth is set, so it never reaches the outer
			// retry layer. It exists only to keep the signature uniform.
			return errors.New("access manager token refreshed after unauthorized response")
		}

		c.logAuthRefresh(req.Context(), "failed", method, requestURL, resp, refreshErr)
	}

	return c.handleRetryAttemptResponse(req.Context(), resp, responseBody, method, requestURL)
}

// refreshAuthToken obtains a fresh token via the configured tokenProvider.
// Concurrent callers that hit a 401 at roughly the same time funnel through
// a singleflight so the underlying provider call runs once. The token, once
// fetched, is written through setAuthTokenLocked so other in-flight
// requests will see it on their next header build.
//
// Returns the token and true on success; empty string and false plus a safe
// diagnostic error when no fresh token could be obtained.
func (c *HTTPClient) refreshAuthToken(ctx context.Context, snapshot httpClientConfigSnapshot) (string, bool, error) {
	if snapshot.tokenProvider == nil {
		return "", false, errors.New("token provider is not configured")
	}

	// Use a stable singleflight key so all concurrent refreshers on this
	// HTTPClient share one underlying call. When an invalidator is present,
	// include its function identity so callers that intentionally install
	// different invalidation callbacks do not collapse onto the same refresh.
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
		return "", false, err
	}

	token, ok := result.(string)
	if !ok || token == "" {
		return "", false, errors.New("token provider returned an empty token")
	}

	c.setAuthTokenLocked(token)

	return token, true, nil
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

// handleRequestExecutionError converts a transport-layer client.Do
// failure into a typed SDK error. Audit 8.1 (CRITICAL): before this
// path landed, network failures (DNS, conn-refused, TLS handshake)
// arrived at callers as bare *net.OpError shapes, so IsNetworkError(err)
// returned false on real network errors.
//
// Now: every transport failure passes through ClassifyTransportError,
// which produces a *errors.Error with the right Category (Network for
// DNS/conn-refused, Timeout for deadline-exceeded, Cancellation for
// context.Canceled, Internal as a final fallback). The wrapped err is
// preserved as Error.Err so errors.Unwrap walks the full causal chain.
//
// The operation string is derived from method + requestURL so the
// transport layer can produce a non-empty Operation even before
// service-method call sites thread their own context through (deferred
// to 8F per the kickoff scope).
func (c *HTTPClient) handleRequestExecutionError(method, requestURL string, err error) error {
	c.debugLogRequestError(method, requestURL, err)

	requestErr := sdkerrors.ClassifyTransportError(transportOperation(method, requestURL), err)

	snapshot := c.cloneConfiguration()
	if snapshot.customRetryPolicy != nil {
		if snapshot.customRetryPolicy(nil, requestErr) {
			return retryableCustomPolicyError{err: requestErr}
		}

		return retry.AsNonRetryable(requestErr)
	}

	return requestErr
}

// transportOperation produces a synthetic Operation string from the
// HTTP method. The transport layer doesn't know the service-method
// name (e.g. "accounts.Create") that called it, so we surface
// "http GET" / "http POST" so the typed Operation field is non-empty
// without leaking the request URL (which may carry tenant identifiers
// or path-borne IDs).
//
// Service-method-aware operations — the threading from the entity
// layer's call sites — is deferred to 8F. _ is reserved for that
// future expansion.
func transportOperation(method, _ string) string {
	if method == "" {
		return "http"
	}

	return "http " + method
}

func (c *HTTPClient) handleRetryAttemptResponse(ctx context.Context, resp *http.Response, responseBody []byte, method, requestURL string) error {
	if ctx == nil && resp != nil && resp.Request != nil {
		ctx = resp.Request.Context()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if resp.StatusCode < http.StatusBadRequest {
		return nil
	}

	requestID := resp.Header.Get("X-Request-ID")

	apiErr := c.handleErrorResponse(ctx, resp.StatusCode, responseBody, method, requestURL, requestID)

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

// retryableCustomPolicyError is the internal sentinel that signals
// "the user-supplied custom retry policy said this is retryable."
// It is NOT a public-facing error shape.
//
// Audit 8.3 (HIGH): the v2 implementation prefixed every Error()
// rendering with "custom retryable: " — internal wrapper noise that
// leaked into user-facing error strings. The prefix is gone in v3.
// The wrapper still implements Unwrap so the underlying typed error
// reaches errors.Is/As walks correctly.
type retryableCustomPolicyError struct {
	err error
}

func (e retryableCustomPolicyError) Error() string {
	if e.err == nil {
		return ""
	}

	return e.err.Error()
}

func (e retryableCustomPolicyError) Unwrap() error {
	return e.err
}

// closeResponseBody safely closes response body with debug logging
func (c *HTTPClient) closeResponseBody(resp *http.Response) {
	if closeErr := resp.Body.Close(); closeErr != nil && c != nil && c.debug.Load() {
		c.debugLog("Failed to close response body: %v", closeErr)
	}
}

// handleErrorResponse processes API error responses.
//
// THEME 4: per-attempt response logging is intentionally DROPPED from this
// path. handleErrorResponse runs once per retry attempt, so emitting a Warn
// line here produced 3N+ duplicate logs for a single transient failure.
// The terminal logging point is [logRetryExhausted] (retry budget gone)
// or [logHTTPPhaseFailure] (transport error). Debug-level diagnostics
// remain available via [SetDebug](true).
func (c *HTTPClient) handleErrorResponse(_ context.Context, statusCode int, responseBody []byte, method, requestURL, requestID string) error {
	// ctx is accepted for signature symmetry with the other request-shape
	// helpers but is no longer consumed inside this function after the
	// per-attempt response-log was dropped (THEME 4). Debug logging below
	// uses the configured *slog.Logger directly, not ctx.

	apiErr := c.parseErrorResponse(statusCode, responseBody, requestID)
	attachHTTPResponseMetadata(apiErr, method, requestURL, requestID, statusCode)

	if c.debug.Load() {
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
	if c != nil && c.debug.Load() {
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

	_, urlPath := safeURLHostPath(requestURL)
	logger.LogAttrs(ctx, slog.LevelWarn, "slow API call",
		slog.String("sdk.name", sdkLoggerName),
		slog.String("sdk.component", "http"),
		slog.String("operation", method),
		slog.String("http.method", method),
		slog.String("url.path", urlPath),
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

func (c *HTTPClient) logHTTPPhaseFailure(ctx context.Context, method, requestURL string, resp *http.Response, err error) {
	phase, requestSent, ok := phaseFromError(err)
	if !ok {
		return
	}

	attrs := safeHTTPLogAttrs(method, requestURL, resp, requestSent)
	attrs = append(attrs,
		slog.String("phase", phase),
		slog.String("error", safeLogError(err)),
	)
	if category := errorCategory(err); category != "" {
		attrs = append(attrs, slog.String("error.category", category))
	}

	c.logDiagnostic(ctx, slog.LevelWarn, "HTTP request phase failed", attrs...)
}

// logHTTPTerminalFailure emits ONE terminal log line for the final
// request-level failure: a transport-phase error, a retry-exhausted
// response, or a single-attempt non-2xx response when retries are
// disabled. Per-attempt response logging is deliberately NOT done here —
// see THEME 4 design note in [handleErrorResponse].
//
// Routing:
//   - phase-tagged error (build/validate/marshal/send/read/decode)   → logHTTPPhaseFailure
//   - retry budget exhausted (errors.Is ErrRetriesExhausted)         → logRetryExhausted
//   - everything else (typed *errors.Error from a single attempt)    → emit a "HTTP response error" line
func (c *HTTPClient) logHTTPTerminalFailure(ctx context.Context, method, requestURL string, resp *http.Response, err error, maxRetries int) {
	if err == nil {
		return
	}

	if _, _, ok := phaseFromError(err); ok {
		c.logHTTPPhaseFailure(ctx, method, requestURL, resp, err)
		return
	}

	if errors.Is(err, retry.ErrRetriesExhausted) {
		c.logRetryExhausted(ctx, method, requestURL, resp, err, maxRetries)
		return
	}

	// Single-attempt HTTP failure (e.g. 500 with MaxRetries=0) reaches
	// here. Emit one Warn line with the same field schema other terminal
	// lines use so retry-aware dashboards stay consistent.
	var (
		statusCode int
		requestID  string
	)

	if resp != nil {
		statusCode = resp.StatusCode
		requestID = strings.TrimSpace(resp.Header.Get("X-Request-ID"))
	} else {
		var statusErr interface{ StatusCode() int }
		if errors.As(err, &statusErr) {
			statusCode = statusErr.StatusCode()
		}
		requestID = requestIDFromError(err)
	}

	attrs := safeHTTPLogAttrs(method, requestURL, resp, true)
	if statusCode > 0 && resp == nil {
		attrs = append(attrs, slog.Int("http.status_code", statusCode))
	}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", sanitizeLogInput(requestID)))
	}
	attrs = append(attrs, slog.String("error", safeLogError(err)))
	if category := errorCategory(err); category != "" {
		attrs = append(attrs, slog.String("error.category", category))
	}

	c.logDiagnostic(ctx, slog.LevelWarn, "HTTP response error", attrs...)
}

// logAuthRefresh emits a structured log line for an auth-refresh state
// transition. Level routing: "started" and "succeeded" are routine
// operational signals → Debug. "failed" indicates the bootstrap or
// refresh credential is wrong, which surfaces as 401-loop cascades → Warn.
func (c *HTTPClient) logAuthRefresh(ctx context.Context, state, method, requestURL string, resp *http.Response, err error) {
	attrs := safeHTTPLogAttrs(method, requestURL, resp, true)
	attrs = append(attrs, slog.String("auth_refresh.state", state))
	if err != nil {
		attrs = append(attrs, slog.String("error", safeLogError(err)))
		if category := errorCategory(err); category != "" {
			attrs = append(attrs, slog.String("error.category", category))
		}
	}

	level := slog.LevelDebug
	if state == "failed" {
		level = slog.LevelWarn
	}

	c.logDiagnostic(ctx, level, "token refresh "+state, attrs...)
}

// logRetryExhausted emits the terminal "retry budget gone" log line.
//
// Gated on the typed [retry.ErrRetriesExhausted] sentinel (not on a
// substring of err.Error()). The brittle "operation failed after" match
// the v2 code used would silently break the moment the retry package
// reworded its error string; the sentinel makes the wire intentional and
// independently versionable.
func (c *HTTPClient) logRetryExhausted(ctx context.Context, method, requestURL string, resp *http.Response, err error, maxRetries int) {
	// Defensive: callers SHOULD only invoke this with err != nil, but
	// keep a guard so future call-site mistakes don't panic on nil deref.
	if err == nil {
		return
	}
	if !errors.Is(err, retry.ErrRetriesExhausted) {
		return
	}

	attrs := safeHTTPLogAttrs(method, requestURL, resp, true)
	attrs = append(attrs,
		slog.Int("attempts", maxRetries+1),
		slog.Int("max_retries", maxRetries),
		slog.String("error", safeLogError(err)),
	)
	if resp == nil {
		var statusErr interface{ StatusCode() int }
		if errors.As(err, &statusErr) && statusErr.StatusCode() > 0 {
			attrs = append(attrs, slog.Int("http.status_code", statusErr.StatusCode()))
		}
		if requestID := requestIDFromError(err); requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}
	}
	if category := errorCategory(err); category != "" {
		attrs = append(attrs, slog.String("error.category", category))
	}

	c.logDiagnostic(ctx, slog.LevelWarn, "retry exhausted", attrs...)
}

func (c *HTTPClient) logDiagnostic(ctx context.Context, level slog.Level, message string, attrs ...slog.Attr) {
	logger := c.cloneConfiguration().logger
	if logger == nil {
		return
	}

	base := make([]slog.Attr, 0, 2+len(attrs))
	base = append(base,
		slog.String("sdk.name", sdkLoggerName),
		slog.String("sdk.component", "http"),
	)
	base = append(base, attrs...)

	logger.LogAttrs(ctx, level, message, base...)
}

func safeHTTPLogAttrs(method, requestURL string, resp *http.Response, requestSent bool) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("operation", sanitizeLogInput(method)),
		slog.String("http.method", sanitizeLogInput(method)),
		slog.Bool("http.request_sent", requestSent),
	}

	if host, path := safeURLHostPath(requestURL); host != "" || path != "" {
		if host != "" {
			attrs = append(attrs, slog.String("url.host", host))
		}
		if path != "" {
			attrs = append(attrs, slog.String("url.path", path))
		}
	}

	if resp != nil {
		attrs = append(attrs, slog.Int("http.status_code", resp.StatusCode))
		if requestID := strings.TrimSpace(resp.Header.Get("X-Request-ID")); requestID != "" {
			attrs = append(attrs, slog.String("request_id", sanitizeLogInput(requestID)))
		}
	}

	return attrs
}

func safeURLHostPath(rawURL string) (host, path string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}

	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = normalizeTelemetryPath(parsed.Path)

	host = parsed.Hostname()
	if port := parsed.Port(); port != "" && host != "" {
		host = net.JoinHostPort(host, port)
	}

	return sanitizeLogInput(host), sanitizeLogInput(parsed.EscapedPath())
}

func safeLogError(err error) string {
	if isNilInterfaceValue(err) {
		return ""
	}

	return sanitizeLogInput(sdkerrors.RedactSensitiveString(err.Error()))
}

func isNilInterfaceValue(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func errorCategory(err error) string {
	var sdkErr *sdkerrors.Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return sanitizeLogInput(string(sdkErr.Category))
	}

	return ""
}

func requestIDFromError(err error) string {
	var sdkErr *sdkerrors.Error
	if errors.As(err, &sdkErr) && sdkErr != nil {
		return sanitizeLogInput(sdkErr.RequestID)
	}

	return ""
}

// attachHTTPResponseMetadata stamps request-shape diagnostic fields onto
// the typed SDK error. The values land on dedicated Method / URLHost /
// URLPath fields rather than on the Details map — the map is processed
// by RedactSensitiveDetails at construction time, so post-construction
// mutations would bypass the redaction sweep. The typed fields go through
// safeURLHostPath, which is already redaction-safe (userinfo / query /
// fragment stripped, dynamic-ID segments collapsed).
func attachHTTPResponseMetadata(err error, method, requestURL, requestID string, statusCode int) {
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) || sdkErr == nil {
		return
	}

	host, path := safeURLHostPath(requestURL)
	sdkErr.Method = method
	sdkErr.URLHost = host
	sdkErr.URLPath = path
	if sdkErr.RequestID == "" {
		sdkErr.RequestID = requestID
	}
	if sdkErr.StatusCode == 0 {
		sdkErr.StatusCode = statusCode
	}
	sdkErr.HTTPRequestSent = true
}

// logResponseDetails logs response information in debug mode
func (c *HTTPClient) logResponseDetails(method, requestURL string, resp *http.Response, responseBody []byte) {
	if c == nil || !c.debug.Load() {
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
	if c == nil || !c.enableIdempotency.Load() || !isUnsafeMethod(method) {
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
	headers[internalAutoIdempotencyHeader] = boolTrue

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

	if closeErr := req.Body.Close(); closeErr != nil && c.debug.Load() {
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
	if len(quoted) >= quotedStringMinLength {
		return quoted[1 : len(quoted)-1]
	}

	return input
}

func redactHeaders(headers http.Header) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for key, values := range headers {
		if sdkerrors.IsSensitiveFieldName(key) {
			redacted[key] = []string{"[REDACTED]"}
			continue
		}

		copied := make([]string, len(values))
		copy(copied, values)
		redacted[key] = copied
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

	parsedURL.User = nil
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	parsedURL.Path = normalizeTelemetryPath(parsedURL.Path)

	return parsedURL.String()
}

func normalizeTelemetryPath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}

		if isLikelyTelemetryIdentifier(segment) {
			segments[i] = ":id"
		}
	}

	return strings.Join(segments, "/")
}

func redactDebugURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return redactUnparseableDebugURL(rawURL)
	}

	parsedURL.User = nil
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	parsedURL.Path = normalizeTelemetryPath(parsedURL.Path)

	return parsedURL.String()
}

func redactUnparseableDebugURL(rawURL string) string {
	withoutQuery, _, _ := strings.Cut(rawURL, "?")
	withoutFragment, _, _ := strings.Cut(withoutQuery, "#")
	if strings.Contains(withoutFragment, "@") {
		return "[REDACTED_URL]"
	}

	return sanitizeLogInput(withoutFragment)
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
		switch v := arg.(type) {
		case string:
			sanitized[i] = sanitizeLogString(v)
		case error:
			// Errors can carry attacker-controlled bytes (server-supplied
			// messages, parsed payloads). Coerce through Error() and
			// sanitize so newline-bearing messages cannot survive into
			// the structured log line.
			if isNilInterfaceValue(v) {
				sanitized[i] = ""
				continue
			}

			sanitized[i] = sanitizeLogString(v.Error())
		case fmt.Stringer:
			if isNilInterfaceValue(v) {
				sanitized[i] = ""
				continue
			}

			sanitized[i] = sanitizeLogString(v.String())
		default:
			sanitized[i] = arg
		}
	}

	return sanitized
}

func sanitizeLogString(value string) string {
	redacted := sdkerrors.RedactSensitiveString(value)
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		redacted = redactDebugURL(value)
	}

	return sanitizeLogInput(redacted)
}

// debugLog routes a debug-level message through the configured *slog.Logger
// when SetDebug(true) was called (typically driven from
// midaz.WithDebug / pkg/config.WithDebug). v3 contract: there is exactly
// one log path. The MIDAZ_DEBUG-bypass that wrote raw text to stderr in
// v2 is gone.
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
