package entities

import (
	"context"
	"errors"
	"net/http"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/retry"
)

// PlaneClients holds the two generated, typed plane clients that back the
// consolidated Midaz server. Ledger serves onboarding + transaction +
// CRM/fees/billing; Tracer serves rules/limits/reservations/validations/audit.
//
// These are the low-level typed surfaces (oapi-codegen ClientWithResponses).
// The hand-written entity facade layers ergonomics (error normalization,
// pagination trinaldo, builders) on top of them; callers of the SDK never see
// the generated types directly.
type PlaneClients struct {
	Ledger *genledger.ClientWithResponses
	Tracer *gentracer.ClientWithResponses
}

// planeClientsConfig carries everything the two-plane builder needs. It is a
// value struct rather than a widening of the option chain because the builder
// is called once, internally, from entity construction.
type planeClientsConfig struct {
	// ledgerURL / tracerURL are the per-plane base URLs, already carrying the
	// "/v1" prefix (config normalizes them). The generated client appends the
	// operation path relative to the base, so a base ending in "/v1" resolves
	// operations under "/v1/...".
	ledgerURL string
	tracerURL string

	// auth is the shared Bearer wiring. Both planes reuse the same token
	// provider so a refresh triggered by one is visible to the other.
	auth authRoundTripperConfig

	// tracerAPIKey, when non-empty, switches the Tracer plane to X-API-Key auth
	// (the Ledger plane always uses the shared Bearer).
	tracerAPIKey string

	// httpClient is the pooled underlying client whose Transport the auth round
	// tripper wraps. Nil falls back to the SDK default pooled client.
	httpClient *http.Client

	// retryOptions is the effective retry policy for the plane money-path,
	// resolved once by the caller (see buildPlaneClients) so the plane retry
	// round tripper and the legacy *HTTPClient agree on the effective values.
	// A zero/unset value degrades to retry.DefaultOptions() in newPlaneClients.
	retryOptions retry.Options

	// customRetryPolicy, when non-nil, is the caller-supplied retry predicate
	// threaded onto the plane retry round tripper. Nil is safe.
	customRetryPolicy func(*http.Response, error) bool
}

// newPlaneClients builds the two typed plane clients, each wired to an auth
// round tripper over the shared pooled transport. The Ledger plane always
// authenticates with the shared Bearer; the Tracer plane uses X-API-Key when
// tracerAPIKey is set, otherwise the same shared Bearer.
func newPlaneClients(cfg planeClientsConfig) (*PlaneClients, error) {
	if cfg.ledgerURL == "" {
		return nil, errors.New("ledger URL is required")
	}

	if cfg.tracerURL == "" {
		return nil, errors.New("tracer URL is required")
	}

	base := transportOf(cfg.httpClient)

	ledgerRT := newAuthRefreshRoundTripper(base, cfg.auth)

	tracerAuth := cfg.auth
	tracerAuth.apiKey = cfg.tracerAPIKey
	tracerRT := newAuthRefreshRoundTripper(base, tracerAuth)

	// A zero/unset retry policy would make the engine reject EVERY plane
	// request in validateOptions (InitialDelay must be positive), silently
	// breaking all plane traffic. Degrade an unconfigured struct to the SDK's
	// standard policy so callers that leave the field unset still get sane
	// retries rather than a broken client.
	retryOpts := cfg.retryOptions
	if retryOpts.InitialDelay <= 0 || retryOpts.MaxDelay <= 0 {
		retryOpts = *retry.DefaultOptions()
	}

	// Compose the chain OUTSIDE-IN: retry round tripper wraps the auth round
	// tripper wraps the pooled transport. Each retry attempt gets a fresh body
	// and identical headers; the inner auth RT still owns 401 refresh-replay.
	retryLedgerRT := newRetryRoundTripper(ledgerRT, retryOpts, cfg.customRetryPolicy)
	retryTracerRT := newRetryRoundTripper(tracerRT, retryOpts, cfg.customRetryPolicy)

	ledger, err := genledger.NewClientWithResponses(
		cfg.ledgerURL,
		genledger.WithHTTPClient(&http.Client{Transport: retryLedgerRT}),
	)
	if err != nil {
		return nil, err
	}

	tracer, err := gentracer.NewClientWithResponses(
		cfg.tracerURL,
		gentracer.WithHTTPClient(&http.Client{Transport: retryTracerRT}),
	)
	if err != nil {
		return nil, err
	}

	return &PlaneClients{Ledger: ledger, Tracer: tracer}, nil
}

// buildPlaneClients assembles the two-plane clients from the resolved config.
// It bridges the entity-construction inputs (Config, AccessManager, normalized
// base URLs) into the [planeClientsConfig] the low-level builder consumes.
//
// The Bearer token provider is the same shared Access Manager exchange the
// legacy per-service *HTTPClient uses (GetTokenFromAccessManager is internally
// cached + singleflighted), so a refresh triggered on either plane hits the
// auth service once. When auth is disabled (anonymous stack) the provider is
// nil and requests go out unauthenticated.
func buildPlaneClients(config Config, pluginAuth auth.AccessManager, normalizedBaseURLs map[string]string) (*PlaneClients, error) {
	authCfg := authRoundTripperConfig{}
	if pluginAuth.Enabled {
		authCfg.tokenProvider = func(ctx context.Context) (string, error) {
			return auth.GetTokenFromAccessManager(ctx, pluginAuth, config.GetHTTPClient())
		}
		authCfg.tokenInvalidator = func() { auth.InvalidateAccessManagerToken(pluginAuth) }
	}

	//nolint:bodyclose // configPlaneRetry returns a retry-policy func (with *http.Response in its signature), not an HTTP response.
	retryOptions, customRetryPolicy := configPlaneRetry(config)

	return newPlaneClients(planeClientsConfig{
		ledgerURL:         normalizedBaseURLs["onboarding"],
		tracerURL:         normalizedBaseURLs["tracer"],
		auth:              authCfg,
		tracerAPIKey:      configTracerAPIKey(config),
		httpClient:        config.GetHTTPClient(),
		retryOptions:      retryOptions,
		customRetryPolicy: customRetryPolicy,
	})
}

// Planes returns the two generated, typed plane clients (Ledger + Tracer).
// Nil-safe: returns nil for a nil entity.
func (e *Entity) Planes() *PlaneClients {
	if e == nil {
		return nil
	}

	return e.planes
}

// transportOf extracts the pooled Transport from the supplied client, falling
// back to the SDK's shared default when absent. The auth round tripper wraps
// this transport so connection pooling is preserved.
func transportOf(client *http.Client) http.RoundTripper {
	if client != nil && client.Transport != nil {
		return client.Transport
	}

	if def := defaultHTTPClient(); def != nil && def.Transport != nil {
		return def.Transport
	}

	return http.DefaultTransport
}
