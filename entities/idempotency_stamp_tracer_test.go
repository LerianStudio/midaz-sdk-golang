// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
)

// TestTracerWritesStampIdempotency proves 5.2.5: every wired tracer write op
// (rules/limits Create/Update/Delete) stamps an auto-generated X-Idempotency
// header (gate on, the default), via setHeaderTracer.
func TestTracerWritesStampIdempotency(t *testing.T) {
	cases := []struct {
		name string
		fire func(t *testing.T, srv *httptest.Server)
	}{
		{"rules.Create", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestRulesFacade(t, srv).Create(context.Background(),
				models.NewCreateRuleInput("block-high-value", "transaction.amount > 1000", models.RuleActionDeny))
		}},
		{"rules.Update", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestRulesFacade(t, srv).Update(context.Background(), stampID,
				models.NewUpdateRuleInput().WithExpression("transaction.amount > 2000"))
		}},
		{"rules.Delete", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_ = newTestRulesFacade(t, srv).Delete(context.Background(), stampID)
		}},
		{"limits.Create", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestLimitsFacade(t, srv).Create(context.Background(), newCreateLimit())
		}},
		{"limits.Update", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestLimitsFacade(t, srv).Update(context.Background(), stampID,
				models.NewUpdateLimitInput().WithMaxAmount(decimal.RequireFromString("500")))
		}},
		{"limits.Delete", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_ = newTestLimitsFacade(t, srv).Delete(context.Background(), stampID)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, key := idempotencyCaptureServer()
			defer srv.Close()

			tc.fire(t, srv)

			if key() == "" {
				t.Fatalf("%s: no X-Idempotency stamped (gate on, auto-gen expected)", tc.name)
			}
		})
	}
}

// TestTracerWriteIdempotencyGate proves 5.2.5 gate semantics on rules.Create:
// gate on → auto-gen; gate off → suppressed; gate off + explicit ctx key → the
// explicit key still stamps.
func TestTracerWriteIdempotencyGate(t *testing.T) {
	input := models.NewCreateRuleInput("block-high-value", "transaction.amount > 1000", models.RuleActionDeny)

	create := func(gate bool, ctx context.Context) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		f := newRulesFacade(newTestTracerClient(t, srv), gate)
		_, _ = f.Create(ctx, input)

		return key()
	}

	if got := create(true, context.Background()); got == "" {
		t.Fatal("gate on: want auto-generated X-Idempotency")
	}

	if got := create(false, context.Background()); got != "" {
		t.Fatalf("gate off: want no auto-gen, got %q", got)
	}

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "explicit-key")
	if got := create(false, ctx); got != "explicit-key" {
		t.Fatalf("gate off + ctx key: got %q, want explicit-key", got)
	}
}

// TestTracerExcludedWritesHeaderless confirms the deliberate exclusions:
// validations.Evaluate and reservations.Reserve dedup on body identifiers
// (requestId/transactionId), NOT on X-Idempotency, so they must remain
// header-free even with the gate on.
func TestTracerExcludedWritesHeaderless(t *testing.T) {
	t.Run("validations.Evaluate", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTestValidationsFacade(t, srv).Evaluate(context.Background(), validInput())

		if got := key(); got != "" {
			t.Fatalf("validations.Evaluate stamped X-Idempotency=%q, want none (body-dedup)", got)
		}
	})

	t.Run("reservations.Reserve", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTestReservationsFacade(t, srv).Reserve(context.Background(), validReserveInput())

		if got := key(); got != "" {
			t.Fatalf("reservations.Reserve stamped X-Idempotency=%q, want none (body-dedup)", got)
		}
	})
}
