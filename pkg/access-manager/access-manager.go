package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/security"
)

const (
	accessManagerOAuthLoginPath       = "/v1/login/oauth/access_token"
	maxAccessManagerResponseBodyBytes = int64(1 << 20)
	accessManagerRefreshSkew          = 30 * time.Second
)

type cachedToken struct {
	token     string
	expiresAt time.Time
}

var accessManagerTokenCache sync.Map

// EntityOption is a function that configures an entity with authentication.
type EntityOption func(e any) error

// WithAccessManager returns an EntityOption that configures plugin-based authentication.
// When plugin-based authentication is enabled, the function will make a request to the authentication service
// to retrieve an authentication token before interacting with Midaz.
//
// Parameters:
//   - AccessManager: The plugin authentication configuration.
//
// Returns:
//   - EntityOption: A function that configures plugin authentication.

// AccessManager represents the configuration for plugin-based authentication.
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

// WithAccessManager returns an EntityOption that configures plugin-based authentication.
func WithAccessManager(accessMgr AccessManager) EntityOption {
	return func(e any) error {
		// Type assertion to access the required methods
		type entityWithAuth interface {
			GetHTTPClient() *http.Client
			SetAuthToken(token string)
			InitServices()
		}

		entity, ok := e.(entityWithAuth)
		if !ok {
			return errors.New("entity does not implement required methods for plugin auth")
		}

		// If plugin auth is not enabled, nothing to do
		if !accessMgr.Enabled {
			return nil
		}

		// Validate plugin auth configuration
		if accessMgr.Address == "" {
			return errors.New("plugin auth address is required when plugin auth is enabled")
		}

		// Get a token from the plugin auth service
		token, err := GetTokenFromAccessManager(context.Background(), accessMgr, entity.GetHTTPClient())
		if err != nil {
			return fmt.Errorf("failed to get token from plugin auth service: %w", err)
		}

		// Set the token on the entity
		entity.SetAuthToken(token)

		// Re-initialize services to update the token
		entity.InitServices()

		return nil
	}
}

// GetTokenFromAccessManager retrieves an authentication token from the plugin auth service
// when plugin authentication is enabled.
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

	tokenResp, err := requestAccessManagerToken(ctx, accessMgr, httpClient)
	if err != nil {
		return "", err
	}

	storeCachedToken(cacheKey, tokenResp)

	return tokenResp.AccessToken, nil
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

func accessManagerCacheKey(accessMgr AccessManager) (string, error) {
	endpoint, err := accessManagerEndpoint(accessMgr.Address)
	if err != nil {
		return "", err
	}

	return endpoint + "|" + strings.TrimSpace(accessMgr.ClientID), nil
}

func loadCachedToken(cacheKey string) (string, bool) {
	value, ok := accessManagerTokenCache.Load(cacheKey)
	if !ok {
		return "", false
	}

	cached, ok := value.(cachedToken)
	if !ok || strings.TrimSpace(cached.token) == "" {
		accessManagerTokenCache.Delete(cacheKey)
		return "", false
	}

	if !cached.expiresAt.IsZero() && time.Now().Add(accessManagerRefreshSkew).After(cached.expiresAt) {
		accessManagerTokenCache.Delete(cacheKey)
		return "", false
	}

	return cached.token, true
}

func storeCachedToken(cacheKey string, tokenResp TokenResponse) {
	expiresAt := time.Time{}

	if tokenResp.ExpiresAt != "" {
		if parsed, err := time.Parse(time.RFC3339, tokenResp.ExpiresAt); err == nil {
			expiresAt = parsed
		}
	}

	accessManagerTokenCache.Store(cacheKey, cachedToken{token: tokenResp.AccessToken, expiresAt: expiresAt})
}
