// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
)

// Route ids are UUID-validated by every create validator, so the fixtures below
// use real ones at both the transaction and the leg level.
const (
	wireRouteID    = "44444444-4444-4444-4444-444444444444"
	wireLegRouteID = "55555555-5555-5555-5555-555555555555"
)

// wireTransactionInput is a deliberately full /json create: every field whose
// struct tag disagrees with the endpoint mapper is set (Amount, AssetCode,
// Template, ExternalID, IdempotencyKey at the transaction level; Account on a
// leg, which the wire names accountAlias), plus the fields that must survive.
func wireTransactionInput() *models.CreateTransactionInput {
	legRouteID := wireLegRouteID

	return &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden transaction",
		Code:                     "TX-CODE-1",
		Route:                    "transfer-route",
		RouteID:                  wireRouteID,
		TransactionDate:          "2026-01-15T10:00:00Z",
		Metadata:                 map[string]any{"aggregateId": "agg-1"},
		Template:                 "legacy-template",
		Amount:                   "150.00",
		AssetCode:                "USD",
		ExternalID:               "ext-1",
		IdempotencyKey:           "idem-1",
		Send: &models.SendInput{
			Asset: "USD",
			Value: "150.00",
			Source: &models.SourceInput{From: []models.FromToInput{
				{
					Account:         "@src-primary",
					Amount:          models.AmountInput{Asset: "USD", Value: "100.00"},
					BalanceKey:      "settlement",
					Description:     "primary leg",
					ChartOfAccounts: "1.1.1",
					Route:           "leg-route",
					RouteID:         &legRouteID,
					Metadata:        map[string]any{"legId": "leg-1"},
				},
				{
					AccountAlias: "@src-secondary",
					Amount:       models.AmountInput{Asset: "USD", Value: "50.00"},
				},
			}},
			Distribute: &models.DistributeInput{To: []models.FromToInput{
				{Account: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "150.00"}},
			}},
		},
	}
}

func wireInflowInput() *models.CreateInflowInput {
	return &models.CreateInflowInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden inflow",
		Code:                     "IN-CODE-1",
		Metadata:                 map[string]any{"aggregateId": "agg-2"},
		Route:                    "inflow-route",
		RouteID:                  wireRouteID,
		TransactionDate:          "2026-01-15T10:00:00Z",
		Send: &models.SendInflowInput{
			Asset: "USD",
			Value: "75.00",
			Distribute: &models.DistributeInput{To: []models.FromToInput{
				{Account: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "75.00"}},
			}},
		},
	}
}

// wireOutflowInput carries Pending instead of a transactionDate: the create
// validators reject the two together.
func wireOutflowInput() *models.CreateOutflowInput {
	return &models.CreateOutflowInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden outflow",
		Code:                     "OUT-CODE-1",
		Metadata:                 map[string]any{"aggregateId": "agg-3"},
		Route:                    "outflow-route",
		RouteID:                  wireRouteID,
		Pending:                  true,
		Send: &models.SendOutflowInput{
			Asset: "USD",
			Value: "25.00",
			Source: &models.SourceInput{From: []models.FromToInput{
				{Account: "@src", Amount: models.AmountInput{Asset: "USD", Value: "25.00"}},
			}},
		},
	}
}

func wireAnnotationInput() *models.CreateAnnotationInput {
	return &models.CreateAnnotationInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden annotation",
		Code:                     "AN-CODE-1",
		Metadata:                 map[string]any{"aggregateId": "agg-4"},
		Route:                    "annotation-route",
		RouteID:                  wireRouteID,
	}
}

// captureWireBody runs one facade create against a stub server and returns the
// exact bytes the SDK put on the wire.
func captureWireBody(t *testing.T, call func(f *transactionsFacade, ctx context.Context) error) []byte {
	t.Helper()

	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}

		gotBody = body

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(txResponseBody()))
	}))
	defer srv.Close()

	if err := call(newTestTransactionsFacade(t, srv), context.Background()); err != nil {
		t.Fatalf("create: %v", err)
	}

	return gotBody
}

