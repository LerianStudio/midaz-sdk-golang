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

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
)

// Route ids are UUID-validated by every create validator, so the fixtures below
// use real ones at both the transaction and the leg level. route and routeId are
// mutually exclusive, so no fixture sets both: the annotation fixture pins the
// route alias on the wire, the others pin routeId.
const (
	wireRouteID    = "44444444-4444-4444-4444-444444444444"
	wireLegRouteID = "55555555-5555-5555-5555-555555555555"
)

// wireTransactionInput is a deliberately full /json create: the fields that must
// survive to the ledger, one leg per value mechanism (fixed amount and share),
// and IdempotencyKey, which travels as a header and must never appear in the
// body.
func wireTransactionInput() *models.CreateTransactionInput {
	legRouteID := wireLegRouteID

	return &models.CreateTransactionInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden transaction",
		Code:                     "TX-CODE-1",
		RouteID:                  wireRouteID,
		TransactionDate:          "2026-01-15T10:00:00Z",
		Metadata:                 map[string]any{"aggregateId": "agg-1"},
		IdempotencyKey:           "idem-1",
		Send: &models.SendInput{
			Asset: "USD",
			Value: "150.00",
			Source: &models.SourceInput{From: []models.FromToInput{
				{
					AccountAlias:    "@src-primary",
					Amount:          models.AmountInput{Asset: "USD", Value: "100.00"},
					BalanceKey:      "settlement",
					Description:     "primary leg",
					ChartOfAccounts: "1.1.1",
					RouteID:         &legRouteID,
					Metadata:        map[string]any{"legId": "leg-1"},
				},
				{
					AccountAlias: "@src-secondary",
					Amount:       models.AmountInput{Asset: "USD", Value: "50.00"},
				},
			}},
			Distribute: &models.DistributeInput{To: []models.FromToInput{
				{AccountAlias: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "150.00"}},
				{AccountAlias: "@dst-share", Share: &models.Share{Percentage: 10}},
			}},
		},
	}
}

// wantTransactionBody is the /transactions/json request body, written out by
// hand. It is the one statement of the wire contract in this suite that does not
// come from the code under test, so a renamed, dropped, or newly added key fails
// here — including the removed legacy keys (amount, assetCode, externalId,
// operations, template) and a leg's former account key, none of which appear.
func wantTransactionBody() map[string]any {
	return map[string]any{
		"chartOfAccountsGroupName": "settlement-group",
		"description":              "wire golden transaction",
		"code":                     "TX-CODE-1",
		"routeId":                  wireRouteID,
		"transactionDate":          "2026-01-15T10:00:00Z",
		"metadata":                 map[string]any{"aggregateId": "agg-1"},
		"send": map[string]any{
			"asset": "USD",
			"value": "150.00",
			"source": map[string]any{
				"from": []any{
					map[string]any{
						"accountAlias":    "@src-primary",
						"amount":          map[string]any{"asset": "USD", "value": "100.00"},
						"balanceKey":      "settlement",
						"description":     "primary leg",
						"chartOfAccounts": "1.1.1",
						"routeId":         wireLegRouteID,
						"metadata":        map[string]any{"legId": "leg-1"},
					},
					map[string]any{
						"accountAlias": "@src-secondary",
						"amount":       map[string]any{"asset": "USD", "value": "50.00"},
					},
				},
			},
			"distribute": map[string]any{
				"to": []any{
					map[string]any{
						"accountAlias": "@dst",
						"amount":       map[string]any{"asset": "USD", "value": "150.00"},
					},
					// A share leg carries no amount: the endpoints reject an
					// empty amount object beside a share.
					map[string]any{
						"accountAlias": "@dst-share",
						"share":        map[string]any{"percentage": float64(10)},
					},
				},
			},
		},
	}
}

func wireInflowInput() *models.CreateInflowInput {
	return &models.CreateInflowInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden inflow",
		Code:                     "IN-CODE-1",
		Metadata:                 map[string]any{"aggregateId": "agg-2"},
		RouteID:                  wireRouteID,
		TransactionDate:          "2026-01-15T10:00:00Z",
		Send: &models.SendInflowInput{
			Asset: "USD",
			Value: "75.00",
			Distribute: &models.DistributeInput{To: []models.FromToInput{
				{AccountAlias: "@dst", Amount: models.AmountInput{Asset: "USD", Value: "75.00"}},
			}},
		},
	}
}

// wantInflowBody pins /transactions/inflow: no source envelope at all, never a
// null one.
func wantInflowBody() map[string]any {
	return map[string]any{
		"chartOfAccountsGroupName": "settlement-group",
		"description":              "wire golden inflow",
		"code":                     "IN-CODE-1",
		"routeId":                  wireRouteID,
		"transactionDate":          "2026-01-15T10:00:00Z",
		"metadata":                 map[string]any{"aggregateId": "agg-2"},
		"send": map[string]any{
			"asset": "USD",
			"value": "75.00",
			"distribute": map[string]any{
				"to": []any{
					map[string]any{
						"accountAlias": "@dst",
						"amount":       map[string]any{"asset": "USD", "value": "75.00"},
					},
				},
			},
		},
	}
}

