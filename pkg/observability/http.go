package observability

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPOption is a function type for configuring the HTTP middleware
type HTTPOption func(*httpMiddleware) error

// httpMiddleware is the internal implementation of the HTTP middleware
type httpMiddleware struct {
	provider      Provider
	ignoreHeaders []string
	ignorePaths   []string
	maskedParams  []string
	hideBody      bool
}

var defaultIgnoredHeaders = []string{
	"authorization",
	"cookie",
	"set-cookie",
	"x-api-key",
	"x-auth-token",
	"x-forwarded-authorization",
	"x-jwt-token",
	"x-middleware-token",
	"x-idempotency",
	"idempotency-key",
	"x-tenant-id",
	"x-organization-id",
	"baggage",
}

var defaultMaskedParams = []string{
	"access_token",
	"api_key",
	"apikey",
	"auth_token",
	"key",
	"password",
	"secret",
	"token",
	"access-token",
	"jwt",
	"refresh_token",
	"refresh-token",
	"document",
}

// ErrNilHTTPResponse is returned by helpers in this package when they are
// asked to record a span/metric for an *http.Response that is nil. Callers
// can use errors.Is to distinguish this case from real I/O failures —
// for example, a transport-level error vs. an SDK programming bug that
// passed a nil response to an enrichment helper.
var ErrNilHTTPResponse = errors.New("observability: nil http response")

// WithIgnoreHeaders specifies HTTP header names that should not be logged
func WithIgnoreHeaders(headers ...string) HTTPOption {
	return func(m *httpMiddleware) error {
		if len(headers) == 0 {
			return errors.New("at least one header must be provided")
		}

		headerMap := make(map[string]struct{})

		for _, h := range m.ignoreHeaders {
			headerMap[strings.ToLower(h)] = struct{}{}
		}

		for _, h := range headers {
			headerMap[strings.ToLower(h)] = struct{}{}
		}

		m.ignoreHeaders = make([]string, 0, len(headerMap))

		for h := range headerMap {
			m.ignoreHeaders = append(m.ignoreHeaders, h)
		}

		return nil
	}
}

// WithIgnorePaths specifies URL paths that should not be traced
func WithIgnorePaths(paths ...string) HTTPOption {
	return func(m *httpMiddleware) error {
		if len(paths) == 0 {
			return errors.New("at least one path must be provided")
		}

		m.ignorePaths = append(m.ignorePaths, paths...)

		return nil
	}
}

// WithMaskedParams specifies query parameters that should have their values masked
func WithMaskedParams(params ...string) HTTPOption {
	return func(m *httpMiddleware) error {
		if len(params) == 0 {
			return errors.New("at least one parameter must be provided")
		}

		m.maskedParams = append(m.maskedParams, params...)

		return nil
	}
}

// WithHideRequestBody specifies whether to hide request bodies from logs
func WithHideRequestBody(hide bool) HTTPOption {
	return func(m *httpMiddleware) error {
		m.hideBody = hide

		return nil
	}
}

// WithDefaultSensitiveHeaders sets the default list of headers to ignore for security
func WithDefaultSensitiveHeaders() HTTPOption {
	return func(m *httpMiddleware) error {
		m.ignoreHeaders = append([]string(nil), defaultIgnoredHeaders...)

		return nil
	}
}

// WithDefaultSensitiveParams sets the default list of parameters to mask for security
func WithDefaultSensitiveParams() HTTPOption {
	return func(m *httpMiddleware) error {
		m.maskedParams = append([]string(nil), defaultMaskedParams...)

		return nil
	}
}

// WithSecurityDefaults sets all default security options
func WithSecurityDefaults() HTTPOption {
	return func(m *httpMiddleware) error {
		if err := WithDefaultSensitiveHeaders()(m); err != nil {
			return err
		}

		if err := WithDefaultSensitiveParams()(m); err != nil {
			return err
		}

		m.hideBody = true

		return nil
	}
}

