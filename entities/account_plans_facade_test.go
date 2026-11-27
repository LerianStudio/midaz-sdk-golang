// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"io"
	"iter"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/sdkctx"
)

// Identifiers of the captured exchanges below.
const (
	apOrgID     = "01a21da6-a657-7c9a-a4e5-1d488031102e"
	apLedgerID  = "01a21da6-a670-7033-ae97-a38bd670f3b8"
	apPlanID    = "01a21da6-a6fa-7044-9058-c1f644284b5a"
	apAccountID = "01a21da6-a6ce-781f-b7d2-ad9e3b66a550"
	apLedgerURL = "/v2/organizations/" + apOrgID + "/ledgers/" + apLedgerID
	apPlansURL  = apLedgerURL + "/account-plans"
	apPlanURL   = apPlansURL + "/" + apPlanID
	apAccPlnURL = apLedgerURL + "/accounts/" + apAccountID + "/account-plan"
)

// The literals below are request and response bodies captured from a local
// Midaz 4.6.0 ledger, in compact form with the server's key order. Tests
// compare against them; none is produced by the SDK's own marshal.
const (
	// POST account-plans, 201
	apCreateRoundingRequest  = `{"label":"plano-1795793790","enable":true,"rounding":{"scale":2,"mode":"halfUp"},"fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","direction":"received","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"creditAccount":"@fees-1795793790","freeQuota":2},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","direction":"sent","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"creditAccount":"@fees-1795793790"}}}`
	apCreateRoundingResponse = `{"id":"01a21da6-a6fa-7044-9058-c1f644284b5a","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"plano-1795793790","enable":true,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:36:30.714Z","updatedAt":"2026-11-27T15:36:30.714Z","version":1,"effectiveFrom":"2026-11-27T15:36:30.714Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":2,"mode":"halfUp"}}`
	// POST account-plans with enable false and no rounding, 201
	apCreateDisabledRequest  = `{"label":"D-1795794176","enable":false,"fees":{"a":{"feeLabel":"f","trigger":{"type":"transaction","direction":"received","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"creditAccount":"@fees-1795793790"},"b":{"feeLabel":"f","trigger":{"type":"transaction","direction":"sent","schemes":["PIX"]},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"creditAccount":"@fees-1795793790"}}}`
	apCreateDisabledResponse = `{"id":"01a21dac-8ae1-714f-93cd-3aeaf14c3850","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"D-1795794176","enable":false,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:42:56.737Z","updatedAt":"2026-11-27T15:42:56.737Z","version":1,"effectiveFrom":"2026-11-27T15:42:56.737Z","fees":{"a":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false},"b":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}}}`
	// GET account-plans/{id}, 200
	apGetResponse = `{"id":"01a21da6-a6fa-7044-9058-c1f644284b5a","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"plano-1795793790","enable":true,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:36:30.714Z","updatedAt":"2026-11-27T15:36:30.714Z","version":1,"effectiveFrom":"2026-11-27T15:36:30.714Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":2,"mode":"halfUp"}}`
	// GET account-plans?enable=false&limit=1&page=1, 200
	apListResponse = `{"items":[{"id":"01a21dac-8ae1-714f-93cd-3aeaf14c3850","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"D-1795794176","enable":false,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:42:56.737Z","updatedAt":"2026-11-27T15:42:56.737Z"}],"page":1,"limit":1,"total":1}`
	// GET account-plans/{id}/versions, 200, with rounding removed from the second item so rounding is present only on the first
	apVersionsResponse = `{"items":[{"accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","version":1,"effectiveFrom":"2026-11-27T15:36:30.714Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":2,"mode":"halfUp"},"createdAt":"2026-11-27T15:36:30.714Z"},{"accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","version":2,"effectiveFrom":"2026-11-27T15:36:30.749Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":2,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"createdAt":"2026-11-27T15:36:30.749Z"}],"page":1,"limit":10,"total":2}`
	// GET account-plans/{id}/accounts, 200; no total
	apAccountsResponse = `{"items":[{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":null}],"limit":10,"page":1}`
	// PUT accounts/{id}/account-plan, 200
	apAssignRequest  = `{"accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a"}`
	apAssignResponse = `{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":null}`
	// GET accounts/{id}/account-plan with an active plan, 200
	apAssignmentsActiveResponse = `{"active":{"id":"01a21dac-8b76-7978-b2af-ac3107bf3c46","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","startedAt":"2026-11-27T15:42:56.886594Z","endedAt":null},"history":[{"id":"01a21dac-8b76-7978-b2af-ac3107bf3c46","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","startedAt":"2026-11-27T15:42:56.886594Z","endedAt":null},{"id":"01a21dac-8b63-77e1-a2e4-a419e5a79ded","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8a87-70fa-a7c8-f02fd0563314","startedAt":"2026-11-27T15:42:56.86749Z","endedAt":"2026-11-27T15:42:56.886594Z"},{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":"2026-11-27T15:36:30.832799Z"}]}`
	// GET accounts/{id}/account-plan after unassign, 200
	apAssignmentsNoActiveResponse = `{"active":null,"history":[{"id":"01a21dac-8b76-7978-b2af-ac3107bf3c46","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","startedAt":"2026-11-27T15:42:56.886594Z","endedAt":"2026-11-27T15:42:56.916827Z"},{"id":"01a21dac-8b63-77e1-a2e4-a419e5a79ded","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21dac-8a87-70fa-a7c8-f02fd0563314","startedAt":"2026-11-27T15:42:56.86749Z","endedAt":"2026-11-27T15:42:56.886594Z"},{"id":"01a21da6-a748-796a-8591-5ba05e7212ea","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountAlias":"@cust-1795793790","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":"2026-11-27T15:36:30.832799Z"}]}`
	// GET accounts/{id}/account-plan/usage, 200
	apUsageResponse = `{"accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountPlanId":"01a21da6-a6fa-7044-9058-c1f644284b5a","accountPlanVersion":2,"period":{"label":"2026-11","start":"2026-11-01T03:00:00Z","end":"2026-12-01T03:00:00Z"},"items":[{"feeKey":"pixIn","trigger":"fee:pixIn:*","period":{"label":"2026-11","start":"2026-11-01T03:00:00Z","end":"2026-12-01T03:00:00Z"},"freeQuota":2,"used":0,"remaining":2,"charged":0},{"feeKey":"pixOut","trigger":"fee:pixOut:*","period":{"label":"2026-11","start":"2026-11-01T03:00:00Z","end":"2026-12-01T03:00:00Z"},"freeQuota":0,"used":0,"remaining":0,"charged":0}]}`
	// GET accounts/{id}/account-plan/usage?period=2026-10, 200
	apUsagePeriodResponse = `{"accountId":"01a21da6-a6ce-781f-b7d2-ad9e3b66a550","accountPlanId":"01a21dac-8ac6-7491-8690-6685df647f59","accountPlanVersion":1,"period":{"label":"2026-10","start":"2026-10-01T03:00:00Z","end":"2026-11-01T03:00:00Z"},"items":[{"feeKey":"a","trigger":"fee:a:*","period":{"label":"2026-10","start":"2026-10-01T03:00:00Z","end":"2026-11-01T03:00:00Z"},"freeQuota":0,"used":0,"remaining":0,"charged":0},{"feeKey":"b","trigger":"fee:b:*","period":{"label":"2026-10","start":"2026-10-01T03:00:00Z","end":"2026-11-01T03:00:00Z"},"freeQuota":0,"used":0,"remaining":0,"charged":0}]}`
	// PATCH description only, 200 with the same version
	apUpdateDescriptionRequest  = `{"description":"x"}`
	apUpdateDescriptionResponse = `{"id":"01a21dac-8a87-70fa-a7c8-f02fd0563314","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"A-1795794176","description":"x","enable":true,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:42:56.647Z","updatedAt":"2026-11-27T15:42:56.765Z","version":1,"effectiveFrom":"2026-11-27T15:42:56.647Z","fees":{"a":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false},"b":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}}}`
	// PATCH fields of one fee, 200 with the next version
	apUpdateFeePatchRequest  = `{"fees":{"pixOut":{"freeQuota":5,"deferrable":true}}}`
	apUpdateFeePatchResponse = `{"id":"01a21de1-be10-7a0e-85fa-ff9b93e4db48","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"FP-1795797663","enable":true,"timezone":"America/Sao_Paulo","currentVersion":2,"createdAt":"2026-11-27T16:41:03.248Z","updatedAt":"2026-11-27T16:41:03.258Z","version":2,"effectiveFrom":"2026-11-27T16:41:03.258Z","fees":{"pixIn":{"feeLabel":"Pix recebido","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false},"pixOut":{"feeLabel":"Pix enviado","trigger":{"type":"transaction","schemes":["PIX"],"direction":"sent"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":5,"period":"monthly","deferrable":true}}}`
	// PATCH removing a fee with null, 200 with the next version
	apUpdateRemoveFeeRequest  = `{"fees":{"b":null}}`
	apUpdateRemoveFeeResponse = `{"id":"01a21dac-8a87-70fa-a7c8-f02fd0563314","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"A-1795794176","description":"x","enable":true,"timezone":"America/Sao_Paulo","currentVersion":2,"createdAt":"2026-11-27T15:42:56.647Z","updatedAt":"2026-11-27T15:42:56.785Z","version":2,"effectiveFrom":"2026-11-27T15:42:56.785Z","fees":{"a":{"feeLabel":"f","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.00"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}}}`
	// PATCH replacing rounding, 200 with the next version
	apUpdateRoundingRequest  = `{"rounding":{"scale":2,"mode":"floor"}}`
	apUpdateRoundingResponse = `{"id":"01a21dc9-d1dd-7984-8308-fac47e5eb539","organizationId":"01a21da6-a657-7c9a-a4e5-1d488031102e","ledgerId":"01a21da6-a670-7033-ae97-a38bd670f3b8","label":"R-1795796095","enable":true,"timezone":"America/Sao_Paulo","currentVersion":2,"createdAt":"2026-11-27T16:14:55.453Z","updatedAt":"2026-11-27T16:14:55.463Z","version":2,"effectiveFrom":"2026-11-27T16:14:55.463Z","fees":{"pixIn":{"feeLabel":"Pix received","trigger":{"type":"transaction","schemes":["PIX"],"direction":"received"},"calculationModel":{"applicationRule":"flatFee","calculations":[{"type":"flat","value":"1.239"}]},"referenceAmount":"originalAmount","creditAccount":"@fees-1795793790","freeQuota":0,"period":"monthly","deferrable":false}},"rounding":{"scale":2,"mode":"floor"}}`
	// 409/0543
	apErrDuplicateLabel = `{"type":"https://errors.lerian.studio/v1/0543","title":"Account Plan Label Duplicated Error","status":409,"detail":"An account plan with this label already exists in the ledger. Please choose a different label and try again.","code":"0543","entityType":"AccountPlan"}`
	// 400/0053
	apErrUnknownField = `{"type":"https://errors.lerian.studio/v1/0053","title":"Unexpected Fields in the Request","status":400,"detail":"The request body contains more fields than expected. Please send only the allowed fields as per the documentation. The unexpected fields are listed in the fields object.","errors":[{"message":"unexpected field","location":"foo","value":1}],"code":"0053"}`
	// 422/0553
	apErrRetroactive = `{"type":"https://errors.lerian.studio/v1/0553","title":"Account Plan Effective From Retroactive Error","status":422,"detail":"The effectiveFrom of the new account plan version precedes the version currently in effect. Please set an effectiveFrom at or after the current version and try again.","code":"0553","entityType":"AccountPlan"}`
	// 422/0556
	apErrSchemesRequired = `{"type":"https://errors.lerian.studio/v1/0556","title":"Account Plan Schemes Required Error","status":422,"detail":"The ledger does not validate accounting routes, so a transaction fee of the account plan must declare at least one scheme or one metadata pair. Please set the schemes or the metadata of the fee trigger and try again.","code":"0556"}`
	// 404/0007
	apErrPlanNotFound = `{"type":"https://errors.lerian.studio/v1/0007","title":"Entity Not Found","status":404,"detail":"No entity was found for the given ID. Please make sure to use the correct ID for the entity you are trying to manage.","code":"0007","entityType":"AccountPlan"}`
	// 409/0546
	apErrActiveAssignments = `{"type":"https://errors.lerian.studio/v1/0546","title":"Account Plan Has Active Assignments Error","status":409,"detail":"The account plan is still assigned to at least one account and cannot be deleted. Please unassign it from every account and try again.","code":"0546","entityType":"AccountPlan"}`
	// 404/0554
	apErrNoAssignment = `{"type":"https://errors.lerian.studio/v1/0554","title":"Account Plan Assignment Not Found Error","status":404,"detail":"The account has no active account plan assignment. Please assign an account plan to the account and try again.","code":"0554","entityType":"AccountPlanAssignment"}`
)

