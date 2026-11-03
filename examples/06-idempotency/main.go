// Package main demonstrates the idempotency surface — the modes
// the SDK supports for the X-Idempotency request header on unsafe (POST,
// PUT, PATCH, DELETE) requests:
//
//  1. No key (default): the SDK sends no random X-Idempotency. A keyless /v1
//     transaction create sends a key derived from the request (SHA-256 of
//     method, path and body), so the same request sent again (even from a new
//     SDK call after a timeout) returns the first outcome, and block and
//     unblock with one body never share the ledger's /v1 slot. Two intentional
//     transactions with identical requests inside the ledger's idempotency
//     window are answered with the first one: pass a distinct key (mode 2).
//     Opt-in: [midaz.WithIdempotency](true) or MIDAZ_IDEMPOTENCY=true makes the
//     SDK generate a random UUID per create, update and delete request instead;
//     commit, cancel and revert carry a key only when you supply one.
//
//  2. Explicit caller-supplied key (recommended for at-least-once
//     producers): pass a stable key via [sdkctx.WithIdempotencyKey]
//     before issuing the request. The SDK emits the caller's key in
//     X-Idempotency. The server uses this to deduplicate retries from
//     YOUR consumer's perspective — re-issuing the same logical
//     transaction does not double-charge.
//
//  3. Precedence: an explicit key set via WithIdempotencyKey wins over
//     [sdkctx.WithoutAutoIdempotency], which only stops the random key of a client
//     that opted in with midaz.WithIdempotency(true), never the /v1 derived key.
//
// Global default: auto-idempotency is off, so only an explicit
// WithIdempotencyKey or a /v1 transaction create's derived key is sent.
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
	demoExplicitWinsOverSuppression(c)
}

// demoDefaultNoKey shows the default behavior: no random X-Idempotency is sent. This
// example has no ledger or accounts, so it creates an organization, which the
// ledger does NOT dedupe by body; it shows only the absent header. Body dedupe
// applies to transaction creates.
func demoDefaultNoKey(c *midaz.Client) {
	fmt.Println("--- No idempotency key (default) ---")

	input := &models.CreateOrganizationInput{
		LegalName:     "Idempotency Demo Keyless",
		LegalDocument: "00000000000191",
	}
	if _, err := c.V2.Organizations.Create(context.Background(), input); err != nil {
		log.Printf("CreateOrganization (keyless): %v", err)
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
