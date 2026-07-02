package entities

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/security"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/version"
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

	// maxExposed4xxBodyBytes is the upper bound on 4xx response bodies
	// attached to SDK errors when WithErrorBodyExposure is enabled. 4xx
	// payloads are typically caller-facing validation envelopes that
	// callers need to inspect to fix their request; the generous cap
	// matches the previous unconditional limit.
	maxExposed4xxBodyBytes = 64 * 1024

	// maxExposed5xxBodyBytes is a tighter cap on 5xx response bodies.
	// 5xx payloads historically leak server-side diagnostics (stack
	// traces, SQL fragments, connection strings) that the redactor
	// catches only by string-pattern match. Capping the exposure window
	// at 4 KiB is defense in depth: even a missed redaction surfaces a
	// bounded slice of the diagnostic.
	maxExposed5xxBodyBytes = 4 * 1024
)

const idempotencyHeader = "X-Idempotency"

// ttlHeader carries the idempotency-slot TTL (seconds) set via
// sdkctx.WithIdempotencyTTL. Omitted when unset — the server applies its
// default (300s).
const ttlHeader = "X-TTL"

// defaultUserAgent returns the SDK's centralized user-agent string.
// The configured value flows in via (*HTTPClient).SetUserAgent (driven
// from pkg/config.Config.UserAgent, which callers may override
// programmatically via [WithUserAgent]).
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
	// Atomic primitives for the hot-path scalar knobs. These fields
	// are read on every request (header build, debug gating, idempotency
	// gating, error-body exposure) but mutated rarely (only via setters called from midaz.New
	// or test plumbing). Going atomic eliminates the RWMutex Lock/Unlock
	// pair from the per-request read path — a measurable win under retry
	// fan-out workloads. The atomic.Pointer[string] for userAgent stores a
	// pointer-to-string so the read path doesn't have to copy the bytes;
	// a nil pointer means "fall back to defaultUserAgent()".
	userAgent         atomic.Pointer[string]
	debug             atomic.Bool
	enableIdempotency atomic.Bool
	exposeErrorBody   atomic.Bool
	// allowInsecureHTTP gates the runtime [security.ValidateOutboundRequest]
	// check against plain http:// non-loopback targets. Default false
	// (strict). Set via [SetAllowInsecureHTTP], typically threaded from
	// [pkg/config.Config.AllowInsecureHTTP] at entity construction.
	// Atomic so the request path reads it without contending with the
	// HTTPClient mutex.
	allowInsecureHTTP atomic.Bool
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
	exposeErrorBody   bool
}

