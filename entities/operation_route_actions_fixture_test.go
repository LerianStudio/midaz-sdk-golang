// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

// The operation route fixtures the /v1, /v2, list and transaction-route read
// tests share. The eight-action and bridge bodies are responses captured from
// a Midaz server; every action carries rubric codes distinct per action and per
// side, so a decode that swapped two action tags or dropped one hands back a
// code the expectation does not hold.
const (
	operationRouteEightActionsID = "01a2187d-81c1-7f26-b162-b41fc292c4b5"
	operationRouteBridgeID       = "01a2187d-3523-7a1b-90f9-d0c41673c018"

	operationRouteEntryDirect    = `"direct":{"debit":{"code":"1001","description":"direct debit"},"credit":{"code":"2001","description":"direct credit"}}`
	operationRouteEntryHold      = `"hold":{"debit":{"code":"1002","description":"hold debit"},"credit":{"code":"2002","description":"hold credit"}}`
	operationRouteEntryCommit    = `"commit":{"debit":{"code":"1005","description":"commit debit"},"credit":{"code":"2005","description":"commit credit"}}`
	operationRouteEntryCancel    = `"cancel":{"debit":{"code":"1006","description":"cancel debit"},"credit":{"code":"2006","description":"cancel credit"}}`
	operationRouteEntryRevert    = `"revert":{"debit":{"code":"1007","description":"revert debit"},"credit":{"code":"2007","description":"revert credit"}}`
	operationRouteEntryOverdraft = `"overdraft":{"debit":{"code":"1008","description":"overdraft debit"},"credit":{"code":"2008","description":"overdraft credit"}}`
	operationRouteEntryBlock     = `"block":{"debit":{"code":"1003","description":"block debit"},"credit":{"code":"2003","description":"block credit"}}`
	operationRouteEntryUnblock   = `"unblock":{"debit":{"code":"1004","description":"unblock debit"},"credit":{"code":"2004","description":"unblock credit"}}`

	operationRouteEnvelopeTail = `"createdAt":"2026-11-26T15:33:08.51566Z","updatedAt":"2026-11-26T15:33:08.51566Z","deletedAt":null}`

	// operationRouteEightActionsJSON is the bidirectional route with every
	// action except crossLedger, which the server refuses next to any other.
	operationRouteEightActionsJSON = `{"id":"` + operationRouteEightActionsID + `","organizationId":"01a2187d-174a-79e1-9a6f-e7ff20a4a7c4","ledgerId":"01a2187d-1768-7280-8222-3adfe0d315fb","title":"probe A v2","code":"PRB-A-v2","operationType":"bidirectional",` +
		`"accountingEntries":{` + operationRouteEntryDirect + `,` + operationRouteEntryHold + `,` + operationRouteEntryCommit + `,` +
		operationRouteEntryCancel + `,` + operationRouteEntryRevert + `,` + operationRouteEntryOverdraft + `,` +
		operationRouteEntryBlock + `,` + operationRouteEntryUnblock + `},` + operationRouteEnvelopeTail

	// operationRouteBridgeJSON is the bridge route, which carries crossLedger alone.
	operationRouteBridgeJSON = `{"id":"` + operationRouteBridgeID + `","organizationId":"01a2187d-174a-79e1-9a6f-e7ff20a4a7c4","ledgerId":"01a2187d-1768-7280-8222-3adfe0d315fb","title":"probe B v2","code":"PRB-B-v2","operationType":"bidirectional",` +
		`"accountingEntries":{"crossLedger":{"debit":{"code":"9001","description":"in"},"credit":{"code":"9002","description":"out"}}},` + operationRouteEnvelopeTail

	// operationRouteBlockWithoutUnblockJSON is a route with direct and block
	// only (block does not require unblock). The body is derived from the
	// captured entries above, not captured from a server.
	operationRouteBlockWithoutUnblockJSON = `{"id":"` + v2UUIDA + `","title":"probe C","operationType":"source",` +
		`"accountingEntries":{` + operationRouteEntryDirect + `,` + operationRouteEntryBlock + `},` + operationRouteEnvelopeTail

	// operationRouteSixActionsJSON stands for what a server without block,
	// unblock and crossLedger returns: the six actions that predate them. The
	// body is derived from the captured entries above, not captured from a server.
	operationRouteSixActionsJSON = `{"id":"` + v2UUIDB + `","title":"probe D","operationType":"bidirectional",` +
		`"accountingEntries":{` + operationRouteEntryDirect + `,` + operationRouteEntryHold + `,` + operationRouteEntryCommit + `,` +
		operationRouteEntryCancel + `,` + operationRouteEntryRevert + `,` + operationRouteEntryOverdraft + `},` + operationRouteEnvelopeTail
)

// operationRouteRubrics is the debit and credit an action is expected to carry.
type operationRouteRubrics struct {
	debitCode, debitDescription, creditCode, creditDescription string
}

