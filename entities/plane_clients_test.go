package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/internal/genledger"
)

// TestNewPlaneClients_ListOrganizationsRoundTrip is the Phase 1 milestone: the
// two-plane builder produces a ledger *ClientWithResponses whose typed
// ListOrganizations round-trips end-to-end against an httptest server, with
// the shared Bearer token injected by the auth round tripper.
func TestNewPlaneClients_ListOrganizationsRoundTrip(t *testing.T) {
	var gotAuth, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	defer srv.Close()

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
	if planes.Ledger == nil || planes.Tracer == nil {
		t.Fatalf("both plane clients must be non-nil, got ledger=%v tracer=%v", planes.Ledger, planes.Tracer)
	}

	limit := "10"
	resp, err := planes.Ledger.ListOrganizationsWithResponse(context.Background(), &genledger.ListOrganizationsParams{Limit: &limit})
	if err != nil {
		t.Fatalf("ListOrganizationsWithResponse: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode(), string(resp.Body))
	}
	if gotAuth != "Bearer tok-1" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer tok-1")
	}
	if gotPath != "/v1/organizations" {
		t.Fatalf("request path = %q, want /v1/organizations", gotPath)
	}
}

// TestNewPlaneClients_TracerAPIKey verifies the tracer plane uses X-API-Key
// when configured, while the ledger plane still uses the shared Bearer.
func TestNewPlaneClients_TracerAPIKey(t *testing.T) {
	var tracerAuth, tracerAPIKey string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracerAuth = r.Header.Get("Authorization")
		tracerAPIKey = r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	planes, err := newPlaneClients(planeClientsConfig{
		ledgerURL: srv.URL + "/v1",
		tracerURL: srv.URL + "/v1",
		auth: authRoundTripperConfig{
			tokenProvider: func(context.Context) (string, error) { return "tok-1", nil },
		},
		tracerAPIKey: "tracer-key-9",
		httpClient:   srv.Client(),
	})
	if err != nil {
		t.Fatalf("newPlaneClients: %v", err)
	}

	if _, err := planes.Tracer.ListRulesWithResponse(context.Background(), nil); err != nil {
		t.Fatalf("ListRulesWithResponse: %v", err)
	}
	if tracerAPIKey != "tracer-key-9" {
		t.Fatalf("tracer X-API-Key = %q, want %q", tracerAPIKey, "tracer-key-9")
	}
	if tracerAuth != "" {
		t.Fatalf("tracer Authorization = %q, want empty (X-API-Key branch)", tracerAuth)
	}
}
