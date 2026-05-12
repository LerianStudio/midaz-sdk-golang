package midaz

import (
	"bytes"
	stderrors "errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewReturnsTypedConfigurationError verifies that midaz.New() returns a
// typed *errors.Error (Category=CategoryConfiguration) when construction fails.
// This is the v3 contract: setup mistakes are distinguishable from runtime
// API failures via errors.Is / errors.As.
func TestNewReturnsTypedConfigurationError(t *testing.T) {
	tests := []struct {
		name             string
		options          []Option
		wantUnwrapPhrase string
	}{
		{
			name:             "nil option at index 0",
			options:          []Option{nil},
			wantUnwrapPhrase: "", // nil option has no underlying err to unwrap
		},
		{
			name:             "nil option at index 1",
			options:          []Option{WithDebug(true), nil},
			wantUnwrapPhrase: "",
		},
		{
			name:             "WithConfig(nil)",
			options:          []Option{WithConfig(nil)},
			wantUnwrapPhrase: "config cannot be nil",
		},
		{
			name:             "WithBaseURL invalid",
			options:          []Option{WithBaseURL("://bad-url")},
			wantUnwrapPhrase: "invalid base URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.options...)
			require.Error(t, err)

			// All construction errors are typed configuration errors.
			assert.True(t, sdkerrors.IsConfigurationError(err),
				"expected ErrConfiguration, got %T: %v", err, err)
			require.ErrorIs(t, err, sdkerrors.ErrConfiguration,
				"errors.Is(err, ErrConfiguration) must return true")

			// Operation context is set so users know where it came from.
			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr, "should be *errors.Error")
			assert.Equal(t, "midaz.New", sdkErr.Operation,
				"operation must be midaz.New for construction errors")

			// Underlying option errors are reachable via Unwrap.
			if tt.wantUnwrapPhrase != "" {
				inner := stderrors.Unwrap(err)
				require.Error(t, inner, "wrapped error must be reachable via Unwrap")
				assert.Contains(t, inner.Error(), tt.wantUnwrapPhrase)
			}
		})
	}
}

// TestNewIndexesNilOptionInError verifies the nil-option index appears in the
// error message so callers can identify which option in their slice is nil.
func TestNewIndexesNilOptionInError(t *testing.T) {
	tests := []struct {
		name     string
		options  []Option
		wantText string
	}{
		{name: "nil at 0", options: []Option{nil}, wantText: "index 0"},
		{name: "nil at 2", options: []Option{WithDebug(true), WithDebug(false), nil}, wantText: "index 2"},
		{name: "nil at 5", options: []Option{
			WithDebug(true), WithDebug(false), WithDebug(true), WithDebug(false), WithDebug(true), nil,
		}, wantText: "index 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.options...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantText,
				"error message should identify the nil option's index")
		})
	}
}

// TestPublicConfigValidate exposes the validation rules as a method on Config.
// Advanced callers can use this to validate a Config they constructed via
// DefaultConfig() and mutated directly.
func TestPublicConfigValidate(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		cfg := createTestConfig(t)
		require.NoError(t, cfg.Validate())
	})

	t.Run("missing onboarding URL fails", func(t *testing.T) {
		cfg := createTestConfig(t)
		delete(cfg.ServiceURLs, config.ServiceOnboarding)
		require.Error(t, cfg.Validate())
	})

	t.Run("missing transaction URL fails", func(t *testing.T) {
		cfg := createTestConfig(t)
		delete(cfg.ServiceURLs, config.ServiceTransaction)
		require.Error(t, cfg.Validate())
	})
}

// TestNewWithValidConfigSucceeds is the happy path: a properly-configured
// client constructs cleanly with no error.
func TestNewWithValidConfigSucceeds(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)))
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Entity)
	require.NotNil(t, c.Accounts, "Accounts service must be initialized via embedded Entity")
}

// TestNewRejectsConstructionWithoutAuthSource verifies the v3 auth-required
// contract: New() with zero auth-related options fails fast with a typed
// configuration error pointing the caller at the two sanctioned options.
//
// This closes v2's silent-localhost footgun where construction succeeded
// with empty credentials and every subsequent API call returned 401.
func TestNewRejectsConstructionWithoutAuthSource(t *testing.T) {
	_, err := New(WithEnvironment(config.EnvironmentLocal))
	require.Error(t, err)

	require.True(t, sdkerrors.IsConfigurationError(err),
		"missing auth source should yield a typed ErrConfiguration")
	require.ErrorIs(t, err, sdkerrors.ErrConfiguration)

	// The actionable message lives on the underlying validation error,
	// reachable via errors.Unwrap.
	inner := stderrors.Unwrap(err)
	require.Error(t, inner, "wrapped validation error must be reachable via Unwrap")
	assert.Contains(t, inner.Error(), "no auth source configured",
		"error must use the documented phrase so callers can grep for it")
	assert.Contains(t, inner.Error(), "WithAccessManager",
		"error must point users at WithAccessManager")
	assert.Contains(t, inner.Error(), "WithAnonymous",
		"error must point users at WithAnonymous")
}

