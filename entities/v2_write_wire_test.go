// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	"github.com/LerianStudio/midaz-sdk-golang/v5/pkg/sdkctx"
)

// Every V2 write has the same two ways to be silently wrong, and neither shows
// up on a live server as anything but a puzzling result:
//
//   - the REQUEST loses a field on its way to the wire, so the resource the
//     ledger creates is not the resource the caller described; or
//   - the RESPONSE loses a field on its way back, so the caller books an id and
//     a status that read as zero values while err is nil.
//
// A row here states both ends: what the caller passed must appear in the body
// that left, and what the server answered must appear in the model that came
// back. Asserting only one end passes a facade that is broken at the other.

// v2WriteRow is one V2 write: the request it must produce and the response it
// must decode.
type v2WriteRow struct {
	name       string
	wantMethod string
	// wantBody are fragments of the marshalled input that must appear in the
	// request body. Empty for the bodiless writes (delete, commit).
	wantBody []string
	status   int
	response string
	// wantFields are the response fields the caller reads, by the name fire
	// returns them under. Empty for writes that return nothing.
	wantFields map[string]string
	fire       func(t *testing.T, srv *httptest.Server) (map[string]string, error)
}

// v2Writes is one row per write operation on the THIRTEEN dual-served V2
// facades. The nine V2-only facades have their own per-facade tests; this table
// is the dual-served half of the surface, not the whole of it.
//
// It is an enumeration, not a sample. Each of these methods reaches the wire
// through its own generated call and its own params struct, so a row that
// exists next door proves nothing about the one that does not.
//
// Three transaction writes are enumerated ELSEWHERE rather than missing, each
// because its own file pins the same request-and-decode pair against behaviour
// this table's fixed fixtures cannot express:
//
//   - V2.Transactions.CreateDirect and V2.Transactions.Revert are rows in
//     empty_success_body_test.go, which drives them against a 2xx carrying NO
//     body — the case that decides whether a caller gets a zero-valued
//     transaction with a nil error.
//   - V2.Transactions.Cancel is pinned by TestCancelStillSynthesizesOnAnEmptyBody
//     in the same file. Cancel is the one write allowed to synthesize a result
//     from the request, so its contract is the empty-body branch, not a decode of
//     a populated one.
//
// Adding a row here for any of the three would assert a plain populated decode
// that is not where their behaviour differs.
var v2Writes = []v2WriteRow{
	{
		name:       "V2.Organizations.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"legalName":"Acme"`, `"legalDocument":"doc-1"`},
		status:     http.StatusCreated,
		response:   `{"id":"org-1","legalName":"Acme","doingBusinessAs":"Acme Co"}`,
		wantFields: map[string]string{"id": "org-1", "legalName": "Acme"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			org, err := newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), &models.CreateOrganizationInput{LegalName: "Acme", LegalDocument: "doc-1"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": org.ID, "legalName": org.LegalName}, nil
		},
	},
	{
		name:       "V2.Organizations.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"legalName":"Acme Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"org-1","legalName":"Acme Renamed"}`,
		wantFields: map[string]string{"id": "org-1", "legalName": "Acme Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			org, err := newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2UUIDA, &models.UpdateOrganizationInput{LegalName: "Acme Renamed"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": org.ID, "legalName": org.LegalName}, nil
		},
	},
	{
		name:       "V2.Organizations.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newOrganizationsV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2UUIDA)
		},
	},
	{
		name:       "V2.Ledgers.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"name":"Treasury"`},
		status:     http.StatusCreated,
		response:   `{"id":"led-1","name":"Treasury","organizationId":"` + v2Org + `"}`,
		wantFields: map[string]string{"id": "led-1", "name": "Treasury"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			led, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, &models.CreateLedgerInput{Name: "Treasury"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": led.ID, "name": led.Name}, nil
		},
	},
	{
		name:       "V2.Ledgers.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"name":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"led-1","name":"Renamed"}`,
		wantFields: map[string]string{"id": "led-1", "name": "Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			led, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2UUIDA, &models.UpdateLedgerInput{Name: "Renamed"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": led.ID, "name": led.Name}, nil
		},
	},
	{
		name:       "V2.Ledgers.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newLedgersV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2UUIDA)
		},
	},
	{
		name:       "V2.Ledgers.UpdateSettings",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"requireHolder":true`},
		status:     http.StatusOK,
		response:   `{"accounting":{"requireHolder":true,"validateRoutes":true},"overrides":{"allowFeeSkip":true}}`,
		wantFields: map[string]string{"requireHolder": "true", "allowFeeSkip": "true"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			settings, err := newLedgersV2Facade(newTestLedgerClient(t, srv), true).
				UpdateSettings(context.Background(), v2Org, v2UUIDA,
					models.NewUpdateLedgerSettingsInput().WithRequireHolder(true))
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"requireHolder": boolText(settings.Accounting.RequireHolder),
				"allowFeeSkip":  boolText(settings.Overrides.AllowFeeSkip),
			}, nil
		},
	},
	{
		name:       "V2.Accounts.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"name":"Checking"`, `"assetCode":"USD"`, `"type":"deposit"`},
		status:     http.StatusCreated,
		response:   `{"id":"acc-1","name":"Checking","assetCode":"USD","alias":"@checking"}`,
		wantFields: map[string]string{"id": "acc-1", "assetCode": "USD", "alias": "@checking"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			acc, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger,
					&models.CreateAccountInput{Name: "Checking", AssetCode: "USD", Type: "deposit"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": acc.ID, "assetCode": acc.AssetCode, "alias": derefString(acc.Alias)}, nil
		},
	},
	{
		name:       "V2.Accounts.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"name":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"acc-1","name":"Renamed","assetCode":"USD"}`,
		wantFields: map[string]string{"id": "acc-1", "name": "Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			acc, err := newAccountsV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA, &models.UpdateAccountInput{Name: "Renamed"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": acc.ID, "name": acc.Name}, nil
		},
	},
	{
		name:       "V2.Accounts.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newAccountsV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.AccountTypes.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"name":"Cash"`, `"keyValue":"CASH"`},
		status:     http.StatusCreated,
		response:   `{"id":"` + v2UUIDA + `","name":"Cash","keyValue":"CASH"}`,
		wantFields: map[string]string{"id": v2UUIDA, "keyValue": "CASH"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			at, err := newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger,
					&models.CreateAccountTypeInput{Name: "Cash", KeyValue: "CASH"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": at.ID.String(), "keyValue": at.KeyValue}, nil
		},
	},
	{
		name:       "V2.AccountTypes.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"name":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"` + v2UUIDA + `","name":"Renamed","keyValue":"CASH"}`,
		wantFields: map[string]string{"id": v2UUIDA, "name": "Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			at, err := newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA,
					models.NewUpdateAccountTypeInput().WithName("Renamed"))
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": at.ID.String(), "name": at.Name}, nil
		},
	},
	{
		name:       "V2.AccountTypes.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newAccountTypesV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.Assets.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"name":"US Dollar"`, `"code":"USD"`, `"type":"currency"`},
		status:     http.StatusCreated,
		response:   `{"id":"ast-1","name":"US Dollar","code":"USD","type":"currency"}`,
		wantFields: map[string]string{"id": "ast-1", "code": "USD"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			asset, err := newAssetsV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger,
					&models.CreateAssetInput{Name: "US Dollar", Code: "USD", Type: "currency"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": asset.ID, "code": asset.Code}, nil
		},
	},
	{
		name:       "V2.Assets.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"name":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"ast-1","name":"Renamed","code":"USD"}`,
		wantFields: map[string]string{"id": "ast-1", "name": "Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			asset, err := newAssetsV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA, &models.UpdateAssetInput{Name: "Renamed"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": asset.ID, "name": asset.Name}, nil
		},
	},
	{
		name:       "V2.Assets.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newAssetsV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.Portfolios.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"name":"Alpha"`, `"entityId":"ent-1"`},
		status:     http.StatusCreated,
		response:   `{"id":"pf-1","name":"Alpha","entityId":"ent-1"}`,
		wantFields: map[string]string{"id": "pf-1", "name": "Alpha"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			pf, err := newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger,
					&models.CreatePortfolioInput{Name: "Alpha", EntityID: "ent-1"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": pf.ID, "name": pf.Name}, nil
		},
	},
	{
		name:       "V2.Portfolios.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"name":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"pf-1","name":"Renamed"}`,
		wantFields: map[string]string{"id": "pf-1", "name": "Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			pf, err := newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA, &models.UpdatePortfolioInput{Name: "Renamed"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": pf.ID, "name": pf.Name}, nil
		},
	},
	{
		name:       "V2.Portfolios.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newPortfoliosV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.Segments.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"name":"North"`},
		status:     http.StatusCreated,
		response:   `{"id":"sg-1","name":"North"}`,
		wantFields: map[string]string{"id": "sg-1", "name": "North"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			sg, err := newSegmentsV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger, &models.CreateSegmentInput{Name: "North"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": sg.ID, "name": sg.Name}, nil
		},
	},
	{
		name:       "V2.Segments.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"name":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"sg-1","name":"Renamed"}`,
		wantFields: map[string]string{"id": "sg-1", "name": "Renamed"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			sg, err := newSegmentsV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA, &models.UpdateSegmentInput{Name: "Renamed"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": sg.ID, "name": sg.Name}, nil
		},
	},
	{
		name:       "V2.Segments.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newSegmentsV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.OperationRoutes.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"title":"Cashin"`, `"operationType":"source"`},
		status:     http.StatusCreated,
		response:   `{"id":"` + v2UUIDA + `","title":"Cashin","operationType":"source","code":"EXT-001"}`,
		wantFields: map[string]string{"id": v2UUIDA, "title": "Cashin", "operationType": "source"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			route, err := newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger,
					models.NewCreateOperationRouteInput("Cashin", "cash-in route", "source"))
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"id": route.ID.String(), "title": route.Title, "operationType": route.OperationType,
			}, nil
		},
	},
	{
		name:       "V2.OperationRoutes.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"title":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"` + v2UUIDA + `","title":"Renamed","operationType":"source"}`,
		wantFields: map[string]string{"id": v2UUIDA, "title": "Renamed", "operationType": "source"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			route, err := newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA,
					models.NewUpdateOperationRouteInput().WithTitle("Renamed"))
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"id": route.ID.String(), "title": route.Title, "operationType": route.OperationType,
			}, nil
		},
	},
	{
		name:       "V2.OperationRoutes.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newOperationRoutesV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.TransactionRoutes.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"title":"Settlement"`, v2UUIDB},
		status:     http.StatusCreated,
		response:   `{"id":"` + v2UUIDA + `","title":"Settlement","description":"settlement route"}`,
		wantFields: map[string]string{"id": v2UUIDA, "title": "Settlement", "description": "settlement route"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			route, err := newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), v2Org, v2Ledger,
					models.NewCreateTransactionRouteInput("Settlement", "settlement route", []string{v2UUIDB}))
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"id": route.ID.String(), "title": route.Title, "description": route.Description,
			}, nil
		},
	},
	{
		name:       "V2.TransactionRoutes.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"title":"Renamed"`},
		status:     http.StatusOK,
		response:   `{"id":"` + v2UUIDA + `","title":"Renamed","description":"settlement route"}`,
		wantFields: map[string]string{"id": v2UUIDA, "title": "Renamed", "description": "settlement route"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			route, err := newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA,
					models.NewUpdateTransactionRouteInput().WithTitle("Renamed"))
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"id": route.ID.String(), "title": route.Title, "description": route.Description,
			}, nil
		},
	},
	{
		name:       "V2.TransactionRoutes.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newTransactionRoutesV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.Balances.CreateBalance",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"key":"asset-freeze"`},
		status:     http.StatusCreated,
		response:   `{"id":"bal-1","key":"asset-freeze","assetCode":"USD","available":"0"}`,
		wantFields: map[string]string{"id": "bal-1", "key": "asset-freeze", "available": "0"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			bal, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				CreateBalance(context.Background(), v2Org, v2Ledger, v2Account,
					&models.CreateBalanceInput{Key: "asset-freeze"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": bal.ID, "key": bal.Key, "available": bal.Available.String()}, nil
		},
	},
	{
		name:       "V2.Balances.UpdateBalance",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"allowSending":false`},
		status:     http.StatusOK,
		response:   `{"id":"bal-1","assetCode":"USD","available":"12.25","allowSending":false}`,
		wantFields: map[string]string{"id": "bal-1", "available": "12.25", "allowSending": "false"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			allow := false

			bal, err := newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				UpdateBalance(context.Background(), v2Org, v2Ledger, v2UUIDA,
					&models.UpdateBalanceInput{AllowSending: &allow})
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"id": bal.ID, "available": bal.Available.String(), "allowSending": boolText(bal.AllowSending),
			}, nil
		},
	},
	{
		name:       "V2.Balances.DeleteBalance",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newBalancesV2Facade(newTestLedgerClient(t, srv), true).
				DeleteBalance(context.Background(), v2Org, v2Ledger, v2UUIDA)
		},
	},
	{
		name:       "V2.MetadataIndexes.Create",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"metadataKey":"customer_id"`, `"unique":true`},
		status:     http.StatusCreated,
		response:   `{"indexName":"idx_customer_id","entityName":"account","metadataKey":"customer_id","unique":true}`,
		wantFields: map[string]string{"indexName": "idx_customer_id", "metadataKey": "customer_id", "unique": "true"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			index, err := newMetadataIndexesV2Facade(newTestLedgerClient(t, srv), true).
				Create(context.Background(), "account",
					models.NewCreateMetadataIndexInput("customer_id").WithUnique(true))
			if err != nil {
				return nil, err
			}

			return map[string]string{
				"indexName": index.IndexName, "metadataKey": index.MetadataKey, "unique": boolText(index.Unique),
			}, nil
		},
	},
	{
		name:       "V2.MetadataIndexes.Delete",
		wantMethod: http.MethodDelete,
		status:     http.StatusNoContent,
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return nil, newMetadataIndexesV2Facade(newTestLedgerClient(t, srv), true).
				Delete(context.Background(), "account", "customer_id")
		},
	},
	{
		name:       "V2.Operations.UpdateTransactionOperation",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"description":"updated"`},
		status:     http.StatusOK,
		response:   `{"id":"op-1","description":"updated","amount":{"value":"25"}}`,
		wantFields: map[string]string{"id": "op-1", "description": "updated"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			op, err := newOperationsV2Facade(newTestLedgerClient(t, srv), true).
				UpdateTransactionOperation(context.Background(), v2Org, v2Ledger, v2UUIDA, v2UUIDB,
					&models.UpdateOperationInput{Description: "updated"})
			if err != nil {
				return nil, err
			}

			return map[string]string{"id": op.ID, "description": op.Description}, nil
		},
	},
	{
		name:       "V2.Transactions.CreateHold",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"asset":"USD"`, `"@src"`, `"@dst"`},
		status:     http.StatusCreated,
		response:   `{"id":"tx-hold","amount":"100","assetCode":"USD","status":{"code":"PENDING"}}`,
		wantFields: map[string]string{"id": "tx-hold", "amount": "100", "status": "PENDING"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return v2TransactionFields(newTestTransactionsV2Facade(t, srv).
				CreateHold(context.Background(), v2Org, v2Ledger, sampleV2Input()))
		},
	},
	{
		name:       "V2.Transactions.CreateBlock",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"asset":"USD"`, `"@src"`},
		status:     http.StatusCreated,
		response:   `{"id":"tx-block","amount":"100","assetCode":"USD","status":{"code":"APPROVED"}}`,
		wantFields: map[string]string{"id": "tx-block", "amount": "100", "status": "APPROVED"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return v2TransactionFields(newTestTransactionsV2Facade(t, srv).
				CreateBlock(context.Background(), v2Org, v2Ledger, sampleV2Input()))
		},
	},
	{
		name:       "V2.Transactions.CreateUnblock",
		wantMethod: http.MethodPost,
		wantBody:   []string{`"asset":"USD"`, `"@dst"`},
		status:     http.StatusCreated,
		response:   `{"id":"tx-unblock","amount":"100","assetCode":"USD","status":{"code":"APPROVED"}}`,
		wantFields: map[string]string{"id": "tx-unblock", "amount": "100", "status": "APPROVED"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return v2TransactionFields(newTestTransactionsV2Facade(t, srv).
				CreateUnblock(context.Background(), v2Org, v2Ledger, sampleV2Input()))
		},
	},
	{
		name:       "V2.Transactions.Update",
		wantMethod: http.MethodPatch,
		wantBody:   []string{`"description":"corrected"`},
		status:     http.StatusOK,
		response:   `{"id":"tx-1","amount":"100","assetCode":"USD","description":"corrected","status":{"code":"APPROVED"}}`,
		wantFields: map[string]string{"id": "tx-1", "amount": "100", "status": "APPROVED"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return v2TransactionFields(newTestTransactionsV2Facade(t, srv).
				Update(context.Background(), v2Org, v2Ledger, v2UUIDA,
					&models.UpdateTransactionV2Input{Description: "corrected"}))
		},
	},
	{
		name:       "V2.Transactions.Commit",
		wantMethod: http.MethodPost,
		status:     http.StatusCreated,
		response:   `{"id":"tx-1","amount":"100","assetCode":"USD","status":{"code":"APPROVED"}}`,
		wantFields: map[string]string{"id": "tx-1", "amount": "100", "status": "APPROVED"},
		fire: func(t *testing.T, srv *httptest.Server) (map[string]string, error) {
			t.Helper()

			return v2TransactionFields(newTestTransactionsV2Facade(t, srv).
				Commit(context.Background(), v2Org, v2Ledger, v2UUIDA))
		},
	},
}

