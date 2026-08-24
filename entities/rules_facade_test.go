// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v5/internal/gentracer"
	"github.com/LerianStudio/midaz-sdk-golang/v5/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
)

const ruleID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

// ruleJSON is a canonical server rule body, camelCase with the ruleId identity.
func ruleJSON(id, status string) string {
	return `{"ruleId":"` + id + `","name":"block-high-value","expression":"transaction.amount > 1000",` +
		`"action":"DENY","status":"` + status + `","scopes":[{"transactionType":"PIX"}],` +
		`"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`
}

// TestRulesFacade_Create201 guards the raw 2xx gate. The server returns 201 on
// create and the generated CreateRuleResp parser is status-exact, so the facade
// MUST route through the raw call + 2xx success gate rather than depend on one
// exact success status.
func TestRulesFacade_Create201(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated) // 201, not 200
		_, _ = w.Write([]byte(ruleJSON(ruleID, "DRAFT")))
	}))
	defer srv.Close()

	rule, err := newTestRulesFacade(t, srv).Create(context.Background(),
		models.NewCreateRuleInput("block-high-value", "transaction.amount > 1000", models.RuleActionDeny))
	if err != nil {
		t.Fatalf("Create @201: %v", err)
	}
	if method != http.MethodPost || path != "/v1/rules" {
		t.Fatalf("create req = %s %s, want POST /v1/rules", method, path)
	}
	if rule == nil || rule.ID != ruleID {
		t.Fatalf("Create @201 returned %+v, want a decoded rule with ID %s", rule, ruleID)
	}
}

