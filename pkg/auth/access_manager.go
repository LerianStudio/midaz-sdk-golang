package auth

import (
	"bytes"
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/security"
	"golang.org/x/sync/singleflight"
)

const (
	accessManagerOAuthLoginPath        = "/v1/login/oauth/access_token"
	accessManagerTokenRequestOperation = "access_manager.token_request"
	accessManagerTokenFetchPhase       = "token_fetch"
	maxAccessManagerResponseBodyBytes  = int64(1 << 20)
	accessManagerRefreshSkew           = 30 * time.Second
	accessManagerTokenRequestTimeout   = 30 * time.Second

	// accessManagerCacheCapacity bounds the in-process token cache. The cache
	// is keyed by (endpoint, clientID, secretHash) so the upper bound is the
	// number of distinct (auth host × client) pairs the SDK ever uses in a
	// single process. 256 entries is generous for any realistic deployment
	// while still preventing the unbounded growth that the previous
	// sync.Map exposed.
	accessManagerCacheCapacity = 256
)

// Exported validation-reason constants for
// [AccessManagerTokenRequestError.ValidationReason]. Tests and callers
// that need to branch on the reason can compare against these instead of
// duplicating the string literals.
const (
	ValidationReasonAuthDisabled        = "auth_disabled"
	ValidationReasonMissingAddress      = "missing_address"
	ValidationReasonMissingClientID     = "missing_client_id"
	ValidationReasonMissingClientSecret = "missing_client_secret"
	ValidationReasonNilHTTPClient       = "nil_http_client"
	ValidationReasonMalformedEndpoint   = "malformed_endpoint"
	ValidationReasonMissingScheme       = "missing_scheme"
	ValidationReasonInvalidScheme       = "invalid_scheme"
	ValidationReasonInsecureScheme      = "insecure_scheme"
	ValidationReasonMissingHost         = "missing_host"
	ValidationReasonUserinfoNotAllowed  = "userinfo_not_allowed"
	ValidationReasonInvalidRequest      = "invalid_request"
	ValidationReasonValidationFailed    = "validation_failed"
)

// ErrAccessManagerTokenFetch marks failures that occur while obtaining an
// Access Manager token. Callers should use errors.Is rather than matching
// rendered error text, because lower-level network messages vary by platform.
var ErrAccessManagerTokenFetch = errors.New("access manager token fetch failed")

// WrapAccessManagerTokenFetchError preserves the concrete token-fetch cause
// while adding a stable sentinel for classification.
func WrapAccessManagerTokenFetchError(err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w: %w", ErrAccessManagerTokenFetch, err)
}

// IsAccessManagerTokenFetchError reports whether err is an Access Manager
// token-fetch failure.
func IsAccessManagerTokenFetchError(err error) bool {
	return errors.Is(err, ErrAccessManagerTokenFetch)
}

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// boundedTokenCache is a small concurrency-safe LRU keyed by string. It exists
// because the previous implementation used a sync.Map with no eviction, which
// is unbounded growth in long-lived SDK processes that rotate credentials.
//
// The cache stores cachedToken values; expired entries are evicted lazily on
// read (loadCachedToken) and proactively on write (Store). Dead entries from
// expired credentials therefore never linger past the next access.
type boundedTokenCache struct {
	mu       sync.Mutex
	capacity int
	entries  map[string]*list.Element
	order    *list.List
}

type cacheEntry struct {
	key   string
	value cachedToken
}

func newBoundedTokenCache(capacity int) *boundedTokenCache {
	if capacity <= 0 {
		capacity = accessManagerCacheCapacity
	}

	return &boundedTokenCache{
		capacity: capacity,
		entries:  make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Store inserts or replaces an entry, evicting the least-recently-used entry
// when capacity is exceeded.
func (c *boundedTokenCache) Store(key string, value cachedToken) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.entries[key]; ok {
		storedEntry, ok := element.Value.(*cacheEntry)
		if !ok {
			c.order.Remove(element)
			delete(c.entries, key)

			return
		}

		storedEntry.value = value

		c.order.MoveToFront(element)

		return
	}

	newEntry := &cacheEntry{key: key, value: value}
	element := c.order.PushFront(newEntry)
	c.entries[key] = element

	for c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest == nil {
			return
		}

		c.order.Remove(oldest)

		if oldestEntry, ok := oldest.Value.(*cacheEntry); ok {
			delete(c.entries, oldestEntry.key)
		}
	}
}