// NewHTTPMiddleware returns a client-side transport middleware: a
// function that wraps an [http.RoundTripper] with W3C trace-context
// propagation, per-request span creation, and metric recording. It is
// intended for outbound HTTP traffic (the SDK's own client and any
// caller-supplied transport), not as an [http.Handler] for serving
// inbound requests.
func NewHTTPMiddleware(provider Provider, opts ...HTTPOption) func(http.RoundTripper) http.RoundTripper {
	if provider == nil {
		// Return a no-op middleware
		return func(next http.RoundTripper) http.RoundTripper {
			return ensureRoundTripper(next)
		}
	}

	// Create with default configuration
	m := &httpMiddleware{
		provider:      provider,
		ignoreHeaders: append([]string(nil), defaultIgnoredHeaders...),
		maskedParams:  append([]string(nil), defaultMaskedParams...),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(m); err != nil {
			// Log error but continue with other options
			if provider.IsEnabled() && provider.Logger() != nil {
				provider.Logger().Errorf("Failed to apply HTTP middleware option: %v", err)
			}
		}
	}

	return m.middleware
}

// middleware wraps an http.RoundTripper with tracing and metrics
func (m *httpMiddleware) middleware(next http.RoundTripper) http.RoundTripper {
	next = ensureRoundTripper(next)

	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req == nil {
			return nil, errors.New("observability: nil http request")
		}

		// Early returns for disabled or ignored paths
		if m == nil || m.provider == nil || !m.provider.IsEnabled() {
			return next.RoundTrip(req)
		}

		if m.shouldIgnorePath(requestPath(req)) {
			return next.RoundTrip(req)
		}

		// Setup tracing context and span
		ctx, span := m.setupTraceSpan(req)

		// Add attributes and trace context to request
		m.addRequestAttributes(span, req)
		req = m.injectTraceContext(ctx, req, span)

		// Execute request with timing and metrics
		return m.executeTracedRequest(ctx, span, req, next)
	})
}

// shouldIgnorePath checks if the request path should be ignored
func (m *httpMiddleware) shouldIgnorePath(path string) bool {
	for _, ignorePath := range m.ignorePaths {
		if strings.HasPrefix(path, ignorePath) {
			return true
		}
	}

	return false
}

// setupTraceSpan creates a new tracing span for the request
func (m *httpMiddleware) setupTraceSpan(req *http.Request) (context.Context, trace.Span) {
	name := fmt.Sprintf("HTTP %s %s", requestMethod(req), requestPath(req))

	return m.provider.Tracer().Start(
		req.Context(),
		name,
		trace.WithSpanKind(trace.SpanKindClient),
	)
}

// addRequestAttributes adds HTTP and custom attributes to the span
func (m *httpMiddleware) addRequestAttributes(span trace.Span, req *http.Request) {
	// Add HTTP attributes
	span.SetAttributes(httpRequestSemconvAttributes(req, m.sanitizeURL(req.URL))...)

	// Add custom attributes
	name := fmt.Sprintf("HTTP %s %s", requestMethod(req), requestPath(req))
	span.SetAttributes(
		attribute.String(KeyOperationName, name),
		attribute.String(KeyOperationType, "http.request"),
	)

	// Add request headers (excluding sensitive ones)
	m.addRequestHeaders(span, req)
}

// addRequestHeaders adds non-sensitive request headers to the span
func (m *httpMiddleware) addRequestHeaders(span trace.Span, req *http.Request) {
	if req == nil {
		return
	}

	for key, values := range req.Header {
		if !m.isIgnoredHeader(key) && len(values) > 0 {
			span.SetAttributes(semconv.HTTPRequestHeader(strings.ToLower(key), values...))
		}
	}
}

// injectTraceContext injects trace context into request headers and updates the request
func (m *httpMiddleware) injectTraceContext(ctx context.Context, req *http.Request, span trace.Span) *http.Request {
	if req == nil {
		return req
	}

	if req.Header == nil {
		req.Header = make(http.Header)
	}

	ctx = WithProvider(ctx, m.provider)
	InjectHTTPContext(ctx, req.Header)

	// Update request with trace context
	return req.WithContext(httptrace.WithClientTrace(ctx, m.createClientTrace(span)))
}

