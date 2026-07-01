// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// resolveIdempotency computes the X-Idempotency key and X-TTL value for a
// write on the generated transport.
//
// This is a money-path invariant. The generated ledger client does NOT inherit
// the legacy *HTTPClient's ctx→header injection: the auth round tripper only
// PRESERVES headers across a 401 refresh replay, it never creates them. Without
// this helper a create would leave with no idempotency key, so a network retry
// would post a second balance mutation — a double-entry violation. Every write
// facade routes its key/TTL through here so the guarantee holds in one place.
//
// Key precedence (first non-empty wins), mirroring the legacy path:
//  1. explicitKey — a caller-supplied input-struct field (most explicit).
//  2. sdkctx.WithIdempotencyKey(ctx) — request-scoped key.
//  3. auto-generated UUID — only when autoGen is true AND the caller did not
//     opt out via sdkctx.WithoutAutoIdempotency. Creates are unsafe-but-
//     idempotent, so they pass autoGen=true (parity with the legacy default).
//     Actions like commit/cancel/revert pass autoGen=false — they carry a key
//     only when one is supplied explicitly or via ctx.
//
// ttl comes from sdkctx.WithIdempotencyTTL(ctx); it is "" when unset, in which
// case the caller must omit X-TTL and let the server apply its default (300s).
func resolveIdempotency(ctx context.Context, explicitKey string, autoGen bool) (key, ttl string) {
	ttl, _ = sdkctx.IdempotencyTTLFromContext(ctx)

	switch {
	case explicitKey != "":
		return explicitKey, ttl
	case sdkctx.IdempotencyKeyFromContext(ctx) != "":
		return sdkctx.IdempotencyKeyFromContext(ctx), ttl
	case autoGen && !sdkctx.AutoIdempotencySuppressed(ctx):
		return uuid.NewString(), ttl
	default:
		return "", ttl
	}
}

// setHeader returns a RequestEditorFn that sets one header on the outbound
// request. It is the header-side sibling of setQueryParam (in
// organizations_facade.go) and carries the X-Idempotency key on generated ops
// that take no params struct (commit/cancel/revert), where a request editor is
// the only injection seam.
func setHeader(key, value string) genledger.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set(key, value)

		return nil
	}
}
