package entities

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/LerianStudio/lib-observability/v3/tracing"
	"github.com/LerianStudio/midaz-sdk-golang/v6/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v6/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/auth"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/retry"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/security"
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
	// ledgerURL / tracerURL are the per-plane base URLs as normalized by
	// [normalizeBaseURLs]. The two planes version themselves differently and the
	// difference is fixed by their OpenAPI contracts, not by preference:
	//
	//   - ledgerURL is BARE (host + optional subpath, no version). The Ledger
	//     spec declares servers:[{url: "/"}] and carries the version inside every
	//     operation path, so the generated client emits "/v1/..." and "/v2/..."
	//     itself. A version on this base would double it ("/v1/v1/...") and is
	//     rejected at normalization time.
	//   - tracerURL CARRIES "/v1". The Tracer spec declares servers:[{url: "/v1"}]
	//     with unversioned paths, so the version has to live on the base.
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

	// actorTokenSource, when non-nil, turns the actor relay on: the Ledger
	// plane (and only it) gets [relayActor] as a request editor. Nil means no
	// editor is installed, so Ledger requests are exactly as without the option.
	actorTokenSource func(ctx context.Context) (string, bool)
}

// propagateTrace injects the caller's trace context through the OTel global
// propagator (a no-op when the host installed none). A host transport such as
// otelhttp runs later in the chain and overwrites the header with its own span.
func propagateTrace(ctx context.Context, req *http.Request) error {
	tracing.InjectHTTPContext(ctx, req.Header)

	return nil
}

// relayActor returns the Ledger request editor that sets X-Lerian-Actor to the
// token source returns for the call's context. No token, or a token that is
// blank after trimming spaces, leaves the request untouched. Set replaces any
// value already on the request, so a caller transport that writes the same
// header still yields a single value. The editor runs once, before the request
// enters the retry and auth round trippers, which clone it per attempt, so the
// header is kept on every retry and on the replay after a 401.
func relayActor(source func(ctx context.Context) (string, bool)) genledger.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		token, ok := source(ctx)
		if !ok {
			return nil
		}

		token = strings.TrimSpace(token)
		if token == "" {
			return nil
		}

		req.Header.Set(actorHeader, token)

		return nil
	}
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
	// request in validateOptions (InitialDelay/MaxDelay must be positive,
	// BackoffFactor >= 1.0), silently breaking all plane traffic. Backfill ONLY
	// the fields that would fail validation from the SDK defaults, PRESERVING
	// the caller's MaxRetries — so an intentional MaxRetries=0 (retries off) is
	// never resurrected to the default. BackoffFactor is backfilled alongside
	// the delays because it is validated too; without it a {MaxRetries:0} struct
	// with a zero BackoffFactor would still be rejected.
	retryOpts := cfg.retryOptions
	defaults := retry.DefaultOptions()

	if retryOpts.InitialDelay <= 0 {
		retryOpts.InitialDelay = defaults.InitialDelay
	}

	if retryOpts.MaxDelay <= 0 {
		retryOpts.MaxDelay = defaults.MaxDelay
	}

	if retryOpts.MaxDelay < retryOpts.InitialDelay {
		retryOpts.MaxDelay = retryOpts.InitialDelay
	}

	if retryOpts.BackoffFactor < 1.0 {
		retryOpts.BackoffFactor = defaults.BackoffFactor
	}

	// Compose the chain OUTSIDE-IN: retry round tripper wraps the auth round
	// tripper wraps the pooled transport. Each retry attempt gets a fresh body
	// and identical headers; the inner auth RT still owns 401 refresh-replay.
	retryLedgerRT := newRetryRoundTripper(ledgerRT, retryOpts, cfg.customRetryPolicy)
	retryTracerRT := newRetryRoundTripper(tracerRT, retryOpts, cfg.customRetryPolicy)

	ledgerOpts := []genledger.ClientOption{
		genledger.WithHTTPClient(newPlaneHTTPClient(retryLedgerRT, cfg.httpClient)),
		genledger.WithRequestEditorFn(propagateTrace),
	}

	// The actor relay is Ledger-only by construction: the Tracer client below
	// never gets this editor, and the Access Manager token exchange builds its
	// own request outside both clients.
	if cfg.actorTokenSource != nil {
		ledgerOpts = append(ledgerOpts, genledger.WithRequestEditorFn(relayActor(cfg.actorTokenSource)))
	}

	ledger, err := genledger.NewClientWithResponses(cfg.ledgerURL, ledgerOpts...)
	if err != nil {
		return nil, err
	}

	tracer, err := gentracer.NewClientWithResponses(
		cfg.tracerURL,
		gentracer.WithHTTPClient(newPlaneHTTPClient(retryTracerRT, cfg.httpClient)),
		gentracer.WithRequestEditorFn(propagateTrace),
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
// The Bearer token provider is the shared Access Manager exchange
// (GetTokenFromAccessManager is internally cached + singleflighted), so a
// refresh triggered on either plane hits the auth service once. When auth is disabled (anonymous stack) the provider is
// nil and requests go out unauthenticated.
func buildPlaneClients(config Config, pluginAuth auth.AccessManager, normalizedBaseURLs map[string]string) (*PlaneClients, error) {
	authCfg := accessManagerAuthConfig(config, pluginAuth)

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
		actorTokenSource:  configActorTokenSource(config),
	})
}

