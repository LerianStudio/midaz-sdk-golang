package config

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/auth"
)

// ----- M5: retry-wait pair inversion -----

// TestWithRetryWaitMin_RejectsValueGreaterThanCurrentMax exercises the
// new defensive check: WithRetryWaitMin must refuse a value that would
// invert the (min, max) pair. The default RetryWaitMax is 30s; a 60s
// minimum would silently produce min > max without this guard.
func TestWithRetryWaitMin_RejectsValueGreaterThanCurrentMax(t *testing.T) {
	_, err := NewConfig(
		WithRetryWaitMin(60*time.Second),
		WithAnonymous(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minimum wait time must be less than or equal to maximum wait time")
}

// TestWithRetryWaitMin_AcceptsValueAtCurrentMax confirms the boundary
// condition: a minimum equal to the current maximum is allowed (the
// retry layer treats min == max as a fixed wait, which is meaningful).
func TestWithRetryWaitMin_AcceptsValueAtCurrentMax(t *testing.T) {
	cfg, err := NewConfig(
		WithRetryWaitMax(30*time.Second),
		WithRetryWaitMin(30*time.Second),
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, cfg.RetryWaitMin)
	assert.Equal(t, 30*time.Second, cfg.RetryWaitMax)
}

// TestWithRetryWaitMin_HonorsRaisedMaxFirst verifies the option-order
// case: raise the max first, then a previously-rejected min becomes
// valid. This is the documented escape hatch for callers who want
// non-default wait pairs.
func TestWithRetryWaitMin_HonorsRaisedMaxFirst(t *testing.T) {
	cfg, err := NewConfig(
		WithRetryWaitMax(120*time.Second),
		WithRetryWaitMin(60*time.Second),
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, cfg.RetryWaitMin)
	assert.Equal(t, 120*time.Second, cfg.RetryWaitMax)
}

// TestValidateConfig_RejectsInvertedRetryWaitPair is the defense-in-depth
// case: a caller mutating the fields directly on a Config they own
// (e.g. starting from DefaultConfig) can still land in min > max if
// they bypass the option chain. validateConfig must catch that.
func TestValidateConfig_RejectsInvertedRetryWaitPair(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryWaitMin = 60 * time.Second
	cfg.RetryWaitMax = 30 * time.Second

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minimum wait time must be less than or equal to maximum wait time")
}

// ----- M6: strict ParseBool for boolean env vars -----

// TestFromEnvironment_DebugStrictParseBool covers the canonical accepted
// forms and the rejected forms. The previous behavior treated any
// non-"true" value as false silently — a typo flipped the flag the
// wrong way without warning. Strict parsing turns that into an error.
func TestFromEnvironment_DebugStrictParseBool(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		wantDebug   bool
		wantErr     bool
		errContains string
	}{
		{name: "true lowercase", envValue: "true", wantDebug: true},
		{name: "True", envValue: "True", wantDebug: true},
		{name: "TRUE", envValue: "TRUE", wantDebug: true},
		{name: "1", envValue: "1", wantDebug: true},
		{name: "t", envValue: "t", wantDebug: true},
		{name: "T", envValue: "T", wantDebug: true},
		{name: "false lowercase", envValue: "false", wantDebug: false},
		{name: "False", envValue: "False", wantDebug: false},
		{name: "FALSE", envValue: "FALSE", wantDebug: false},
		{name: "0", envValue: "0", wantDebug: false},
		{name: "f", envValue: "f", wantDebug: false},
		{name: "F", envValue: "F", wantDebug: false},
		{name: "yes rejected", envValue: "yes", wantErr: true, errContains: "invalid MIDAZ_DEBUG"},
		{name: "no rejected", envValue: "no", wantErr: true, errContains: "invalid MIDAZ_DEBUG"},
		{name: "on rejected", envValue: "on", wantErr: true, errContains: "invalid MIDAZ_DEBUG"},
		{name: "off rejected", envValue: "off", wantErr: true, errContains: "invalid MIDAZ_DEBUG"},
		{name: "garbage rejected", envValue: "tru3", wantErr: true, errContains: "invalid MIDAZ_DEBUG"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MIDAZ_DEBUG", tc.envValue)

			cfg, err := NewConfig(FromEnvironment(), WithAnonymous())

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantDebug, cfg.Debug)
		})
	}
}