var (
	wantDirectRubrics    = &operationRouteRubrics{"1001", "direct debit", "2001", "direct credit"}
	wantHoldRubrics      = &operationRouteRubrics{"1002", "hold debit", "2002", "hold credit"}
	wantCommitRubrics    = &operationRouteRubrics{"1005", "commit debit", "2005", "commit credit"}
	wantCancelRubrics    = &operationRouteRubrics{"1006", "cancel debit", "2006", "cancel credit"}
	wantRevertRubrics    = &operationRouteRubrics{"1007", "revert debit", "2007", "revert credit"}
	wantOverdraftRubrics = &operationRouteRubrics{"1008", "overdraft debit", "2008", "overdraft credit"}
	wantBlockRubrics     = &operationRouteRubrics{"1003", "block debit", "2003", "block credit"}
	wantUnblockRubrics   = &operationRouteRubrics{"1004", "unblock debit", "2004", "unblock credit"}
)

// The expectations name all nine actions; nil marks an action the server did
// not send and the SDK must not fabricate.
var (
	wantEightActions = map[string]*operationRouteRubrics{
		"direct": wantDirectRubrics, "hold": wantHoldRubrics, "commit": wantCommitRubrics,
		"cancel": wantCancelRubrics, "revert": wantRevertRubrics, "overdraft": wantOverdraftRubrics,
		"block": wantBlockRubrics, "unblock": wantUnblockRubrics, "crossLedger": nil,
	}

	wantBridgeActions = map[string]*operationRouteRubrics{
		"direct": nil, "hold": nil, "commit": nil, "cancel": nil, "revert": nil, "overdraft": nil,
		"block": nil, "unblock": nil, "crossLedger": {"9001", "in", "9002", "out"},
	}

	wantBlockWithoutUnblockActions = map[string]*operationRouteRubrics{
		"direct": wantDirectRubrics, "hold": nil, "commit": nil, "cancel": nil, "revert": nil, "overdraft": nil,
		"block": wantBlockRubrics, "unblock": nil, "crossLedger": nil,
	}

	wantSixActions = map[string]*operationRouteRubrics{
		"direct": wantDirectRubrics, "hold": wantHoldRubrics, "commit": wantCommitRubrics,
		"cancel": wantCancelRubrics, "revert": wantRevertRubrics, "overdraft": wantOverdraftRubrics,
		"block": nil, "unblock": nil, "crossLedger": nil,
	}
)

// operationRouteActions names every action field of the decoded model.
func operationRouteActions(entries *models.AccountingEntries) map[string]*models.AccountingEntry {
	return map[string]*models.AccountingEntry{
		"direct":      entries.Direct,
		"hold":        entries.Hold,
		"commit":      entries.Commit,
		"cancel":      entries.Cancel,
		"revert":      entries.Revert,
		"overdraft":   entries.Overdraft,
		"block":       entries.Block,
		"unblock":     entries.Unblock,
		"crossLedger": entries.CrossLedger,
	}
}

// assertOperationRouteActions checks the decoded route id and, for each of the
// nine actions, either the debit and credit code and description the server
// sent or the absence of an action it did not send.
func assertOperationRouteActions(t *testing.T, route *models.OperationRoute, wantID string, want map[string]*operationRouteRubrics) {
	t.Helper()

	if route == nil {
		t.Fatalf("route = nil, want %s", wantID)
	}

	if route.ID.String() != wantID {
		t.Fatalf("id = %q, want %q", route.ID.String(), wantID)
	}

	if route.AccountingEntries == nil {
		t.Fatalf("route %s: accountingEntries = nil, want the actions the server sent", wantID)
	}

	got := operationRouteActions(route.AccountingEntries)
	if len(got) != len(want) {
		t.Fatalf("route %s: expectation names %d actions, operationRouteActions lists %d", wantID, len(want), len(got))
	}

	for action, rubrics := range want {
		entry, ok := got[action]
		if !ok {
			t.Fatalf("route %s: expectation names unknown action %q", wantID, action)
		}

		if rubrics == nil {
			if entry != nil {
				t.Fatalf("route %s: %s = %+v, want nil — the server did not send it", wantID, action, entry)
			}

			continue
		}

		if entry == nil || entry.Debit == nil || entry.Credit == nil {
			t.Fatalf("route %s: %s = %+v, want debit and credit — the server sent both", wantID, action, entry)
		}

		gotRubrics := operationRouteRubrics{entry.Debit.Code, entry.Debit.Description, entry.Credit.Code, entry.Credit.Description}
		if gotRubrics != *rubrics {
			t.Fatalf("route %s: %s = %+v, want %+v", wantID, action, gotRubrics, *rubrics)
		}
	}
}

