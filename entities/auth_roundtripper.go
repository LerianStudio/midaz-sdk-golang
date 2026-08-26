package entities

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/sync/singleflight"
)

// errUnrewindableBody marks a request whose body cannot be replayed (GetBody
// is nil). The round tripper falls back to the original 401 response rather
// than risk re-sending a request whose body would be empty on replay.
var errUnrewindableBody = errors.New("request body is not rewindable for auth replay")

// authRoundTripperConfig carries the per-plane auth wiring for
// [newAuthRefreshRoundTripper].
type authRoundTripperConfig struct {
	// tokenProvider fetches (or returns a cached) Access Manager Bearer token.
	// Shared across both planes so a refresh triggered by one plane is visible
	// to the other. Nil means "no auth" (anonymous stack).
	tokenProvider func(context.Context) (string, error)

	// tokenInvalidator drops the cached token so the next tokenProvider call
	// performs a real exchange. Invoked before the post-401 refresh. Nil is a
	// no-op.
	tokenInvalidator func()

	// apiKey, when non-empty, switches the plane to X-API-Key auth: the round
	// tripper injects "X-API-Key: <apiKey>" and does NOT attach a Bearer token.
	// Empty (the default) means the plane shares the Access Manager Bearer.
	apiKey string
}

// authRefreshRoundTripper is the transport-level auth layer for the two
// generated plane clients. It concentrates three concerns that the generated
// oapi-codegen client cannot express (it only accepts a single HttpRequestDoer):
//
//   - Auth injection: Bearer token from the shared provider, OR X-API-Key when
//     the plane was configured with one.
//   - 401 → refresh → replay-once: on an Unauthorized response it invalidates
//     the cached token, fetches a fresh one, and replays the SAME request
//     exactly once with the fresh credential. The per-roundtripper singleflight
//     (refreshGroup) collapses a concurrent 401 burst on THIS plane's requests
//     onto one refresh call; the cross-plane collapse (ledger + tracer sharing
//     one exchange) happens a layer below in GetTokenFromAccessManager, which is
//     itself cached + singleflighted.
//   - Money-path invariant: the replay reuses the identical *http.Request (body
//     re-read via GetBody), so caller-supplied X-Idempotency / X-TTL headers
//     survive the replay byte-for-byte. Reauthenticating never mutates the
//     idempotency key or the body — that would be a second balance mutation and
//     a double-entry violation.
//
// The refresh-once behavior mirrors the legacy entities/http_retry_response.go
// wrapper (singleflight tokenRefreshGroup + refreshedAuth latch); it is
// re-homed here because the generated client routes every request through a
// single RoundTripper rather than the legacy per-service *HTTPClient.
type authRefreshRoundTripper struct {
	base             http.RoundTripper
	tokenProvider    func(context.Context) (string, error)
	tokenInvalidator func()
	apiKey           string
	refreshGroup     singleflight.Group
}

// newAuthRefreshRoundTripper wraps base with the auth + 401-refresh layer.
// base is the pooled *http.Transport; nil falls back to http.DefaultTransport.
func newAuthRefreshRoundTripper(base http.RoundTripper, cfg authRoundTripperConfig) *authRefreshRoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	return &authRefreshRoundTripper{
		base:             base,
		tokenProvider:    cfg.tokenProvider,
		tokenInvalidator: cfg.tokenInvalidator,
		apiKey:           cfg.apiKey,
	}
}

// RoundTrip injects auth, performs the request, and on a 401 refreshes the
// token once and replays the identical request a single time.
func (rt *authRefreshRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := rt.injectAuth(req.Context(), req); err != nil {
		return nil, err
	}

	resp, err := rt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Refresh-and-replay is only meaningful for Bearer auth backed by a
	// provider. The X-API-Key plane and anonymous stacks have nothing to
	// refresh, so their 401 surfaces verbatim.
	if resp.StatusCode != http.StatusUnauthorized || rt.apiKey != "" || rt.tokenProvider == nil {
		return resp, nil
	}

	token, refreshed := rt.refreshToken(req.Context())
	if !refreshed {
		return resp, nil
	}

	replay, err := cloneRequestForReplay(req)
	if err != nil {
		// Can't safely replay (unbufferable body); surface the original 401.
		return resp, nil //nolint:nilerr // deliberate: fall back to the original response
	}

	// Drain and close the 401 response so its connection returns to the pool
	// before we issue the replay.
	drainAndCloseResponseBody(nil, resp)

	replay.Header.Set("Authorization", formatAuthorizationHeader(token))

	return rt.base.RoundTrip(replay)
}

// injectAuth stamps the outbound auth header. X-API-Key wins when configured;
// otherwise the shared Bearer token is attached (skipped when the provider is
// absent or returns empty — the request goes out unauthenticated and the
// server decides).
func (rt *authRefreshRoundTripper) injectAuth(ctx context.Context, req *http.Request) error {
	if rt.apiKey != "" {
		req.Header.Set("X-API-Key", rt.apiKey)

		return nil
	}

	if rt.tokenProvider == nil {
		return nil
	}

	token, err := rt.tokenProvider(ctx)
	if err != nil {
		return err
	}

	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", formatAuthorizationHeader(token))
	}

	return nil
}

// refreshToken invalidates the cached token and fetches a fresh one, funneling
// concurrent post-401 refreshers through a singleflight so the underlying
// exchange runs once. Returns the fresh token and true on success.
func (rt *authRefreshRoundTripper) refreshToken(ctx context.Context) (string, bool) {
	result, err, _ := rt.refreshGroup.Do("tokenrefresh", func() (any, error) {
		if rt.tokenInvalidator != nil {
			rt.tokenInvalidator()
		}

		token, tokenErr := rt.tokenProvider(ctx)
		if tokenErr != nil {
			return "", tokenErr
		}

		return strings.TrimSpace(token), nil
	})
	if err != nil {
		return "", false
	}

	token, ok := result.(string)
	if !ok || token == "" {
		return "", false
	}

	return token, true
}

// cloneRequestForReplay builds a fresh *http.Request that re-issues the same
// method, URL, headers, and body. The body is re-read through GetBody so the
// replay carries an identical payload; a request whose body cannot be rewound
// (GetBody == nil while Body != nil) is not replayable and returns an error so
// the caller falls back to the original response.
func cloneRequestForReplay(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())

	if req.Body == nil {
		return clone, nil
	}

	if req.GetBody == nil {
		return nil, errUnrewindableBody
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}

	clone.Body = body

	return clone, nil
}