// apRequest is one request the test server received.
type apRequest struct {
	method string
	path   string
	query  url.Values
	raw    string
	body   string
	header http.Header
}

// apServer answers every request with one status and body and records what it
// received. It is safe for the concurrent handler goroutines httptest runs.
type apServer struct {
	mu       sync.Mutex
	requests []apRequest
	respond  func(r *http.Request) (int, string)
}

func newAPServer(t *testing.T, status int, body string) (*apServer, *httptest.Server) {
	t.Helper()

	return newAPServerFunc(t, func(*http.Request) (int, string) { return status, body })
}

func newAPServerFunc(t *testing.T, respond func(r *http.Request) (int, string)) (*apServer, *httptest.Server) {
	t.Helper()

	rec := &apServer{respond: respond}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)

		rec.mu.Lock()
		rec.requests = append(rec.requests, apRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.Query(),
			raw:    r.URL.RawQuery,
			body:   string(payload),
			header: r.Header.Clone(),
		})
		rec.mu.Unlock()

		status, body := rec.respond(r)
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}

		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return rec, srv
}

func (s *apServer) all() []apRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]apRequest(nil), s.requests...)
}

func (s *apServer) only(t *testing.T) apRequest {
	t.Helper()

	got := s.all()
	require.Len(t, got, 1, "want exactly one request")

	return got[0]
}

func newTestAccountPlansFacade(t *testing.T, srv *httptest.Server) *accountPlansFacade {
	t.Helper()

	return newAccountPlansFacade(newTestLedgerClient(t, srv))
}

