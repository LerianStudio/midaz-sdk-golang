package midaz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
//	c.Accounts.CreateAccount(ctx, ...)
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