// NewHTTPClient creates a new HTTP client with safe v3 defaults.
//
// v3 invariant (Track 3): no environment variables are read here. Configuration
// flows in exclusively through pkg/config.Config and the SetDebug / SetUserAgent /
// SetEnableIdempotency / SetExposeErrorBody / WithRetryOptions setters (plus (*Entity).SetObservability
// for observability). Callers who want env-driven configuration must opt in via
// config.NewConfig(config.FromEnvironment()).
//
// Defaults: debug=false, userAgent=version.UserAgent(), enableIdempotency=true,
// exposeErrorBody=false, retryOptions=retry.DefaultOptions().
//
// Parameters:
//   - client: The underlying *http.Client. If nil, the SDK's package-level default is used.
//   - authToken: The authentication token for API authorization.
//   - provider: The observability provider for tracing, metrics, and logging (can be nil).
func NewHTTPClient(client *http.Client, authToken string, provider observability.Provider) *HTTPClient {
	if client == nil {
		client = defaultHTTPClient()
	} else {
		client = security.EnsureRedirectPolicy(client)
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
	c.exposeErrorBody.Store(false)

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
		Timeout:       30 * time.Second,
		CheckRedirect: validateSDKRedirect,
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

func validateSDKRedirect(req *http.Request, via []*http.Request) error {
	return security.ValidateRedirect(req, via)
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

// SetExposeErrorBody enables or disables attaching raw upstream 4xx/5xx
// response bodies to SDK errors. Bodies are attached without redaction and
// only truncated by the error package.
func (c *HTTPClient) SetExposeErrorBody(enabled bool) {
	if c == nil {
		return
	}

	c.exposeErrorBody.Store(enabled)
}

// SetAllowInsecureHTTP gates the runtime outbound-URL guard against plain
// http:// non-loopback targets. Default false (strict). Wired from
// [pkg/config.Config.AllowInsecureHTTP] by [NewEntityWithConfigContext];
// callers building entities by hand can flip the flag here.
//
// SECURITY: leave this off in production over the public internet. The
// flag exists for in-cluster Kubernetes Service DNS and dev/test
// deployments behind a controlled network boundary.
//
// Lock-free: backed by an atomic.Bool so the request path reads it
// without contending with the HTTPClient mutex.
func (c *HTTPClient) SetAllowInsecureHTTP(allow bool) {
	if c == nil {
		return
	}

	c.allowInsecureHTTP.Store(allow)
}

// AllowInsecureHTTP reports whether the data-plane insecure-HTTP opt-in
// is active. Exposed for diagnostic readers and for the retry layer
// snapshot helpers.
func (c *HTTPClient) AllowInsecureHTTP() bool {
	if c == nil {
		return false
	}

	return c.allowInsecureHTTP.Load()
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
		exposeErrorBody:   c.exposeErrorBody.Load(),
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
	c.exposeErrorBody.Store(snapshot.exposeErrorBody)
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
//     write it directly into X-Idempotency. That header tells this function
//     the caller has spoken; we MUST NOT overwrite the value here.
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
		callerSupplied := headerValueCaseInsensitive(headers, idempotencyHeader) != ""
		if !callerSupplied {
			removeHeaderCaseInsensitive(headers, idempotencyHeader)
			headers[idempotencyHeader] = key
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

	req, headers, err := buildRawHTTPRequest(ctx, method, requestURL, headers, body, c.allowInsecureHTTP.Load())
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

func buildRawHTTPRequest(ctx context.Context, method, requestURL string, headers map[string]string, body []byte, allowInsecureHTTP bool) (*http.Request, map[string]string, error) {
	reader, headers, err := prepareRawRequestBody(headers, body)
	if err != nil {
		return nil, nil, err
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, nil, wrapHTTPPhaseError(httpPhaseRequestBuild, false, fmt.Errorf("failed to parse request URL: %w", err))
	}

	validationReq := &http.Request{URL: parsedURL}
	if err := security.ValidateOutboundRequestWithInsecureHTTP(validationReq, allowInsecureHTTP); err != nil {
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

	if err := security.ValidateOutboundRequestWithInsecureHTTP(req, c.allowInsecureHTTP.Load()); err != nil {
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

	if err := security.ValidateOutboundRequestWithInsecureHTTP(&http.Request{URL: parsedURL}, c.allowInsecureHTTP.Load()); err != nil {
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

	// *http.Request.Header.Get is case-insensitive via header
	// canonicalization (textproto.CanonicalMIMEHeaderKey), so
	// "x-idempotency", "X-IDEMPOTENCY", and "X-Idempotency" all resolve
	// to the same slot. This is the IETF-intended behaviour for HTTP
	// headers and is distinct from the map[string]string paths elsewhere
	// in this file (e.g. ensureIdempotencyHeader → headerValueCaseInsensitive)
	// which operate on a raw map without canonicalization and therefore
	// MUST use the explicit case-insensitive helper.
	hasIdempotencyKey := strings.TrimSpace(req.Header.Get(idempotencyHeader)) != ""

	effectiveRetryOptions := cloneRetryOptions(snapshot.retryOptions)
	if effectiveRetryOptions == nil {
		effectiveRetryOptions = retry.DefaultOptions()
	}

	// Replace the previous "append a magic 'custom retryable' substring to
	// RetryableErrors" workaround with a typed predicate. Compose it with
	// the caller's predicate so public retry options remain honored.
	userRetryPredicate := effectiveRetryOptions.ErrorPredicate
	effectiveRetryOptions.ErrorPredicate = func(err error) bool {
		if isInternalRetryableError(err) {
			return true
		}

		return userRetryPredicate != nil && userRetryPredicate(err)
	}

	if httpRetriesSuppressed(ctx) || (isUnsafeMethod(req.Method) && !hasIdempotencyKey) {
		effectiveRetryOptions.MaxRetries = 0
	}

	retryCtx := retry.WithOptionsContext(ctx, effectiveRetryOptions)

	retryCtx = withRetryAttemptDiagnostics(retryCtx, snapshot, method, requestURL, effectiveRetryOptions.MaxRetries)

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

func withRetryAttemptDiagnostics(ctx context.Context, snapshot httpClientConfigSnapshot, method, requestURL string, maxRetries int) context.Context {
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
			slog.Int("max_retries", maxRetries),
			slog.Int64("delay_ms", delay.Milliseconds()),
			slog.String("cause", causeMsg),
		)

		// Guard against typed-nil status implementations: errors.As can
		// hand us a wrapper whose interface value is (T, nil), which
		// passes `!= nil` but panics on StatusCode() call. Mirrors the
		// matchesRetryableHTTPStatus check in pkg/retry/retry.go.
		if errors.As(cause, &statusErr) && !isNilInterfaceValue(statusErr) && statusErr.StatusCode() > 0 {
			attrs = append(attrs, slog.Int("http.status_code", statusErr.StatusCode()))
		}
		if requestID := requestIDFromError(cause); requestID != "" {
			attrs = append(attrs, slog.String("request_id", requestID))
		}
		if category := errorCategory(cause); category != "" {
			attrs = append(attrs, slog.String("error.category", category))
		}

		level := slog.LevelDebug
		if attempt >= maxRetries {
			// Final attempt before exhaustion → warn level so it shows up
			// in production log filters that suppress debug.
			level = slog.LevelWarn
		}

		logger.LogAttrs(hookCtx, level, "retrying request", attrs...)

		if metrics != nil {
			metrics.RecordRetry(hookCtx, method, "http", attempt)
		}
	}

	return retry.WithAttemptHook(ctx, hook)
}

// sdkLoggerName is the value emitted under the sdk.name structured log
// field. Constant so callers can rely on a stable string.
