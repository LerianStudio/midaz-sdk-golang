// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// TestTransactionsCreateIdempotencyGated proves Task 5.3.1: transaction creates
// honor the gate — auto-gen only when ON — while an explicit input key stamps
// regardless of the flag.
func TestTransactionsCreateIdempotencyGated(t *testing.T) {
	create := func(ctx context.Context, gate bool, in *models.CreateTransactionInput) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTransactionsFacade(newTestLedgerClient(t, srv), gate).CreateJSON(ctx, txOrgID, txLedgerID, in)

		return key()
	}

	if got := create(context.Background(), true, sampleTransactionInput()); got == "" {
		t.Fatal("gate on: want auto-generated X-Idempotency")
	}

	if got := create(context.Background(), false, sampleTransactionInput()); got != "" {
		t.Fatalf("gate off: want no auto-gen, got %q", got)
	}

	in := sampleTransactionInput()
	in.IdempotencyKey = "explicit-key"

	if got := create(context.Background(), false, in); got != "explicit-key" {
		t.Fatalf("gate off + explicit input key: got %q, want explicit-key", got)
	}
}

// TestTransactionsUpdatesGated proves the fix: UpdateTransaction and
// UpdateOperation (PATCH) honor the gate for auto-gen AND let a ctx/explicit
// key ride — parity with the legacy PATCH (which carried auto-gen + honored the
// ctx key), and with the retrofit's "explicit/ctx key always wins" contract.
func TestTransactionsUpdatesGated(t *testing.T) {
	updateTx := func(ctx context.Context, gate bool) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTransactionsFacade(newTestLedgerClient(t, srv), gate).
			UpdateTransaction(ctx, txOrgID, txLedgerID, txID, models.NewUpdateTransactionInput().WithDescription("x"))

		return key()
	}

	updateOp := func(ctx context.Context, gate bool) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTransactionsFacade(newTestLedgerClient(t, srv), gate).
			UpdateOperation(ctx, txOrgID, txLedgerID, txID, "77777777-7777-7777-7777-777777777777", models.NewUpdateOperationInput().WithDescription("adjusted"))

		return key()
	}

	for name, fn := range map[string]func(context.Context, bool) string{"UpdateTransaction": updateTx, "UpdateOperation": updateOp} {
		if got := fn(context.Background(), true); got == "" {
			t.Fatalf("%s gate on: want auto-generated X-Idempotency", name)
		}

		if got := fn(context.Background(), false); got != "" {
			t.Fatalf("%s gate off: want no auto-gen, got %q", name, got)
		}

		ctx := sdkctx.WithIdempotencyKey(context.Background(), "k")
		if got := fn(ctx, false); got != "k" {
			t.Fatalf("%s gate off + ctx key: got %q, want k", name, got)
		}
	}
}

// TestTransactionsCommitUnaffectedByGate proves the lifecycle actions are
// autoGen=false and thus unaffected by the flag: no key => header-free whether
// the gate is on or off, and a ctx key still rides.
func TestTransactionsCommitUnaffectedByGate(t *testing.T) {
	commit := func(ctx context.Context, gate bool) string {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newTransactionsFacade(newTestLedgerClient(t, srv), gate).Commit(ctx, txOrgID, txLedgerID, txID)

		return key()
	}

	if got := commit(context.Background(), true); got != "" {
		t.Fatalf("commit gate on, no key: want headerless, got %q", got)
	}

	if got := commit(context.Background(), false); got != "" {
		t.Fatalf("commit gate off, no key: want headerless, got %q", got)
	}

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "k")
	if got := commit(ctx, false); got != "k" {
		t.Fatalf("commit + ctx key: got %q, want k (actions still honor an explicit key)", got)
	}
}
