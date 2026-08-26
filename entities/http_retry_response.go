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
	"strconv"
	"strings"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/security"
	"github.com/google/uuid"
)

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
	resp                  *http.Response
	responseBody          []byte
	responseBodyTruncated bool
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
	execution.responseBodyTruncated = false

	if err := resetRequestBody(req); err != nil {
		return wrapHTTPPhaseError(httpPhaseRequestBuild, false, err)
	}

	if err := security.ValidateOutboundRequestWithInsecureHTTP(req, c.allowInsecureHTTP.Load()); err != nil {
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
		if resp.StatusCode < http.StatusBadRequest {
			return wrapHTTPPhaseError(httpPhaseResponseRead, true, fmt.Errorf("response body exceeds %d bytes", maxHTTPResponseBodyBytes))
		}

		responseBody = responseBody[:maxHTTPResponseBodyBytes]
		execution.responseBodyTruncated = true
	}

	execution.responseBody = responseBody

	// Snapshot only when we may need tokenProvider/tokenInvalidator — the
	// 401-and-refresh-eligible branch. Successful 2xx and most non-401
	// failures bypass this allocation entirely.
	if resp.StatusCode == http.StatusUnauthorized && !execution.refreshedAuth {
		snapshot := c.cloneConfiguration()
		if snapshot.tokenProvider == nil {
			return c.handleRetryAttemptResponse(req.Context(), resp, responseBody, execution.responseBodyTruncated, method, requestURL)
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

	return c.handleRetryAttemptResponse(req.Context(), resp, responseBody, execution.responseBodyTruncated, method, requestURL)
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
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxHTTPResponseBodyBytes)); err != nil && c != nil && c.debug.Load() {
		c.debugLog("Failed to drain response body: %v", err)
	}

	if c != nil {
		c.closeResponseBody(resp)
		return
	}

	if err := resp.Body.Close(); err != nil {
		// No client/logger is available on this fallback path; the response
		// body was already drained best-effort and cleanup failure is not
		// actionable for callers at this point.
		return
	}
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

	// When the data-plane insecure-HTTP opt-in is active, the redirect
	// policy installed at construction (strict ValidateRedirect) would
	// reject the very kind of http://*.svc.cluster.local target the
	// caller asked us to accept. Wrap the client with the permissive
	// ValidateRedirectWithInsecureHTTP variant on a shallow copy so
	// the caller-supplied transport, jar, and timeout are preserved.
	if c.allowInsecureHTTP.Load() {
		return dataPlaneInsecureHTTPClient(c.client)
	}

	return c.client
}

// dataPlaneInsecureHTTPClient returns a shallow client copy whose
// CheckRedirect delegates to [security.ValidateRedirectWithInsecureHTTP]
// before any caller-supplied policy. Mirrors the pattern used by
// [pkg/auth.accessManagerHTTPClient] for the auth plane. The wrapper is
// produced per request snapshot, but the underlying Transport / Jar /
// Timeout are shared via field assignment so the per-request cost is a
// single struct copy plus one closure allocation.
func dataPlaneInsecureHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return nil
	}

	clientCopy := *client
	callerRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := security.ValidateRedirectWithInsecureHTTP(req, via, true); err != nil {
			return err
		}
		if callerRedirect != nil {
			return callerRedirect(req, via)
		}

		return nil
	}

	return &clientCopy
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
// The operation string is derived from the HTTP method so the transport layer
// always produces a non-empty Operation without leaking request URLs or path IDs.
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
// The URL parameter is intentionally ignored; operation names at this layer stay
// coarse-grained to avoid exposing path-borne tenant or resource identifiers.
func transportOperation(method, _ string) string {
	if method == "" {
		return "http"
	}

	return "http " + method
}

func (c *HTTPClient) handleRetryAttemptResponse(ctx context.Context, resp *http.Response, responseBody []byte, responseBodyTruncated bool, method, requestURL string) error {
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

	apiErr := c.handleErrorResponse(ctx, resp.StatusCode, responseBody, responseBodyTruncated, method, requestURL, requestID)

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
	// The legacy *HTTPClient path always embeds a parsed apiErr, but the
	// plane retryRoundTripper sits below the facade's error-parsing layer and
	// constructs this wrapper with only the status code. Fall back to a
	// status-derived message so Error()/errors.Is walks never nil-panic.
	if e.err == nil {
		return fmt.Sprintf("http status %d", e.statusCode)
	}

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
func (c *HTTPClient) handleErrorResponse(_ context.Context, statusCode int, responseBody []byte, responseBodyTruncated bool, method, requestURL, requestID string) error {
	// ctx is accepted for signature symmetry with the other request-shape
	// helpers but is no longer consumed inside this function after the
	// per-attempt response-log was dropped (THEME 4). Debug logging below
	// uses the configured *slog.Logger directly, not ctx.

	apiErr := c.parseErrorResponse(statusCode, responseBody, requestID)
	if c.exposeErrorBody.Load() {
		apiErr = sdkerrors.AttachUpstreamBodyWithLimit(apiErr, responseBody, responseBodyTruncated, exposedBodyByteCap(statusCode))
	}
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

// httpStatusClassDivisor folds an HTTP status code into its hundreds
// class (e.g. 503/100 == 5). Named so the constant doesn't show up as
// a magic number to mnd, and the intent is clear at call sites.
const (
	httpStatusClassDivisor = 100
	httpServerErrorClass   = 5
)

// exposedBodyByteCap returns the byte cap to apply to an attached
// upstream body based on the HTTP status code class. 5xx bodies get a
// tighter cap because they more often carry server-side diagnostic
// content (stack traces, SQL strings) the regex redactor can miss.
// 4xx bodies keep the generous cap so caller-facing validation
// envelopes remain inspectable.
func exposedBodyByteCap(statusCode int) int {
	if statusCode/httpStatusClassDivisor == httpServerErrorClass {
		return maxExposed5xxBodyBytes
	}

	return maxExposed4xxBodyBytes
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
		if errors.As(err, &statusErr) && !isNilInterfaceValue(statusErr) {
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
		if errors.As(err, &statusErr) && !isNilInterfaceValue(statusErr) && statusErr.StatusCode() > 0 {
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

// isNilInterfaceValue forwards to [sdkerrors.IsNilInterfaceValue] so the
// typed-nil semantics live in exactly one place. The local thin
// forwarder is kept (rather than inlining the call) so existing call
// sites read uniformly across the package.
func isNilInterfaceValue(value any) bool {
	return sdkerrors.IsNilInterfaceValue(value)
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

	if headerValueCaseInsensitive(headers, idempotencyHeader) != "" {
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

	removeHeaderCaseInsensitive(headers, idempotencyHeader)
	headers[idempotencyHeader] = uuid.NewString()

	return headers
}

func headerValueCaseInsensitive(headers map[string]string, name string) string {
	if headers == nil {
		return ""
	}

	for key, value := range headers {
		if strings.EqualFold(key, name) {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func removeHeaderCaseInsensitive(headers map[string]string, name string) {
	for key := range headers {
		if strings.EqualFold(key, name) {
			delete(headers, key)
		}
	}
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
		return redactUnparseableDebugURL(sdkerrors.RedactSensitiveString(rawURL))
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
