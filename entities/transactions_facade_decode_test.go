// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

// A create that Midaz accepted and answered with a body the SDK cannot decode is
// the most dangerous shape on the money path: the transaction may already exist.
// The error must say so — request sent, response received — instead of claiming
// the SDK never left the process, which is what a generic internal error says.
//
// The body below is a real 201 with an unparseable timestamp: it fails on
// time.Time, not on JSON syntax, so a caller that recognises only
// *json.SyntaxError would replay a create that already moved money.
func TestTransactionsFacade_CreateJSON_UndecodableResponseIsReportedAsAnswered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + txID + `","createdAt":"18/08/2026"}`))
	}))
	defer srv.Close()

	_, err := newTestTransactionsFacade(t, srv).CreateJSON(context.Background(), txOrgID, txLedgerID, sampleTransactionInput())
	if err == nil {
		t.Fatal("expected the undecodable response to surface as an error")
	}

	if !sdkerrors.IsResponseDecodeError(err) {
		t.Fatalf("want a response-decode error, got %#v", err)
	}
	if !sdkerrors.HTTPRequestSent(err) {
		t.Error("the create was posted; the error must not claim otherwise")
	}
	if !sdkerrors.HTTPResponseReceived(err) {
		t.Error("Midaz answered; the error must not claim otherwise")
	}
	if status, upstream := sdkerrors.ActualHTTPStatus(err); upstream {
		t.Errorf("a body we could not read is not an upstream error status, got %d", status)
	}
}
