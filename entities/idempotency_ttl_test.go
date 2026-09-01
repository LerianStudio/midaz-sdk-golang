// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// idemTTLCaptureServer records both X-Idempotency and X-TTL of the request.
func idemTTLCaptureServer() (*httptest.Server, func() (idem, ttl string)) {
	var (
		mu   sync.Mutex
		idem string
		ttl  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		idem = r.Header.Get(idempotencyHeader)
		ttl = r.Header.Get(ttlHeader)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))

	return srv, func() (string, string) {
		mu.Lock()
		defer mu.Unlock()

		return idem, ttl
	}
}

// TestEditorsHonorIdempotencyTTL proves FW3: an editor-stamped write emits X-TTL
// alongside X-Idempotency when the caller set sdkctx.WithIdempotencyTTL, on both
// planes; unset → no X-TTL (server default applies).
func TestEditorsHonorIdempotencyTTL(t *testing.T) {
	t.Run("ledger op emits X-TTL", func(t *testing.T) {
		srv, get := idemTTLCaptureServer()
		defer srv.Close()

		ctx := sdkctx.WithIdempotencyTTL(context.Background(), 60)
		_, _ = newTestInstrumentsFacade(t, srv).Create(ctx, instrumentsFacadeOrgID, instrumentsFacadeHolderID,
			validCreateInstrumentInput())

		idem, ttl := get()
		if idem == "" {
			t.Fatal("want X-Idempotency present")
		}
		if ttl != "60" {
			t.Fatalf("X-TTL=%q, want 60", ttl)
		}
	})

	t.Run("tracer op emits X-TTL", func(t *testing.T) {
		srv, get := idemTTLCaptureServer()
		defer srv.Close()

		ctx := sdkctx.WithIdempotencyTTL(context.Background(), 60)
		_, _ = newTestRulesFacade(t, srv).Create(ctx,
			models.NewCreateRuleInput("block-high-value", "transaction.amount > 1000", models.RuleActionDeny))

		idem, ttl := get()
		if idem == "" {
			t.Fatal("want X-Idempotency present")
		}
		if ttl != "60" {
			t.Fatalf("X-TTL=%q, want 60", ttl)
		}
	})

	t.Run("no X-TTL when unset", func(t *testing.T) {
		srv, get := idemTTLCaptureServer()
		defer srv.Close()

		_, _ = newTestInstrumentsFacade(t, srv).Create(context.Background(), instrumentsFacadeOrgID, instrumentsFacadeHolderID,
			validCreateInstrumentInput())

		if _, ttl := get(); ttl != "" {
			t.Fatalf("X-TTL=%q, want none (server default)", ttl)
		}
	})
}