// TestNewWithAnonymousSucceeds verifies WithAnonymous is the explicit
// auth-less escape hatch — construction succeeds without any AccessManager.
func TestNewWithAnonymousSucceeds(t *testing.T) {
	c, err := New(
		WithEnvironment(config.EnvironmentLocal),
		WithAnonymous(),
	)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.True(t, c.GetConfig().Anonymous,
		"WithAnonymous must flip the Anonymous flag on the underlying Config")
	require.False(t, c.GetConfig().AccessManager.Enabled,
		"Anonymous mode must leave AccessManager disabled")
}

func TestNewClassifiesLocalAccessManagerBootstrapFailureAsConfiguration(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AccessManager = auth.AccessManager{
		Enabled:      true,
		Address:      "https://auth.example.com",
		ClientID:     "client-id",
		ClientSecret: "super-secret-value",
	}
	cfg.HTTPClient = nil

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	_, err := New(WithConfig(cfg), WithLogger(logger))
	require.Error(t, err)
	require.True(t, sdkerrors.IsConfigurationError(err), "local bootstrap validation failures are configuration errors")
	require.False(t, sdkerrors.IsAuthenticationError(err), "local bootstrap validation failures must not be authentication errors")

	rendered := err.Error()
	assert.Contains(t, rendered, "operation=access_manager.token_request")
	assert.Contains(t, rendered, "phase=token_fetch")
	assert.Contains(t, rendered, "httpRequestSent=false")
	assert.Contains(t, rendered, "localValidationFailed=true")
	assert.Contains(t, rendered, "validationReason=nil_http_client")
	assert.NotContains(t, rendered, "super-secret-value")

	logLine := logs.String()
	assert.Contains(t, logLine, `"sdk.name":"midaz-go-sdk"`)
	assert.Contains(t, logLine, `"sdk.component":"bootstrap"`)
	assert.Contains(t, logLine, `"operation":"midaz.New"`)
	assert.Contains(t, logLine, `"failure.phase":"token_fetch"`)
	assert.Contains(t, logLine, `"auth.scheme":"https"`)
	assert.Contains(t, logLine, `"auth.host":"auth.example.com"`)
	assert.Contains(t, logLine, `"auth.path":"/v1/login/oauth/access_token"`)
	assert.Contains(t, logLine, `"httpRequestSent":false`)
	assert.Contains(t, logLine, `"localValidationFailed":true`)
	assert.Contains(t, logLine, `"validationReason":"nil_http_client"`)
	assert.NotContains(t, logLine, "super-secret-value")
	assert.NotContains(t, logLine, "Authorization")
}

