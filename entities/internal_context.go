package entities

// Internal context bridges.
//
// This file contains UNEXPORTED helpers that delegate to pkg/sdkctx for
// reading values that the canonical sdkctx package writes via its public
// helpers. They exist because entities/http.go is the consumer of these
// values during request preparation, and we want a single in-package
// indirection point so that swapping the underlying context-key
// implementation is a one-file change.
//
// External callers should always use pkg/sdkctx directly:
//
//	ctx = sdkctx.WithIdempotencyKey(ctx, key)
//	ctx = sdkctx.WithoutAutoIdempotency(ctx)
//
// There is intentionally no public re-export of these readers — entities
// has no reason to surface context values to user code; sdkctx is that
// surface.

import (
	"context"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// getIdempotencyKeyFromContext returns the idempotency key previously
// attached via sdkctx.WithIdempotencyKey, or the empty string.
func getIdempotencyKeyFromContext(ctx context.Context) string {
	return sdkctx.IdempotencyKeyFromContext(ctx)
}

// autoIdempotencySuppressed returns true when the context was tagged via
// sdkctx.WithoutAutoIdempotency, signalling that the HTTPClient must not
// generate an automatic idempotency key for the next request.
func autoIdempotencySuppressed(ctx context.Context) bool {
	return sdkctx.AutoIdempotencySuppressed(ctx)
}

// httpRetriesSuppressed returns true when the context was tagged via
// sdkctx.WithoutHTTPRetries, signalling that the HTTPClient must perform a
// single attempt even if client-level retries are enabled.
func httpRetriesSuppressed(ctx context.Context) bool {
	return sdkctx.HTTPRetriesSuppressed(ctx)
}