// TestV2WriteSendsWhatTheCallerDescribed pins the request half: the method the
// ledger routes on, and the caller's own values inside the body that left.
//
// A write that reached the right URL with a body missing a field creates a
// resource the caller did not describe, and the 201 that comes back looks
// exactly like success.
func TestV2WriteSendsWhatTheCallerDescribed(t *testing.T) {
	for _, write := range v2Writes {
		t.Run(write.name, func(t *testing.T) {
			var (
				method string
				body   string
			)

			srv := writeCapturingServer(t, &method, &body, write.status, write.response)

			if _, err := write.fire(t, srv); err != nil {
				t.Fatalf("write: %v", err)
			}

			if method != write.wantMethod {
				t.Fatalf("method = %s, want %s", method, write.wantMethod)
			}

			for _, fragment := range write.wantBody {
				if !strings.Contains(body, fragment) {
					t.Fatalf("body = %q, want it to carry %s — the caller's value never left", body, fragment)
				}
			}
		})
	}
}

// TestV2WriteDecodesTheResourceTheServerReturned pins the response half.
//
// This is the failure that costs the most on a money path: a create whose 201
// body the facade dropped returns a zero-valued resource with a nil error, so
// the caller records an empty id against a transfer that really happened.
func TestV2WriteDecodesTheResourceTheServerReturned(t *testing.T) {
	for _, write := range v2Writes {
		if len(write.wantFields) == 0 {
			continue // delete: nothing comes back to decode.
		}

		t.Run(write.name, func(t *testing.T) {
			var method, body string

			srv := writeCapturingServer(t, &method, &body, write.status, write.response)

			got, err := write.fire(t, srv)
			if err != nil {
				t.Fatalf("write: %v", err)
			}

			for field, want := range write.wantFields {
				if got[field] != want {
					t.Fatalf("%s = %q, want %q — the server's answer did not reach the caller",
						field, got[field], want)
				}
			}
		})
	}
}

