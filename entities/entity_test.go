package entities

import (
	"context"
	"net/http"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability"
	"github.com/stretchr/testify/require"
)

// TestMainFunction serves as an entry point for all tests in the entities package.
func TestMainFunction(_ *testing.T) {
	// This is an empty test function that ensures the package has at least one test
	// so that the testing package will execute all test files in the package.
}

// newTestEntity is the package-internal test helper for constructing an Entity
// directly from raw inputs. It mirrors the contract of the deleted public
// NewEntity constructor for tests that cannot route through midaz.New() because
// of the import cycle. External callers must always go through midaz.New().
//
// Post-construction tuning is exposed through the same setters production code
// uses: (*HTTPClient).SetDebug, SetUserAgent and (*Entity).SetObservability.
// Tests that previously passed entities.WithDebug/WithUserAgent/WithObservability
// as options must invoke the equivalent setter on the returned *Entity directly.
//
//nolint:unparam // provider's only non-nil callers are the business_observability tests, all t.Skip'd pending Task 5.2.6 (plan:621); unparam treats the calls after t.Skip as unreachable and sees only the nil callers. The param stays so those tests keep compiling and exercise it again when 5.2.6 restores plane-path business events.
func newTestEntity(t *testing.T, client *http.Client, authToken string, baseURLs map[string]string, provider observability.Provider) *Entity {
	t.Helper()

	normalizedBaseURLs, err := normalizeBaseURLs(baseURLs, false)
	require.NoError(t, err)

	tracerURL := normalizedBaseURLs["tracer"]
	if tracerURL == "" {
		tracerURL = normalizedBaseURLs["onboarding"]
	}

	authCfg := authRoundTripperConfig{}
	if authToken != "" {
		authCfg.tokenProvider = func(context.Context) (string, error) { return authToken, nil }
	}

	// Epic 5.3: the ledger accessors are now plane facades, so a test entity
	// needs plane clients pointed at the same server. Retries stay off for
	// deterministic single-attempt behavior.
	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL:    normalizedBaseURLs["onboarding"],
		tracerURL:    tracerURL,
		auth:         authCfg,
		httpClient:   client,
		retryOptions: planeTestRetryOptions(),
	})
	require.NoError(t, err)

	entity := &Entity{
		httpClient:    NewHTTPClient(client, authToken, provider),
		baseURLs:      normalizedBaseURLs,
		planes:        planes,
		observability: provider,
	}

	entity.initServices()

	return entity
}

func TestNormalizeBaseURLs_DefaultsMissingCRMURLToOnboarding(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "")

	normalized, err := normalizeBaseURLs(map[string]string{
		"onboarding":  "https://api.example.com/onboarding",
		"transaction": "https://api.example.com/transaction",
	}, false)

	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/onboarding/v1", normalized["crm"])
}

func TestNormalizeBaseURLs_RequiresOnboardingKey(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "https://api.example.com/crm")

	baseURLs, err := normalizeBaseURLs(map[string]string{
		"transaction": "https://api.example.com/transaction",
	}, false)

	require.Error(t, err)
	require.Nil(t, baseURLs)
	require.Contains(t, err.Error(), "missing onboarding URL")
}
