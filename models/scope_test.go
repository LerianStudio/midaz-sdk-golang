// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestScope_RoundTrip proves the Scope wire shape matches the generated
// gentracer.Scope: camelCase keys, all fields omitempty. A server-issued scope
// decodes into the SDK struct and re-encodes byte-compatibly.
func TestScope_RoundTrip(t *testing.T) {
	const wire = `{"accountId":"acc-1","merchantId":"mer-1","portfolioId":"por-1","segmentId":"seg-1","subType":"PAYMENT","transactionType":"PIX"}`

	var s Scope
	if err := json.Unmarshal([]byte(wire), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.AccountID == nil || *s.AccountID != "acc-1" {
		t.Fatalf("AccountID = %v, want acc-1", s.AccountID)
	}
	if s.TransactionType == nil || *s.TransactionType != "PIX" {
		t.Fatalf("TransactionType = %v, want PIX", s.TransactionType)
	}

	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"accountId", "merchantId", "portfolioId", "segmentId", "subType", "transactionType"} {
		if !strings.Contains(string(out), `"`+key+`"`) {
			t.Fatalf("marshaled scope missing camelCase key %q: %s", key, out)
		}
	}
}

// TestScope_EmptyOmitsEverything proves a zero-value scope encodes to {} — every
// field omitempty, so "match anything" carries no keys on the wire.
func TestScope_EmptyOmitsEverything(t *testing.T) {
	out, err := json.Marshal(Scope{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(out) != "{}" {
		t.Fatalf("empty scope = %s, want {}", out)
	}
}
