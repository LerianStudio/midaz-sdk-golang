package sdkctx_test

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
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
	//nolint:staticcheck // intentional nil-context for nil-safety verification
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
	//nolint:staticcheck // intentional nil-context for nil-safety verification
	if !sdkctx.AutoIdempotencySuppressed(sdkctx.WithoutAutoIdempotency(nil)) {
		t.Error("nil ctx should be promoted to background and accept the flag")
	}
}

func TestWithIdempotencyTTL(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context //nolint:containedctx // table-driven test
		seconds int
		want    string
		wantOK  bool
	}{
		{name: "positive TTL stored as string", ctx: context.Background(), seconds: 600, want: "600", wantOK: true},
		{name: "zero TTL is a no-op (server default applies)", ctx: context.Background(), seconds: 0, want: "", wantOK: false},
		{name: "negative TTL is a no-op", ctx: context.Background(), seconds: -5, want: "", wantOK: false},
		{name: "nil ctx becomes background", ctx: nil, seconds: 300, want: "300", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl, ok := sdkctx.IdempotencyTTLFromContext(sdkctx.WithIdempotencyTTL(tt.ctx, tt.seconds))
			if ok != tt.wantOK {
				t.Errorf("IdempotencyTTLFromContext ok = %v, want %v", ok, tt.wantOK)
			}
			if ttl != tt.want {
				t.Errorf("IdempotencyTTLFromContext = %q, want %q", ttl, tt.want)
			}
		})
	}
}

func TestIdempotencyTTLFromContext_NilCtx(t *testing.T) {
	//nolint:staticcheck // intentional nil-context for nil-safety verification
	if ttl, ok := sdkctx.IdempotencyTTLFromContext(nil); ok || ttl != "" {
		t.Errorf("expected (\"\", false) from nil ctx, got (%q, %v)", ttl, ok)
	}
}

func TestHTTPRetrySuppressionContextHelpers(t *testing.T) {
	//nolint:staticcheck // intentional nil-context for nil-safety verification
	if sdkctx.HTTPRetriesSuppressed(nil) {
		t.Fatal("nil context must not suppress HTTP retries")
	}

	ctx := context.Background()
	if sdkctx.HTTPRetriesSuppressed(ctx) {
		t.Fatal("default context must not suppress HTTP retries")
	}

	//nolint:staticcheck // intentional nil-context for nil-safety verification
	ctx = sdkctx.WithoutHTTPRetries(nil)
	if ctx == nil {
		t.Fatal("WithoutHTTPRetries(nil) must return a usable context")
	}
	if !sdkctx.HTTPRetriesSuppressed(ctx) {
		t.Fatal("WithoutHTTPRetries must tag the context")
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
	//nolint:staticcheck // intentional nil-context for nil-safety verification
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
	//nolint:staticcheck // intentional nil-context for nil-safety verification
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
	// Verify all context channels coexist without interference.
	ctx := context.Background()
	ctx = sdkctx.WithIdempotencyKey(ctx, "k1")
	ctx = sdkctx.WithoutAutoIdempotency(ctx)
	ctx = sdkctx.WithIncludeDeleted(ctx, true)
	ctx = sdkctx.WithHardDelete(ctx, true)

	if got := sdkctx.IdempotencyKeyFromContext(ctx); got != "k1" {
		t.Errorf("idempotency key: got %q, want %q", got, "k1")
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
