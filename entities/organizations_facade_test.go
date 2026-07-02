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

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

// TestOrganizationsFacade_ListAndPaginate is the Phase 1 milestone (Task 1.P1):
// the hand-written Organizations facade lists organizations end-to-end over the
// generated genledger ClientWithResponses, normalizing the response into the
// public surface (models.Organization + the List/Pages/All trinaldo) and
// chaining two pages via the response next_cursor.
func TestOrganizationsFacade_ListAndPaginate(t *testing.T) {
	page1 := `{"items":[{"id":"11111111-1111-1111-1111-111111111111","legalName":"Org One","legalDocument":"doc-1"}],"limit":1,"next_cursor":"cursor-2"}`
	page2 := `{"items":[{"id":"22222222-2222-2222-2222-222222222222","legalName":"Org Two","legalDocument":"doc-2"}],"limit":1}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Route by requested page so the two test phases (single List, then
		// All) are independent of any call counter.
		if page := r.URL.Query().Get("page"); page == "2" {
			_, _ = w.Write([]byte(page2))
		} else {
			_, _ = w.Write([]byte(page1))
		}
	}))
	defer srv.Close()

	facade := newTestOrganizationsFacade(t, srv)

	// Single-page List returns the typed public model.
	first, err := facade.List(context.Background(), models.OrganizationsListOpts{PageListOpts: models.PageListOpts{Limit: 1}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(first.Items) != 1 || first.Items[0].LegalName != "Org One" {
		t.Fatalf("List page 1 = %+v, want single Org One", first.Items)
	}

	if !first.Pagination.HasMore() {
		t.Fatalf("List page 1 must report HasMore via next_cursor, got %+v", first.Pagination)
	}

	// All chains both pages through the trinaldo.
	all, err := CollectAll(facade.All(context.Background(), models.OrganizationsListOpts{PageListOpts: models.PageListOpts{Limit: 1}}))
	if err != nil {
		t.Fatalf("CollectAll: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("All yielded %d organizations, want 2 (two pages chained via next_cursor)", len(all))
	}

	if all[0].LegalName != "Org One" || all[1].LegalName != "Org Two" {
		t.Fatalf("All order = [%q, %q], want [Org One, Org Two]", all[0].LegalName, all[1].LegalName)
	}
}

// TestOrganizationsFacade_ErrorDecodes is the second half of the milestone: an
// RFC 9457 problem+json error body decodes into *errors.Error with the correct
// code, status, and retryability, never leaking the generated types.
func TestOrganizationsFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0178","title":"Service Unavailable","detail":"upstream is down","status":503}`))
	}))
	defer srv.Close()

	facade := newTestOrganizationsFacade(t, srv)

	_, err := facade.List(context.Background(), models.OrganizationsListOpts{})
	if err == nil {
		t.Fatalf("List against a 503 must return an error")
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error (generated types must not leak)", err)
	}

	if sdkErr.APICode != "LEDGER-0178" {
		t.Fatalf("APICode = %q, want LEDGER-0178", sdkErr.APICode)
	}

	if sdkErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", sdkErr.StatusCode)
	}

	if !sdkErr.Retryable() {
		t.Fatalf("503 (code suffix 0178) must be retryable")
	}
}

