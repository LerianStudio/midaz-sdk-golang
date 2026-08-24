package midaz

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- from client_test.go ---

// createTestConfig creates a test config with sensible defaults.
// It uses t.Setenv for automatic cleanup and t.Fatalf on config errors.
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	cfg, err := config.NewConfig(
		config.WithAnonymous(),
		config.WithEnvironment(config.EnvironmentLocal),
	)
	if err != nil {
		t.Fatalf("createTestConfig: %v", err)
	}

	return cfg
}

func TestNewClient(t *testing.T) {
	// Test creating a new client with a test config
	client, err := New(WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Check that default config was created
	if client.config == nil {
		t.Fatal("Expected config to be set, got nil")
	}

	// Check that context was set
	if client.ctx == nil {
		t.Fatal("Expected context to be set, got nil")
	}

	// Test creating a client with options
	customHTTPClient := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Create a base config
	testCfg := createTestConfig(t)

	client, err = New(
		WithConfig(testCfg),
		WithHTTPClient(customHTTPClient),
		WithLedgerURL("https://test.example.com"),
		WithTracerURL("https://test.example.com/tracer/v1"),
		WithTimeout(30*time.Second),
		WithDebug(true),
		WithEnvironment(config.EnvironmentDevelopment),
	)
	if err != nil {
		t.Fatalf("Failed to create client with options: %v", err)
	}

	// Check that all options were applied
	if client.config.AccessManager.Enabled {
		t.Errorf("Expected AccessManager.Enabled to be false, got true")
	}

	if client.config.HTTPClient == customHTTPClient {
		t.Error("Expected HTTP client to be cloned before SDK redirect policy installation")
	}

	if client.config.HTTPClient.Timeout != customHTTPClient.Timeout {
		t.Errorf("Expected HTTP client timeout to be preserved, got %s", client.config.HTTPClient.Timeout)
	}

	if customHTTPClient.CheckRedirect != nil {
		t.Error("Expected caller-owned HTTP client to remain unmodified")
	}

	if client.config.HTTPClient.CheckRedirect == nil {
		t.Error("Expected SDK HTTP client clone to install redirect policy")
	}

	if client.config.Environment != config.EnvironmentDevelopment {
		t.Errorf("Expected environment to be 'development', got '%s'", client.config.Environment)
	}

	if !client.config.Debug {
		t.Error("Expected debug to be true")
	}

	if got := client.config.ServiceURLs[config.ServiceTracer]; got != "https://test.example.com/tracer/v1" {
		t.Errorf("Expected Tracer URL to be applied, got %q", got)
	}

	require.NotNil(t, client.Entity)
	require.NotNil(t, client.Holders)
	require.NotNil(t, client.MetadataIndexes)

	// Test creating a client with a complete config
	cfg, err := config.NewConfig(
		config.WithAnonymous(),
		config.WithEnvironment(config.EnvironmentProduction),
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	client, err = New(WithConfig(cfg))
	if err != nil {
		t.Fatalf("Failed to create client with config: %v", err)
	}

	if client.config.Environment != config.EnvironmentProduction {
		t.Errorf("Expected environment to be 'production', got '%s'", client.config.Environment)
	}
}

func TestEntityAlwaysInitialized(t *testing.T) {
	// v3: Entity surface is always initialized; no opt-in required.
	c, err := New(WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	require.NotNil(t, c.Entity, "v3 must always initialize Entity")
	require.NotNil(t, c.Accounts)
	require.NotNil(t, c.Transactions)
	require.NotNil(t, c.Organizations)
}

func TestGetConfig(t *testing.T) {
	client, err := New(WithConfig(createTestConfig(t)))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	cfg := client.GetConfig()
	if cfg == nil {
		t.Fatal("Expected config to be returned, got nil")
	}
}

// --- from client_coverage_test.go ---

func TestClientOptionsAccessorsAndConstructors(t *testing.T) {
	ctx := context.WithValue(context.Background(), testContextKey{}, "value")
	customHTTPClient := &http.Client{Timeout: 5 * time.Second}

	// v3: build a Config with retry knobs at the config layer using the
	// individual single-concern Options. The deleted v2 WithRetries(int,dur,dur)
	// macro was a 3-positional-arg shortcut; the v3 expression is more verbose
	// but every Option has exactly one concern. WithRetryOptions at the client
	// layer is the override path for retry-package knobs that don't have a
	// Config counterpart (BackoffFactor, JitterFactor, etc.).
	cfg := createTestConfig(t)
	require.NoError(t, config.WithMaxRetries(2)(cfg))
	require.NoError(t, config.WithRetryWaitMin(10*time.Millisecond)(cfg))
	require.NoError(t, config.WithRetryWaitMax(20*time.Millisecond)(cfg))

	c, err := New(
		WithConfig(cfg),
		WithContext(ctx),
		WithHTTPClient(customHTTPClient),
		WithBaseURL("https://api.example.com"),
		WithUserAgent("midaz-test/coverage"),
		WithCustomRetryPolicy(func(resp *http.Response, err error) bool {
			return err != nil || (resp != nil && resp.StatusCode == http.StatusTooManyRequests)
		}),
		// v3: WithObservability(t,m,l bool) was deleted. The replacement is
		// WithObservabilityOptions(observability.WithComponentEnabled(t,m,l)).
		// All-disabled here matches the New()-installed default; this call
		// exists to exercise the WithObservabilityOptions path without
		// changing the effective enabled state.
		WithObservabilityOptions(observability.WithComponentEnabled(false, false, false)),
	)
	require.NoError(t, err)
	require.NotNil(t, c.Entity)

	assert.NotSame(t, customHTTPClient, c.config.HTTPClient)
	assert.Equal(t, customHTTPClient.Timeout, c.config.HTTPClient.Timeout)
	assert.Nil(t, customHTTPClient.CheckRedirect)
	assert.NotNil(t, c.config.HTTPClient.CheckRedirect)
	assert.Equal(t, "midaz-test/coverage", c.config.UserAgent)
	// In v3, retries are off iff MaxRetries == 0; here we set 2 above.
	assert.Equal(t, 2, c.config.MaxRetries)
	assert.Equal(t, 10*time.Millisecond, c.config.RetryWaitMin)
	assert.Equal(t, 20*time.Millisecond, c.config.RetryWaitMax)
	assert.Equal(t, "value", c.GetContext().Value(testContextKey{}))
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.Logger())
	// In v3, WithObservabilityOptions builds a MetricsCollector whenever
	// provider.IsEnabled() returns true, regardless of which OTel components
	// (tracing/metrics/logging) are individually toggled. The collector
	// emits noop counters when the metrics component is off, so its
	// presence is harmless. The deleted v2 WithObservability(t,m,l)
	// macro short-circuited via its closure bool — that asymmetry is now
	// gone in favor of uniform construction.
	assert.NotNil(t, c.GetMetricsCollector())
	// v3: the six factory-trap methods (NewAccount/NewLedger/NewOrganization/
	// NewTransaction/NewOperation/NewAsset on *Client) were deleted —
	// they returned bare zero-value structs without engaging the API and had
	// zero production callers. Use the models package directly when you need
	// a zero-value request shape:
	//   in := &models.CreateAccountInput{...}
	assert.Equal(t, Version, c.GetVersion())

	called := false

	require.NoError(t, c.Trace("disabled-span", func(traceCtx context.Context) error {
		called = true

		assert.Equal(t, "value", traceCtx.Value(testContextKey{}))

		return nil
	}))
	assert.True(t, called)

	returnedConfig := c.GetConfiguration()
	require.NotNil(t, returnedConfig)
	returnedConfig.ServiceURLs[config.ServiceOnboarding] = "https://mutated.example.com"
	assert.NotEqual(t, "https://mutated.example.com", c.GetConfig().ServiceURLs[config.ServiceOnboarding])

	require.NoError(t, c.Shutdown(context.Background()))
}

func TestWithRetryOptionsRejectsInvalidOptions(t *testing.T) {
	err := validateRetryOptions(retry.WithMaxRetries(-1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry option at index 0 failed")

	err = validateRetryOptions(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry option at index 0 cannot be nil")

	_, err = New(WithConfig(createTestConfig(t)), WithRetryOptions(retry.WithMaxRetries(-1)))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))

	_, err = New(WithConfig(createTestConfig(t)), WithRetryOptions(nil))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
}

func TestClientOptionErrorsAndNilReceivers(t *testing.T) {
	// v3: every construction error is a typed *errors.Error with
	// Category=CategoryConfiguration. The wrapped underlying cause is
	// reachable via errors.Unwrap.

	_, err := New(nil)
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err), "nil option should yield ErrConfiguration")
	assert.Contains(t, err.Error(), "index 0", "error should identify which option index was nil")

	_, err = New(WithConfig(nil))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
	assert.Contains(t, errors.Unwrap(err).Error(), "config cannot be nil",
		"underlying option error should be reachable via Unwrap")

	var nilContext context.Context

	_, err = New(WithConfig(createTestConfig(t)), WithContext(nilContext))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
	assert.Contains(t, errors.Unwrap(err).Error(), "context cannot be nil")

	_, err = New(WithConfig(createTestConfig(t)), WithBaseURL("://bad-url"))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))
	assert.Contains(t, errors.Unwrap(err).Error(), "invalid base URL")

	// v3: DisableRetries() was renamed to WithoutRetries(). Sets MaxRetries=0;
	// the EnableRetries field was deleted (one source of truth).
	c, err := New(WithConfig(createTestConfig(t)), WithoutRetries())
	require.NoError(t, err)
	assert.Equal(t, 0, c.config.MaxRetries, "WithoutRetries should set MaxRetries to 0")
	require.NotNil(t, c.Entity)

	assert.Nil(t, (*Client)(nil).GetConfiguration())
	require.NoError(t, (*Client)(nil).Shutdown(context.Background()))
	require.Error(t, (*Client)(nil).Trace("nil", func(context.Context) error { return nil }))
	require.Error(t, c.Trace("nil-callback", nil))

	expectedErr := errors.New("boom")
	assert.ErrorIs(t, c.Trace("callback-error", func(context.Context) error { return expectedErr }), expectedErr)
}