// accessManagerAuthConfig wires the Access Manager token exchange into the
// plane round tripper. Anonymous stacks get an empty config (no provider).
func accessManagerAuthConfig(config Config, pluginAuth auth.AccessManager) authRoundTripperConfig {
	if !pluginAuth.Enabled {
		return authRoundTripperConfig{}
	}

	return authRoundTripperConfig{
		tokenProvider: func(ctx context.Context) (string, error) {
			return auth.GetTokenFromAccessManager(ctx, pluginAuth, config.GetHTTPClient())
		},
		refreshKey:       pluginAuth.TokenCacheKey,
		tokenInvalidator: func(ctx context.Context) { auth.InvalidateAccessManagerTokenContext(ctx, pluginAuth) },
	}
}

// Planes returns the two generated, typed plane clients (Ledger + Tracer).
// Nil-safe: returns nil for a nil entity.
func (e *Entity) Planes() *PlaneClients {
	if e == nil {
		return nil
	}

	return e.planes
}

// newPlaneHTTPClient wraps the composed plane RoundTripper (retry → auth →
// pooled transport) in an *http.Client that restores the two client-level
// policies the bare &http.Client{Transport: rt} form silently dropped:
//
//   - Timeout: copied from the configured client (WithTimeout / MIDAZ_TIMEOUT),
//     falling back to the SDK default when src is nil or unset. Because retries
//     live INSIDE rt, this Timeout bounds the TOTAL wall-clock across every
//     retry attempt + backoff — one deadline for the whole call, NOT a
//     per-attempt budget. Dropping it made WithTimeout a no-op on all plane
//     traffic (incl. the money path): a hung server would block forever.
//
//   - CheckRedirect: FIXED to [security.ValidatePlaneRedirect] regardless of
//     src — the planes talk to a single fixed, configured Midaz host, so a
//     cross-origin 302 is never legitimate and is refused outright. This is
//     stricter than the legacy path's validateSDKRedirect (which refuses only
//     credential-bearing cross-origin hops): the plane auth round tripper
//     injects the Bearer / X-API-Key BELOW net/http's redirect-header-stripping
//     layer, so a followed cross-host redirect would re-stamp the token onto
//     the foreign host — a leak that safe-method (GET) reads would otherwise
//     hit. Same-host http→https upgrades are still allowed.
//
// The base transport is NOT cloned here (rt already wraps the shared pooled
// transport), so both planes keep sharing one connection pool.
func newPlaneHTTPClient(rt http.RoundTripper, src *http.Client) *http.Client {
	var timeout time.Duration
	if src != nil {
		timeout = src.Timeout
	}

	if timeout == 0 {
		if def := defaultHTTPClient(); def != nil {
			timeout = def.Timeout
		}
	}

	return &http.Client{
		Transport:     rt,
		Timeout:       timeout,
		CheckRedirect: security.ValidatePlaneRedirect,
	}
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
