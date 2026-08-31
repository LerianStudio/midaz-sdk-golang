// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

func TestResolveIdempotency_KeyPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context //nolint:containedctx // table-driven test
		explicitKey string
		autoGen     bool
		wantKey     string
		// wantAutoGen asserts the returned key is a fresh UUID (non-empty and
		// not one of the fixed inputs) rather than checking an exact value.
		wantAutoGen bool
	}{
		{
			name:        "explicit key wins over ctx",
			ctx:         sdkctx.WithIdempotencyKey(context.Background(), "ctx-key"),
			explicitKey: "explicit-key",
			autoGen:     true,
			wantKey:     "explicit-key",
		},
		{
			name:    "ctx key used when no explicit key",
			ctx:     sdkctx.WithIdempotencyKey(context.Background(), "ctx-key"),
			autoGen: true,
			wantKey: "ctx-key",
		},
		{
			name:        "auto-gen when neither explicit nor ctx and autoGen=true",
			ctx:         context.Background(),
			autoGen:     true,
			wantAutoGen: true,
		},
		{
			name:    "no key when autoGen=false and no explicit/ctx",
			ctx:     context.Background(),
			autoGen: false,
			wantKey: "",
		},
		{
			name:    "WithoutAutoIdempotency suppresses auto-gen",
			ctx:     sdkctx.WithoutAutoIdempotency(context.Background()),
			autoGen: true,
			wantKey: "",
		},
		{
			name:        "explicit key survives suppression",
			ctx:         sdkctx.WithoutAutoIdempotency(context.Background()),
			explicitKey: "explicit-key",
			autoGen:     true,
			wantKey:     "explicit-key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, _ := resolveIdempotency(tt.ctx, tt.explicitKey, tt.autoGen)

			if tt.wantAutoGen {
				if key == "" || key == tt.explicitKey || key == "ctx-key" {
					t.Errorf("expected a fresh auto-generated key, got %q", key)
				}
				return
			}

			if key != tt.wantKey {
				t.Errorf("resolveIdempotency key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestResolveIdempotency_TTL(t *testing.T) {
	t.Run("ttl from ctx", func(t *testing.T) {
		ctx := sdkctx.WithIdempotencyTTL(context.Background(), 600)
		if _, ttl := resolveIdempotency(ctx, "k", false); ttl != "600" {
			t.Errorf("ttl = %q, want %q", ttl, "600")
		}
	})

	t.Run("ttl empty when absent", func(t *testing.T) {
		if _, ttl := resolveIdempotency(context.Background(), "k", false); ttl != "" {
			t.Errorf("ttl = %q, want empty", ttl)
		}
	})
}

func TestSetHeader_SetsIdempotency(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/", http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	editor := setHeader(idempotencyHeader, "key-789")
	if err := editor(context.Background(), req); err != nil {
		t.Fatalf("setHeader editor returned error: %v", err)
	}

	if got := req.Header.Get(idempotencyHeader); got != "key-789" {
		t.Errorf("%s header = %q, want %q", idempotencyHeader, got, "key-789")
	}
}