// TestNewSurfacesEntitySetupCause pins the operator-facing rendering of a
// construction failure. The entity layer composes an actionable diagnostic (the
// rejected "/v1" suffix, the variable to edit); before this test existed, New
// wrapped it in a configuration error whose Error() printed only "failed to
// initialize entity API" — so the one line that reaches a log aggregator carried
// none of the reason. err.Error() must carry the cause, and errors.Unwrap must
// still reach it.
func TestNewSurfacesEntitySetupCause(t *testing.T) {
	_, err := New(WithConfig(createTestConfig(t)), WithBaseURL("http://localhost:3002/v1"))
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))

	assert.Contains(t, err.Error(), `must not end in "/v1"`,
		"Error() must carry the cause, not just the generic bootstrap message")
	assert.Contains(t, err.Error(), "MIDAZ_LEDGER_URL",
		"Error() must name the setting the operator has to edit")

	require.Error(t, errors.Unwrap(err), "the cause must stay reachable via errors.Unwrap")
	assert.Contains(t, errors.Unwrap(err).Error(), `must not end in "/v1"`)
}

func TestClientObservabilityOptionVariants(t *testing.T) {
	provider, err := observability.New(context.Background(), observability.WithServiceName("coverage-provider"), observability.WithComponentEnabled(false, false, false))
	require.NoError(t, err)

	c, err := New(
		WithConfig(createTestConfig(t)),
		WithObservabilityProvider(provider),
		WithObservabilityOptions(observability.WithServiceName("coverage-options"), observability.WithComponentEnabled(false, false, false)),
	)
	require.NoError(t, err)
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.GetMetricsCollector())

	require.NoError(t, c.Shutdown(context.Background()))
}