// TestV2WriteStampsIdempotencyOnTheWire is the replay guard, asserted where it
// is actually observable: the header on the request.
//
// The gate is what a caller configures; the header is what protects them. With
// the gate on, a create the network retried must not settle twice — so the
// header has to be present and stable within the call. With the gate off, a key
// the caller supplied explicitly still has to win, because they asked for it.
func TestV2WriteStampsIdempotencyOnTheWire(t *testing.T) {
	t.Run("auto key with the gate on", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		_, _ = newAccountsV2Facade(newTestLedgerClient(t, srv), true).
			Create(context.Background(), v2Org, v2Ledger,
				&models.CreateAccountInput{Name: "Checking", AssetCode: "USD", Type: "deposit"})

		if key() == "" {
			t.Fatal("gate on: no X-Idempotency reached the server, so a retried create can settle twice")
		}
	})

	t.Run("explicit key wins with the gate off", func(t *testing.T) {
		srv, key := idempotencyCaptureServer()
		defer srv.Close()

		ctx := sdkctx.WithIdempotencyKey(context.Background(), "caller-key")

		_, _ = newAccountsV2Facade(newTestLedgerClient(t, srv), false).
			Create(ctx, v2Org, v2Ledger,
				&models.CreateAccountInput{Name: "Checking", AssetCode: "USD", Type: "deposit"})

		if got := key(); got != "caller-key" {
			t.Fatalf("X-Idempotency = %q, want caller-key — the caller's own key was discarded", got)
		}
	})
}

