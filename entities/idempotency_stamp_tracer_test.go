// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// TestTracerWritesStampIdempotency proves 5.2.5 + FW2: every wired tracer write
// op (rules/limits Create/Update/Delete) stamps an auto-generated X-Idempotency
// header with the gate ON, and emits nothing (no ctx key) with the gate OFF.
func TestTracerWritesStampIdempotency(t *testing.T) {
	cases := []struct {
		name string
		fire func(t *testing.T, srv *httptest.Server, gate bool)
	}{
		{"rules.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newRulesFacade(newTestTracerClient(t, srv), gate).Create(context.Background(),
				models.NewCreateRuleInput("block-high-value", "transaction.amount > 1000", models.RuleActionDeny))
		}},
		{"rules.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newRulesFacade(newTestTracerClient(t, srv), gate).Update(context.Background(), stampID,
				models.NewUpdateRuleInput().WithExpression("transaction.amount > 2000"))
		}},
		{"rules.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newRulesFacade(newTestTracerClient(t, srv), gate).Delete(context.Background(), stampID)
		}},
		{"limits.Create", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newLimitsFacade(newTestTracerClient(t, srv), gate).Create(context.Background(), newCreateLimit())
		}},
		{"limits.Update", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_, _ = newLimitsFacade(newTestTracerClient(t, srv), gate).Update(context.Background(), stampID,
				models.NewUpdateLimitInput().WithMaxAmount(decimal.RequireFromString("500")))
		}},
		{"limits.Delete", func(t *testing.T, srv *httptest.Server, gate bool) {
			t.Helper()
			_ = newLimitsFacade(newTestTracerClient(t, srv), gate).Delete(context.Background(), stampID)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGatedStamp(t, tc.name, tc.fire)
		})
	}
}

// TestTracerWriteIdempotencyGate proves the explicit-key override on a
// representative tracer write: gate off + sdkctx.WithIdempotencyKey → the key
// still stamps. On/off auto-gen is covered by the parametrized table above.
func TestTracerWriteIdempotencyGate(t *testing.T) {
	input := models.NewCreateRuleInput("block-high-value", "transaction.amount > 1000", models.RuleActionDeny)

	srv, key := idempotencyCaptureServer()
	defer srv.Close()

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "explicit-key")
	_, _ = newRulesFacade(newTestTracerClient(t, srv), false).Create(ctx, input)

	if got := key(); got != "explicit-key" {
		t.Fatalf("gate off + ctx key: got %q, want explicit-key", got)
	}
}

// TestTracerExcludedWritesHeaderless confirms the deliberate exclusions:
// validations.Evaluate and ALL mutating reservation methods dedup on body
// identifiers (requestId/transactionId), NOT on X-Idempotency, so they must
// remain header-free even with the gate on.
func TestTracerExcludedWritesHeaderless(t *testing.T) {
	cases := []struct {
		name string
		fire func(t *testing.T, srv *httptest.Server)
	}{
		{"validations.Evaluate", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestValidationsFacade(t, srv).Evaluate(context.Background(), validInput())
		}},
		{"reservations.Reserve", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestReservationsFacade(t, srv).Reserve(context.Background(), validReserveInput())
		}},
		{"reservations.Confirm", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestReservationsFacade(t, srv).Confirm(context.Background(), stampID)
		}},
		{"reservations.Release", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestReservationsFacade(t, srv).Release(context.Background(), stampID)
		}},
		{"reservations.ConfirmByTransaction", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestReservationsFacade(t, srv).ConfirmByTransaction(context.Background(), stampID)
		}},
		{"reservations.ReleaseByTransaction", func(t *testing.T, srv *httptest.Server) {
			t.Helper()
			_, _ = newTestReservationsFacade(t, srv).ReleaseByTransaction(context.Background(), stampID)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, key := idempotencyCaptureServer()
			defer srv.Close()

			tc.fire(t, srv)

			if got := key(); got != "" {
				t.Fatalf("%s stamped X-Idempotency=%q, want none (body-dedup)", tc.name, got)
			}
		})
	}
}