// TestFromEnvironment_DebugEmptyEnvLeavesDefault covers the
// "unset" path: an absent or empty env var must not flip an existing
// programmatic value. Previously, the code branched on `== "true"`
// and silently no-op'd for every other value, including empty;
// the new strict-parse path must preserve that no-op semantics.
func TestFromEnvironment_DebugEmptyEnvLeavesDefault(t *testing.T) {
	unsetEnv(t, "MIDAZ_DEBUG")

	cfg, err := NewConfig(
		WithDebug(true),
		FromEnvironment(),
		WithAnonymous(),
	)
	require.NoError(t, err)
	assert.True(t, cfg.Debug, "empty MIDAZ_DEBUG must not overwrite programmatic Debug=true")
}

// TestFromEnvironment_IdempotencyStrictParseBool mirrors the debug
// test for MIDAZ_IDEMPOTENCY. The most damaging silent default in v2
// was MIDAZ_IDEMPOTENCY=yes — a sensible-looking value that flipped
// idempotency OFF, breaking at-least-once safety expectations.
func TestFromEnvironment_IdempotencyStrictParseBool(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		wantIdempotent bool
		wantErr        bool
		errContains    string
	}{
		{name: "true", envValue: "true", wantIdempotent: true},
		{name: "false", envValue: "false", wantIdempotent: false},
		{name: "1", envValue: "1", wantIdempotent: true},
		{name: "0", envValue: "0", wantIdempotent: false},
		{name: "yes rejected (was the worst silent default)", envValue: "yes", wantErr: true, errContains: "invalid MIDAZ_IDEMPOTENCY"},
		{name: "no rejected", envValue: "no", wantErr: true, errContains: "invalid MIDAZ_IDEMPOTENCY"},
		{name: "garbage rejected", envValue: "maybe", wantErr: true, errContains: "invalid MIDAZ_IDEMPOTENCY"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MIDAZ_IDEMPOTENCY", tc.envValue)

			cfg, err := NewConfig(FromEnvironment(), WithAnonymous())

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantIdempotent, cfg.EnableIdempotency)
		})
	}
}

func TestFromEnvironment_ErrorExposeBodyStrictParseBool(t *testing.T) {
	tests := []struct {
		name       string
		envValue   string
		wantExpose bool
		wantErr    bool
	}{
		{name: "true", envValue: "true", wantExpose: true},
		{name: "false", envValue: "false", wantExpose: false},
		{name: "1", envValue: "1", wantExpose: true},
		{name: "0", envValue: "0", wantExpose: false},
		{name: "yes rejected", envValue: "yes", wantErr: true},
		{name: "no rejected", envValue: "no", wantErr: true},
		{name: "garbage rejected", envValue: "maybe", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MIDAZ_ERROR_EXPOSE_BODY", tc.envValue)

			cfg, err := NewConfig(FromEnvironment(), WithAnonymous())

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid MIDAZ_ERROR_EXPOSE_BODY")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantExpose, cfg.ExposeErrorBody)
		})
	}
}