func TestClientCollectorEndpointOptionCreatesProvider(t *testing.T) {
	// v3: midaz.WithCollectorEndpoint was deleted. It was sugar for the
	// equivalent observability.Option chain; users now compose explicitly.
	// This test ensures the canonical path produces a working provider +
	// metrics collector for the same input shape (collector endpoint +
	// all components enabled).
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithObservabilityOptions(
			observability.WithServiceName("midaz-go-sdk"),
			observability.WithCollectorEndpoint("localhost:4317"),
			observability.WithComponentEnabled(true, true, true),
		),
	)
	require.NoError(t, err)
	assert.NotNil(t, c.GetObservabilityProvider())
	assert.NotNil(t, c.GetMetricsCollector())
}

type testContextKey struct{}

// --- from midaz_surface_regression_test.go ---

// TestClientSetObservability_PropagatesToGetObservabilityProvider is the H1
// regression test. Before the fix, Client embedded *entities.Entity, so the
// promoted Entity.SetObservability method updated only Entity's view; Client
// kept its own duplicate observability field. Calling
// c.SetObservability(p) (which dispatched to Entity via embedding) followed
// by c.GetObservabilityProvider() (which read Client's stale copy) returned
// the OLD provider. The two views silently drifted.
//
// Post-fix: Client wraps SetObservability explicitly and routes
// GetObservabilityProvider through the Entity, the single source of truth.
func TestClientSetObservability_PropagatesToGetObservabilityProvider(t *testing.T) {
	c, err := New(
		WithConfig(createTestConfig(t)),
	)
	require.NoError(t, err)

	defaultProvider := c.GetObservabilityProvider()
	require.NotNil(t, defaultProvider, "default disabled provider should be installed by New")

	// Build a fresh provider distinct from the New()-installed default.
	replacement, err := observability.New(context.Background(),
		observability.WithServiceName("h1-regression-replacement"),
		observability.WithComponentEnabled(false, false, false),
	)
	require.NoError(t, err)

	require.NoError(t, c.SetObservability(replacement))

	// The drift bug returned defaultProvider here. Post-fix, the Client view
	// must reflect the replacement.
	got := c.GetObservabilityProvider()
	require.NotNil(t, got)
	assert.Same(t, replacement, got,
		"GetObservabilityProvider must return the provider installed by SetObservability — not the stale Client-side copy")

	// Verify the Entity view agrees, since both views must be the same handle.
	assert.Same(t, replacement, c.Entity.GetObservabilityProvider(),
		"Entity view must agree with Client view")
}

