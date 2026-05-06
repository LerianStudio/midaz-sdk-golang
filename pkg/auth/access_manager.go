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
	"strings"
	"sync"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/security"
	"golang.org/x/sync/singleflight"
)

const (
	accessManagerOAuthLoginPath       = "/v1/login/oauth/access_token"
	maxAccessManagerResponseBodyBytes = int64(1 << 20)
	accessManagerRefreshSkew          = 30 * time.Second
	accessManagerTokenRequestTimeout  = 30 * time.Second

	// accessManagerCacheCapacity bounds the in-process token cache. The cache
	// is keyed by (endpoint, clientID, secretHash) so the upper bound is the
	// number of distinct (auth host × client) pairs the SDK ever uses in a
	// single process. 256 entries is generous for any realistic deployment
	// while still preventing the unbounded growth that the previous
	// sync.Map exposed.
	accessManagerCacheCapacity = 256
)

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

// Load returns the cached value for key and whether it was present. Touching
// an entry promotes it to the front of the LRU.
func (c *boundedTokenCache) Load(key string) (cachedToken, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	element, ok := c.entries[key]
	if !ok {
		return cachedToken{}, false
	}

	c.order.MoveToFront(element)

	entry, ok := element.Value.(*cacheEntry)
	if !ok {
		c.order.Remove(element)
		delete(c.entries, key)

		return cachedToken{}, false
	}

	return entry.value, true
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
// Construct one and pass it via [github.com/LerianStudio/midaz-sdk-golang/v3.WithAccessManager]
// (or [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config.WithAccessManager]
// when building a *config.Config directly). The midaz package re-exports this
// type as [github.com/LerianStudio/midaz-sdk-golang/v3.AccessManager] so a
// typical setup needs only one import.
//
// The Enabled field is auto-populated by WithAccessManager — callers do not
// set it themselves. Address, ClientID, and ClientSecret are all required.
type AccessManager struct {
	Enabled      bool
	Address      string
	ClientID     string
	ClientSecret string // #nosec G117 -- configuration field required by public SDK and OAuth client-credentials flow
}

// TokenResponse represents the response from the plugin auth service
type TokenResponse struct {
	AccessToken  string `json:"accessToken"` // #nosec G117 -- response contract from external OAuth provider
	IDToken      string `json:"idToken"`
	TokenType    string `json:"tokenType"`
	RefreshToken string `json:"refreshToken"` // #nosec G117 -- response contract from external OAuth provider
	ExpiresAt    string `json:"expiresAt,omitempty"`
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

func boundedAccessManagerTokenContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.WithoutCancel(ctx)
	hardDeadline := time.Now().Add(accessManagerTokenRequestTimeout)

	if deadline, ok := ctx.Deadline(); ok && deadline.Before(hardDeadline) {
		return context.WithDeadline(base, deadline)
	}

	return context.WithDeadline(base, hardDeadline)
}

func validateAccessManagerTokenRequest(accessMgr AccessManager, httpClient *http.Client) error {
	if !accessMgr.Enabled {
		return errors.New("plugin authentication is not enabled")
	}

	if strings.TrimSpace(accessMgr.Address) == "" {
		return errors.New("plugin auth address is required when plugin auth is enabled")
	}

	if strings.TrimSpace(accessMgr.ClientID) == "" {
		return errors.New("plugin auth client id is required when plugin auth is enabled")
	}

	if strings.TrimSpace(accessMgr.ClientSecret) == "" {
		return errors.New("plugin auth client secret is required when plugin auth is enabled")
	}

	if httpClient == nil {
		return errors.New("HTTP client cannot be nil")
	}

	return nil
}

func requestAccessManagerToken(ctx context.Context, accessMgr AccessManager, httpClient *http.Client) (TokenResponse, error) {
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

	req, err := newAccessManagerTokenRequest(ctx, accessMgr.Address, payloadBytes)
	if err != nil {
		return TokenResponse{}, err
	}

	// Make the request
	resp, err := httpClient.Do(req) // #nosec G704 -- request URL validated via security.ValidateOutboundRequest
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to connect to plugin auth service: %w", err)
	}
	defer resp.Body.Close()

	tokenResp, err := readAccessManagerTokenResponse(resp)
	if err != nil {
		return TokenResponse{}, err
	}

	return tokenResp, nil
}

func newAccessManagerTokenRequest(ctx context.Context, address string, payloadBytes []byte) (*http.Request, error) {
	// Create a request to the plugin auth service with the payload
	tokenURL, err := accessManagerEndpoint(address)
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

	if err := security.ValidateOutboundRequest(req); err != nil {
		return nil, fmt.Errorf("invalid plugin auth request URL: %w", err)
	}

	return req, nil
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

func accessManagerEndpoint(address string) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(address))
	if err != nil {
		return "", fmt.Errorf("invalid plugin auth address: %w", err)
	}

	if baseURL.Scheme == "" || baseURL.Host == "" {
		return "", errors.New("plugin auth address must include scheme and host")
	}

	if baseURL.User != nil {
		return "", errors.New("plugin auth address must not include user information")
	}

	baseURL.Path = strings.TrimRight(baseURL.Path, "/") + accessManagerOAuthLoginPath
	baseURL.RawQuery = ""
	baseURL.Fragment = ""

	return baseURL.String(), nil
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
	endpoint, err := accessManagerEndpoint(accessMgr.Address)
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