func apCreateRoundingInput() *models.CreateAccountPlanInput {
	calculation := models.FeeCalculationModel{
		ApplicationRule: "flatFee",
		Calculations:    []models.Calculation{{Type: "flat", Value: "1.239"}},
	}

	return models.NewCreateAccountPlanInput("plano-1795793790", true, map[string]models.AccountPlanFeeInput{
		"pixIn": {
			FeeLabel: "Pix recebido",
			Trigger: models.AccountPlanTriggerInput{
				Type:      models.AccountPlanTriggerTypeTransaction,
				Direction: models.AccountPlanTriggerDirectionReceived,
				Schemes:   []string{"PIX"},
			},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
			FreeQuota:        2,
		},
		"pixOut": {
			FeeLabel: "Pix enviado",
			Trigger: models.AccountPlanTriggerInput{
				Type:      models.AccountPlanTriggerTypeTransaction,
				Direction: models.AccountPlanTriggerDirectionSent,
				Schemes:   []string{"PIX"},
			},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
		},
	}).WithRounding(models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeHalfUp})
}

func apCreateDisabledInput() *models.CreateAccountPlanInput {
	calculation := models.FeeCalculationModel{
		ApplicationRule: "flatFee",
		Calculations:    []models.Calculation{{Type: "flat", Value: "1.00"}},
	}
	fee := func(direction models.AccountPlanTriggerDirection) models.AccountPlanFeeInput {
		return models.AccountPlanFeeInput{
			FeeLabel: "f",
			Trigger: models.AccountPlanTriggerInput{
				Type:      models.AccountPlanTriggerTypeTransaction,
				Direction: direction,
				Schemes:   []string{"PIX"},
			},
			CalculationModel: calculation,
			CreditAccount:    "@fees-1795793790",
		}
	}

	return models.NewCreateAccountPlanInput("D-1795794176", false, map[string]models.AccountPlanFeeInput{
		"a": fee(models.AccountPlanTriggerDirectionReceived),
		"b": fee(models.AccountPlanTriggerDirectionSent),
	})
}

func TestAccountPlansFacade_Get(t *testing.T) {
	t.Run("leitura-de-plano", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apGetResponse)

		plan, err := newTestAccountPlansFacade(t, srv).Get(context.Background(), apOrgID, apLedgerID, apPlanID)
		require.NoError(t, err)

		req := rec.only(t)
		assert.Equal(t, http.MethodGet, req.method)
		assert.Equal(t, apPlanURL, req.path)
		assert.Empty(t, req.raw)

		assert.Equal(t, apPlanID, plan.ID)
		assert.Equal(t, "plano-1795793790", plan.Label)
		assert.True(t, plan.Enable)
		assert.Equal(t, 1, plan.Version)
		assert.Equal(t, "2026-11-27T15:36:30.714Z", plan.EffectiveFrom.Format("2006-01-02T15:04:05.000Z"))
		require.NotNil(t, plan.Rounding)
		assert.Equal(t, models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeHalfUp}, *plan.Rounding)
		require.Len(t, plan.Fees, 2)
		assert.Equal(t, models.AccountPlanTriggerDirectionReceived, plan.Fees["pixIn"].Trigger.Direction)
		assert.Equal(t, 2, plan.Fees["pixIn"].FreeQuota)
	})

	t.Run("plano-sem-rounding", func(t *testing.T) {
		_, srv := newAPServer(t, http.StatusOK, apCreateDisabledResponse)

		plan, err := newTestAccountPlansFacade(t, srv).Get(context.Background(), apOrgID, apLedgerID, apPlanID)
		require.NoError(t, err)

		assert.Nil(t, plan.Rounding, "a response without rounding must decode as absent, not zero")
		assert.False(t, plan.Enable)
	})
}

func TestAccountPlansFacade_List(t *testing.T) {
	t.Run("listagem-de-planos", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apListResponse)
		disabled := false

		page, err := newTestAccountPlansFacade(t, srv).List(context.Background(), apOrgID, apLedgerID, models.AccountPlansListOpts{
			PageListOpts: models.PageListOpts{Limit: 5, Page: 2},
			Filters:      models.AccountPlansFilters{Enable: &disabled},
		})
		require.NoError(t, err)

		req := rec.only(t)
		assert.Equal(t, http.MethodGet, req.method)
		assert.Equal(t, apPlansURL, req.path)
		assert.Equal(t, url.Values{"limit": {"5"}, "page": {"2"}, "enable": {"false"}}, req.query)

		require.Len(t, page.Items, 1)
		assert.Equal(t, "01a21dac-8ae1-714f-93cd-3aeaf14c3850", page.Items[0].ID)
		assert.False(t, page.Items[0].Enable)
		assert.Equal(t, 1, page.Pagination.Page)
		assert.Equal(t, 1, page.Pagination.Limit)
		assert.Equal(t, 1, page.Pagination.Total)
	})

	t.Run("listagem-sem-opcoes", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apListResponse)

		_, err := newTestAccountPlansFacade(t, srv).List(context.Background(), apOrgID, apLedgerID, models.AccountPlansListOpts{})
		require.NoError(t, err)

		assert.Empty(t, rec.only(t).raw, "no options must send no query string")
	})

	t.Run("undocumented options are not sent", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apListResponse)

		_, err := newTestAccountPlansFacade(t, srv).List(context.Background(), apOrgID, apLedgerID, models.AccountPlansListOpts{
			PageListOpts: models.PageListOpts{SortDirection: models.SortDescending, StartDate: "2026-01-01", EndDate: "2026-01-31"},
		})
		require.NoError(t, err)

		assert.Empty(t, rec.only(t).raw)
	})
}

func TestAccountPlansFacade_ListVersions(t *testing.T) {
	rec, srv := newAPServer(t, http.StatusOK, apVersionsResponse)

	page, err := newTestAccountPlansFacade(t, srv).ListVersions(context.Background(), apOrgID, apLedgerID, apPlanID, models.AccountPlanVersionsListOpts{})
	require.NoError(t, err)

	req := rec.only(t)
	assert.Equal(t, http.MethodGet, req.method)
	assert.Equal(t, apPlanURL+"/versions", req.path)
	assert.Empty(t, req.raw)

	require.Len(t, page.Items, 2)
	assert.Equal(t, 1, page.Items[0].Version)
	require.NotNil(t, page.Items[0].Rounding)
	assert.Equal(t, models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeHalfUp}, *page.Items[0].Rounding)
	assert.Equal(t, 2, page.Items[1].Version)
	assert.Nil(t, page.Items[1].Rounding)
	assert.Equal(t, 2, page.Pagination.Total)
}