// TestFromEnvironment_PluginAuthEnabledStrictParseBool covers the
// remaining boolean env var on the auth path. PLUGIN_AUTH_ENABLED=yes
// previously silently disabled auth (treated as not-"true" → false),
// which would push misconfiguration to runtime as 401 cascades.
//
// Note: the test omits WithAnonymous so the env var alone drives the
// auth source. Adding WithAnonymous after FromEnvironment would clear
// AccessManager.Enabled regardless of env (last-applied option wins),
// which is correct behavior but would mask the env-parsing assertion.
func TestFromEnvironment_PluginAuthEnabledStrictParseBool(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		wantEnabled bool
		wantErr     bool
		errContains string
		// useAnonymous adds WithAnonymous when env=false to satisfy the
		// auth-required gate (PLUGIN_AUTH_ENABLED=false leaves no active
		// auth source).
		useAnonymous bool
	}{
		{name: "true enables", envValue: "true", wantEnabled: true},
		{name: "false disables", envValue: "false", wantEnabled: false, useAnonymous: true},
		{name: "yes rejected", envValue: "yes", wantErr: true, errContains: "invalid PLUGIN_AUTH_ENABLED"},
		{name: "garbage rejected", envValue: "enabled", wantErr: true, errContains: "invalid PLUGIN_AUTH_ENABLED"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PLUGIN_AUTH_ENABLED", tc.envValue)
			t.Setenv("PLUGIN_AUTH_ADDRESS", "https://auth.example.com")
			t.Setenv("MIDAZ_CLIENT_ID", "id")
			t.Setenv("MIDAZ_CLIENT_SECRET", "secret")
			t.Setenv("MIDAZ_ENVIRONMENT", "development")

			options := []Option{FromEnvironment()}
			if tc.useAnonymous {
				options = append(options, WithAnonymous())
			}

			cfg, err := NewConfig(options...)

			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantEnabled, cfg.AccessManager.Enabled)
		})
	}
}

func TestFromEnvironment_AccessManagerAllowInsecureHTTPStrictParseBool(t *testing.T) {
	t.Run("true applies without PLUGIN_AUTH_ENABLED", func(t *testing.T) {
		unsetEnv(t, "PLUGIN_AUTH_ENABLED")
		t.Setenv("MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP", "true")

		cfg, err := NewConfig(
			WithEnvironment(EnvironmentDevelopment),
			WithAccessManager(auth.AccessManager{
				Address:      "http://auth.internal.example.com",
				ClientID:     "client-id",
				ClientSecret: "client-secret",
			}),
			FromEnvironment(),
		)
		require.NoError(t, err)

		assert.True(t, cfg.AccessManager.AllowInsecureHTTP)
	})

	t.Run("false overrides programmatic true without PLUGIN_AUTH_ENABLED", func(t *testing.T) {
		unsetEnv(t, "PLUGIN_AUTH_ENABLED")
		t.Setenv("MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP", "false")

		cfg, err := NewConfig(
			WithEnvironment(EnvironmentDevelopment),
			WithAccessManager(auth.AccessManager{
				Address:           "https://auth.example.com",
				ClientID:          "client-id",
				ClientSecret:      "client-secret",
				AllowInsecureHTTP: true,
			}),
			FromEnvironment(),
		)
		require.NoError(t, err)

		assert.False(t, cfg.AccessManager.AllowInsecureHTTP)
	})

	t.Run("invalid value rejected without PLUGIN_AUTH_ENABLED", func(t *testing.T) {
		unsetEnv(t, "PLUGIN_AUTH_ENABLED")
		t.Setenv("MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP", "yes")

		_, err := NewConfig(FromEnvironment(), WithAnonymous())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid MIDAZ_ACCESS_MANAGER_ALLOW_INSECURE_HTTP")
	})
}

func TestFromEnvironment_RejectsSkipAuthCheck(t *testing.T) {
	t.Setenv("MIDAZ_SKIP_AUTH_CHECK", "true")

	_, err := NewConfig(FromEnvironment(), WithAnonymous())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MIDAZ_SKIP_AUTH_CHECK is not supported")
}

// ----- M7: programmatic AccessManager preservation -----

