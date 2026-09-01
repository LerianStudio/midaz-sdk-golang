// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

// TestV2LegScopeIgnoresWhitespace pins the whitespace half of the leg-scope
// reconciliation.
//
// The emptiness test trimmed and the comparison did not, so a leg naming the
// addressed ledger with a stray space was refused as a leg naming a DIFFERENT
// ledger — a refusal pointing at a conflict that did not exist, on the one field
// a caller is most likely to paste in. Both now ignore surrounding whitespace,
// and the trimmed value is what reaches the wire, so what the facade accepted
// and what the ledger reads are the same string.
func TestV2LegScopeIgnoresWhitespace(t *testing.T) {
	var body []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + txID + `","status":{"code":"APPROVED"}}`))
	}))
	defer srv.Close()

	input := sampleV2Input()
	input.Debits[0].LedgerID = "  " + txLedgerID + "  "
	input.Debits[0].OrganizationID = "\t" + txOrgID + "\n"

	if _, err := newTestTransactionsV2Facade(t, srv).
		CreateDirect(context.Background(), txOrgID, txLedgerID, input); err != nil {
		t.Fatalf("a leg naming the addressed scope with stray whitespace is not a conflict: %v", err)
	}

	var sent struct {
		Debits []struct {
			OrganizationID string `json:"organizationId"`
			LedgerID       string `json:"ledgerId"`
		} `json:"debits"`
	}

	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("unreadable request body: %v", err)
	}

	if got := sent.Debits[0].LedgerID; got != txLedgerID {
		t.Errorf("ledgerId on the wire = %q, want the trimmed %q", got, txLedgerID)
	}

	if got := sent.Debits[0].OrganizationID; got != txOrgID {
		t.Errorf("organizationId on the wire = %q, want the trimmed %q", got, txOrgID)
	}
}

// TestV2LegScopeNormalizesAPaddedAddressedScope pins the other direction: the
// padding on the ADDRESSED pair, not on a leg.
//
// Whitespace around the orgID or ledgerID argument used to reach the body
// unevenly. A leg the facade filled got the raw " uuid ", while a leg that
// already named the scope got the trimmed "uuid", so one create carried two
// spellings of one ledger and the server refused the whole body for a conflict
// the caller could not see in their input. The scope is trimmed once now, so
// every leg carries one spelling.
func TestV2LegScopeNormalizesAPaddedAddressedScope(t *testing.T) {
	var body []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + txID + `","status":{"code":"APPROVED"}}`))
	}))
	defer srv.Close()

	// One leg names the scope already; the other leaves it empty for the facade to
	// fill. They must come out identical.
	input := sampleV2Input()
	input.Debits[0].OrganizationID = txOrgID
	input.Debits[0].LedgerID = txLedgerID
	input.Credits[0].OrganizationID = ""
	input.Credits[0].LedgerID = ""

	if _, err := newTestTransactionsV2Facade(t, srv).
		CreateDirect(context.Background(), "  "+txOrgID+"  ", "\t"+txLedgerID+"\n", input); err != nil {
		t.Fatalf("a padded addressed scope is not a conflict: %v", err)
	}

	var sent struct {
		Debits []struct {
			OrganizationID string `json:"organizationId"`
			LedgerID       string `json:"ledgerId"`
		} `json:"debits"`
		Credits []struct {
			OrganizationID string `json:"organizationId"`
			LedgerID       string `json:"ledgerId"`
		} `json:"credits"`
	}

	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("unreadable request body: %v", err)
	}

	if sent.Credits[0].LedgerID != txLedgerID || sent.Credits[0].OrganizationID != txOrgID {
		t.Errorf("filled credit leg = %q/%q, want the trimmed %q/%q",
			sent.Credits[0].OrganizationID, sent.Credits[0].LedgerID, txOrgID, txLedgerID)
	}

	if sent.Debits[0].LedgerID != sent.Credits[0].LedgerID ||
		sent.Debits[0].OrganizationID != sent.Credits[0].OrganizationID {
		t.Errorf("legs disagree: debit %q/%q vs credit %q/%q — the server refuses a body whose legs name different ledgers",
			sent.Debits[0].OrganizationID, sent.Debits[0].LedgerID,
			sent.Credits[0].OrganizationID, sent.Credits[0].LedgerID)
	}
}

// TestV2LegScopeStillRefusesARealConflict guards the boundary: trimming must not
// have turned the conflict refusal into a no-op. A leg naming a different ledger
// is the mistake the reconciliation exists to catch.
func TestV2LegScopeStillRefusesARealConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("a contradicting leg scope must not reach the wire")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	input := sampleV2Input()
	input.Credits[0].LedgerID = "  44444444-4444-4444-4444-444444444444  "

	_, err := newTestTransactionsV2Facade(t, srv).
		CreateDirect(context.Background(), txOrgID, txLedgerID, input)
	if err == nil {
		t.Fatal("a leg naming a different ledger must still be refused")
	}
}

// TestV2LegScopeDoesNotMutateTheCallersInput: the trim writes into the scoped
// COPY. A caller reusing one input across two ledgers must not find the first
// call's normalization in the second.
func TestV2LegScopeDoesNotMutateTheCallersInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + txID + `","status":{"code":"APPROVED"}}`))
	}))
	defer srv.Close()

	input := sampleV2Input()
	padded := "  " + txLedgerID + "  "
	input.Debits[0].LedgerID = padded

	if _, err := newTestTransactionsV2Facade(t, srv).
		CreateDirect(context.Background(), txOrgID, txLedgerID, input); err != nil {
		t.Fatalf("unexpected refusal: %v", err)
	}

	if input.Debits[0].LedgerID != padded {
		t.Fatalf("the caller's input was mutated: ledgerId = %q, want %q",
			input.Debits[0].LedgerID, padded)
	}
}

var _ = models.TransactionV2Leg{}
