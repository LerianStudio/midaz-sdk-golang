// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package models

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestValidationJudgesTheBytesThatLeave pins the rule that local validation and
// the wire must agree.
//
// A padded money string used to pass here and travel to the ledger verbatim,
// where it fails as a malformed decimal. The SDK does not rewrite money text, so
// the only way for the local answer to mean anything is to judge the exact bytes
// that go out.
func TestValidationJudgesTheBytesThatLeave(t *testing.T) {
	tests := []struct {
		name    string
		amount  any
		wantErr bool
	}{
		{name: "clean decimal string", amount: "100"},
		{name: "clean decimal with fraction", amount: "100.55"},
		{name: "leading and trailing spaces", amount: "  100  ", wantErr: true},
		{name: "leading space", amount: " 100", wantErr: true},
		{name: "trailing newline", amount: "100\n", wantErr: true},
		{name: "not a decimal", amount: "abc", wantErr: true},
		{name: "zero", amount: "0", wantErr: true},
		{name: "negative", amount: "-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePositiveDecimalString(tt.amount)
			if tt.wantErr && err == nil {
				t.Fatalf("%q must be refused locally; sending it verbatim earns a server 400", tt.amount)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("%q must be accepted, got %v", tt.amount, err)
			}
		})
	}
}

// TestV2LegRefusesCompositeAlias mirrors the ledger's own v2 alias guard.
//
// The ledger rewrites an accepted alias into a composite "index#alias#balanceKey"
// form and keys its per-entry maps on it, leaving an alias that already looks
// composite spelled as the client sent it. A client-supplied '#' therefore
// reaches those maps unmutated, where it collides with another entry's key or
// matches none — and a transaction that loses one side's entry moves value in
// one direction only.
func TestV2LegRefusesCompositeAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr bool
	}{
		{name: "plain alias", alias: "@person1"},
		{name: "external account keeps its slash", alias: "@external/USD"},
		{name: "colon and dash stay legal", alias: "@acct:main-1"},
		{name: "separator in the middle", alias: "@person1#default", wantErr: true},
		{name: "separator leading", alias: "#person1", wantErr: true},
		{name: "separator trailing", alias: "@person1#", wantErr: true},
		{name: "forged composite entry key", alias: "0#@person1#default", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			leg := TransactionV2Leg{
				Alias:          tt.alias,
				OrganizationID: "org",
				LedgerID:       "ledger",
				Amount:         "100",
			}

			err := leg.validate()
			if tt.wantErr && err == nil {
				t.Fatalf("alias %q must be refused: the ledger refuses it and a lost leg moves value one way", tt.alias)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("alias %q must be accepted, got %v", tt.alias, err)
			}
		})
	}
}

// TestV2LegDescriptionBound pins the per-leg description against the ledger's
// own "max=256". The leg description is optional, so the interesting cases are
// the two ends: absent must stay valid, and one character over must be refused
// here rather than travelling out to earn a 400 on a money-moving request.
func TestV2LegDescriptionBound(t *testing.T) {
	newLeg := func(description string) TransactionV2Leg {
		return TransactionV2Leg{
			Alias:          "@person1",
			OrganizationID: "org",
			LedgerID:       "ledger",
			Amount:         "100",
			Description:    description,
		}
	}

	tests := []struct {
		name        string
		description string
		wantErr     bool
	}{
		{name: "absent"},
		{name: "short", description: "card settlement"},
		{name: "at the limit", description: strings.Repeat("x", maxTransactionDescriptionLength)},
		{name: "one over the limit", description: strings.Repeat("x", maxTransactionDescriptionLength+1), wantErr: true},
		// The ledger counts RUNES, not bytes. A Portuguese description of 256
		// accented characters is 512 bytes on the wire and legal on the server;
		// counting bytes here would refuse a money-moving request the ledger
		// would have accepted.
		{name: "at the limit in multi-byte characters", description: strings.Repeat("ç", maxTransactionDescriptionLength)},
		{name: "one over the limit in multi-byte characters", description: strings.Repeat("ç", maxTransactionDescriptionLength+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newLeg(tt.description).validate()
			if tt.wantErr && err == nil {
				t.Fatalf("a %d-character leg description must be refused locally; the ledger caps it at %d",
					utf8.RuneCountInString(tt.description), maxTransactionDescriptionLength)
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("a %d-character (%d-byte) leg description must be accepted, got %v",
					utf8.RuneCountInString(tt.description), len(tt.description), err)
			}
		})
	}
}

// TestUpdateTransactionDescriptionBoundIsTheSameOnBothSurfaces: /v1 refused a
// description over 256 characters and /v2 did not, so the same patch was
// refused locally on one surface and rejected by the server on the other. One
// server-side struct with one "max=256" validator serves both.
func TestUpdateTransactionDescriptionBoundIsTheSameOnBothSurfaces(t *testing.T) {
	tooLong := strings.Repeat("x", maxTransactionDescriptionLength+1)
	atLimit := strings.Repeat("x", maxTransactionDescriptionLength)

	t.Run("v2 refuses an over-long description", func(t *testing.T) {
		if err := (&UpdateTransactionV2Input{Description: tooLong}).Validate(); err == nil {
			t.Fatal("a description over the server bound must be refused locally, as /v1 does")
		}
	})

	t.Run("v1 refuses an over-long description", func(t *testing.T) {
		if err := (&UpdateTransactionInput{Description: tooLong}).Validate(); err == nil {
			t.Fatal("v1 lost its description bound")
		}
	})

	t.Run("both accept the limit exactly", func(t *testing.T) {
		if err := (&UpdateTransactionV2Input{Description: atLimit}).Validate(); err != nil {
			t.Fatalf("v2 must accept a description at the bound: %v", err)
		}

		if err := (&UpdateTransactionInput{Description: atLimit}).Validate(); err != nil {
			t.Fatalf("v1 must accept a description at the bound: %v", err)
		}
	})

	// The ledger spec marks description AND metadata required on the shared
	// update schema, but both routes register SkipValidateBody and decode
	// imperatively into a struct with no required tag on either field, so a
	// description-only patch is accepted. The SDK must not invent a stricter rule
	// than the handler enforces.
	t.Run("a description-only patch is accepted on both", func(t *testing.T) {
		if err := (&UpdateTransactionV2Input{Description: "settled"}).Validate(); err != nil {
			t.Fatalf("v2 must accept a description-only patch: %v", err)
		}

		if err := (&UpdateTransactionInput{Description: "settled"}).Validate(); err != nil {
			t.Fatalf("v1 must accept a description-only patch: %v", err)
		}
	})
}