// executeTracedRequest executes the request with timing, metrics, and response handling
func (m *httpMiddleware) executeTracedRequest(ctx context.Context, span trace.Span, req *http.Request, next http.RoundTripper) (*http.Response, error) {
	start := time.Now()

	// Execute the request
	resp, err := ensureRoundTripper(next).RoundTrip(req)
	duration := time.Since(start)

	// Record metrics
	m.recordRequestMetrics(ctx, req, resp, err, duration)

	// Handle error case
	if err != nil {
		return m.handleRequestError(span, resp, err)
	}

	// Handle successful response
	return m.handleSuccessfulResponse(span, resp)
}

// handleRequestError processes request errors and sets span status
func (*httpMiddleware) handleRequestError(span trace.Span, resp *http.Response, err error) (*http.Response, error) {
	sanitizedErr := sanitizeSensitiveString(err.Error())
	span.SetStatus(codes.Error, sanitizedErr)
	span.SetAttributes(semconv.ErrorType(err))
	span.RecordError(errors.New(sanitizedErr))
	span.End()

	return resp, err
}

// handleSuccessfulResponse processes successful responses and adds response attributes
func (m *httpMiddleware) handleSuccessfulResponse(span trace.Span, resp *http.Response) (*http.Response, error) {
	if resp == nil {
		span.SetStatus(codes.Ok, "")
		span.End()

		return nil, ErrNilHTTPResponse
	}

	// Add response attributes
	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))
	if resp.ProtoMajor > 0 {
		span.SetAttributes(semconv.NetworkProtocolVersion(httpProtocolVersion(resp.ProtoMajor, resp.ProtoMinor)))
	}

	// Add response headers (excluding sensitive ones)
	m.addResponseHeaders(span, resp)

	// Set span status based on response code
	m.setResponseStatus(span, resp.StatusCode)

	span.End()

	return resp, nil
}

// addResponseHeaders adds non-sensitive response headers to the span
func (m *httpMiddleware) addResponseHeaders(span trace.Span, resp *http.Response) {
	if resp == nil {
		return
	}

	for key, values := range resp.Header {
		if !m.isIgnoredHeader(key) && len(values) > 0 {
			span.SetAttributes(semconv.HTTPResponseHeader(strings.ToLower(key), values...))
		}
	}
}

// setResponseStatus sets the span status based on HTTP response code
func (*httpMiddleware) setResponseStatus(span trace.Span, statusCode int) {
	if statusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP status code: %d", statusCode))
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(statusCode)))
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// isIgnoredHeader checks if a header should be ignored (case-insensitive)
func (m *httpMiddleware) isIgnoredHeader(header string) bool {
	lowerHeader := strings.ToLower(header)
	for _, ignored := range m.ignoreHeaders {
		if lowerHeader == ignored {
			return true
		}
	}

	return false
}

// recordRequestMetrics records metrics about the HTTP request
func (m *httpMiddleware) recordRequestMetrics(ctx context.Context, req *http.Request, resp *http.Response, err error, duration time.Duration) {
	// Create attributes for the metrics
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(semconvHTTPMethod(requestMethod(req))),
		semconv.URLPath(requestPath(req)),
	}
	attrs = append(attrs, serverAddressPortAttributes(req)...)

	// Add status code attribute if we have a response
	if resp != nil {
		attrs = append(attrs, semconv.HTTPResponseStatusCode(resp.StatusCode))
	}

	// Record count
	RecordMetric(ctx, m.provider, MetricRequestTotal, 1, attrs...)

	// Record duration
	RecordDuration(ctx, m.provider, MetricRequestDuration, time.Now().Add(-duration), attrs...)

	// Record error or success
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		errorStatus := "unknown"
		if resp != nil {
			errorStatus = fmt.Sprintf("%d", resp.StatusCode)
		}

		attrs = append(attrs, semconv.ErrorTypeKey.String(errorStatus))
		RecordMetric(ctx, m.provider, MetricRequestErrorTotal, 1, attrs...)
	} else {
		RecordMetric(ctx, m.provider, MetricRequestSuccess, 1, attrs...)
	}
}

