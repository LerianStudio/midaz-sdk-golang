// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// TestTracerTransitionsRideCtxKey proves FW1: lifecycle transitions
// (Activate/Deactivate/Draft) are actions (autoGen=false) — no auto-gen key,
// but a caller's sdkctx.WithIdempotencyKey MUST still ride. Activate is the
// representative; Deactivate/Draft route through the same ruleTransition /
// limitTransition helper.
func TestTracerTransitionsRideCtxKey(t *testing.T) {
	t.Run("rules.Activate rides ctx key", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		ctx := sdkctx.WithIdempotencyKey(context.Background(), "k")
		_, _ = newTestRulesFacade(t, srv).Activate(ctx, stampID)

		if got := key(); got != "k" {
			t.Fatalf("rules.Activate: X-Idempotency=%q, want k (ctx key must ride)", got)
		}
	})

	t.Run("rules.Activate headerless without key", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTestRulesFacade(t, srv).Activate(context.Background(), stampID)

		if got := key(); got != "" {
			t.Fatalf("rules.Activate: X-Idempotency=%q, want none (autoGen=false, no key)", got)
		}
	})

	t.Run("limits.Activate rides ctx key", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		ctx := sdkctx.WithIdempotencyKey(context.Background(), "k")
		_, _ = newTestLimitsFacade(t, srv).Activate(ctx, stampID)

		if got := key(); got != "k" {
			t.Fatalf("limits.Activate: X-Idempotency=%q, want k (ctx key must ride)", got)
		}
	})

	t.Run("limits.Activate headerless without key", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTestLimitsFacade(t, srv).Activate(context.Background(), stampID)

		if got := key(); got != "" {
			t.Fatalf("limits.Activate: X-Idempotency=%q, want none (autoGen=false, no key)", got)
		}
	})
}

// TestInstrumentsDeleteRelatedPartyStamps proves FW1: DeleteRelatedParty is an
// auto-gen unsafe write (gated, like its sibling Delete).
func TestInstrumentsDeleteRelatedPartyStamps(t *testing.T) {
	del := func(gate bool, ctx context.Context) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		f := newInstrumentsFacade(newTestLedgerClient(t, srv), gate)
		_ = f.DeleteRelatedParty(ctx, instrumentsFacadeOrgID, instrumentsFacadeHolderID, stampID, stampID)

		return key()
	}

	if got := del(true, context.Background()); got == "" {
		t.Fatal("gate on: want auto-generated X-Idempotency")
	}

	if got := del(false, context.Background()); got != "" {
		t.Fatalf("gate off: want no auto-gen, got %q", got)
	}

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "explicit-key")
	if got := del(false, ctx); got != "explicit-key" {
		t.Fatalf("gate off + ctx key: got %q, want explicit-key", got)
	}
}