func TestAccountPlansFacade_ListAccounts(t *testing.T) {
	rec, srv := newAPServer(t, http.StatusOK, apAccountsResponse)

	page, err := newTestAccountPlansFacade(t, srv).ListAccounts(context.Background(), apOrgID, apLedgerID, apPlanID, models.AccountPlanAccountsListOpts{
		PageListOpts: models.PageListOpts{SortDirection: models.SortDescending, StartDate: "2026-01-01"},
	})
	require.NoError(t, err)

	req := rec.only(t)
	assert.Equal(t, http.MethodGet, req.method)
	assert.Equal(t, apPlanURL+"/accounts", req.path)
	assert.Equal(t, url.Values{"sort_order": {"desc"}}, req.query)

	require.Len(t, page.Items, 1)
	assert.Equal(t, models.AccountPlanAssignment{
		ID:             "01a21da6-a748-796a-8591-5ba05e7212ea",
		OrganizationID: apOrgID,
		LedgerID:       apLedgerID,
		AccountID:      apAccountID,
		AccountAlias:   "@cust-1795793790",
		AccountPlanID:  apPlanID,
		StartedAt:      page.Items[0].StartedAt,
	}, page.Items[0])
	assert.Equal(t, "2026-11-27T15:36:30.792581Z", page.Items[0].StartedAt.Format("2006-01-02T15:04:05.999999Z"))
	assert.Equal(t, 10, page.Pagination.Limit)
	assert.Equal(t, 0, page.Pagination.Total, "the plan accounts listing carries no total")
}

func TestAccountPlansFacade_GetAssignment(t *testing.T) {
	t.Run("atribuicao-ativa", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apAssignmentsActiveResponse)

		got, err := newTestAccountPlansFacade(t, srv).GetAssignment(context.Background(), apOrgID, apLedgerID, apAccountID)
		require.NoError(t, err)

		req := rec.only(t)
		assert.Equal(t, http.MethodGet, req.method)
		assert.Equal(t, apAccPlnURL, req.path)

		require.NotNil(t, got.Active)
		assert.Equal(t, "01a21dac-8ac6-7491-8690-6685df647f59", got.Active.AccountPlanID)
		assert.Nil(t, got.Active.EndedAt)
		require.Len(t, got.History, 3)
		assert.NotNil(t, got.History[1].EndedAt)
	})

	t.Run("conta-sem-plano-ativo", func(t *testing.T) {
		_, srv := newAPServer(t, http.StatusOK, apAssignmentsNoActiveResponse)

		got, err := newTestAccountPlansFacade(t, srv).GetAssignment(context.Background(), apOrgID, apLedgerID, apAccountID)
		require.NoError(t, err)

		assert.Nil(t, got.Active)
		require.Len(t, got.History, 3)

		for _, entry := range got.History {
			assert.NotNil(t, entry.EndedAt)
		}
	})
}

func TestAccountPlansFacade_GetUsage(t *testing.T) {
	t.Run("uso-do-periodo-corrente", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apUsageResponse)

		usage, err := newTestAccountPlansFacade(t, srv).GetUsage(context.Background(), apOrgID, apLedgerID, apAccountID, models.AccountPlanUsageOpts{})
		require.NoError(t, err)

		req := rec.only(t)
		assert.Equal(t, http.MethodGet, req.method)
		assert.Equal(t, apAccPlnURL+"/usage", req.path)
		assert.Empty(t, req.raw)

		assert.Equal(t, apPlanID, usage.AccountPlanID)
		assert.Equal(t, 2, usage.AccountPlanVersion)
		assert.Equal(t, "2026-11", usage.Period.Label)
		require.Len(t, usage.Items, 2)
		assert.Equal(t, models.AccountPlanFeeUsage{
			FeeKey: "pixIn", Trigger: "fee:pixIn:*", Period: usage.Period, FreeQuota: 2, Remaining: 2,
		}, usage.Items[0])
	})

	t.Run("uso-de-periodo-informado", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusOK, apUsagePeriodResponse)

		usage, err := newTestAccountPlansFacade(t, srv).GetUsage(context.Background(), apOrgID, apLedgerID, apAccountID, models.AccountPlanUsageOpts{Period: "2026-10"})
		require.NoError(t, err)

		assert.Equal(t, url.Values{"period": {"2026-10"}}, rec.only(t).query)
		assert.Equal(t, "2026-10", usage.Period.Label)
		assert.Equal(t, 1, usage.AccountPlanVersion)
	})
}

// apPagedListing is one of the three paginated listings, with a captured item
// and the iterators reduced to the item ids they yield.
type apPagedListing struct {
	name  string
	path  string
	ids   []string
	item  func(n int) string
	pages func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[int, error]
	all   func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[string, error]
}

func apPagedListings() []apPagedListing {
	return []apPagedListing{
		{
			name: "plans",
			path: apPlansURL,
			ids:  []string{"plan-1", "plan-2", "plan-3"},
			item: func(n int) string {
				return `{"id":"plan-` + strconv.Itoa(n) + `","organizationId":"` + apOrgID + `","ledgerId":"` + apLedgerID + `","label":"D-1795794176","enable":false,"timezone":"America/Sao_Paulo","currentVersion":1,"createdAt":"2026-11-27T15:42:56.737Z","updatedAt":"2026-11-27T15:42:56.737Z"}`
			},
			pages: func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[int, error] {
				return apPageSizes(f.Pages(ctx, apOrgID, apLedgerID, models.AccountPlansListOpts{PageListOpts: models.PageListOpts{Limit: limit}}))
			},
			all: func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[string, error] {
				return apItemIDs(f.All(ctx, apOrgID, apLedgerID, models.AccountPlansListOpts{PageListOpts: models.PageListOpts{Limit: limit}}),
					func(p models.AccountPlan) string { return p.ID })
			},
		},
		{
			name: "versions",
			path: apPlanURL + "/versions",
			ids:  []string{"1", "2", "3"},
			item: func(n int) string {
				return `{"accountPlanId":"` + apPlanID + `","version":` + strconv.Itoa(n) + `,"effectiveFrom":"2026-11-27T15:36:30.714Z","fees":{},"createdAt":"2026-11-27T15:36:30.714Z"}`
			},
			pages: func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[int, error] {
				return apPageSizes(f.VersionPages(ctx, apOrgID, apLedgerID, apPlanID, models.AccountPlanVersionsListOpts{PageListOpts: models.PageListOpts{Limit: limit}}))
			},
			all: func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[string, error] {
				return apItemIDs(f.AllVersions(ctx, apOrgID, apLedgerID, apPlanID, models.AccountPlanVersionsListOpts{PageListOpts: models.PageListOpts{Limit: limit}}),
					func(v models.AccountPlanVersion) string { return strconv.Itoa(v.Version) })
			},
		},
		{
			name: "plan accounts",
			path: apPlanURL + "/accounts",
			ids:  []string{"assignment-1", "assignment-2", "assignment-3"},
			item: func(n int) string {
				return `{"id":"assignment-` + strconv.Itoa(n) + `","organizationId":"` + apOrgID + `","ledgerId":"` + apLedgerID + `","accountId":"` + apAccountID + `","accountAlias":"@cust-1795793790","accountPlanId":"` + apPlanID + `","startedAt":"2026-11-27T15:36:30.792581Z","endedAt":null}`
			},
			pages: func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[int, error] {
				return apPageSizes(f.AccountPages(ctx, apOrgID, apLedgerID, apPlanID, models.AccountPlanAccountsListOpts{PageListOpts: models.PageListOpts{Limit: limit}}))
			},
			all: func(ctx context.Context, f *accountPlansFacade, limit int) iter.Seq2[string, error] {
				return apItemIDs(f.AllAccounts(ctx, apOrgID, apLedgerID, apPlanID, models.AccountPlanAccountsListOpts{PageListOpts: models.PageListOpts{Limit: limit}}),
					func(a models.AccountPlanAssignment) string { return a.ID })
			},
		},
	}
}