// Delete removes the entry for key (no-op when absent).
func (c *boundedTokenCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if element, ok := c.entries[key]; ok {
		c.order.Remove(element)
		delete(c.entries, key)
	}
}

func (c *boundedTokenCache) loadValid(key string, now time.Time) (cachedToken, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.entries[key]
	if !ok {
		return cachedToken{}, false
	}

	entry, ok := element.Value.(*cacheEntry)
	if !ok {
		c.order.Remove(element)
		delete(c.entries, key)

		return cachedToken{}, false
	}

	if strings.TrimSpace(entry.value.token) == "" ||
		entry.value.expiresAt.IsZero() ||
		now.Add(accessManagerRefreshSkew).After(entry.value.expiresAt) {
		c.order.Remove(element)
		delete(c.entries, key)

		return cachedToken{}, false
	}

	c.order.MoveToFront(element)

	return entry.value, true
}

var (
	accessManagerTokenCache   = newBoundedTokenCache(accessManagerCacheCapacity)
	accessManagerSingleFlight singleflight.Group
)

// AccessManager represents the configuration for plugin-based authentication.
//
// Construct one and pass it via [github.com/LerianStudio/midaz-sdk-golang/v5.WithAccessManager]
// (or [github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config.WithAccessManager]
// when building a *config.Config directly). The midaz package re-exports this
// type as [github.com/LerianStudio/midaz-sdk-golang/v5.AccessManager] so a
// typical setup needs only one import.
//
// The Enabled field is auto-populated by WithAccessManager — callers do not
// set it themselves. Address, ClientID, and ClientSecret are all required.
//
// AllowInsecureHTTP opts INTO accepting plain http:// Access Manager URLs
// for non-loopback hosts. The default is strict (HTTPS or loopback only),
// which catches misconfiguration before credentials cross a plaintext link.
// Production deployments must leave this false; the flag exists for the
// in-cluster Kubernetes Service pattern
// (e.g. http://plugin-access-manager-auth.svc.cluster.local) where the
// transport is already protected by a service mesh or trusted network
// segment.
type AccessManager struct {
	Enabled           bool
	Address           string
	ClientID          string
	ClientSecret      string // #nosec G117 -- configuration field required by public SDK and OAuth client-credentials flow
	AllowInsecureHTTP bool
}