func TestNewPreservesAccessManagerBootstrapUpstreamHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		name          string
		code          int
		check         func(error) bool
		mustNotMatch  []func(error) bool
		wantCategory  sdkerrors.ErrorCategory
		wantErrorCode sdkerrors.ErrorCode
	}{
		{
			name:          "401 authentication",
			code:          http.StatusUnauthorized,
			check:         sdkerrors.IsAuthenticationError,
			mustNotMatch:  []func(error) bool{sdkerrors.IsAuthorizationError},
			wantCategory:  sdkerrors.CategoryAuthentication,
			wantErrorCode: sdkerrors.CodeAuthentication,
		},
		{
			name:          "403 authorization",
			code:          http.StatusForbidden,
			check:         sdkerrors.IsAuthorizationError,
			mustNotMatch:  []func(error) bool{sdkerrors.IsAuthenticationError},
			wantCategory:  sdkerrors.CategoryAuthorization,
			wantErrorCode: sdkerrors.CodePermission,
		},
		{
			name:          "429 rate limit",
			code:          http.StatusTooManyRequests,
			check:         sdkerrors.IsRateLimitError,
			mustNotMatch:  []func(error) bool{sdkerrors.IsAuthenticationError, sdkerrors.IsAuthorizationError, sdkerrors.IsAuthError},
			wantCategory:  sdkerrors.CategoryLimitExceeded,
			wantErrorCode: sdkerrors.CodeRateLimit,
		},
		{
			name:          "500 internal",
			code:          http.StatusInternalServerError,
			check:         sdkerrors.IsInternalError,
			mustNotMatch:  []func(error) bool{sdkerrors.IsAuthenticationError, sdkerrors.IsAuthorizationError, sdkerrors.IsAuthError},
			wantCategory:  sdkerrors.CategoryInternal,
			wantErrorCode: sdkerrors.CodeInternal,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"error":"upstream failed","access_token":"secret-token-value"}`))
			}))
			defer server.Close()

			_, err := New(
				WithAccessManager(AccessManager{
					Address:      server.URL,
					ClientID:     "client-id-" + tc.name,
					ClientSecret: "super-secret-value",
				}),
				WithHTTPClient(server.Client()),
			)

			require.Error(t, err)
			require.True(t, tc.check(err), "unexpected category for %d: %v", tc.code, err)
			for _, mustNotMatch := range tc.mustNotMatch {
				assert.False(t, mustNotMatch(err), "unexpected auth-like classification for %d: %v", tc.code, err)
			}

			actual, ok := sdkerrors.ActualHTTPStatus(err)
			require.True(t, ok)
			assert.Equal(t, tc.code, actual)
			assert.Equal(t, tc.code, sdkerrors.SuggestedHTTPStatus(err))
			assert.True(t, sdkerrors.HTTPRequestSent(err))
			assert.True(t, sdkerrors.HTTPResponseReceived(err))

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			assert.Equal(t, tc.wantCategory, sdkErr.Category)
			assert.Equal(t, tc.wantErrorCode, sdkErr.Code)
			assert.Equal(t, tc.code, sdkErr.GetStatusCode())
			assert.Equal(t, sdkerrors.ErrorSourceHTTPResponse, sdkErr.GetSource())
			assert.Equal(t, sdkerrors.StatusCodeSourceUpstream, sdkErr.GetStatusCodeSource())
			assert.NotContains(t, err.Error(), "super-secret-value")
			assert.NotContains(t, err.Error(), "secret-token-value")
		})
	}
}

// TestWithAccessManagerAutoEnables verifies the v3 ergonomic decision: callers
// don't set the Enabled field themselves — the act of calling
// WithAccessManager IS the opt-in.
//
// We assert the auto-enabled flag at the Config layer rather than letting the
// full midaz.New() flow run; NewEntityWithConfig eagerly fetches a token from
// the configured Access Manager URL, which would force every Config-shape
// test to spin up a full OAuth-mock server. That coverage belongs in a
// dedicated integration test (entities/access_manager_test.go), not here.
func TestWithAccessManagerAutoEnables(t *testing.T) {
	cfg, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentLocal),
		config.WithAccessManager(AccessManager{
			Address:      "https://auth.example.com",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		}),
	)
	require.NoError(t, err)
	require.True(t, cfg.AccessManager.Enabled,
		"WithAccessManager must auto-enable; the user did not set Enabled=true")
	require.Equal(t, "https://auth.example.com", cfg.AccessManager.Address)
	require.False(t, cfg.Anonymous,
		"WithAccessManager must clear any prior Anonymous flag")
}

// TestAccessManagerAndAnonymousMutualExclusion verifies that applying
// WithAnonymous after WithAccessManager flips the active auth source to
// Anonymous, AND vice-versa. Last-applied wins.
//
// Tested at the Config layer to avoid the eager token-fetch path in
// NewEntityWithConfig — see TestWithAccessManagerAutoEnables for the
// rationale.
func TestAccessManagerAndAnonymousMutualExclusion(t *testing.T) {
	t.Run("WithAnonymous after WithAccessManager wins", func(t *testing.T) {
		cfg, err := config.NewConfig(
			config.WithEnvironment(config.EnvironmentLocal),
			config.WithAccessManager(AccessManager{
				Address:      "https://unused.example.com",
				ClientID:     "x",
				ClientSecret: "y",
			}),
			config.WithAnonymous(),
		)
		require.NoError(t, err)
		assert.True(t, cfg.Anonymous)
		assert.False(t, cfg.AccessManager.Enabled,
			"WithAnonymous must disable a previously-applied AccessManager")
	})

	t.Run("WithAccessManager after WithAnonymous wins", func(t *testing.T) {
		cfg, err := config.NewConfig(
			config.WithEnvironment(config.EnvironmentLocal),
			config.WithAnonymous(),
			config.WithAccessManager(AccessManager{
				Address:      "https://unused.example.com",
				ClientID:     "x",
				ClientSecret: "y",
			}),
		)
		require.NoError(t, err)
		assert.False(t, cfg.Anonymous,
			"WithAccessManager must clear a previous Anonymous flag")
		assert.True(t, cfg.AccessManager.Enabled)
	})
}