// TestFromEnvironment_PreservesProgrammaticAccessManagerWhenEnvEmpty is
// the regression test for the "FromEnvironment silently wipes credentials"
// bug. The previous configureAccessManager unconditionally overwrote
// Address/ClientID/ClientSecret with os.Getenv's empty-string return
// whenever PLUGIN_AUTH_ENABLED was set, so chaining
// WithAccessManager(am) → FromEnvironment() with PLUGIN_AUTH_ENABLED=false
// in env silently wiped the programmatic credentials.
//
// PLUGIN_AUTH_ENABLED is the user's explicit on/off toggle and the
// env value still wins for that bool — but the credential strings now
// only get overwritten when the corresponding env var is non-empty.
// A subsequent WithAnonymous satisfies the auth-required gate so the
// test can introspect the preserved values without failing validation.
func TestFromEnvironment_PreservesProgrammaticAccessManagerWhenEnvEmpty(t *testing.T) {
	// Programmatic credentials are populated; env sets only
	// PLUGIN_AUTH_ENABLED=false with no address/id/secret. FromEnvironment
	// must NOT zero out the programmatic credential strings.
	t.Setenv("PLUGIN_AUTH_ENABLED", "false")
	unsetEnv(t, "PLUGIN_AUTH_ADDRESS")
	unsetEnv(t, "MIDAZ_CLIENT_ID")
	unsetEnv(t, "MIDAZ_CLIENT_SECRET")

	cfg, err := NewConfig(
		WithEnvironment(EnvironmentDevelopment),
		WithAccessManager(auth.AccessManager{
			Address:      "https://programmatic.example.com",
			ClientID:     "programmatic-id",
			ClientSecret: "programmatic-secret",
		}),
		FromEnvironment(),
		WithAnonymous(),
	)
	require.NoError(t, err)

	assert.False(t, cfg.AccessManager.Enabled,
		"WithAnonymous (last-applied) must clear Enabled")
	assert.Equal(t, "https://programmatic.example.com", cfg.AccessManager.Address,
		"programmatic Address must survive empty PLUGIN_AUTH_ADDRESS")
	assert.Equal(t, "programmatic-id", cfg.AccessManager.ClientID,
		"programmatic ClientID must survive empty MIDAZ_CLIENT_ID")
	assert.Equal(t, "programmatic-secret", cfg.AccessManager.ClientSecret,
		"programmatic ClientSecret must survive empty MIDAZ_CLIENT_SECRET")
}

// TestFromEnvironment_OverwritesProgrammaticAccessManagerOnExplicitEnv
// is the matching positive case: when env DOES set a credential field,
// it overrides the programmatic value (this is the intended precedence
// of "env over code" for FromEnvironment-applied config).
func TestFromEnvironment_OverwritesProgrammaticAccessManagerOnExplicitEnv(t *testing.T) {
	t.Setenv("PLUGIN_AUTH_ENABLED", "true")
	t.Setenv("PLUGIN_AUTH_ADDRESS", "https://env.example.com")
	t.Setenv("MIDAZ_CLIENT_ID", "env-id")
	t.Setenv("MIDAZ_CLIENT_SECRET", "env-secret")
	t.Setenv("MIDAZ_ENVIRONMENT", "development")

	cfg, err := NewConfig(
		WithAccessManager(auth.AccessManager{
			Address:      "https://programmatic.example.com",
			ClientID:     "programmatic-id",
			ClientSecret: "programmatic-secret",
		}),
		FromEnvironment(),
	)
	require.NoError(t, err)

	assert.True(t, cfg.AccessManager.Enabled)
	assert.Equal(t, "https://env.example.com", cfg.AccessManager.Address)
	assert.Equal(t, "env-id", cfg.AccessManager.ClientID)
	assert.Equal(t, "env-secret", cfg.AccessManager.ClientSecret)
}

// ----- M8: TLS minimum version on default Transport -----

// TestNewDefaultHTTPClient_PinsTLSMinimumVersion asserts the default
// HTTP client's transport explicitly pins TLS 1.2 as the floor. Go's
// runtime default already lands here, but pinning it makes the floor
// visible and insulates the SDK from any future runtime change that
// would lower it.
func TestNewDefaultHTTPClient_PinsTLSMinimumVersion(t *testing.T) {
	client := NewDefaultHTTPClient(30 * time.Second)
	require.NotNil(t, client)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "default client must use *http.Transport")
	require.NotNil(t, transport.TLSClientConfig, "default transport must set TLSClientConfig")

	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion,
		"default transport must pin TLS 1.2 as the minimum version")
}
