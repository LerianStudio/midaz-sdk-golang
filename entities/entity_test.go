package entities

import (
	"net/http"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
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
func newTestEntity(t *testing.T, client *http.Client, authToken string, baseURLs map[string]string, provider observability.Provider) *Entity {
	t.Helper()

	normalizedBaseURLs, err := normalizeBaseURLs(baseURLs)
	require.NoError(t, err)

	entity := &Entity{
		httpClient:    NewHTTPClient(client, authToken, provider),
		baseURLs:      normalizedBaseURLs,
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
	})

	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/onboarding/v1", normalized["crm"])
}

func TestNormalizeBaseURLs_RequiresOnboardingURL(t *testing.T) {
	t.Setenv("MIDAZ_CRM_URL", "https://api.example.com/crm")

	baseURLs, err := normalizeBaseURLs(map[string]string{
		"transaction": "https://api.example.com/transaction",
	})

	require.Error(t, err)
	require.Nil(t, baseURLs)
	require.Contains(t, err.Error(), "missing onboarding URL")
}