// TokenResponse represents the response from the plugin auth service
type TokenResponse struct {
	AccessToken  string `json:"accessToken"` // #nosec G117 -- response contract from external OAuth provider
	IDToken      string `json:"idToken"`
	TokenType    string `json:"tokenType"`
	RefreshToken string `json:"refreshToken"` // #nosec G117 -- response contract from external OAuth provider
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

// AccessManagerTokenRequestError carries diagnostic-safe context about the
// Access Manager token request. It intentionally excludes client credentials,
// bearer tokens, request bodies, and response bodies.
type AccessManagerTokenRequestError struct {
	Operation             string
	Phase                 string
	EndpointScheme        string
	EndpointHost          string
	EndpointPath          string
	LocalValidationFailed bool
	HTTPRequestSent       bool
	ValidationReason      string
	StatusCodeValue       int
	Err                   error
}

func (e *AccessManagerTokenRequestError) Error() string {
	if e == nil {
		return ""
	}

	parts := []string{
		"access manager token request failed",
		"operation=" + e.Operation,
		"phase=" + e.Phase,
	}

	if e.EndpointScheme != "" || e.EndpointHost != "" || e.EndpointPath != "" {
		parts = append(parts, fmt.Sprintf("endpoint=%s://%s%s", e.EndpointScheme, e.EndpointHost, e.EndpointPath))
	}

	parts = append(parts,
		fmt.Sprintf("httpRequestSent=%t", e.HTTPRequestSent),
		fmt.Sprintf("localValidationFailed=%t", e.LocalValidationFailed),
	)

	if e.ValidationReason != "" {
		parts = append(parts, "validationReason="+e.ValidationReason)
	}

	if e.StatusCodeValue > 0 {
		parts = append(parts, fmt.Sprintf("statusCode=%d", e.StatusCodeValue))
	}

	if !isNilAccessManagerError(e.Err) {
		parts = append(parts, sdkerrors.RedactSensitiveString(e.Err.Error()))
	}

	return strings.Join(parts, "; ")
}

func (e *AccessManagerTokenRequestError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// StatusCode returns the upstream HTTP status code, when one was received.
func (e *AccessManagerTokenRequestError) StatusCode() int {
	if e == nil {
		return 0
	}

	return e.StatusCodeValue
}

func isNilAccessManagerError(err error) bool {
	if err == nil {
		return true
	}

	value := reflect.ValueOf(err)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// AccessManagerPhase returns the diagnostic-safe Access Manager request phase.
func (e *AccessManagerTokenRequestError) AccessManagerPhase() string {
	if e == nil {
		return ""
	}

	return e.Phase
}

// AccessManagerEndpointScheme returns the redacted endpoint scheme.
func (e *AccessManagerTokenRequestError) AccessManagerEndpointScheme() string {
	if e == nil {
		return ""
	}

	return e.EndpointScheme
}

// AccessManagerEndpointHost returns the redacted endpoint host.
func (e *AccessManagerTokenRequestError) AccessManagerEndpointHost() string {
	if e == nil {
		return ""
	}

	return e.EndpointHost
}

// AccessManagerEndpointPath returns the redacted endpoint path.
func (e *AccessManagerTokenRequestError) AccessManagerEndpointPath() string {
	if e == nil {
		return ""
	}

	return e.EndpointPath
}

// AccessManagerLocalValidationFailed reports whether local request validation failed.
func (e *AccessManagerTokenRequestError) AccessManagerLocalValidationFailed() bool {
	if e == nil {
		return false
	}

	return e.LocalValidationFailed
}

// AccessManagerHTTPRequestSent reports whether the token request was sent upstream.
func (e *AccessManagerTokenRequestError) AccessManagerHTTPRequestSent() bool {
	if e == nil {
		return false
	}

	return e.HTTPRequestSent
}

// AccessManagerValidationReason returns the local validation failure reason, when present.
func (e *AccessManagerTokenRequestError) AccessManagerValidationReason() string {
	if e == nil {
		return ""
	}

	return e.ValidationReason
}

// GetTokenFromAccessManager retrieves an authentication token from the plugin auth service
// when plugin authentication is enabled.
//
// Concurrent callers asking for a token under the same cache key share a
// single underlying HTTP exchange via singleflight — a thundering herd of
// SDK-internal goroutines (e.g. workers reacting to a 401 fan-out) only
// hits the auth service once.
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation and timeouts.
//   - accessMgr: The plugin access manager configuration.
//   - httpClient: The HTTP client to use for the request.
//
// Returns:
//   - string: The authentication token retrieved from the plugin auth service.
//   - error: An error if the token retrieval fails.
func GetTokenFromAccessManager(ctx context.Context, accessMgr AccessManager, httpClient *http.Client) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := validateAccessManagerTokenRequest(accessMgr, httpClient); err != nil {
		return "", err
	}

	cacheKey, err := accessManagerCacheKey(accessMgr)
	if err != nil {
		return "", err
	}

	if token, ok := loadCachedToken(cacheKey); ok {
		return token, nil
	}

	// Use singleflight so concurrent requests for the same cacheKey share a
	// single backend call. The closure re-checks the cache to catch the
	// "two callers, second arrived after the first one's token landed"
	// race: we don't want to issue a second HTTP request just because the
	// caller raced past loadCachedToken.
	resultCh := accessManagerSingleFlight.DoChan(cacheKey, func() (any, error) {
		if token, ok := loadCachedToken(cacheKey); ok {
			return token, nil
		}

		tokenCtx, cancel := boundedAccessManagerTokenContext(ctx)
		defer cancel()

		tokenResp, err := requestAccessManagerToken(tokenCtx, accessMgr, httpClient)
		if err != nil {
			return "", err
		}

		storeCachedToken(cacheKey, tokenResp)

		return tokenResp.AccessToken, nil
	})

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return "", result.Err
		}

		token, ok := result.Val.(string)
		if !ok {
			return "", errors.New("plugin auth service returned an unexpected token type")
		}

		return token, nil
	}
}

