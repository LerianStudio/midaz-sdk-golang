// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const encryptionOrgID = "11111111-1111-1111-1111-111111111111"

func encryptionProvisionPath() string {
	return "/v1/organizations/" + encryptionOrgID + "/encryption/provision"
}

func encryptionStatusPath() string {
	return "/v1/organizations/" + encryptionOrgID + "/encryption/status"
}

// TestEncryptionFacade_Provision_Accepts201 is the KEY RED assert (case a). The
// server returns 201 Created on success. If Provision were routed through the
// generated ...WithResponse parser (ParseProvisionEncryptionResp), its
// JSON200 gate on StatusCode==200 exactly would leave JSON200 nil at 201 and the
// body would fall into the error branch — so a decoded response with no error at
// 201 is only reachable through the raw isSuccess(2xx) path. This test fails
// under the parser routing and passes under the raw path.
func TestEncryptionFacade_Provision_Accepts201(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated) // 201 — the trap
		_, _ = w.Write([]byte(`{"organization_id":"` + encryptionOrgID + `","kek_path":"gcp-kms://keys/k1","aead_primary_key_id":42,"prf_primary_key_id":7,"status":"provisioned"}`))
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).Provision(context.Background(), encryptionOrgID,
		models.NewProvisionEncryptionInput("svc-account", "initial provisioning"))
	if err != nil {
		t.Fatalf("Provision @201: %v", err)
	}

	if method != http.MethodPost || path != encryptionProvisionPath() {
		t.Fatalf("provision req = %s %s, want POST %s", method, path, encryptionProvisionPath())
	}
	// The input marshals to the wire actor/reason fields.
	if body == "" {
		t.Fatalf("provision body empty; want marshaled actor/reason")
	}

	if got.OrganizationID != encryptionOrgID {
		t.Errorf("OrganizationID = %q, want %q", got.OrganizationID, encryptionOrgID)
	}
	if got.KEKPath != "gcp-kms://keys/k1" {
		t.Errorf("KEKPath = %q, want gcp-kms://keys/k1", got.KEKPath)
	}
	if got.AEADPrimaryKeyID != 42 {
		t.Errorf("AEADPrimaryKeyID = %d, want 42", got.AEADPrimaryKeyID)
	}
	if got.PRFPrimaryKeyID != 7 {
		t.Errorf("PRFPrimaryKeyID = %d, want 7", got.PRFPrimaryKeyID)
	}
	if got.Status != "provisioned" {
		t.Errorf("Status = %q, want provisioned", got.Status)
	}
}

// TestEncryptionFacade_Status_ProvisionedFalse covers case (b): a 200 with
// provisioned:false decodes to Provisioned==false with no error — a real
// not-yet-provisioned org, distinct from a 404.
func TestEncryptionFacade_Status_ProvisionedFalse(t *testing.T) {
	var method, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"organization_id":"` + encryptionOrgID + `","provisioned":false}`))
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).GetProvisioningStatus(context.Background(), encryptionOrgID)
	if err != nil {
		t.Fatalf("GetProvisioningStatus @200 false: %v", err)
	}
	if method != http.MethodGet || path != encryptionStatusPath() {
		t.Fatalf("status req = %s %s, want GET %s", method, path, encryptionStatusPath())
	}
	if got.Provisioned {
		t.Fatalf("Provisioned = true, want false")
	}
	if got.OrganizationID != encryptionOrgID {
		t.Errorf("OrganizationID = %q, want %q", got.OrganizationID, encryptionOrgID)
	}
}

// TestEncryptionFacade_Status_ProvisionedTrue covers case (c): a 200 with
// provisioned:true plus a status string populates both.
func TestEncryptionFacade_Status_ProvisionedTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"organization_id":"` + encryptionOrgID + `","provisioned":true,"status":"active"}`))
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).GetProvisioningStatus(context.Background(), encryptionOrgID)
	if err != nil {
		t.Fatalf("GetProvisioningStatus @200 true: %v", err)
	}
	if !got.Provisioned {
		t.Fatalf("Provisioned = false, want true")
	}
	if got.Status != "active" {
		t.Errorf("Status = %q, want active", got.Status)
	}
}