// writeCapturingServer records the method and body of every request and answers
// each one with status and body.
func writeCapturingServer(t *testing.T, method, body *string, status int, response string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*method = r.Method

		raw, _ := io.ReadAll(r.Body)
		*body = string(raw)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(srv.Close)

	return srv
}

// v2TransactionFields collapses the read/err/extract dance the five transaction
// rows repeat, and guards the nil dereference a failed write would cause.
//
// A nil transaction with a nil error is returned as an EMPTY field set rather
// than swallowed, because that pair is itself the defect these rows exist to
// catch: the caller's assertions then fail naming the field that never arrived,
// instead of the test panicking on a dereference and hiding which one it was.
func v2TransactionFields(tx *models.TransactionV2, err error) (map[string]string, error) {
	if err != nil {
		return nil, err
	}

	if tx == nil {
		return map[string]string{}, nil
	}

	return map[string]string{"id": tx.ID, "amount": tx.Amount, "status": tx.Status.Code}, nil
}

// boolText renders a bool the way the wantFields map spells it.
func boolText(v bool) string {
	if v {
		return "true"
	}

	return "false"
}

// derefString reads an optional string field without panicking when the server
// omitted it — which is itself the failure the assertion is looking for.
func derefString(v *string) string {
	if v == nil {
		return ""
	}

	return *v
}