// TestClientShutdown_UsesCanonicalProvider is the H1 follow-on. Shutdown
// previously called Shutdown on Client's stale duplicate field; post-fix it
// reads via GetObservabilityProvider, which routes through Entity. This
// guards against a regression where Shutdown closes the wrong handle.
func TestClientShutdown_UsesCanonicalProvider(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)))
	require.NoError(t, err)

	// Replacing observability post-construction must leave Shutdown reaching
	// the new provider, not the original disabled default.
	replacement, err := observability.New(context.Background(),
		observability.WithServiceName("h1-shutdown-replacement"),
		observability.WithComponentEnabled(false, false, false),
	)
	require.NoError(t, err)
	require.NoError(t, c.SetObservability(replacement))

	require.NoError(t, c.Shutdown(context.Background()),
		"Shutdown via the canonical provider must succeed with the replacement provider installed")
}

// TestWithConfig_AfterMutation_FailsLoud is the M1 regression test. v2's
// WithConfig silently replaced c.config mid-chain, voiding any prior
// WithBaseURL/WithUserAgent/WithDebug/etc. Now WithConfig errors out with a
// clear message when invoked after a config-mutating option.
func TestWithConfig_AfterMutation_FailsLoud(t *testing.T) {
	_, err := New(
		WithBaseURL("https://api.example.com"),
		WithConfig(createTestConfig(t)),
	)
	require.Error(t, err)
	assert.True(t, sdkerrors.IsConfigurationError(err))

	unwrapped := errors.Unwrap(err)
	require.Error(t, unwrapped, "outer config error must wrap a typed inner cause")
	assert.Contains(t, unwrapped.Error(), "WithConfig must come before any other config-mutating option",
		"WithConfig must surface the ordering rule explicitly")
}

// TestWithConfig_FirstInChain_StillWorks confirms the canonical placement —
// WithConfig at the head of the option chain — is still accepted.
func TestWithConfig_FirstInChain_StillWorks(t *testing.T) {
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithBaseURL("https://api.example.com"),
		WithUserAgent("m1-canonical-test/1.0"),
	)
	require.NoError(t, err)
	assert.Equal(t, "m1-canonical-test/1.0", c.config.UserAgent)
}

func TestWithErrorBodyExposure_PropagatesToConfig(t *testing.T) {
	c, err := New(
		WithAnonymous(),
		WithErrorBodyExposure(false),
	)
	require.NoError(t, err)

	assert.False(t, c.config.ExposeErrorBody)
}

