// Package main demonstrates the idempotency surface — the three modes
// the SDK supports for the X-Idempotency request header on unsafe (POST,
// PUT, PATCH, DELETE) requests:
//
//  1. No key (default): the SDK sends no X-Idempotency. The ledger dedupes a
//     keyless transaction create by its body, so the same body sent again
//     (even from a new SDK call after a timeout) returns the first outcome.
//     Two intentional transactions with byte-identical bodies inside the
//     ledger's idempotency window are answered with the first one: pass a
//     distinct key (mode 2) for each intentional duplicate.
//     Opt-in: [midaz.WithIdempotency](true) or MIDAZ_IDEMPOTENCY=true makes the
//     SDK generate a random UUID per unsafe request instead.
//
//  2. Explicit caller-supplied key (recommended for at-least-once
//     producers): pass a stable key via [sdkctx.WithIdempotencyKey]
//     before issuing the request. The SDK emits the caller's key in
//     X-Idempotency. The server uses this to deduplicate retries from
//     YOUR consumer's perspective — re-issuing the same logical
//     transaction does not double-charge.
//
//  3. Per-call opt-out: [sdkctx.WithoutAutoIdempotency] suppresses the
//     auto-generated key for the next request. Use for the rare endpoint
//     where idempotency is genuinely undesired (e.g., fire-and-forget
//     diagnostics) AND you have no caller-supplied key. Note: an explicit
//     key set via WithIdempotencyKey ALWAYS wins, even when
//     WithoutAutoIdempotency is also on the same context.
//
// Global default: auto-idempotency is off, so only requests with an explicit
// WithIdempotencyKey emit the X-Idempotency header.
//
// Usage:
//
//	go run ./examples/06-idempotency
//
// Requires a local Midaz stack with auth disabled.
package main

import (
	"context"
	"fmt"
	"log"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v6"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

func main() {
	c, err := midaz.New(
		midaz.WithEnvironment(config.EnvironmentLocal),
		midaz.WithAnonymous(),
	)
	if err != nil {
		log.Fatalf("midaz.New: %v", err)
	}
	defer func() {
		if err := c.Shutdown(context.Background()); err != nil {
			log.Printf("client shutdown: %v", err)
		}
	}()

	demoDefaultNoKey(c)
	demoExplicitKey(c)
	demoSuppressedKey(c)
	demoExplicitWinsOverSuppression(c)
}

// demoDefaultNoKey shows the default behavior: no X-Idempotency is sent. For a
// transaction create the ledger dedupes the request by its body.
func demoDefaultNoKey(c *midaz.Client) {
	fmt.Println("--- No idempotency key (default) ---")

	input := &models.CreateOrganizationInput{
		LegalName:     "Idempotency Demo Auto",
		LegalDocument: "00000000000191",
	}
	if _, err := c.V2.Organizations.Create(context.Background(), input); err != nil {
		log.Printf("CreateOrganization (auto-key): %v", err)
		return
	}
	fmt.Println("created with NO X-Idempotency header on the wire")
}

// demoExplicitKey shows the recommended pattern for at-least-once
// producers: the caller carries a stable idempotency key (e.g., one
// per saga step, per outbox row, per UI submission). The SDK emits
// the caller's key verbatim. Re-running this with the same key against
// the same endpoint MUST produce the same result without creating a
// duplicate resource.
func demoExplicitKey(c *midaz.Client) {
	fmt.Println("--- Caller-supplied idempotency key ---")

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "demo-stable-key-001")

	input := &models.CreateOrganizationInput{
		LegalName:     "Idempotency Demo Explicit",
		LegalDocument: "00000000000191",
	}
	if _, err := c.V2.Organizations.Create(ctx, input); err != nil {
		log.Printf("CreateOrganization (explicit key): %v", err)
		return
	}
	fmt.Println("created with X-Idempotency=demo-stable-key-001 (re-runs are safe)")
}

// demoSuppressedKey shows the per-call opt-out, which only matters on a client
// that opted in with midaz.WithIdempotency(true): the SDK emits NO X-Idempotency
// header for this request.
func demoSuppressedKey(c *midaz.Client) {
	fmt.Println("--- Auto-idempotency suppressed for one call (opt-in clients) ---")

	ctx := sdkctx.WithoutAutoIdempotency(context.Background())

	input := &models.CreateOrganizationInput{
		LegalName:     "Idempotency Demo Suppressed",
		LegalDocument: "00000000000191",
	}
	if _, err := c.V2.Organizations.Create(ctx, input); err != nil {
		log.Printf("CreateOrganization (suppressed): %v", err)
		return
	}
	fmt.Println("created with NO X-Idempotency header on the wire")
}

// demoExplicitWinsOverSuppression illustrates the precedence rule:
// when a context has both WithIdempotencyKey and WithoutAutoIdempotency,
// the explicit key wins. The SDK emits X-Idempotency=<caller-key>. This
// matters when middleware blanket-applies WithoutAutoIdempotency but a
// specific call site wants idempotency back on.
func demoExplicitWinsOverSuppression(c *midaz.Client) {
	fmt.Println("--- Explicit key wins over suppression ---")

	ctx := sdkctx.WithoutAutoIdempotency(context.Background())
	ctx = sdkctx.WithIdempotencyKey(ctx, "demo-overrides-suppression-002")

	input := &models.CreateOrganizationInput{
		LegalName:     "Idempotency Demo Override",
		LegalDocument: "00000000000191",
	}
	if _, err := c.V2.Organizations.Create(ctx, input); err != nil {
		log.Printf("CreateOrganization (override): %v", err)
		return
	}
	fmt.Println("created with X-Idempotency=demo-overrides-suppression-002")
}