func apPageSizes[T any](pages iter.Seq2[*models.ListResponse[T], error]) iter.Seq2[int, error] {
	return func(yield func(int, error) bool) {
		for page, err := range pages {
			size := 0
			if page != nil {
				size = len(page.Items)
			}

			if !yield(size, err) {
				return
			}
		}
	}
}

func apItemIDs[T any](items iter.Seq2[T, error], id func(T) string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for item, err := range items {
			if !yield(id(item), err) {
				return
			}
		}
	}
}

// apPagesServer serves page 1 full and page 2 short, both with total equal to
// the items on the page, as the server does. Any other page fails the test.
func apPagesServer(t *testing.T, listing apPagedListing, limit int, page2Status int) (*apServer, *httptest.Server) {
	t.Helper()

	envelope := func(page int, items ...string) string {
		body := `{"items":[`
		for i, item := range items {
			if i > 0 {
				body += ","
			}

			body += item
		}

		n := strconv.Itoa(len(items))

		return body + `],"page":` + strconv.Itoa(page) + `,"limit":` + strconv.Itoa(limit) + `,"total":` + n + `}`
	}

	return newAPServerFunc(t, func(r *http.Request) (int, string) {
		if r.URL.Path != listing.path {
			t.Errorf("path = %q, want %q", r.URL.Path, listing.path)
		}

		switch r.URL.Query().Get("page") {
		case "1":
			return http.StatusOK, envelope(1, listing.item(1), listing.item(2))
		case "2":
			if page2Status != http.StatusOK {
				return page2Status, apErrPlanNotFound
			}

			return http.StatusOK, envelope(2, listing.item(3))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))

			return http.StatusTeapot, ""
		}
	})
}

func TestAccountPlansFacade_PagesStopOnShortPage(t *testing.T) {
	for _, listing := range apPagedListings() {
		t.Run(listing.name, func(t *testing.T) {
			rec, srv := apPagesServer(t, listing, 2, http.StatusOK)
			f := newTestAccountPlansFacade(t, srv)

			var sizes []int
			for size, err := range listing.pages(context.Background(), f, 2) {
				require.NoError(t, err)
				sizes = append(sizes, size)
			}

			assert.Equal(t, []int{2, 1}, sizes)
			require.Len(t, rec.all(), 2, "a full page then a short one is exactly two requests")
			assert.Equal(t, url.Values{"limit": {"2"}, "page": {"1"}}, rec.all()[0].query)
			assert.Equal(t, url.Values{"limit": {"2"}, "page": {"2"}}, rec.all()[1].query)
		})
	}
}

func TestAccountPlansFacade_AllYieldsEveryItemInOrder(t *testing.T) {
	for _, listing := range apPagedListings() {
		t.Run(listing.name, func(t *testing.T) {
			rec, srv := apPagesServer(t, listing, 2, http.StatusOK)

			var ids []string
			for id, err := range listing.all(context.Background(), newTestAccountPlansFacade(t, srv), 2) {
				require.NoError(t, err)
				ids = append(ids, id)
			}

			assert.Equal(t, listing.ids, ids)
			assert.Len(t, rec.all(), 2)
		})
	}
}

func TestAccountPlansFacade_AllStopsWhenTheConsumerBreaks(t *testing.T) {
	for _, listing := range apPagedListings() {
		t.Run(listing.name, func(t *testing.T) {
			rec, srv := apPagesServer(t, listing, 2, http.StatusOK)

			for _, err := range listing.all(context.Background(), newTestAccountPlansFacade(t, srv), 2) {
				require.NoError(t, err)

				break
			}

			assert.Len(t, rec.all(), 1, "breaking inside the first page must not request another")
		})
	}
}

func TestAccountPlansFacade_AllYieldsAMidIterationErrorOnce(t *testing.T) {
	for _, listing := range apPagedListings() {
		t.Run(listing.name, func(t *testing.T) {
			rec, srv := apPagesServer(t, listing, 2, http.StatusNotFound)

			var ids []string

			var errs []error

			for id, err := range listing.all(context.Background(), newTestAccountPlansFacade(t, srv), 2) {
				if err != nil {
					errs = append(errs, err)

					continue
				}

				ids = append(ids, id)
			}

			assert.Equal(t, listing.ids[:2], ids)
			require.Len(t, errs, 1)
			assert.Equal(t, http.StatusNotFound, sdkerrors.GetStatusCode(errs[0]))
			assert.Len(t, rec.all(), 2)
		})
	}
}

func TestAccountPlansFacade_PagesWithoutLimitFollowTheEchoedLimit(t *testing.T) {
	// With no limit the server applies and echoes its default (10); one item on
	// a page of 10 is a short page, so the iteration ends after one request.
	rec, srv := newAPServer(t, http.StatusOK, apAccountsResponse)

	var pages int
	for _, err := range newTestAccountPlansFacade(t, srv).AccountPages(context.Background(), apOrgID, apLedgerID, apPlanID, models.AccountPlanAccountsListOpts{}) {
		require.NoError(t, err)
		pages++
	}

	assert.Equal(t, 1, pages)
	assert.Equal(t, url.Values{"page": {"1"}}, rec.only(t).query)
}

func TestAccountPlansFacade_Create(t *testing.T) {
	t.Run("criacao-de-plano", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusCreated, apCreateRoundingResponse)

		plan, err := newTestAccountPlansFacade(t, srv).Create(context.Background(), apOrgID, apLedgerID, apCreateRoundingInput())
		require.NoError(t, err)

		req := rec.only(t)
		assert.Equal(t, http.MethodPost, req.method)
		assert.Equal(t, apPlansURL, req.path)
		assert.Equal(t, "application/json", req.header.Get("Content-Type"))
		assert.JSONEq(t, apCreateRoundingRequest, req.body)

		assert.Equal(t, apPlanID, plan.ID)
		assert.Equal(t, 1, plan.Version)
		assert.Equal(t, "America/Sao_Paulo", plan.Timezone)
		require.NotNil(t, plan.Rounding)
		assert.Equal(t, models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeHalfUp}, *plan.Rounding)
		assert.Equal(t, models.AccountPlanTriggerDirectionReceived, plan.Fees["pixIn"].Trigger.Direction)
		assert.Equal(t, models.AccountPlanTriggerDirectionSent, plan.Fees["pixOut"].Trigger.Direction)
	})

	t.Run("corpo-de-criacao-enviado", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusCreated, apCreateDisabledResponse)

		plan, err := newTestAccountPlansFacade(t, srv).Create(context.Background(), apOrgID, apLedgerID, apCreateDisabledInput())
		require.NoError(t, err)

		body := rec.only(t).body
		assert.JSONEq(t, apCreateDisabledRequest, body)
		assert.Contains(t, body, `"enable":false`)

		for _, absent := range []string{`"description"`, `"timezone"`, `"rounding"`} {
			assert.NotContains(t, body, absent)
		}

		assert.False(t, plan.Enable)
		assert.Nil(t, plan.Rounding)
	})
}