// wireOutflowInput carries Pending instead of a transactionDate (the create
// validators reject the two together) and states its money as Go numbers: every
// value on the wire must still be a decimal string, never a JSON number.
func wireOutflowInput() *models.CreateOutflowInput {
	return &models.CreateOutflowInput{
		ChartOfAccountsGroupName: "settlement-group",
		Description:              "wire golden outflow",
		Code:                     "OUT-CODE-1",
		Metadata:                 map[string]any{"aggregateId": "agg-3"},
		RouteID:                  wireRouteID,
		Pending:                  true,
		Send: &models.SendOutflowInput{
			Asset: "USD",
			Value: 25,
			Source: &models.SourceInput{From: []models.FromToInput{
				{AccountAlias: "@src", Amount: models.AmountInput{Asset: "USD", Value: 25}},
			}},
		},
	}
}

// wantOutflowBody pins /transactions/outflow: no distribute envelope, pending
// present, and the numeric input values rendered as decimal strings (a JSON
// number here would be a float hop on the money path).
func wantOutflowBody() map[string]any {
	return map[string]any{
		"chartOfAccountsGroupName": "settlement-group",
		"description":              "wire golden outflow",
		"code":                     "OUT-CODE-1",
		"routeId":                  wireRouteID,
		"pending":                  true,
		"metadata":                 map[string]any{"aggregateId": "agg-3"},
		"send": map[string]any{
			"asset": "USD",
			"value": "25",
			"source": map[string]any{
				"from": []any{
					map[string]any{
						"accountAlias": "@src",
						"amount":       map[string]any{"asset": "USD", "value": "25"},
					},
				},
			},
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
	}
}

// wantAnnotationBody pins /transactions/annotation: the route alias survives
// under route (not routeId), and a metadata-only annotation ships no send.
func wantAnnotationBody() map[string]any {
	return map[string]any{
		"chartOfAccountsGroupName": "settlement-group",
		"description":              "wire golden annotation",
		"code":                     "AN-CODE-1",
		"route":                    "annotation-route",
		"metadata":                 map[string]any{"aggregateId": "agg-4"},
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

// TestTransactionsFacadeWire_GoldenBodies pins the create contract on both
// sides at once, per endpoint: the body the facade actually put on the wire and
// json.Marshal of the same input must each equal a hand-written golden payload.
//
// Checking both against the golden — instead of against each other — is what
// makes this a test. The facade serializes with json.Marshal(input), so
// comparing the wire body to json.Marshal(input) compares an expression with
// itself and cannot fail; only the literal payload catches a mapper that renames
// accountAlias, drops metadata, or starts emitting a removed legacy key.
//
// Compared as decoded objects, not bytes: Go map iteration makes key order
// arbitrary, and key order carries no meaning on the wire.
func TestTransactionsFacadeWire_GoldenBodies(t *testing.T) {
	txInput := wireTransactionInput()
	inflowInput := wireInflowInput()
	outflowInput := wireOutflowInput()
	annotationInput := wireAnnotationInput()

	tests := []struct {
		name  string
		input any
		want  map[string]any
		call  func(f *transactionsFacade, ctx context.Context) error
	}{
		{
			name:  "json",
			input: txInput,
			want:  wantTransactionBody(),
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateJSON(ctx, txOrgID, txLedgerID, txInput)

				return err
			},
		},
		{
			name:  "inflow",
			input: inflowInput,
			want:  wantInflowBody(),
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateInflow(ctx, txOrgID, txLedgerID, inflowInput)

				return err
			},
		},
		{
			name:  "outflow",
			input: outflowInput,
			want:  wantOutflowBody(),
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateOutflow(ctx, txOrgID, txLedgerID, outflowInput)

				return err
			},
		},
		{
			name:  "annotation",
			input: annotationInput,
			want:  wantAnnotationBody(),
			call: func(f *transactionsFacade, ctx context.Context) error {
				_, err := f.CreateAnnotation(ctx, txOrgID, txLedgerID, annotationInput)

				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBody := captureWireBody(t, tt.call)

			wire := decodeWireObject(t, "http body", gotBody)
			if !reflect.DeepEqual(wire, tt.want) {
				t.Errorf("wire body is not the golden payload\n got: %s\nwant: %v", gotBody, tt.want)
			}

			marshaled, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("json.Marshal(input): %v", err)
			}

			direct := decodeWireObject(t, "json.Marshal(input)", marshaled)
			if !reflect.DeepEqual(direct, tt.want) {
				t.Errorf("json.Marshal(input) is not the golden payload\n got: %s\nwant: %v", marshaled, tt.want)
			}
		})
	}
}
