// Package sdkctx provides per-request context helpers for the Midaz Go SDK.
//
// These helpers attach metadata to a context.Context that the SDK reads at
// request-construction time. They are the per-call counterparts to client-level
// configuration set via midaz.With*.
//
// # Available helpers
//
//   - [WithIdempotencyKey] / [IdempotencyKeyFromContext] — explicit idempotency
//     key for unsafe HTTP requests (POST/PUT/PATCH/DELETE). Takes precedence
//     over auto-generated keys.
//   - [WithoutAutoIdempotency] / [AutoIdempotencySuppressed] — suppress
//     auto-idempotency for a single call when client-level idempotency is on.
//   - [WithRequestTenantID] / [TenantIDFromContext] — override the client-level
//     default tenant ID for a single call.
//   - [WithIncludeDeleted] / [IncludeDeletedFromContext] — include soft-deleted
//     resources in Get/List operations.
//   - [WithHardDelete] / [HardDeleteFromContext] — perform a hard delete instead
//     of the default soft delete.
//
// # Design rationale
//
// Per-request context helpers complement (but do not replace) client-level
// configuration. The precedence order for each concern is documented per
// helper. As a general rule: per-request context > client option > env var.
//
// In v2, these helpers lived in the entities package. v3 moved them here as
// part of the convergence pass; entities.With* shims remain for one minor
// release window before v3.0 hard-removes them.
package sdkctx

import (
	"context"
	"strings"
)

// ----- Idempotency key -----

type idempotencyKeyType struct{}

// WithIdempotencyKey attaches an idempotency key to the request context.
// The HTTP client emits it as the X-Idempotency header on the next unsafe
// request. An empty key is a no-op.
//
// An explicit key set via this helper always takes precedence over an
// auto-generated key (see [WithoutAutoIdempotency] for the per-call
// opt-out from auto-generation).
//
// See also [IdempotencyKeyFromContext], [WithoutAutoIdempotency].
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if key == "" {
		return ctx
	}

	return context.WithValue(ctx, idempotencyKeyType{}, key)
}

// IdempotencyKeyFromContext returns the idempotency key previously stored
// via [WithIdempotencyKey], or empty string if none was set.
func IdempotencyKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if v := ctx.Value(idempotencyKeyType{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// ----- Auto-idempotency suppression -----

type suppressAutoIdempotencyType struct{}

// WithoutAutoIdempotency tags the context so the HTTP client will NOT
// generate an automatic idempotency key for the next unsafe request,
// even when client-level auto-idempotency is enabled.
//
// Use this for the rare call where idempotency is genuinely undesired
// (e.g., fire-and-forget administrative endpoints).
//
// An explicit key set via [WithIdempotencyKey] takes precedence — it is
// honored even when WithoutAutoIdempotency is set, since an explicit
// caller-supplied key always wins.
//
// See also [AutoIdempotencySuppressed], [WithIdempotencyKey].
func WithoutAutoIdempotency(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, suppressAutoIdempotencyType{}, true)
}

// AutoIdempotencySuppressed reports whether [WithoutAutoIdempotency] was
// applied to the context.
func AutoIdempotencySuppressed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	v, ok := ctx.Value(suppressAutoIdempotencyType{}).(bool)

	return ok && v
}

// ----- Request tenant ID -----

type tenantIDType struct{}

// WithRequestTenantID attaches a tenant ID to the request context, overriding
// the client-level default for this single call. The HTTP client emits it
// as the X-Tenant-ID header for deployments that honor explicit tenant
// headers. In the reference Midaz path, authenticated claims remain the
// primary tenant source of truth.
//
// An empty (or whitespace-only) tenantID is a no-op. The context is
// returned unchanged and no header will be set from context.
//
// Precedence: per-request context > client-level WithTenantID > env var.
//
// See also [TenantIDFromContext], midaz.WithTenantID for the client-level default.
func WithRequestTenantID(ctx context.Context, tenantID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}

	return context.WithValue(ctx, tenantIDType{}, tenantID)
}

// TenantIDFromContext returns the tenant ID previously stored via
// [WithRequestTenantID], or empty string if none was set.
func TenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if v := ctx.Value(tenantIDType{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// ----- Include deleted -----

type includeDeletedType struct{}

// WithIncludeDeleted tags the context so that Get/List operations include
// soft-deleted resources in the response. The default behavior is to exclude
// them.
//
// This replaces the v2 boolean parameter on Holders.GetHolder and
// Aliases.GetAlias, harmonizing soft-delete handling across all entities.
//
// See also [IncludeDeletedFromContext], [WithHardDelete].
func WithIncludeDeleted(ctx context.Context, include bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, includeDeletedType{}, include)
}

// IncludeDeletedFromContext reports whether [WithIncludeDeleted] was applied
// to the context with a true value.
func IncludeDeletedFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	v, ok := ctx.Value(includeDeletedType{}).(bool)

	return ok && v
}

// ----- Hard delete -----

type hardDeleteType struct{}

// WithHardDelete tags the context so that Delete operations perform a hard
// delete (permanent removal) instead of the default soft delete (marking as
// deleted but preserving the record).
//
// This replaces the v2 boolean parameter on Holders.DeleteHolder and
// Aliases.DeleteAlias, harmonizing delete handling across all entities.
//
// Use with care — hard deletes are non-recoverable.
//
// See also [HardDeleteFromContext], [WithIncludeDeleted].
func WithHardDelete(ctx context.Context, hard bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, hardDeleteType{}, hard)
}

// HardDeleteFromContext reports whether [WithHardDelete] was applied to the
// context with a true value.
func HardDeleteFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	v, ok := ctx.Value(hardDeleteType{}).(bool)

	return ok && v
}