func TestAccountPlansFacade_Update(t *testing.T) {
	tests := []struct {
		name        string
		input       *models.UpdateAccountPlanInput
		wantBody    string
		response    string
		wantVersion int
		check       func(t *testing.T, plan *models.AccountPlanView)
	}{
		{
			name:        "edicao-de-cadastro",
			input:       models.NewUpdateAccountPlanInput().WithDescription("x"),
			wantBody:    apUpdateDescriptionRequest,
			response:    apUpdateDescriptionResponse,
			wantVersion: 1,
			check: func(t *testing.T, plan *models.AccountPlanView) {
				t.Helper()
				require.NotNil(t, plan.Description)
				assert.Equal(t, "x", *plan.Description)
			},
		},
		{
			name: "edicao-de-fees-cria-versao",
			input: models.NewUpdateAccountPlanInput().WithFeePatch("pixOut", models.AccountPlanFeePatch{
				FreeQuota:  apIntPtr(5),
				Deferrable: apBoolPtr(true),
			}),
			wantBody:    apUpdateFeePatchRequest,
			response:    apUpdateFeePatchResponse,
			wantVersion: 2,
			check: func(t *testing.T, plan *models.AccountPlanView) {
				t.Helper()
				assert.Equal(t, 5, plan.Fees["pixOut"].FreeQuota)
				assert.True(t, plan.Fees["pixOut"].Deferrable)
			},
		},
		{
			name:        "remocao-de-fee",
			input:       models.NewUpdateAccountPlanInput().WithoutFee("b"),
			wantBody:    apUpdateRemoveFeeRequest,
			response:    apUpdateRemoveFeeResponse,
			wantVersion: 2,
			check: func(t *testing.T, plan *models.AccountPlanView) {
				t.Helper()
				assert.NotContains(t, plan.Fees, "b")
				assert.Contains(t, plan.Fees, "a")
			},
		},
		{
			name:        "substituicao-de-rounding",
			input:       models.NewUpdateAccountPlanInput().WithRounding(models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeFloor}),
			wantBody:    apUpdateRoundingRequest,
			response:    apUpdateRoundingResponse,
			wantVersion: 2,
			check: func(t *testing.T, plan *models.AccountPlanView) {
				t.Helper()
				require.NotNil(t, plan.Rounding)
				assert.Equal(t, models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeFloor}, *plan.Rounding)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, srv := newAPServer(t, http.StatusOK, tt.response)

			plan, err := newTestAccountPlansFacade(t, srv).Update(context.Background(), apOrgID, apLedgerID, apPlanID, tt.input)
			require.NoError(t, err)

			req := rec.only(t)
			assert.Equal(t, http.MethodPatch, req.method)
			assert.Equal(t, apPlanURL, req.path)
			assert.Equal(t, tt.wantBody, req.body, "only the fields set are sent, as the captured request")
			assert.Equal(t, tt.wantVersion, plan.Version)
			tt.check(t, plan)
		})
	}
}

func TestAccountPlansFacade_Delete(t *testing.T) {
	rec, srv := newAPServer(t, http.StatusNoContent, "")

	require.NoError(t, newTestAccountPlansFacade(t, srv).Delete(context.Background(), apOrgID, apLedgerID, apPlanID))

	req := rec.only(t)
	assert.Equal(t, http.MethodDelete, req.method)
	assert.Equal(t, apPlanURL, req.path)
	assert.Empty(t, req.body)
}

func TestAccountPlansFacade_Assign(t *testing.T) {
	rec, srv := newAPServer(t, http.StatusOK, apAssignResponse)

	assignment, err := newTestAccountPlansFacade(t, srv).Assign(context.Background(), apOrgID, apLedgerID, apAccountID, models.NewAssignAccountPlanInput(apPlanID))
	require.NoError(t, err)

	req := rec.only(t)
	assert.Equal(t, http.MethodPut, req.method)
	assert.Equal(t, apAccPlnURL, req.path)
	assert.Equal(t, apAssignRequest, req.body)

	assert.Equal(t, "01a21da6-a748-796a-8591-5ba05e7212ea", assignment.ID)
	assert.Equal(t, apPlanID, assignment.AccountPlanID)
	assert.Equal(t, "@cust-1795793790", assignment.AccountAlias)
	assert.Nil(t, assignment.EndedAt)
}

func TestAccountPlansFacade_Unassign(t *testing.T) {
	rec, srv := newAPServer(t, http.StatusNoContent, "")

	require.NoError(t, newTestAccountPlansFacade(t, srv).Unassign(context.Background(), apOrgID, apLedgerID, apAccountID))

	req := rec.only(t)
	assert.Equal(t, http.MethodDelete, req.method)
	assert.Equal(t, apAccPlnURL, req.path)
}

func apIntPtr(v int) *int    { return &v }
func apBoolPtr(v bool) *bool { return &v }

// apOperation is one of the eleven facade operations, called with three path
// values in the order its path takes them.
type apOperation struct {
	name   string
	params [3]string
	call   func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error
}

func apOperations() []apOperation {
	plan := [3]string{"orgID", "ledgerID", "id"}
	account := [3]string{"orgID", "ledgerID", "accountID"}

	return []apOperation{
		{"Create", [3]string{"orgID", "ledgerID", ""}, func(ctx context.Context, f *accountPlansFacade, org, ledger, _ string) error {
			_, err := f.Create(ctx, org, ledger, apCreateRoundingInput())
			return err
		}},
		{"List", [3]string{"orgID", "ledgerID", ""}, func(ctx context.Context, f *accountPlansFacade, org, ledger, _ string) error {
			_, err := f.List(ctx, org, ledger, models.AccountPlansListOpts{})
			return err
		}},
		{"Get", plan, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.Get(ctx, org, ledger, id)
			return err
		}},
		{"Update", plan, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.Update(ctx, org, ledger, id, models.NewUpdateAccountPlanInput().WithDescription("x"))
			return err
		}},
		{"Delete", plan, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			return f.Delete(ctx, org, ledger, id)
		}},
		{"ListVersions", plan, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.ListVersions(ctx, org, ledger, id, models.AccountPlanVersionsListOpts{})
			return err
		}},
		{"ListAccounts", plan, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.ListAccounts(ctx, org, ledger, id, models.AccountPlanAccountsListOpts{})
			return err
		}},
		{"Assign", account, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.Assign(ctx, org, ledger, id, models.NewAssignAccountPlanInput(apPlanID))
			return err
		}},
		{"GetAssignment", account, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.GetAssignment(ctx, org, ledger, id)
			return err
		}},
		{"Unassign", account, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			return f.Unassign(ctx, org, ledger, id)
		}},
		{"GetUsage", account, func(ctx context.Context, f *accountPlansFacade, org, ledger, id string) error {
			_, err := f.GetUsage(ctx, org, ledger, id, models.AccountPlanUsageOpts{})
			return err
		}},
	}
}

func (op apOperation) callWith(ctx context.Context, f *accountPlansFacade, values [3]string) error {
	return op.call(ctx, f, values[0], values[1], values[2])
}