const (
	operationRouteCreatedID = "77777777-7777-7777-7777-777777777777"

	// operationRouteDirectBlockUnblockJSON answers the create and update
	// scenarios below. The body is derived from the captured entries above, not
	// captured from a server.
	operationRouteDirectBlockUnblockJSON = `{"id":"` + operationRouteCreatedID + `","title":"probe C","operationType":"source",` +
		`"accountingEntries":{` + operationRouteEntryDirect + `,` + operationRouteEntryBlock + `,` + operationRouteEntryUnblock + `},` +
		operationRouteEnvelopeTail
)

var wantDirectBlockUnblockActions = map[string]*operationRouteRubrics{
	"direct": wantDirectRubrics, "hold": nil, "commit": nil, "cancel": nil, "revert": nil, "overdraft": nil,
	"block": wantBlockRubrics, "unblock": wantUnblockRubrics, "crossLedger": nil,
}

// accountingEntryFrom builds the input entry that carries rubrics.
func accountingEntryFrom(rubrics *operationRouteRubrics) *models.AccountingEntry {
	return &models.AccountingEntry{
		Debit:  &models.AccountingRubric{Code: rubrics.debitCode, Description: rubrics.debitDescription},
		Credit: &models.AccountingRubric{Code: rubrics.creditCode, Description: rubrics.creditDescription},
	}
}

// operationRouteWriteScenario is one create or update the /v1 and /v2 write
// tests both run (exactly one of create and update is set): the exact body the
// facade must send and the route the server answers with.
type operationRouteWriteScenario struct {
	name     string
	create   *models.CreateOperationRouteInput
	update   *models.UpdateOperationRouteInput
	wantBody string
	response string
	wantID   string
	want     map[string]*operationRouteRubrics
}

// operationRouteWriteScenarios returns fresh inputs on every call, so no test
// can leak a mutation into another.
func operationRouteWriteScenarios() []operationRouteWriteScenario {
	return []operationRouteWriteScenario{
		{
			name: "create with direct, block and unblock",
			create: &models.CreateOperationRouteInput{
				Title:         "probe C",
				OperationType: "source",
				AccountingEntries: &models.AccountingEntries{
					Direct:  accountingEntryFrom(wantDirectRubrics),
					Block:   accountingEntryFrom(wantBlockRubrics),
					Unblock: accountingEntryFrom(wantUnblockRubrics),
				},
			},
			wantBody: `{"title":"probe C","operationType":"source","accountingEntries":{` +
				operationRouteEntryDirect + `,` + operationRouteEntryBlock + `,` + operationRouteEntryUnblock + `},"metadata":null}`,
			response: operationRouteDirectBlockUnblockJSON,
			wantID:   operationRouteCreatedID,
			want:     wantDirectBlockUnblockActions,
		},
		{
			name: "create bridge route",
			create: &models.CreateOperationRouteInput{
				Title:             "probe B v2",
				Code:              "PRB-B-v2",
				OperationType:     "bidirectional",
				AccountingEntries: &models.AccountingEntries{CrossLedger: accountingEntryFrom(wantBridgeActions["crossLedger"])},
			},
			wantBody: `{"title":"probe B v2","code":"PRB-B-v2","operationType":"bidirectional","accountingEntries":{` +
				`"crossLedger":{"debit":{"code":"9001","description":"in"},"credit":{"code":"9002","description":"out"}}},"metadata":null}`,
			response: operationRouteBridgeJSON,
			wantID:   operationRouteBridgeID,
			want:     wantBridgeActions,
		},
		{
			// The server merges a PATCH, so the route it answers with still
			// carries the direct action the update did not resend.
			name: "update with block and unblock",
			update: &models.UpdateOperationRouteInput{
				AccountingEntries: &models.AccountingEntries{
					Block:   accountingEntryFrom(wantBlockRubrics),
					Unblock: accountingEntryFrom(wantUnblockRubrics),
				},
			},
			wantBody: `{"accountingEntries":{` + operationRouteEntryBlock + `,` + operationRouteEntryUnblock + `}}`,
			response: operationRouteDirectBlockUnblockJSON,
			wantID:   operationRouteCreatedID,
			want:     wantDirectBlockUnblockActions,
		},
	}
}

// routeWriteServer records the method, path and body of every request and
// answers each one with contentType, status and response, as the /v1 CRUD tests
// do inline.
func routeWriteServer(t *testing.T, method, path, body *string, contentType string, status int, response string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*method, *path = r.Method, r.URL.Path

		raw, _ := io.ReadAll(r.Body)
		*body = string(raw)

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))

	t.Cleanup(srv.Close)

	return srv
}

// readCapturingServer records the method and path of every request and answers
// each one with body.
func readCapturingServer(t *testing.T, method, path *string, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*method, *path = r.Method, r.URL.Path

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))

	t.Cleanup(srv.Close)

	return srv
}
