package sdkctx_test

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
)

func TestWithIdempotencyKey(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context //nolint:containedctx // table-driven test
		key  string
		want string
	}{
		{name: "non-empty key set", ctx: context.Background(), key: "key-123", want: "key-123"},
		{name: "empty key is no-op", ctx: context.Background(), key: "", want: ""},
		{name: "nil ctx becomes background", ctx: nil, key: "k", want: "k"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sdkctx.IdempotencyKeyFromContext(sdkctx.WithIdempotencyKey(tt.ctx, tt.key))
			if got != tt.want {
				t.Errorf("IdempotencyKeyFromContext = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIdempotencyKeyFromContext_NilCtx(t *testing.T) {
	if got := sdkctx.IdempotencyKeyFromContext(nil); got != "" {
		t.Errorf("expected empty string from nil ctx, got %q", got)
	}
}

func TestWithoutAutoIdempotency(t *testing.T) {
	ctx := context.Background()
	if sdkctx.AutoIdempotencySuppressed(ctx) {
		t.Error("default ctx should not suppress auto-idempotency")
	}
	if !sdkctx.AutoIdempotencySuppressed(sdkctx.WithoutAutoIdempotency(ctx)) {
		t.Error("WithoutAutoIdempotency should set suppression flag")
	}
	if !sdkctx.AutoIdempotencySuppressed(sdkctx.WithoutAutoIdempotency(nil)) {
		t.Error("nil ctx should be promoted to background and accept the flag")
	}
}

func TestWithRequestTenantID(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context //nolint:containedctx // table-driven test
		tenantID string
		want     string
	}{
		{name: "non-empty tenant", ctx: context.Background(), tenantID: "acme", want: "acme"},
		{name: "trims whitespace", ctx: context.Background(), tenantID: "  acme  ", want: "acme"},
		{name: "empty tenant no-op", ctx: context.Background(), tenantID: "", want: ""},
		{name: "whitespace-only no-op", ctx: context.Background(), tenantID: "   ", want: ""},
		{name: "nil ctx promoted", ctx: nil, tenantID: "acme", want: "acme"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sdkctx.TenantIDFromContext(sdkctx.WithRequestTenantID(tt.ctx, tt.tenantID))
			if got != tt.want {
				t.Errorf("TenantIDFromContext = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTenantIDFromContext_NilCtx(t *testing.T) {
	if got := sdkctx.TenantIDFromContext(nil); got != "" {
		t.Errorf("expected empty string from nil ctx, got %q", got)
	}
}

func TestWithIncludeDeleted(t *testing.T) {
	ctx := context.Background()
	if sdkctx.IncludeDeletedFromContext(ctx) {
		t.Error("default ctx should not include deleted")
	}
	if !sdkctx.IncludeDeletedFromContext(sdkctx.WithIncludeDeleted(ctx, true)) {
		t.Error("WithIncludeDeleted(true) should set the flag")
	}
	if sdkctx.IncludeDeletedFromContext(sdkctx.WithIncludeDeleted(ctx, false)) {
		t.Error("WithIncludeDeleted(false) should NOT set the flag")
	}
	if !sdkctx.IncludeDeletedFromContext(sdkctx.WithIncludeDeleted(nil, true)) {
		t.Error("nil ctx should be promoted to background")
	}
}

func TestWithHardDelete(t *testing.T) {
	ctx := context.Background()
	if sdkctx.HardDeleteFromContext(ctx) {
		t.Error("default ctx should not request hard delete")
	}
	if !sdkctx.HardDeleteFromContext(sdkctx.WithHardDelete(ctx, true)) {
		t.Error("WithHardDelete(true) should set the flag")
	}
	if sdkctx.HardDeleteFromContext(sdkctx.WithHardDelete(ctx, false)) {
		t.Error("WithHardDelete(false) should NOT set the flag")
	}
	if !sdkctx.HardDeleteFromContext(sdkctx.WithHardDelete(nil, true)) {
		t.Error("nil ctx should be promoted to background")
	}
}

func TestExplicitKeyTakesPrecedenceOverSuppression(t *testing.T) {
	// An explicit idempotency key set via WithIdempotencyKey is honored even
	// when WithoutAutoIdempotency is also set on the same context.
	ctx := sdkctx.WithoutAutoIdempotency(context.Background())
	ctx = sdkctx.WithIdempotencyKey(ctx, "explicit-key-456")

	if got := sdkctx.IdempotencyKeyFromContext(ctx); got != "explicit-key-456" {
		t.Errorf("explicit key should survive, got %q", got)
	}
	if !sdkctx.AutoIdempotencySuppressed(ctx) {
		t.Error("suppression flag should still be set")
	}
}

func TestContextHelpersIndependent(t *testing.T) {
	// Verify all five context channels coexist without interference.
	ctx := context.Background()
	ctx = sdkctx.WithIdempotencyKey(ctx, "k1")
	ctx = sdkctx.WithRequestTenantID(ctx, "t1")
	ctx = sdkctx.WithoutAutoIdempotency(ctx)
	ctx = sdkctx.WithIncludeDeleted(ctx, true)
	ctx = sdkctx.WithHardDelete(ctx, true)

	if got := sdkctx.IdempotencyKeyFromContext(ctx); got != "k1" {
		t.Errorf("idempotency key: got %q, want %q", got, "k1")
	}
	if got := sdkctx.TenantIDFromContext(ctx); got != "t1" {
		t.Errorf("tenant ID: got %q, want %q", got, "t1")
	}
	if !sdkctx.AutoIdempotencySuppressed(ctx) {
		t.Error("auto-idempotency suppression not propagated")
	}
	if !sdkctx.IncludeDeletedFromContext(ctx) {
		t.Error("include-deleted not propagated")
	}
	if !sdkctx.HardDeleteFromContext(ctx) {
		t.Error("hard-delete not propagated")
	}
}