// ValidateAccessManagerAddress validates the Access Manager base address using
// the same local outbound-request rules that protect token fetches. The
// returned error is diagnostic-safe: it carries scheme/host/path and validation
// reason, but never credentials, tokens, request bodies, response bodies, query
// strings, or Authorization data.
//
// Strict mode (the default): plain http:// is accepted only for loopback
// addresses (127.0.0.0/8, ::1, localhost). Use
// [ValidateAccessManagerAddressWithInsecure] to opt into accepting plain
// http:// for any host (e.g. in-cluster Kubernetes service DNS).
func ValidateAccessManagerAddress(address string) error {
	return ValidateAccessManagerAddressWithInsecure(address, false)
}

// ValidateAccessManagerAddressWithInsecure mirrors
// [ValidateAccessManagerAddress] but lets the caller opt into plain http://
// URLs even for non-loopback hosts. Production deployments must pass
// allowInsecureHTTP=false. The escape hatch exists for in-cluster service
// DNS where the transport is already protected by the service mesh.
func ValidateAccessManagerAddressWithInsecure(address string, allowInsecureHTTP bool) error {
	_, err := validatedAccessManagerEndpointURL(address, allowInsecureHTTP)
	return err
}

func boundedAccessManagerTokenContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	hardDeadline := time.Now().Add(accessManagerTokenRequestTimeout)

	if deadline, ok := ctx.Deadline(); ok && deadline.Before(hardDeadline) {
		return context.WithDeadline(ctx, deadline)
	}

	return context.WithDeadline(ctx, hardDeadline)
}

func validateAccessManagerTokenRequest(accessMgr AccessManager, httpClient *http.Client) error {
	if !accessMgr.Enabled {
		return newAccessManagerTokenRequestErrorFromURL(nil, false, true, ValidationReasonAuthDisabled, 0,
			errors.New("plugin authentication is not enabled"))
	}

	if strings.TrimSpace(accessMgr.Address) == "" {
		return newAccessManagerTokenRequestErrorFromURL(nil, false, true, ValidationReasonMissingAddress, 0,
			errors.New("plugin auth address is required when plugin auth is enabled"))
	}

	if strings.TrimSpace(accessMgr.ClientID) == "" {
		return newAccessManagerTokenRequestErrorFromURL(nil, false, true, ValidationReasonMissingClientID, 0,
			errors.New("plugin auth client id is required when plugin auth is enabled"))
	}

	if strings.TrimSpace(accessMgr.ClientSecret) == "" {
		return newAccessManagerTokenRequestErrorFromURL(nil, false, true, ValidationReasonMissingClientSecret, 0,
			errors.New("plugin auth client secret is required when plugin auth is enabled"))
	}

	if httpClient == nil {
		endpoint, endpointErr := accessManagerEndpointURLWithoutValidation(accessMgr.Address)
		if endpointErr != nil {
			endpoint = nil
		}

		return newAccessManagerTokenRequestErrorFromURL(endpoint, false, true, ValidationReasonNilHTTPClient, 0,
			errors.New("HTTP client cannot be nil"))
	}

	return ValidateAccessManagerAddressWithInsecure(accessMgr.Address, accessMgr.AllowInsecureHTTP)
}