// TestOrganizationsFacade_ErrorPropagatesRequestID is the money-path
// correlation guard: the server stamps X-Request-ID on the error response, and
// the SDK must thread it into the decoded *errors.Error so a client-side
// failure can be correlated with the server-side log/trace. Covers a 409
// (idempotency conflict) and a 503 (transient unavailability).
func TestOrganizationsFacade_ErrorPropagatesRequestID(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		requestID string
		body      string
	}{
		{
			name:      "409 idempotency conflict",
			status:    http.StatusConflict,
			requestID: "req-409-abc123",
			body:      `{"code":"LEDGER-0084","title":"Idempotency conflict","detail":"duplicate request","status":409}`,
		},
		{
			name:      "503 service unavailable",
			status:    http.StatusServiceUnavailable,
			requestID: "req-503-def456",
			body:      `{"code":"LEDGER-0178","title":"Service Unavailable","detail":"upstream is down","status":503}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("X-Request-ID", tc.requestID)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			facade := newTestOrganizationsFacade(t, srv)

			_, err := facade.List(context.Background(), models.OrganizationsListOpts{})
			if err == nil {
				t.Fatalf("List against a %d must return an error", tc.status)
			}

			var sdkErr *sdkerrors.Error
			if !errors.As(err, &sdkErr) {
				t.Fatalf("error type = %T, want *errors.Error", err)
			}

			if sdkErr.RequestID != tc.requestID {
				t.Fatalf("RequestID = %q, want %q (server↔client correlation lost)", sdkErr.RequestID, tc.requestID)
			}
		})
	}
}

// TestListOrganizationsParams_FilterBranches locks each opts.* → genledger.*
// field mapping so a silent field swap (e.g. StartDate written into EndDate) is
// caught. Pure mapping test over the params builder — no server.
func TestListOrganizationsParams_FilterBranches(t *testing.T) {
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}

	tests := []struct {
		name string
		opts models.OrganizationsListOpts
		want func(t *testing.T, p *genledger.ListOrganizationsParams)
	}{
		{
			name: "sort direction",
			opts: models.OrganizationsListOpts{PageListOpts: models.PageListOpts{SortDirection: "desc"}},
			want: func(t *testing.T, p *genledger.ListOrganizationsParams) {
				t.Helper()
				if got := deref(p.SortOrder); got != "desc" {
					t.Fatalf("SortOrder = %q, want desc", got)
				}
			},
		},
		{
			name: "start date",
			opts: models.OrganizationsListOpts{PageListOpts: models.PageListOpts{StartDate: "2026-01-01"}},
			want: func(t *testing.T, p *genledger.ListOrganizationsParams) {
				t.Helper()
				if got := deref(p.StartDate); got != "2026-01-01" {
					t.Fatalf("StartDate = %q, want 2026-01-01", got)
				}
				if p.EndDate != nil {
					t.Fatalf("EndDate = %q, want nil (start must not leak into end)", deref(p.EndDate))
				}
			},
		},
		{
			name: "end date",
			opts: models.OrganizationsListOpts{PageListOpts: models.PageListOpts{EndDate: "2026-12-31"}},
			want: func(t *testing.T, p *genledger.ListOrganizationsParams) {
				t.Helper()
				if got := deref(p.EndDate); got != "2026-12-31" {
					t.Fatalf("EndDate = %q, want 2026-12-31", got)
				}
				if p.StartDate != nil {
					t.Fatalf("StartDate = %q, want nil (end must not leak into start)", deref(p.StartDate))
				}
			},
		},
		{
			name: "legal name",
			opts: models.OrganizationsListOpts{Filters: models.OrganizationsFilters{LegalName: "Acme"}},
			want: func(t *testing.T, p *genledger.ListOrganizationsParams) {
				t.Helper()
				if got := deref(p.LegalName); got != "Acme" {
					t.Fatalf("LegalName = %q, want Acme", got)
				}
			},
		},
		{
			name: "status",
			opts: models.OrganizationsListOpts{Filters: models.OrganizationsFilters{Status: "ACTIVE"}},
			want: func(t *testing.T, p *genledger.ListOrganizationsParams) {
				t.Helper()
				if got := deref(p.Status); got != "ACTIVE" {
					t.Fatalf("Status = %q, want ACTIVE", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want(t, listOrganizationsParams(tt.opts))
		})
	}
}

// TestOrganizationsFacade_IncludeDeleted verifies the IncludeDeleted filter is
// propagated as the legacy include_deleted=true query param when set, and
// absent otherwise. The OAS spec omits the field from ListOrganizations
// (server-side gap), so the SDK injects it via a request editor rather than a
// generated param — this test guards that it is not silently dropped.
func TestOrganizationsFacade_IncludeDeleted(t *testing.T) {
	cases := []struct {
		name           string
		includeDeleted bool
		wantParam      string
	}{
		{name: "set", includeDeleted: true, wantParam: "true"},
		{name: "unset", includeDeleted: false, wantParam: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotIncludeDeleted, gotLimit string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotIncludeDeleted = r.URL.Query().Get("include_deleted")
				gotLimit = r.URL.Query().Get("limit")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"items":[],"limit":5}`))
			}))
			defer srv.Close()

			facade := newTestOrganizationsFacade(t, srv)

			_, err := facade.List(context.Background(), models.OrganizationsListOpts{
				PageListOpts: models.PageListOpts{Limit: 5},
				Filters:      models.OrganizationsFilters{IncludeDeleted: tc.includeDeleted},
			})
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			if gotIncludeDeleted != tc.wantParam {
				t.Fatalf("include_deleted query = %q, want %q", gotIncludeDeleted, tc.wantParam)
			}
			// The editor must not clobber existing params.
			if gotLimit != "5" {
				t.Fatalf("limit query = %q, want 5 (editor must preserve existing params)", gotLimit)
			}
		})
	}
}