func TestAccountPlansFacade_PathIDRefusals(t *testing.T) {
	valid := [3]string{apOrgID, apLedgerID, apPlanID}

	operations := apOperations()
	require.Len(t, operations, 11)

	for _, op := range operations {
		for position, param := range op.params {
			if param == "" {
				continue
			}

			for _, bad := range []string{"", "..", "a/b", " " + apPlanID} {
				t.Run(op.name+"/"+param+"="+strconv.Quote(bad), func(t *testing.T) {
					rec, srv := newAPServer(t, http.StatusOK, apGetResponse)

					values := valid
					values[position] = bad

					err := op.callWith(context.Background(), newTestAccountPlansFacade(t, srv), values)
					require.Error(t, err)
					assert.True(t, sdkerrors.IsValidationError(err), "want a validation error, got %v", err)

					if bad == "" {
						assert.Contains(t, err.Error(), "missing required parameter: "+param)
					}

					assert.Empty(t, rec.all(), "a refused path id must never reach the server")
				})
			}
		}
	}
}

func TestAccountPlansFacade_LocalRefusals(t *testing.T) {
	tests := []struct {
		name      string
		call      func(ctx context.Context, f *accountPlansFacade) error
		wantField string
	}{
		{"input-de-criacao-invalido-recusado: blank label", func(ctx context.Context, f *accountPlansFacade) error {
			input := apCreateRoundingInput()
			input.Label = " "
			_, err := f.Create(ctx, apOrgID, apLedgerID, input)
			return err
		}, "label"},
		{"input-de-criacao-invalido-recusado: rounding scale 19", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Create(ctx, apOrgID, apLedgerID, apCreateRoundingInput().WithRounding(models.FeeRounding{Scale: 19, Mode: models.FeeRoundingModeHalfUp}))
			return err
		}, "rounding.scale"},
		{"input-de-criacao-invalido-recusado: unknown direction", func(ctx context.Context, f *accountPlansFacade) error {
			input := apCreateRoundingInput()
			fee := input.Fees["pixIn"]
			fee.Trigger.Direction = "both"
			input.Fees["pixIn"] = fee
			_, err := f.Create(ctx, apOrgID, apLedgerID, input)
			return err
		}, "fees[pixIn].trigger.direction"},
		{"input-nulo-recusado: create", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Create(ctx, apOrgID, apLedgerID, nil)
			return err
		}, ""},
		{"input-nulo-recusado: update", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Update(ctx, apOrgID, apLedgerID, apPlanID, nil)
			return err
		}, ""},
		{"input-nulo-recusado: assign", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Assign(ctx, apOrgID, apLedgerID, apAccountID, nil)
			return err
		}, ""},
		{"edicao-vazia-recusada", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Update(ctx, apOrgID, apLedgerID, apPlanID, models.NewUpdateAccountPlanInput())
			return err
		}, ""},
		{"substituicao-de-rounding: invalid mode", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Update(ctx, apOrgID, apLedgerID, apPlanID, models.NewUpdateAccountPlanInput().WithRounding(models.FeeRounding{Scale: 2, Mode: "ceil"}))
			return err
		}, "rounding.mode"},
		{"plano-invalido-na-atribuicao", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Assign(ctx, apOrgID, apLedgerID, apAccountID, models.NewAssignAccountPlanInput("plano-1795793790"))
			return err
		}, "accountPlanId"},
		{"list limit over max", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.List(ctx, apOrgID, apLedgerID, models.AccountPlansListOpts{PageListOpts: models.PageListOpts{Limit: models.MaxLimit + 1}})
			return err
		}, ""},
		{"versions negative page", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.ListVersions(ctx, apOrgID, apLedgerID, apPlanID, models.AccountPlanVersionsListOpts{PageListOpts: models.PageListOpts{Page: -1}})
			return err
		}, ""},
		{"plan accounts unknown sort", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.ListAccounts(ctx, apOrgID, apLedgerID, apPlanID, models.AccountPlanAccountsListOpts{PageListOpts: models.PageListOpts{SortDirection: "up"}})
			return err
		}, ""},
		{"usage period not YYYY-MM", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.GetUsage(ctx, apOrgID, apLedgerID, apAccountID, models.AccountPlanUsageOpts{Period: "2026-1"})
			return err
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, srv := newAPServer(t, http.StatusOK, apGetResponse)

			err := tt.call(context.Background(), newTestAccountPlansFacade(t, srv))
			require.Error(t, err)
			assert.True(t, sdkerrors.IsValidationError(err), "want a validation error, got %v", err)

			if tt.wantField != "" {
				assert.Contains(t, err.Error(), tt.wantField)
			}

			assert.Empty(t, rec.all(), "a locally refused call must never reach the server")
		})
	}
}

func TestAccountPlansFacade_ServerRefusalsKeepStatusAndCode(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantCode string
		call     func(ctx context.Context, f *accountPlansFacade) error
	}{
		{"recusa-do-servidor-repassada: duplicate label", http.StatusConflict, apErrDuplicateLabel, "0543", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Create(ctx, apOrgID, apLedgerID, apCreateRoundingInput())
			return err
		}},
		{"recusa-do-servidor-repassada: unknown field", http.StatusBadRequest, apErrUnknownField, "0053", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Create(ctx, apOrgID, apLedgerID, apCreateRoundingInput())
			return err
		}},
		{"recusa-do-servidor-repassada: retroactive effectiveFrom", http.StatusUnprocessableEntity, apErrRetroactive, "0553", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Update(ctx, apOrgID, apLedgerID, apPlanID, models.NewUpdateAccountPlanInput().
				WithRounding(models.FeeRounding{Scale: 2, Mode: models.FeeRoundingModeHalfUp}))
			return err
		}},
		{"recusa-do-servidor-repassada: schemes required", http.StatusUnprocessableEntity, apErrSchemesRequired, "0556", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Create(ctx, apOrgID, apLedgerID, apCreateRoundingInput())
			return err
		}},
		{"recusa-do-servidor-repassada: plan not found", http.StatusNotFound, apErrPlanNotFound, "0007", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Get(ctx, apOrgID, apLedgerID, apPlanID)
			return err
		}},
		{"remocao-com-atribuicao-ativa", http.StatusConflict, apErrActiveAssignments, "0546", func(ctx context.Context, f *accountPlansFacade) error {
			return f.Delete(ctx, apOrgID, apLedgerID, apPlanID)
		}},
		{"desatribuicao-sem-plano", http.StatusNotFound, apErrNoAssignment, "0554", func(ctx context.Context, f *accountPlansFacade) error {
			return f.Unassign(ctx, apOrgID, apLedgerID, apAccountID)
		}},
		{"uso-sem-plano", http.StatusNotFound, apErrNoAssignment, "0554", func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.GetUsage(ctx, apOrgID, apLedgerID, apAccountID, models.AccountPlanUsageOpts{})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, srv := newAPServer(t, tt.status, tt.body)

			err := tt.call(context.Background(), newTestAccountPlansFacade(t, srv))
			require.Error(t, err)

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			assert.Equal(t, tt.status, sdkerrors.GetStatusCode(err))
			assert.Equal(t, tt.wantCode, sdkErr.APICode)
			assert.Len(t, rec.all(), 1)
		})
	}
}