func requestAccessManagerToken(ctx context.Context, accessMgr AccessManager, httpClient *http.Client) (TokenResponse, error) {
	// Defensive: validateAccessManagerTokenRequest is the canonical
	// gate on this argument shape, but requestAccessManagerToken is
	// reachable from in-package code paths that bypass that gate
	// (refresh loops, test harnesses). Returning a clean error here
	// turns "future caller forgot to validate" from a nil-deref panic
	// into a recoverable failure with the same diagnostic envelope as
	// the validator emits.
	if httpClient == nil {
		return TokenResponse{}, newAccessManagerTokenRequestErrorFromURL(
			nil, false, true, ValidationReasonNilHTTPClient, 0,
			errors.New("HTTP client cannot be nil"))
	}

	// Create the request payload
	payload := map[string]string{
		"grantType":    "client_credentials",
		"clientId":     accessMgr.ClientID,
		"clientSecret": accessMgr.ClientSecret,
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	req, err := newAccessManagerTokenRequest(ctx, accessMgr.Address, payloadBytes, accessMgr.AllowInsecureHTTP)
	if err != nil {
		return TokenResponse{}, err
	}

	// Make the request. Token requests carry client credentials in the JSON
	// body, so the SDK redirect guard must run even when callers supplied a
	// permissive custom CheckRedirect policy.
	resp, err := accessManagerHTTPClient(httpClient, accessMgr.AllowInsecureHTTP).Do(req) // #nosec G704 -- request URL validated via security.ValidateOutboundRequest
	if err != nil {
		return TokenResponse{}, newAccessManagerTokenRequestError(req, true, false, "", 0,
			fmt.Errorf("failed to connect to plugin auth service: %w", err))
	}
	defer drainAndCloseAccessManagerResponseBody(resp)

	tokenResp, err := readAccessManagerTokenResponse(resp)
	if err != nil {
		return TokenResponse{}, newAccessManagerTokenRequestError(req, true, false, "", resp.StatusCode, err)
	}

	return tokenResp, nil
}

func accessManagerHTTPClient(client *http.Client, allowInsecureHTTP bool) *http.Client {
	if client == nil {
		return nil
	}

	clientCopy := *client
	callerRedirect := client.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := security.ValidateRedirectWithInsecureHTTP(req, via, allowInsecureHTTP); err != nil {
			return err
		}
		if callerRedirect != nil {
			return callerRedirect(req, via)
		}

		return nil
	}

	return &clientCopy
}

// drainAndCloseAccessManagerResponseBody drains the response body so the
// connection can be returned to the keep-alive pool, then closes it.
// Drain and close errors are intentionally discarded: pkg/auth does not
// have access to the SDK debug logger, the response body has already
// been consumed for the token decode by the time we get here, and a
// failure on either step is non-actionable in this credential-flow
// codepath (the auth call has already succeeded or failed by status
// code). entities/http.go has a richer variant that surfaces these
// errors through the debug log — wire that in only if pkg/auth gains
// a similar logger handle.
func drainAndCloseAccessManagerResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	// Drain and close errors are intentionally discarded via blank
	// assignment; see godoc above for rationale.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxAccessManagerResponseBodyBytes))
	_ = resp.Body.Close()
}

func newAccessManagerTokenRequest(ctx context.Context, address string, payloadBytes []byte, allowInsecureHTTP bool) (*http.Request, error) {
	// Create a request to the plugin auth service with the payload
	tokenURL, err := accessManagerEndpoint(address, allowInsecureHTTP)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURL,
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to plugin auth service: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if reason, err := validateAccessManagerOutboundRequest(req, allowInsecureHTTP); err != nil {
		return nil, newAccessManagerTokenRequestError(req, false, true, reason, 0,
			fmt.Errorf("invalid plugin auth request URL: %w", err))
	}

	return req, nil
}