// TestEncryptionFacade_Status_LegacyMode404 covers case (d): a 404 with a BARE
// non-RFC-9457 body (a bare router "Cannot GET" message) maps to *errors.Error
// with StatusCode 404 — NOT an internal error, NOT a panic, NOT a fabricated
// provisioned:false value. This is the legacy-mode signal (envelope encryption
// disabled at the deployment level) and it must stay distinguishable from a
// provisioned:false 200.
func TestEncryptionFacade_Status_LegacyMode404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-legacy-404")
		w.WriteHeader(http.StatusNotFound)
		// Bare non-RFC-9457 body, e.g. a router 404, not a problem+json envelope.
		_, _ = w.Write([]byte(`{"message":"Cannot GET /v1/organizations/x/encryption/status"}`))
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).GetProvisioningStatus(context.Background(), encryptionOrgID)
	if got != nil {
		t.Fatalf("got = %+v, want nil (404 must never fabricate a value)", got)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404 (legacy mode)", sdkErr.StatusCode)
	}
	// It is a real API-mapped error, not an SDK-internal wrapper.
	if sdkErr.Category == sdkerrors.CategoryInternal {
		t.Fatalf("Category = internal, want a status-mapped category for a clean 404")
	}
}

// TestEncryptionFacade_Provision_LegacyMode404 covers case (d) on the write path
// too: a 404 on provision is the same legacy-mode signal, mapped clean.
func TestEncryptionFacade_Provision_LegacyMode404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Cannot POST /v1/organizations/x/encryption/provision"}`))
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).Provision(context.Background(), encryptionOrgID,
		models.NewProvisionEncryptionInput("svc-account", "initial provisioning"))
	if got != nil {
		t.Fatalf("got = %+v, want nil on 404", got)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404 (legacy mode)", sdkErr.StatusCode)
	}
}

// TestEncryptionFacade_Provision_ValidationShortCircuit covers gap 4(a): an
// invalid input (empty Actor/Reason) returns the validation error BEFORE any
// HTTP call. The handler increments a counter that must stay 0.
func TestEncryptionFacade_Provision_ValidationShortCircuit(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).Provision(context.Background(), encryptionOrgID,
		models.NewProvisionEncryptionInput("", ""))
	if got != nil {
		t.Fatalf("got = %+v, want nil on validation failure", got)
	}
	if err == nil {
		t.Fatal("Provision(invalid input) = nil error, want a validation error")
	}
	if hits != 0 {
		t.Fatalf("server hits = %d, want 0 (validation must short-circuit before any HTTP call)", hits)
	}
}

// TestEncryptionFacade_Provision_NonNotFoundStatus covers gap 4(b): a non-404
// error status (409 Conflict with an RFC-9457 body) maps through the general
// non-2xx path to *errors.Error carrying that status — exercising the mapping
// beyond the 404 case.
func TestEncryptionFacade_Provision_NonNotFoundStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Conflict","status":409,"detail":"already provisioned"}`))
	}))
	defer srv.Close()

	got, err := newTestEncryptionFacade(t, srv).Provision(context.Background(), encryptionOrgID,
		models.NewProvisionEncryptionInput("svc", "rotate"))
	if got != nil {
		t.Fatalf("got = %+v, want nil on 409", got)
	}

	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.StatusCode != http.StatusConflict {
		t.Fatalf("StatusCode = %d, want 409", sdkErr.StatusCode)
	}
}

func newTestEncryptionFacade(t *testing.T, srv *httptest.Server) *encryptionFacade {
	t.Helper()
	return newEncryptionFacade(newTestLedgerClient(t, srv), true)
}