// TestOrganizationsFacade_CRUD is the Task 2.1.0 write-exemplar milestone:
// Create/Get/Update/Delete round-trip end-to-end over the generated client,
// each normalizing into the public model (or *errors.Error) without leaking
// generated types. Create/Update send the JSON body via the write-facade
// pattern (WithBody + a rewindable reader) so the auth round tripper can replay
// the request after a 401 refresh.
//
//nolint:revive // cognitive-complexity: the CRUD lifecycle plus replay subtests, each with its own httptest server closure and assertions; the complexity is the subtest count, not branching logic. Matches the repo's per-test convention.
func TestOrganizationsFacade_CRUD(t *testing.T) {
	const orgID = "11111111-1111-1111-1111-111111111111"

	t.Run("create", func(t *testing.T) {
		var gotMethod, gotPath, gotContentType, gotBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath, gotContentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + orgID + `","legalName":"Acme","legalDocument":"doc-1"}`))
		}))
		defer srv.Close()

		facade := newTestOrganizationsFacade(t, srv)

		org, err := facade.Create(context.Background(), &models.CreateOrganizationInput{
			LegalName:     "Acme",
			LegalDocument: "doc-1",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q, want POST", gotMethod)
		}
		if gotPath != "/v1/organizations" {
			t.Fatalf("path = %q, want /v1/organizations", gotPath)
		}
		if gotContentType != "application/json" {
			t.Fatalf("content-type = %q, want application/json", gotContentType)
		}
		if !strings.Contains(gotBody, `"legalName":"Acme"`) || !strings.Contains(gotBody, `"legalDocument":"doc-1"`) {
			t.Fatalf("request body = %q, want the marshaled CreateOrganizationInput", gotBody)
		}
		if org.ID != orgID || org.LegalName != "Acme" {
			t.Fatalf("Create returned %+v, want Acme/%s", org, orgID)
		}
	})

	t.Run("get", func(t *testing.T) {
		var gotMethod, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + orgID + `","legalName":"Acme","legalDocument":"doc-1"}`))
		}))
		defer srv.Close()

		facade := newTestOrganizationsFacade(t, srv)

		org, err := facade.Get(context.Background(), orgID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		if gotMethod != http.MethodGet {
			t.Fatalf("method = %q, want GET", gotMethod)
		}
		if gotPath != "/v1/organizations/"+orgID {
			t.Fatalf("path = %q, want /v1/organizations/%s", gotPath, orgID)
		}
		if org.ID != orgID || org.LegalName != "Acme" {
			t.Fatalf("Get returned %+v, want Acme/%s", org, orgID)
		}
	})

	t.Run("update", func(t *testing.T) {
		var gotMethod, gotPath, gotBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"` + orgID + `","legalName":"Acme Renamed","legalDocument":"doc-1"}`))
		}))
		defer srv.Close()

		facade := newTestOrganizationsFacade(t, srv)

		org, err := facade.Update(context.Background(), orgID, &models.UpdateOrganizationInput{
			LegalName: "Acme Renamed",
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}

		if gotMethod != http.MethodPatch {
			t.Fatalf("method = %q, want PATCH", gotMethod)
		}
		if gotPath != "/v1/organizations/"+orgID {
			t.Fatalf("path = %q, want /v1/organizations/%s", gotPath, orgID)
		}
		if !strings.Contains(gotBody, `"legalName":"Acme Renamed"`) {
			t.Fatalf("request body = %q, want the marshaled UpdateOrganizationInput", gotBody)
		}
		if org.LegalName != "Acme Renamed" {
			t.Fatalf("Update returned %+v, want LegalName Acme Renamed", org)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var gotMethod, gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		facade := newTestOrganizationsFacade(t, srv)

		if err := facade.Delete(context.Background(), orgID); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		if gotMethod != http.MethodDelete {
			t.Fatalf("method = %q, want DELETE", gotMethod)
		}
		if gotPath != "/v1/organizations/"+orgID {
			t.Fatalf("path = %q, want /v1/organizations/%s", gotPath, orgID)
		}
	})

	t.Run("error decodes", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.Header().Set("X-Request-ID", "req-crud-err")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","detail":"bad input","status":422}`))
		}))
		defer srv.Close()

		facade := newTestOrganizationsFacade(t, srv)

		_, err := facade.Create(context.Background(), &models.CreateOrganizationInput{
			LegalName:     "Acme",
			LegalDocument: "doc-1",
		})
		if err == nil {
			t.Fatalf("Create against a 422 must return an error")
		}

		var sdkErr *sdkerrors.Error
		if !errors.As(err, &sdkErr) {
			t.Fatalf("error type = %T, want *errors.Error (generated types must not leak)", err)
		}
		if sdkErr.APICode != "LEDGER-0009" {
			t.Fatalf("APICode = %q, want LEDGER-0009", sdkErr.APICode)
		}
		if sdkErr.RequestID != "req-crud-err" {
			t.Fatalf("RequestID = %q, want req-crud-err", sdkErr.RequestID)
		}
	})
}