// TestRulesFacade_ListFlatEnvelope is the load-bearing envelope red. The tracer
// serializes lists as the FLAT {rules:[...],nextCursor} envelope, not
// {items,pagination}. A straight json.Unmarshal into models.ListResponse[Rule]
// reads Items from the "items" key only and yields EMPTY Items. The facade MUST
// map the domain-keyed envelope so Items is non-empty.
func TestRulesFacade_ListFlatEnvelope(t *testing.T) {
	body := `{"rules":[` + ruleJSON(ruleID, "ACTIVE") + `],"hasMore":false,"nextCursor":""}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	page, err := newTestRulesFacade(t, srv).List(context.Background(), models.RulesListOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("List Items = %d, want 1 (flat {rules:[...]} envelope must map to Items)", len(page.Items))
	}
	if page.Items[0].ID != ruleID || page.Items[0].Status != "ACTIVE" {
		t.Fatalf("List Items[0] = %+v", page.Items[0])
	}
}

// TestRulesFacade_PagesCursorStop chains two cursor pages and stops on an empty
// nextCursor, asserting the cursor advances and exactly two requests are made. A
// loop that never advanced the cursor or never stopped on "" would loop forever.
func TestRulesFacade_PagesCursorStop(t *testing.T) {
	page1 := `{"rules":[` + ruleJSON("11111111-1111-1111-1111-111111111111", "ACTIVE") + `],"hasMore":true,"nextCursor":"c2"}`
	page2 := `{"rules":[` + ruleJSON("22222222-2222-2222-2222-222222222222", "ACTIVE") + `],"hasMore":false,"nextCursor":""}`

	var cursors []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursors = append(cursors, r.URL.Query().Get("cursor"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "c2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	all, err := CollectAll(newTestRulesFacade(t, srv).ListAll(context.Background(), models.RulesListOpts{
		CursorListOpts: models.CursorListOpts{Limit: 1},
	}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("collected %d rules, want 2", len(all))
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "c2" {
		t.Fatalf("cursor chain = %v, want ['' 'c2'] and exactly 2 requests", cursors)
	}
}

// TestRulesFacade_PagesCtxCancel proves a cancelled context terminates iteration
// with the context error before any request.
func TestRulesFacade_PagesCtxCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rules":[],"nextCursor":""}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := CollectAll(newTestRulesFacade(t, srv).ListAll(ctx, models.RulesListOpts{}))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// TestRulesFacade_Delete204 proves delete succeeds on a 204 no-body response with
// nothing to decode.
func TestRulesFacade_Delete204(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestRulesFacade(t, srv).Delete(context.Background(), ruleID); err != nil {
		t.Fatalf("Delete @204: %v", err)
	}
	if method != http.MethodDelete || path != "/v1/rules/"+ruleID {
		t.Fatalf("delete req = %s %s, want DELETE /v1/rules/%s", method, path, ruleID)
	}
}

// TestRulesFacade_Lifecycle proves each body-less state transition POSTs the
// right path and decodes the 200 rule body.
func TestRulesFacade_Lifecycle(t *testing.T) {
	tests := []struct {
		name     string
		call     func(f *rulesFacade) (*models.Rule, error)
		wantPath string
		status   string
	}{
		{"activate", func(f *rulesFacade) (*models.Rule, error) { return f.Activate(context.Background(), ruleID) }, "/v1/rules/" + ruleID + "/activate", "ACTIVE"},
		{"deactivate", func(f *rulesFacade) (*models.Rule, error) { return f.Deactivate(context.Background(), ruleID) }, "/v1/rules/" + ruleID + "/deactivate", "INACTIVE"},
		{"draft", func(f *rulesFacade) (*models.Rule, error) { return f.Draft(context.Background(), ruleID) }, "/v1/rules/" + ruleID + "/draft", "DRAFT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var method, path string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method, path = r.Method, r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(ruleJSON(ruleID, tt.status)))
			}))
			defer srv.Close()

			rule, err := tt.call(newTestRulesFacade(t, srv))
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if method != http.MethodPost || path != tt.wantPath {
				t.Fatalf("%s req = %s %s, want POST %s", tt.name, method, path, tt.wantPath)
			}
			if rule == nil || rule.Status != tt.status {
				t.Fatalf("%s returned %+v, want status %s", tt.name, rule, tt.status)
			}
		})
	}
}

// TestRulesFacade_Update proves the PATCH omit-unset body reaches the server and
// the 200 rule decodes.
func TestRulesFacade_Update(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ruleJSON(ruleID, "DRAFT")))
	}))
	defer srv.Close()

	_, err := newTestRulesFacade(t, srv).Update(context.Background(), ruleID,
		models.NewUpdateRuleInput().WithExpression("transaction.amount > 2000"))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if method != http.MethodPatch || path != "/v1/rules/"+ruleID {
		t.Fatalf("update req = %s %s, want PATCH /v1/rules/%s", method, path, ruleID)
	}
	if !strings.Contains(body, `"expression"`) || strings.Contains(body, `"name"`) {
		t.Fatalf("update body = %q, want only the set expression field", body)
	}
}

// TestRulesFacade_Error maps a non-2xx problem+json into *errors.Error with the
// server request-ID threaded through.
func TestRulesFacade_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-rule-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"TRACER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestRulesFacade(t, srv).Get(context.Background(), ruleID)
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0009" || sdkErr.RequestID != "req-rule-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestRulesFacade_ListError maps a non-2xx problem+json from the LIST endpoint
// into *errors.Error with the APICode and server request-ID extracted. This
// exercises List's own DecodeProblemJSON branch — distinct from the decodeOne
// path Get uses in TestRulesFacade_Error.
func TestRulesFacade_ListError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-rule-list-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"TRACER-0011","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestRulesFacade(t, srv).List(context.Background(), models.RulesListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "TRACER-0011" || sdkErr.RequestID != "req-rule-list-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestRulesFacade_ListMalformedBody proves a 200 whose body is not valid JSON
// for the flat {rules:[...]} envelope surfaces as a typed internal error rather
// than an empty page or a panic.
func TestRulesFacade_ListMalformedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rules": not-valid-json`))
	}))
	defer srv.Close()

	_, err := newTestRulesFacade(t, srv).List(context.Background(), models.RulesListOpts{})
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.Code != sdkerrors.CodeInternal {
		t.Fatalf("error code = %q, want %q (malformed body must be an internal error)", sdkErr.Code, sdkerrors.CodeInternal)
	}
}

// TestRulesFacade_ValidateBeforeWire proves bad input is rejected before any
// round trip (no server contact).
func TestRulesFacade_ValidateBeforeWire(t *testing.T) {
	var hit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	f := newTestRulesFacade(t, srv)

	if _, err := f.Create(context.Background(), models.NewCreateRuleInput("r", "amount > 1", "MAYBE")); err == nil {
		t.Fatalf("bad action should be rejected before the wire")
	}
	if _, err := f.Create(context.Background(), models.NewCreateRuleInput("r", "  ", models.RuleActionAllow)); err == nil {
		t.Fatalf("empty expression should be rejected before the wire")
	}
	if hit {
		t.Fatalf("validation failures must not contact the server")
	}
}

func newTestRulesFacade(t *testing.T, srv *httptest.Server) *rulesFacade {
	t.Helper()
	return newRulesFacade(newTestTracerClient(t, srv), true)
}

// newTestTracerClient builds a tracer plane client pointed at the test server
// with a static Bearer token. Shared by the Phase 4 tracer-facade tests.
func newTestTracerClient(t *testing.T, srv *httptest.Server) *gentracer.ClientWithResponses {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL,
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient:   srv.Client(),
		retryOptions: planeTestRetryOptions(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	return planes.Tracer
}
