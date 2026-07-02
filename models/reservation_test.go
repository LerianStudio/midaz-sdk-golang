// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models_test

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
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