// TestOrganizationsFacade_WriteReplaySafe is the money-path guard for the
// write-facade pattern: after the server rejects the first attempt with a 401,
// the auth round tripper must be able to rewind and replay the JSON body. A
// non-rewindable body (e.g. a bare struct passed to the typed WithResponse
// path) would make GetBody nil and the replay would go out empty — this test
// asserts the replayed request carries the full body.
func TestOrganizationsFacade_WriteReplaySafe(t *testing.T) {
	var attempts int
	var replayedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"LEDGER-0001","title":"Unauthorized","status":401}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		replayedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111","legalName":"Acme","legalDocument":"doc-1"}`))
	}))
	defer srv.Close()

	facade := newTestOrganizationsFacade(t, srv)

	_, err := facade.Create(context.Background(), &models.CreateOrganizationInput{
		LegalName:     "Acme",
		LegalDocument: "doc-1",
	})
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}

	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2 (401 must trigger a replay)", attempts)
	}
	if !strings.Contains(replayedBody, `"legalName":"Acme"`) {
		t.Fatalf("replayed body = %q, want the full JSON (non-rewindable body dropped it)", replayedBody)
	}
}

// TestOrganizationsFacade_Count HEADs the metrics/count endpoint and reads the
// total from the X-Total-Count header.
func TestOrganizationsFacade_Count(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set(HeaderTotalCount, "7")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n, err := newTestOrganizationsFacade(t, srv).Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if gotMethod != http.MethodHead {
		t.Fatalf("method = %s, want HEAD", gotMethod)
	}
	if gotPath != "/v1/organizations/metrics/count" {
		t.Fatalf("path = %q, want /v1/organizations/metrics/count", gotPath)
	}
	if n != 7 {
		t.Fatalf("count = %d, want 7", n)
	}
}

// TestOrganizationsFacade_CountErrorEmptyBody proves the readCount error path: a
// HEAD error status carries a JSON content-type header with an EMPTY body, and
// still maps to the real status (authorization) — not an internal error.
func TestOrganizationsFacade_CountErrorEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := newTestOrganizationsFacade(t, srv).Count(context.Background())
	if err == nil {
		t.Fatal("expected error on 403 count")
	}
	if sdkerrors.IsInternalError(err) {
		t.Fatalf("403 empty-body count must not map to internal error, got: %v", err)
	}
	if !sdkerrors.IsAuthorizationError(err) {
		t.Fatalf("403 empty-body count must map to authorization error, got: %v", err)
	}
}

// TestOrganizationsFacade_CountMissingHeader surfaces a missing X-Total-Count on
// a 2xx as an error, never a silent zero.
func TestOrganizationsFacade_CountMissingHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // no X-Total-Count header
	}))
	defer srv.Close()

	if _, err := newTestOrganizationsFacade(t, srv).Count(context.Background()); err == nil {
		t.Fatal("expected error on missing X-Total-Count header")
	}
}

// newTestOrganizationsFacade builds the facade over a ledger plane client
// pointed at the test server, with a static Bearer token.
func newTestOrganizationsFacade(t *testing.T, srv *httptest.Server) *organizationsFacade {
	t.Helper()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL + "/v1",
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		httpClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	return newOrganizationsFacade(planes.Ledger, true)
}