func TestAccountPlansFacade_ServerWithoutTheRoute(t *testing.T) {
	// A Midaz older than 4.4.0 has none of these routes; the SDK passes the 404 on.
	for _, op := range apOperations() {
		t.Run(op.name, func(t *testing.T) {
			_, srv := newAPServer(t, http.StatusNotFound, "")

			err := op.callWith(context.Background(), newTestAccountPlansFacade(t, srv), [3]string{apOrgID, apLedgerID, apPlanID})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, sdkerrors.GetStatusCode(err))
		})
	}
}

func TestAccountPlansFacade_ListsRefuseABodyWithoutAPage(t *testing.T) {
	listings := []struct {
		name string
		call func(f *accountPlansFacade) (bool, error)
	}{
		{"List", func(f *accountPlansFacade) (bool, error) {
			page, err := f.List(context.Background(), apOrgID, apLedgerID, models.AccountPlansListOpts{})
			return page == nil, err
		}},
		{"ListVersions", func(f *accountPlansFacade) (bool, error) {
			page, err := f.ListVersions(context.Background(), apOrgID, apLedgerID, apPlanID, models.AccountPlanVersionsListOpts{})
			return page == nil, err
		}},
		{"ListAccounts", func(f *accountPlansFacade) (bool, error) {
			page, err := f.ListAccounts(context.Background(), apOrgID, apLedgerID, apPlanID, models.AccountPlanAccountsListOpts{})
			return page == nil, err
		}},
	}

	for _, listing := range listings {
		for _, body := range []string{"", "null", "{}", "[]", `"page"`} {
			t.Run(listing.name+"/"+strconv.Quote(body), func(t *testing.T) {
				_, srv := newAPServer(t, http.StatusOK, body)

				isNil, err := listing.call(newTestAccountPlansFacade(t, srv))
				require.Error(t, err)
				assert.True(t, sdkerrors.IsResponseDecodeError(err), "want a response-decode error, got %v", err)
				assert.True(t, isNil, "the page must be nil alongside the error")
			})
		}
	}
}

func TestAccountPlansFacade_ObjectsRefuseANonObjectBody(t *testing.T) {
	for _, body := range []string{"[]", `"plan"`, "42"} {
		t.Run(strconv.Quote(body), func(t *testing.T) {
			_, srv := newAPServer(t, http.StatusOK, body)

			plan, err := newTestAccountPlansFacade(t, srv).Get(context.Background(), apOrgID, apLedgerID, apPlanID)
			require.Error(t, err)
			assert.True(t, sdkerrors.IsResponseDecodeError(err), "want a response-decode error, got %v", err)
			assert.Nil(t, plan)
		})
	}
}

// apWrite is one of the five account plan writes, with the status and body the
// server answers it with on success.
type apWrite struct {
	name   string
	status int
	body   string
	call   func(ctx context.Context, f *accountPlansFacade) error
}

func apWrites() []apWrite {
	return []apWrite{
		{"Create", http.StatusCreated, apCreateRoundingResponse, func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Create(ctx, apOrgID, apLedgerID, apCreateRoundingInput())
			return err
		}},
		{"Update", http.StatusOK, apUpdateFeePatchResponse, func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Update(ctx, apOrgID, apLedgerID, apPlanID, models.NewUpdateAccountPlanInput().
				WithFeePatch("pixOut", models.AccountPlanFeePatch{FreeQuota: apIntPtr(5)}))
			return err
		}},
		{"Delete", http.StatusNoContent, "", func(ctx context.Context, f *accountPlansFacade) error {
			return f.Delete(ctx, apOrgID, apLedgerID, apPlanID)
		}},
		{"Assign", http.StatusOK, apAssignResponse, func(ctx context.Context, f *accountPlansFacade) error {
			_, err := f.Assign(ctx, apOrgID, apLedgerID, apAccountID, models.NewAssignAccountPlanInput(apPlanID))
			return err
		}},
		{"Unassign", http.StatusNoContent, "", func(ctx context.Context, f *accountPlansFacade) error {
			return f.Unassign(ctx, apOrgID, apLedgerID, apAccountID)
		}},
	}
}

func TestAccountPlansFacade_WritesSendNoIdempotencyKey(t *testing.T) {
	configs := []struct {
		name   string
		ctx    func() context.Context
		facade func(t *testing.T, srv *httptest.Server) *accountPlansFacade
	}{
		{
			name: "sem-chave-por-padrao",
			ctx:  context.Background,
			facade: func(t *testing.T, srv *httptest.Server) *accountPlansFacade {
				t.Helper()

				return newV2Services(newTestLedgerClient(t, srv), false).AccountPlans
			},
		},
		{
			name: "sem-chave-com-a-opcao-ligada",
			ctx:  context.Background,
			facade: func(t *testing.T, srv *httptest.Server) *accountPlansFacade {
				t.Helper()

				return newV2Services(newTestLedgerClient(t, srv), true).AccountPlans
			},
		},
		{
			name: "chave-do-contexto-nao-enviada",
			ctx: func() context.Context {
				return sdkctx.WithIdempotencyKey(context.Background(), "caller-key-1")
			},
			facade: func(t *testing.T, srv *httptest.Server) *accountPlansFacade {
				t.Helper()

				return newV2Services(newTestLedgerClient(t, srv), true).AccountPlans
			},
		},
	}

	for _, cfg := range configs {
		for _, write := range apWrites() {
			t.Run(cfg.name+"/"+write.name, func(t *testing.T) {
				rec, srv := newAPServer(t, write.status, write.body)

				require.NoError(t, write.call(cfg.ctx(), cfg.facade(t, srv)))

				req := rec.only(t)
				assert.Empty(t, req.header.Values("X-Idempotency"), "account plan writes never send X-Idempotency")
				assert.Empty(t, req.header.Values("X-TTL"))
			})
		}
	}
}

// newRetryingAccountPlansFacade builds the facade over a ledger client whose
// retry engine is ENABLED, so a request that is retryable is retried.
func newRetryingAccountPlansFacade(t *testing.T, srv *httptest.Server) *accountPlansFacade {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL,
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:   srv.Client(),
		retryOptions: fastRetryOpts(),
	})
	require.NoError(t, err)

	return newV2Services(planes.Ledger, true).AccountPlans
}

func TestAccountPlansFacade_WriteRetryIsNotAttempted(t *testing.T) {
	t.Run("control: a read is retried on 503", func(t *testing.T) {
		rec, srv := newAPServer(t, http.StatusServiceUnavailable, "")

		_, err := newRetryingAccountPlansFacade(t, srv).Get(context.Background(), apOrgID, apLedgerID, apPlanID)
		require.Error(t, err)
		assert.Greater(t, len(rec.all()), 1, "the retry engine must be on, or the write rows prove nothing")
	})

	for _, write := range apWrites() {
		t.Run("escrita-nao-e-repetida/"+write.name, func(t *testing.T) {
			rec, srv := newAPServer(t, http.StatusServiceUnavailable, "")

			err := write.call(sdkctx.WithIdempotencyKey(context.Background(), "caller-key-1"), newRetryingAccountPlansFacade(t, srv))
			require.Error(t, err)
			assert.Equal(t, http.StatusServiceUnavailable, sdkerrors.GetStatusCode(err))
			assert.Len(t, rec.all(), 1, "a keyless account plan write is sent exactly once")
		})
	}
}
