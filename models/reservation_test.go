// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/shopspring/decimal"
)

const (
	resTxID      = "77777777-7777-7777-7777-777777777777"
	resRequestID = "88888888-8888-8888-8888-888888888888"
	resAccountID = "99999999-9999-9999-9999-999999999999"
)

func validReserveInput() *models.ReserveInput {
	return models.NewReserveInput(
		resTxID,
		resRequestID,
		decimal.RequireFromString("100.50"),
		"USD",
		"2026-01-01T00:00:00Z",
	)
}

// TestReserveInput_ValidateRelaxed proves the reserve validation mirrors the
// server's RELAXED ValidateForReserve: transactionId/requestId/amount>0/currency/
// timestamp are required, but transactionType and account are OPTIONAL. The
// minimal input (no account, no transactionType) MUST pass.
func TestReserveInput_ValidateRelaxed(t *testing.T) {
	if err := validReserveInput().Validate(); err != nil {
		t.Fatalf("minimal reserve input (no account/transactionType) must pass relaxed validation: %v", err)
	}
}

// TestReserveInput_LongLivedMarshals proves WithLongLived(true) puts longLived on
// the wire (the PENDING/direct TTL selector), while a bare NewReserveInput omits
// longLived (omitempty on false) and every unset optional context
// (account/segment/portfolio/merchant). A leaked default would silently pick the
// wrong reservation lifetime.
func TestReserveInput_LongLivedMarshals(t *testing.T) {
	body, err := json.Marshal(validReserveInput().WithLongLived(true))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"longLived":true`) {
		t.Fatalf("body missing longLived:true: %s", body)
	}

	bare, err := json.Marshal(validReserveInput())
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}
	s := string(bare)
	if strings.Contains(s, "longLived") {
		t.Fatalf("bare input leaked longLived (omitempty on false): %s", s)
	}
	for _, key := range []string{"account", "segment", "portfolio", "merchant"} {
		if strings.Contains(s, key) {
			t.Fatalf("bare input leaked unset optional %q: %s", key, s)
		}
	}
}

// TestReserveInput_ValidateRejects proves the required fields are still enforced.
func TestReserveInput_ValidateRejects(t *testing.T) {
	cases := []struct {
		name  string
		mutfn func(*models.ReserveInput)
	}{
		{"empty transactionId", func(i *models.ReserveInput) { i.TransactionID = "  " }},
		{"empty requestId", func(i *models.ReserveInput) { i.RequestID = "" }},
		{"zero amount", func(i *models.ReserveInput) { i.Amount = decimal.Zero }},
		{"negative amount", func(i *models.ReserveInput) { i.Amount = decimal.RequireFromString("-1") }},
		{"bad currency", func(i *models.ReserveInput) { i.Currency = "US" }},
		{"empty timestamp", func(i *models.ReserveInput) { i.TransactionTimestamp = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validReserveInput()
			tc.mutfn(in)
			if err := in.Validate(); err == nil {
				t.Fatalf("%s: want validation error", tc.name)
			}
		})
	}
}