// createClientTrace creates an httptrace.ClientTrace to track HTTP request lifecycle events
func (*httpMiddleware) createClientTrace(span trace.Span) *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			span.AddEvent("http.get_conn", trace.WithAttributes(serverAddressPortFromHostPort(hostPort)...))
		},
		GotConn: func(info httptrace.GotConnInfo) {
			span.AddEvent("http.got_conn", trace.WithAttributes(
				attribute.Bool("reused", info.Reused),
				attribute.Bool("was_idle", info.WasIdle),
				attribute.String("idle_time", info.IdleTime.String()),
			))
		},
		PutIdleConn: func(err error) {
			attrs := []attribute.KeyValue{}
			if err != nil {
				attrs = append(attrs, attribute.String("error", sanitizeSensitiveString(err.Error())))
			}

			span.AddEvent("http.put_idle_conn", trace.WithAttributes(attrs...))
		},
		DNSStart: func(info httptrace.DNSStartInfo) {
			span.AddEvent("http.dns_start", trace.WithAttributes(
				attribute.String("host", info.Host),
			))
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			addDNSDoneEvent(span, info)
		},
		ConnectStart: func(network, addr string) {
			span.AddEvent("http.connect_start", trace.WithAttributes(
				attribute.String("network", network),
				attribute.String("addr", addr),
			))
		},
		ConnectDone: func(network, addr string, err error) {
			attrs := []attribute.KeyValue{
				attribute.String("network", network),
				attribute.String("addr", addr),
			}
			if err != nil {
				attrs = append(attrs, attribute.String("error", sanitizeSensitiveString(err.Error())))
			}

			span.AddEvent("http.connect_done", trace.WithAttributes(attrs...))
		},
		TLSHandshakeStart: func() {
			span.AddEvent("http.tls_handshake_start")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			attrs := []attribute.KeyValue{
				attribute.String("version", tlsVersionString(state.Version)),
				attribute.String("cipher_suite", tlsCipherSuiteString(state.CipherSuite)),
			}
			if err != nil {
				attrs = append(attrs, attribute.String("error", sanitizeSensitiveString(err.Error())))
			}

			span.AddEvent("http.tls_handshake_done", trace.WithAttributes(attrs...))
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			attrs := []attribute.KeyValue{}
			if info.Err != nil {
				attrs = append(attrs, attribute.String("error", sanitizeSensitiveString(info.Err.Error())))
			}

			span.AddEvent("http.wrote_request", trace.WithAttributes(attrs...))
		},
		GotFirstResponseByte: func() {
			span.AddEvent("http.got_first_response_byte")
		},
	}
}

func addDNSDoneEvent(span trace.Span, info httptrace.DNSDoneInfo) {
	attrs := []attribute.KeyValue{
		attribute.Int("address_count", len(info.Addrs)),
	}
	if len(info.Addrs) > 0 {
		attrs = append(attrs, attribute.String("address", info.Addrs[0].String()))
	}

	if info.Err != nil {
		attrs = append(attrs, attribute.String("error", sanitizeSensitiveString(info.Err.Error())))
	}

	span.AddEvent("http.dns_done", trace.WithAttributes(attrs...))
}

func httpRequestSemconvAttributes(req *http.Request, sanitizedURL string) []attribute.KeyValue {
	method := requestMethod(req)
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(semconvHTTPMethod(method)),
		semconv.URLFull(sanitizedURL),
		semconv.URLPath(requestPath(req)),
	}
	if original := semconvHTTPMethodOriginal(method); original != "" {
		attrs = append(attrs, semconv.HTTPRequestMethodOriginal(original))
	}
	if req != nil && req.URL != nil && req.URL.Scheme != "" {
		attrs = append(attrs, semconv.URLScheme(req.URL.Scheme))
	}
	if req != nil && req.ProtoMajor > 0 {
		attrs = append(attrs, semconv.NetworkProtocolVersion(httpProtocolVersion(req.ProtoMajor, req.ProtoMinor)))
	}
	attrs = append(attrs, serverAddressPortAttributes(req)...)

	return attrs
}