// validateAccessManagerOutboundRequest runs the SDK-wide outbound URL
// guard against an Access Manager request. When allowInsecureHTTP is true,
// plain http:// is permitted even for non-loopback hosts — used for the
// in-cluster Kubernetes Service pattern. Strict mode (the default) still
// rejects everything that fails security.ValidateOutboundRequest.
func validateAccessManagerOutboundRequest(req *http.Request, allowInsecureHTTP bool) (string, error) {
	if allowInsecureHTTP && req != nil && req.URL != nil && strings.EqualFold(req.URL.Scheme, "http") {
		// Bypass the security guard's HTTPS enforcement, but still reject
		// userinfo and missing-host shapes — those are independent of the
		// transport-security check.
		if req.URL.User != nil {
			return ValidationReasonUserinfoNotAllowed, errors.New("URL must not include user information")
		}
		if req.URL.Hostname() == "" {
			return ValidationReasonMissingHost, errors.New("URL must include host")
		}

		return "", nil
	}

	if err := security.ValidateOutboundRequest(req); err != nil {
		return classifyAccessManagerValidationReason(req, err), err
	}

	return "", nil
}

func classifyAccessManagerValidationReason(req *http.Request, err error) string {
	if err == nil {
		return ""
	}

	if req == nil || req.URL == nil {
		return ValidationReasonInvalidRequest
	}

	if req.URL.User != nil {
		return ValidationReasonUserinfoNotAllowed
	}

	if req.URL.Hostname() == "" {
		return ValidationReasonMissingHost
	}

	scheme := strings.ToLower(req.URL.Scheme)
	if scheme == "http" {
		return ValidationReasonInsecureScheme
	}
	if scheme == "" {
		return ValidationReasonMissingScheme
	}
	// At this point scheme is non-empty and not "http". The only valid
	// non-http alternative is "https"; anything else is an invalid scheme.
	if scheme != "https" {
		return ValidationReasonInvalidScheme
	}

	return ValidationReasonValidationFailed
}

func newAccessManagerTokenRequestError(req *http.Request, sent, localValidation bool, validationReason string, statusCode int, err error) error {
	var endpoint *url.URL
	if req != nil {
		endpoint = req.URL
	}

	return newAccessManagerTokenRequestErrorFromURL(endpoint, sent, localValidation, validationReason, statusCode, err)
}

func newAccessManagerTokenRequestErrorFromURL(endpoint *url.URL, sent, localValidation bool, validationReason string, statusCode int, err error) error {
	wrapped := &AccessManagerTokenRequestError{
		Operation:             accessManagerTokenRequestOperation,
		Phase:                 accessManagerTokenFetchPhase,
		LocalValidationFailed: localValidation,
		HTTPRequestSent:       sent,
		ValidationReason:      validationReason,
		StatusCodeValue:       statusCode,
		Err:                   err,
	}

	if endpoint != nil {
		wrapped.EndpointScheme = strings.ToLower(endpoint.Scheme)
		wrapped.EndpointHost = endpoint.Host
		wrapped.EndpointPath = endpoint.EscapedPath()
	}

	return wrapped
}

func readAccessManagerTokenResponse(resp *http.Response) (TokenResponse, error) {
	// Read the response body
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAccessManagerResponseBodyBytes+1))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to read response from plugin auth service: %w", err)
	}

	if int64(len(body)) > maxAccessManagerResponseBodyBytes {
		return TokenResponse{}, fmt.Errorf("plugin auth response body exceeds %d bytes", maxAccessManagerResponseBodyBytes)
	}

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, fmt.Errorf("plugin auth service returned non-OK status: %d", resp.StatusCode)
	}

	return decodeAccessManagerTokenResponse(body)
}

func decodeAccessManagerTokenResponse(body []byte) (TokenResponse, error) {
	// Parse the response
	var tokenResp TokenResponse

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return TokenResponse{}, fmt.Errorf("failed to parse response from plugin auth service: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return TokenResponse{}, errors.New("plugin auth service returned empty token")
	}

	return tokenResp, nil
}

// InvalidateAccessManagerToken removes any cached token for the provided access manager configuration.
func InvalidateAccessManagerToken(accessMgr AccessManager) {
	cacheKey, err := accessManagerCacheKey(accessMgr)
	if err != nil {
		return
	}

	accessManagerTokenCache.Delete(cacheKey)
}

func accessManagerEndpoint(address string, allowInsecureHTTP bool) (string, error) {
	endpoint, err := validatedAccessManagerEndpointURL(address, allowInsecureHTTP)
	if err != nil {
		return "", err
	}

	return endpoint.String(), nil
}

