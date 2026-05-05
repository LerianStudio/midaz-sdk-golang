// Package entities ... (see entity.go for the package doc).
//
// This file contains DEPRECATED context helper shims. v3 moved the canonical
// context helpers to pkg/sdkctx. These wrappers delegate so that v2 code
// keeps compiling during the transition window. They will be removed in v3.0.
package entities

import (
	"context"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

// WithIdempotencyKey attaches an idempotency key to the request context.
//
// Deprecated: use [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithIdempotencyKey]
// instead. This shim will be removed in v3.0.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	return sdkctx.WithIdempotencyKey(ctx, key)
}

// WithTenantID attaches a tenant ID to the request context.
//
// Deprecated: use [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithRequestTenantID]
// instead. The v3 name disambiguates the per-request context helper from the
// client-level [midaz.WithTenantID] option. This shim will be removed in v3.0.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return sdkctx.WithRequestTenantID(ctx, tenantID)
}

// TenantIDFromContext extracts the tenant ID previously stored via
// [WithTenantID] or [sdkctx.WithRequestTenantID].
//
// Deprecated: use [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.TenantIDFromContext]
// instead. This shim will be removed in v3.0.
func TenantIDFromContext(ctx context.Context) string {
	return sdkctx.TenantIDFromContext(ctx)
}

// WithoutAutoIdempotency tags the context so that the HTTP client will NOT
// generate an automatic idempotency key for the next request.
//
// Deprecated: use [github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx.WithoutAutoIdempotency]
// instead. This shim will be removed in v3.0.
func WithoutAutoIdempotency(ctx context.Context) context.Context {
	return sdkctx.WithoutAutoIdempotency(ctx)
}

// Internal helpers used by HTTPClient. These delegate to sdkctx so that
// the canonical context keys live in one place.
func getIdempotencyKeyFromContext(ctx context.Context) string {
	return sdkctx.IdempotencyKeyFromContext(ctx)
}

func autoIdempotencySuppressed(ctx context.Context) bool {
	return sdkctx.AutoIdempotencySuppressed(ctx)
}