// TestNewAccessManagerTokenFetchError_IsClassifiedAsAuth is the M4 regression
// test. v2 wrapped a transient Access Manager auth-fetch failure as a
// Configuration error, so callers using IsConfigurationError to gate retries
// would treat a temporary OAuth blip as a permanent setup mistake. Post-fix
// the classifier emits an Authentication error so IsAuthError(err) returns
// true and the caller knows to retry.
func TestNewAccessManagerTokenFetchError_IsClassifiedAsAuth(t *testing.T) {
	// We can't easily hit the auth fetch path without exercising the real
	// Access Manager request, so we test the classifier helper directly. The
	// integration path is exercised by entities/access_manager_test.go.
	wrappedErr := fmt.Errorf("failed to get token from plugin auth service: %w", auth.WrapAccessManagerTokenFetchError(errors.New("dial tcp: connection refused")))
	assert.True(t, isAccessManagerTokenFetchError(wrappedErr),
		"helper must match the entities-package wrap message")
	assert.False(t, isAccessManagerTokenFetchError(errors.New("missing onboarding URL in config")),
		"non-auth wraps must not match")
	assert.False(t, isAccessManagerTokenFetchError(nil), "nil must not match")
}

// TestFactoryTrapMethodsRemoved_AtCompileTime is the H13 regression. The six
// NewAccount/NewLedger/NewOrganization/NewTransaction/NewOperation/NewAsset
// methods on *Client returned bare zero-value structs without engaging the
// API, had zero production callers, and only existed to pad coverage. They
// are gone in v3. This test exists as a documentation marker — if you find
// yourself "fixing" a build failure here by re-adding the methods, the right
// move is to use the models package directly:
//
//	in := &models.CreateAccountInput{...}
//	c.Accounts.Create(ctx, ...)
//
// instead of the empty trap that NewAccount used to return.
func TestFactoryTrapMethodsRemoved_AtCompileTime(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)))
	require.NoError(t, err)

	clientType := reflect.TypeOf(c)
	for _, methodName := range []string{"NewAccount", "NewLedger", "NewOrganization", "NewTransaction", "NewOperation", "NewAsset"} {
		_, ok := clientType.MethodByName(methodName)
		assert.False(t, ok, "%s must stay removed from *Client", methodName)
	}

	require.NotNil(t, c.Accounts, "service surface must remain — only the trap factories are gone")
}

// TestServiceHandles_PersistAcrossPostConstructionMutations is the M2
// regression. Before the fix, setupEntity called entity.InitServices() AFTER
// NewEntityWithConfig already ran initServices() once internally. The double
// call recreated all 16 service entities just to push fresh config snapshots
// through them. Post-fix, setupEntity tunes the parent HTTPClient once and
// the per-service HTTPClient consolidation in entities/entity.go means every
// service shares the same instance — no second InitServices call, no
// recreated handles.
//
// This test asserts that mutating observability post-New does not silently
// swap the service handles out from under the caller. Code holding a
// reference to c.Accounts must keep working after a SetObservability call.
func TestServiceHandles_PersistAcrossPostConstructionMutations(t *testing.T) {
	c, err := New(
		WithConfig(createTestConfig(t)),
		WithUserAgent("m2-refresh-test/1.0"),
	)
	require.NoError(t, err)
	require.NotNil(t, c.Entity)

	// Capture the service handle before any further mutation.
	originalAccounts := c.Accounts

	// A subsequent SetObservability triggers Entity.RefreshHTTPConfiguration
	// internally via Entity.SetObservability. The service handles must
	// survive intact — only the config snapshots inside their HTTPClients
	// should refresh.
	replacement, err := observability.New(context.Background(),
		observability.WithServiceName("m2-refresh"),
		observability.WithComponentEnabled(false, false, false),
	)
	require.NoError(t, err)
	require.NoError(t, c.SetObservability(replacement))

	assert.Equal(t, originalAccounts, c.Accounts,
		"RefreshHTTPConfiguration must preserve service handles — recreating them on every config tweak is the v2-era waste this test guards against")
}