func validatedAccessManagerEndpointURL(address string, allowInsecureHTTP bool) (*url.URL, error) {
	endpoint, err := accessManagerEndpointURLWithoutValidation(address)
	if err != nil {
		return nil, newAccessManagerTokenRequestErrorFromURL(nil, false, true, ValidationReasonMalformedEndpoint, 0,
			errors.New("invalid plugin auth address"))
	}

	if endpoint.Scheme == "" {
		return nil, newAccessManagerTokenRequestErrorFromURL(endpoint, false, true, ValidationReasonMissingScheme, 0,
			errors.New("plugin auth address must include scheme"))
	}

	if endpoint.Host == "" {
		return nil, newAccessManagerTokenRequestErrorFromURL(endpoint, false, true, ValidationReasonMissingHost, 0,
			errors.New("plugin auth address must include host"))
	}

	if endpoint.User != nil {
		return nil, newAccessManagerTokenRequestErrorFromURL(endpoint, false, true, ValidationReasonUserinfoNotAllowed, 0,
			errors.New("plugin auth address must not include user information"))
	}

	req := &http.Request{URL: endpoint}
	if reason, err := validateAccessManagerOutboundRequest(req, allowInsecureHTTP); err != nil {
		return nil, newAccessManagerTokenRequestErrorFromURL(endpoint, false, true, reason, 0,
			fmt.Errorf("invalid plugin auth request URL: %w", err))
	}

	return endpoint, nil
}

func accessManagerEndpointURLWithoutValidation(address string) (*url.URL, error) {
	baseURL, err := url.Parse(strings.TrimSpace(address))
	if err != nil {
		return nil, err
	}

	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + accessManagerOAuthLoginPath
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	return baseURL, nil
}

// accessManagerCacheKey derives a stable cache identifier from the access
// manager configuration. We deliberately fold the SHA-256 of the client
// secret (truncated to the first 16 hex chars) into the key — when a caller
// rotates ClientSecret while keeping ClientID + Address constant, the cache
// must NOT serve the previous credential's cached token. Hashing rather
// than including the raw secret keeps the secret out of memory in the
// cache map's keys (which can survive longer than the value if a single
// allocation is replayed elsewhere).
func accessManagerCacheKey(accessMgr AccessManager) (string, error) {
	endpoint, err := accessManagerEndpoint(accessMgr.Address, accessMgr.AllowInsecureHTTP)
	if err != nil {
		return "", err
	}

	secretHash := hashClientSecret(accessMgr.ClientSecret)

	return endpoint + "|" + strings.TrimSpace(accessMgr.ClientID) + "|" + secretHash, nil
}

// hashClientSecret returns a 16-hex-character SHA-256 prefix of the client
// secret. The prefix length matches the previous "best-effort fingerprint"
// shape used elsewhere in the SDK and is sufficient to detect rotation; we
// deliberately do NOT use the raw secret as part of the key.
func hashClientSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])[:16]
}

func loadCachedToken(cacheKey string) (string, bool) {
	cached, ok := accessManagerTokenCache.loadValid(cacheKey, time.Now())
	if !ok {
		return "", false
	}

	return cached.token, true
}

// storeCachedToken caches a successful token exchange.
//
// We only cache tokens whose ExpiresAt is non-empty AND parses cleanly as
// RFC 3339. The previous behavior cached tokens with zero expiry, which
// effectively cached them forever (the loadCachedToken guard treated
// IsZero as "no expiry, always valid"). Refusing to cache zero-expiry
// tokens turns that silent-forever-cache bug into a clean re-exchange
// every time, which is the safest fallback when the upstream provider
// withholds expiry metadata.
func storeCachedToken(cacheKey string, tokenResp TokenResponse) {
	if tokenResp.ExpiresAt == "" {
		return
	}

	parsed, err := time.Parse(time.RFC3339, tokenResp.ExpiresAt)
	if err != nil {
		return
	}

	accessManagerTokenCache.Store(cacheKey, cachedToken{
		token:     tokenResp.AccessToken,
		expiresAt: parsed,
	})
}
