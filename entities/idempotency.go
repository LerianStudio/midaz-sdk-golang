// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/gentracer"
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
//
// This resolver assumes idempotency is ENABLED: it does not consult the
// client-level MIDAZ_IDEMPOTENCY flag (the legacy ensureIdempotencyHeader gates
// on it). During coexistence the facade has no access to that flag; the enable
// gate is applied one layer up, in the Phase 5 wiring cutover. Defaulting to
// auto-generate errs toward more deduplication, which is the safe side for an
// unsafe money write.
func resolveIdempotency(ctx context.Context, explicitKey string, autoGen bool) (key, ttl string) {
	ttl, _ = sdkctx.IdempotencyTTLFromContext(ctx)
	ctxKey := sdkctx.IdempotencyKeyFromContext(ctx)

	switch {
	case explicitKey != "":
		return explicitKey, ttl
	case ctxKey != "":
		return ctxKey, ttl
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

// idempotencyEditors builds the request-editor list that stamps X-Idempotency
// on a wired LEDGER write. autoGen gates ONLY the auto-generated key (parity
// with the legacy ensureIdempotencyHeader gate); an explicit or context key
// (sdkctx.WithIdempotencyKey) still resolves and stamps when autoGen is false.
// Returns nil (header-free) when no key resolves. The stamp is applied when the
// generated client builds the request, so it survives transport-level retries
// (the retry round tripper clones the already-stamped request).
func idempotencyEditors(ctx context.Context, autoGen bool) []genledger.RequestEditorFn {
	key, ttl := resolveIdempotency(ctx, "", autoGen)
	if key == "" {
		return nil
	}

	editors := []genledger.RequestEditorFn{setHeader(idempotencyHeader, key)}
	if ttl != "" {
		editors = append(editors, setHeader(ttlHeader, ttl))
	}

	return editors
}

// setHeaderTracer is the gentracer twin of setHeader. The two generated packages
// declare distinct RequestEditorFn named types over the same underlying
// func(context.Context, *http.Request) error, so a tracer op cannot take a
// genledger editor — hence the near-duplicate.
func setHeaderTracer(key, value string) gentracer.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set(key, value)

		return nil
	}
}

// idempotencyEditorsTracer is the gentracer twin of idempotencyEditors, same
// gate semantics.
//
// Nuance (rules/limits): the tracer server does NOT honor idempotency today —
// it neither reads X-Idempotency nor body-dedups these writes. The key is
// stamped anyway for parity with the legacy path and forward-compat with a
// future server-side check. The consequence of the retry gate is that a 503
// arriving AFTER the server committed a rule/limit could latently double-create
// on replay. This is ACCEPTED: rules and limits are CONFIG, not money — no
// balance moves — so a rare duplicate config write is a benign, correctable
// nuisance, unlike a double balance mutation. (validations/reservations are
// deliberately NOT stamped; they dedup on body identifiers.)
func idempotencyEditorsTracer(ctx context.Context, autoGen bool) []gentracer.RequestEditorFn {
	key, ttl := resolveIdempotency(ctx, "", autoGen)
	if key == "" {
		return nil
	}

	editors := []gentracer.RequestEditorFn{setHeaderTracer(idempotencyHeader, key)}
	if ttl != "" {
		editors = append(editors, setHeaderTracer(ttlHeader, ttl))
	}

	return editors
}
