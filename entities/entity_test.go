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
		planes:        planes,
		observability: provider,
	}

	entity.initServices()

	return entity
}

// TestNormalizeBaseURLs_LedgerVersionSuffix pins the asymmetry between the two
// planes: the Ledger base URL must be bare (its version rides inside every
// operation path), the Tracer base URL must carry "/v1" (its spec declares
// servers:[{url: "/v1"}] with unversioned paths).
//
// The rejection exists because the SDK used to pin "/v1" onto the Ledger base,
// so .env files in the wild still carry MIDAZ_LEDGER_URL=...:3002/v1. Accepting
// it would double the segment and 404 every request with nothing pointing at the
// base URL as the cause.
func TestNormalizeBaseURLs_LedgerVersionSuffix(t *testing.T) {
	tests := []struct {
		name          string
		baseURLs      map[string]string
		wantOnboard   string
		wantTracer    string
		wantErrSubstr string
	}{
		{
			name:        "bare ledger base is accepted unchanged",
			baseURLs:    map[string]string{"onboarding": "https://api.example.com"},
			wantOnboard: "https://api.example.com",
			wantTracer:  "https://api.example.com/v1",
		},
		{
			name:        "ledger subpath base is accepted unchanged",
			baseURLs:    map[string]string{"onboarding": "https://api.example.com/midaz"},
			wantOnboard: "https://api.example.com/midaz",
			wantTracer:  "https://api.example.com/midaz/v1",
		},
		{
			name:          "v1-suffixed ledger base is rejected",
			baseURLs:      map[string]string{"onboarding": "https://api.example.com/v1"},
			wantErrSubstr: `base URL must not end in "/v1"`,
		},
		{
			name:          "v2-suffixed ledger base is rejected",
			baseURLs:      map[string]string{"onboarding": "https://api.example.com/v2"},
			wantErrSubstr: `base URL must not end in "/v2"`,
		},
		{
			name:          "v1-suffixed ledger subpath base is rejected",
			baseURLs:      map[string]string{"onboarding": "https://api.example.com/midaz/v1"},
			wantErrSubstr: `base URL must not end in "/v1"`,
		},
		{
			name: "explicit tracer base keeps its v1 suffix",
			baseURLs: map[string]string{
				"onboarding": "https://api.example.com",
				"tracer":     "https://tracer.example.com/v1",
			},
			wantOnboard: "https://api.example.com",
			wantTracer:  "https://tracer.example.com/v1",
		},
		{
			name: "bare tracer base gets v1 stamped on",
			baseURLs: map[string]string{
				"onboarding": "https://api.example.com",
				"tracer":     "https://tracer.example.com",
			},
			wantOnboard: "https://api.example.com",
			wantTracer:  "https://tracer.example.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := normalizeBaseURLs(tt.baseURLs, false)

			if tt.wantErrSubstr != "" {
				require.Error(t, err)
				require.Nil(t, normalized)
				require.Contains(t, err.Error(), tt.wantErrSubstr)
				require.Contains(t, err.Error(), "MIDAZ_LEDGER_URL",
					"the error must name the setting the operator has to edit")

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantOnboard, normalized["onboarding"])
			require.Equal(t, tt.wantTracer, normalized["tracer"])
		})
	}
}

func TestNormalizeBaseURLs_RequiresOnboardingKey(t *testing.T) {
	baseURLs, err := normalizeBaseURLs(map[string]string{}, false)

	require.Error(t, err)
	require.Nil(t, baseURLs)
	require.Contains(t, err.Error(), "missing onboarding URL")
}