func decodeWireObject(t *testing.T, label string, data []byte) map[string]any {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("%s is not a JSON object: %v (%s)", label, err, data)
	}

	return decoded
}

// TestTransactionsFacadeWire_MarshalIsAuthoritative pins the create contract
// invariant: json.Marshal of a create input IS the request body, for every
// create endpoint. Anyone inspecting, logging, or persisting the marshaled input
// sees exactly what the ledger received — the struct tags cannot describe a
// different payload than the one that moves money.
//
// Compared as decoded objects, not bytes: Go map iteration makes key order
// arbitrary, and key order carries no meaning on the wire.
func TestTransactionsFacadeWire_MarshalIsAuthoritative(t *testing.T) {
	txInput := wireTransactionInput()
	inflowInput := wireInflowInput()
	outflowInput := wireOutflowInput()
	annotationInput := wireAnnotationInput()

	tests := []struct {
		name  string
		input any
		call  func(f *transactionsFacade, ctx context.Context) error
	}{
		{
			name:  "json",
			input: txInput,
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateJSON(ctx, txOrgID, txLedgerID, txInput)

				return err
			},
		},
		{
			name:  "inflow",
			input: inflowInput,
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateInflow(ctx, txOrgID, txLedgerID, inflowInput)

				return err
			},
		},
		{
			name:  "outflow",
			input: outflowInput,
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateOutflow(ctx, txOrgID, txLedgerID, outflowInput)

				return err
			},
		},
		{
			name:  "annotation",
			input: annotationInput,
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateAnnotation(ctx, txOrgID, txLedgerID, annotationInput)

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBody := captureWireBody(t, tt.call)

			marshaled, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal(input): %v", err)
			}

			wire := decodeWireObject(t, "http body", gotBody)
			direct := decodeWireObject(t, "json.Marshal(input)", marshaled)

			if !reflect.DeepEqual(wire, direct) {
				t.Fatalf("json.Marshal(input) is not the wire body\n   http body: %s\nmarshal(input): %s", gotBody, marshaled)
			}
		})
	}
}

// TestTransactionsFacadeWire_LegacyFieldsNeverShip is the other half of the
// invariant: proving marshal == wire is worthless if both carry a field the
// ledger rejects. The legacy transaction-level inputs stay out of the payload,
// and a leg reaches the ledger as accountAlias only.
func TestTransactionsFacadeWire_LegacyFieldsNeverShip(t *testing.T) {
	input := wireTransactionInput()

	gotBody := captureWireBody(t, func(f *transactionsFacade, ctx context.Context) error {
		_, err := f.CreateJSON(ctx, txOrgID, txLedgerID, input)

		return err
	})

	wire := decodeWireObject(t, "http body", gotBody)

	for _, key := range []string{"amount", "assetCode", "template", "operations", "externalId", "idempotencyKey"} {
		if _, ok := wire[key]; ok {
			t.Errorf("wire body carries legacy transaction field %q: %s", key, gotBody)
		}
	}

	send, ok := wire["send"].(map[string]any)
	if !ok {
		t.Fatalf("wire body has no send envelope: %s", gotBody)
	}

	source, ok := send["source"].(map[string]any)
	if !ok {
		t.Fatalf("send has no source: %s", gotBody)
	}

	from, ok := source["from"].([]any)
	if !ok || len(from) == 0 {
		t.Fatalf("source has no from legs: %s", gotBody)
	}

	leg, ok := from[0].(map[string]any)
	if !ok {
		t.Fatalf("first from leg is not an object: %s", gotBody)
	}

	if _, ok := leg["account"]; ok {
		t.Errorf("leg carries account instead of accountAlias only: %s", gotBody)
	}

	if leg["accountAlias"] != "@src-primary" {
		t.Errorf("leg accountAlias = %v, want %q: %s", leg["accountAlias"], "@src-primary", gotBody)
	}
}
