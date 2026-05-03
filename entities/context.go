package entities

import (
	"context"
	"strings"
)

// idempotency context helpers
type contextKeyIdempotency struct{}

// WithIdempotencyKey attaches an idempotency key to the request context.
// The HTTP client will add it as an 'X-Idempotency' header.
func WithIdempotencyKey(ctx context.Context, key string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if key == "" {
		return ctx
	}

	return context.WithValue(ctx, contextKeyIdempotency{}, key)
}

func getIdempotencyKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if v := ctx.Value(contextKeyIdempotency{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// tenant ID context helpers
type contextKeyTenantID struct{}

// WithTenantID attaches a tenant ID to the request context.
// The HTTP client will add it as an 'X-Tenant-ID' header for deployments that
// honor explicit tenant headers. In the reference Midaz path, authenticated claims
// remain the primary tenant source of truth. If tenantID is empty, the context is
// returned unchanged and no header will be set from context.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}

	return context.WithValue(ctx, contextKeyTenantID{}, tenantID)
}

// TenantIDFromContext extracts the tenant ID previously stored via WithTenantID.
// Returns an empty string if no tenant ID is present in the context.
func TenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if v := ctx.Value(contextKeyTenantID{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}
