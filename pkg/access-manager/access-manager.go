package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

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
	ClientSecret string
}

// TokenResponse represents the response from the plugin auth service
type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	IdToken      string `json:"idToken"`
	TokenType    string `json:"tokenType"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}

func WithAccessManager(AccessManager AccessManager) EntityOption {
	return func(e any) error {
		// Type assertion to access the required methods
		type entityWithAuth interface {
			GetHTTPClient() *http.Client
			SetAuthToken(token string)
			InitServices()
		}

		entity, ok := e.(entityWithAuth)
		if !ok {
			return fmt.Errorf("entity does not implement required methods for plugin auth")
		}

		// If plugin auth is not enabled, nothing to do
		if !AccessManager.Enabled {
			return nil
		}

		// Validate plugin auth configuration
		if AccessManager.Address == "" {
			return fmt.Errorf("plugin auth address is required when plugin auth is enabled")
		}

		// Get a token from the plugin auth service
		token, err := GetTokenFromAccessManager(context.Background(), AccessManager, entity.GetHTTPClient())
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
//   - AccessManager: The plugin access manager configuration.
//   - httpClient: The HTTP client to use for the request.
//
// Returns:
//   - string: The authentication token retrieved from the plugin auth service.
//   - error: An error if the token retrieval fails.
func GetTokenFromAccessManager(ctx context.Context, AccessManager AccessManager, httpClient *http.Client) (string, error) {
	if !AccessManager.Enabled {
		return "", fmt.Errorf("plugin authentication is not enabled")
	}

	if AccessManager.Address == "" {
		return "", fmt.Errorf("plugin auth address is required when plugin auth is enabled")
	}

	// Create the request payload
	payload := map[string]string{
		"grantType":    "client_credentials",
		"clientId":     AccessManager.ClientID,
		"clientSecret": AccessManager.ClientSecret,
	}

	// Marshal the payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth payload: %w", err)
	}

	// Create a request to the plugin auth service with the payload
	url := fmt.Sprintf("%s/v1/login/oauth/access_token", AccessManager.Address)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewBuffer(payloadBytes),
	)

	if err != nil {
		return "", fmt.Errorf("failed to create request to plugin auth service: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to plugin auth service: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response from plugin auth service: %w", err)
	}

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("plugin auth service returned non-OK status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var tokenResp TokenResponse

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse response from plugin auth service: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("plugin auth service returned empty token")
	}

	return tokenResp.AccessToken, nil
}