// TestClientNew_AccessManagerWithBadAddress_DoesNotMisclassifyAsConfiguration
// is the M4 integration check. With Access Manager enabled but receiving a real
// upstream HTTP response, construction must preserve the upstream status instead
// of collapsing the failure into a configuration or synthetic auth error.
func TestClientNew_AccessManagerWithBadAddress_DoesNotMisclassifyAsConfiguration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg, err := config.NewConfig(
		config.WithEnvironment(config.EnvironmentLocal),
		config.WithAccessManager(AccessManager{
			Address:      srv.URL,
			ClientID:     "id",
			ClientSecret: "secret",
		}),
	)
	require.NoError(t, err)

	_, err = New(WithConfig(cfg))
	require.Error(t, err)

	assert.False(t, sdkerrors.IsConfigurationError(err),
		"Access Manager upstream HTTP failures must not be configuration errors, got %v", err)
	assert.False(t, sdkerrors.IsAuthError(err),
		"Access Manager 5xx failures must not be authentication errors, got %v", err)
	actual, ok := sdkerrors.ActualHTTPStatus(err)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, actual)
	assert.Equal(t, http.StatusInternalServerError, sdkerrors.SuggestedHTTPStatus(err))
}

// --- from validation_contract_test.go ---

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
				inner := errors.Unwrap(err)
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

	t.Run("missing ledger URL surfaces as ledger URL error", func(t *testing.T) {
		cfg := createTestConfig(t)
		delete(cfg.ServiceURLs, config.ServiceOnboarding)
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ledger URL is required")
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
	inner := errors.Unwrap(err)
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

	_, err := New(WithConfig(cfg), WithEnvironment(config.EnvironmentLocal), WithLogger(logger))
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
				WithEnvironment(config.EnvironmentLocal),
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

func TestNewClassifiesAccessManagerBootstrapTransportFailureAsNetwork(t *testing.T) {
	transportErr := errors.New("dial tcp: connection refused")
	client := &http.Client{
		Transport: accessManagerBootstrapRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, transportErr
		}),
	}

	_, err := New(
		WithEnvironment(config.EnvironmentLocal),
		WithAccessManager(AccessManager{
			Address:      "https://auth.example.com",
			ClientID:     "network-client-id",
			ClientSecret: "super-secret-value",
		}),
		WithHTTPClient(client),
	)

	require.Error(t, err)
	assert.True(t, sdkerrors.IsNetworkError(err), "pre-response token fetch failures must classify as network errors")
	assert.True(t, sdkerrors.IsBootstrapError(err), "network token fetch failures are still bootstrap failures")
	assert.False(t, sdkerrors.IsAuthenticationError(err), "transport failures must not masquerade as auth failures")
	assert.False(t, sdkerrors.IsConfigurationError(err), "transport failures must not masquerade as local configuration failures")
	assert.True(t, sdkerrors.HTTPRequestSent(err))
	assert.False(t, sdkerrors.HTTPResponseReceived(err))

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, sdkerrors.CategoryNetwork, sdkErr.Category)
	assert.Equal(t, sdkerrors.CodeNetwork, sdkErr.Code)
	assert.Equal(t, http.StatusServiceUnavailable, sdkErr.GetStatusCode())
	assert.Equal(t, sdkerrors.ErrorSourceTransport, sdkErr.GetSource())
	assert.Equal(t, sdkerrors.StatusCodeSourceSynthetic, sdkErr.GetStatusCodeSource())
	assert.NotContains(t, err.Error(), "super-secret-value")
}

type accessManagerBootstrapRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn accessManagerBootstrapRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
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

// TestNewClientWiresTwoPlaneClients is the Task 1.3.2 construction guard:
// midaz.New produces a Client whose embedded Entity carries both generated
// plane clients (Ledger + Tracer), promoted via Planes().
func TestNewClientWiresTwoPlaneClients(t *testing.T) {
	c, err := New(
		WithEnvironment(config.EnvironmentLocal),
		WithAnonymous(),
		WithLedgerURL("http://localhost:3002"),
		WithTracerURL("http://localhost:4020/v1"),
	)
	require.NoError(t, err)

	planes := c.Planes()
	require.NotNil(t, planes, "Planes() must be non-nil after New")
	assert.NotNil(t, planes.Ledger, "ledger plane client must be wired")
	assert.NotNil(t, planes.Tracer, "tracer plane client must be wired")
}
