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