func serverAddressPortAttributes(req *http.Request) []attribute.KeyValue {
	if req == nil || req.URL == nil {
		return nil
	}

	return serverAddressPortFromURL(req.URL)
}

func serverAddressPortFromURL(rawURL *url.URL) []attribute.KeyValue {
	if rawURL == nil {
		return nil
	}

	host := rawURL.Hostname()
	if host == "" {
		return nil
	}

	attrs := []attribute.KeyValue{semconv.ServerAddress(host)}
	if port, ok := urlPort(rawURL); ok {
		attrs = append(attrs, semconv.ServerPort(port))
	}

	return attrs
}

func serverAddressPortFromHostPort(hostPort string) []attribute.KeyValue {
	host, portString, err := net.SplitHostPort(hostPort)
	if err != nil {
		return []attribute.KeyValue{semconv.ServerAddress(hostPort)}
	}

	attrs := []attribute.KeyValue{semconv.ServerAddress(host)}
	if port, err := strconv.Atoi(portString); err == nil {
		attrs = append(attrs, semconv.ServerPort(port))
	}

	return attrs
}

func urlPort(rawURL *url.URL) (int, bool) {
	if rawURL == nil {
		return 0, false
	}
	if portString := rawURL.Port(); portString != "" {
		port, err := strconv.Atoi(portString)
		return port, err == nil
	}
	switch strings.ToLower(rawURL.Scheme) {
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

func ensureRoundTripper(next http.RoundTripper) http.RoundTripper {
	if next != nil {
		return next
	}

	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req == nil {
			return nil, errors.New("observability: nil http request")
		}

		return nil, errors.New("observability: nil http round tripper")
	})
}

func requestMethod(req *http.Request) string {
	if req == nil || req.Method == "" {
		return http.MethodGet
	}

	return req.Method
}

func requestPath(req *http.Request) string {
	if req == nil || req.URL == nil {
		return ""
	}

	return req.URL.Path
}

func (m *httpMiddleware) sanitizeURL(rawURL *url.URL) string {
	if rawURL == nil {
		return ""
	}

	safeURL := *rawURL

	query := safeURL.Query()
	for key, values := range query {
		if m.isMaskedParam(key) {
			maskedValues := make([]string, len(values))
			for i := range maskedValues {
				maskedValues[i] = "[REDACTED]"
			}

			query[key] = maskedValues
		}
	}

	safeURL.RawQuery = query.Encode()

	return safeURL.String()
}

func (m *httpMiddleware) isMaskedParam(param string) bool {
	lowerParam := strings.ToLower(param)
	for _, masked := range m.maskedParams {
		if lowerParam == strings.ToLower(masked) {
			return true
		}
	}

	return false
}

// roundTripperFunc adapts a function to the RoundTripper interface
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper
func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Helper functions for TLS information

func tlsVersionString(version uint16) string {
	switch version {
	case 0x0301:
		return "TLS 1.0"
	case 0x0302:
		return "TLS 1.1"
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("unknown (0x%04x)", version)
	}
}

func tlsCipherSuiteString(cipherSuite uint16) string {
	switch cipherSuite {
	case 0x1301:
		return "TLS_AES_128_GCM_SHA256"
	case 0x1302:
		return "TLS_AES_256_GCM_SHA384"
	case 0x1303:
		return "TLS_CHACHA20_POLY1305_SHA256"
	case 0xc02b:
		return "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	case 0xc02c:
		return "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	case 0xc02f:
		return "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case 0xc030:
		return "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	default:
		return fmt.Sprintf("unknown (0x%04x)", cipherSuite)
	}
}
