// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	sdkErr, ok := err.(*sdkerrors.Error)
	if !ok {
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

	return newOrganizationsFacade(planes.Ledger)
}
