package sdkctx_test

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// ExampleWithIdempotencyKey shows the recommended pattern for
// at-least-once producers: the caller carries a stable idempotency key
// (one per saga step, outbox row, or UI submission) and the SDK emits
// it as the X-Idempotency request header. Re-issuing the same logical
// request will not produce a duplicate resource.
func ExampleWithIdempotencyKey() {
	ctx := sdkctx.WithIdempotencyKey(context.Background(), "tx-2026-05-06-001")

	fmt.Println(sdkctx.IdempotencyKeyFromContext(ctx))
	// Output: tx-2026-05-06-001
}

// ExampleWithoutAutoIdempotency demonstrates the per-call opt-out from
// auto-generated idempotency keys. Use sparingly — only for endpoints
// where idempotency is genuinely undesired (e.g., fire-and-forget
// diagnostics that must always re-execute).
func ExampleWithoutAutoIdempotency() {
	ctx := sdkctx.WithoutAutoIdempotency(context.Background())

	fmt.Println(sdkctx.AutoIdempotencySuppressed(ctx))
	// Output: true
}

// ExampleWithIncludeDeleted shows how to opt into seeing soft-deleted
// resources for a single request. Use for admin / audit code paths;
// production traffic should leave this off.
func ExampleWithIncludeDeleted() {
	ctx := sdkctx.WithIncludeDeleted(context.Background(), true)

	fmt.Println(sdkctx.IncludeDeletedFromContext(ctx))
	// Output: true
}

// ExampleWithIdempotencyKey_overridesSuppression demonstrates the
// precedence rule: an explicit caller-supplied key always wins over
// WithoutAutoIdempotency, even when both are applied to the same
// context. Useful when middleware blanket-disables auto-idempotency
// but a specific call site needs idempotency back on.
func ExampleWithIdempotencyKey_overridesSuppression() {
	ctx := sdkctx.WithoutAutoIdempotency(context.Background())
	ctx = sdkctx.WithIdempotencyKey(ctx, "explicit-wins")

	fmt.Println(sdkctx.IdempotencyKeyFromContext(ctx))
	fmt.Println(sdkctx.AutoIdempotencySuppressed(ctx))
	// Output:
	// explicit-wins
	// true
}
