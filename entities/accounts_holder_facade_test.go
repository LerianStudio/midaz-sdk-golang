// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

const accountsHolderID = "44444444-4444-4444-4444-444444444444"

func newHolderAccountInput() *models.CreateAccountInput {
	return models.NewCreateAccountInput("Borrower position", "BRL", "deposit").
		WithHolderID(accountsHolderID)
}

// TestAccountsV2Facade_CreateCarriesTheHolderLink proves the v2 create puts the
// holder on the wire under holderId, at the /v2 path the server reads it on.
func TestAccountsV2Facade_CreateCarriesTheHolderLink(t *testing.T) {
	var gotPath string

	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `","holderId":"` + accountsHolderID + `"}`))
	}))
	defer srv.Close()

	facade := newAccountsV2Facade(newTestLedgerClient(t, srv), true)

	account, err := facade.Create(context.Background(), accountsOrgID, accountsLedgerID, newHolderAccountInput())
	if err != nil {
		t.Fatalf("V2 Create: %v", err)
	}

	if want := "/v2/organizations/" + accountsOrgID + "/ledgers/" + accountsLedgerID + "/accounts"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}

	if got := gotBody["holderId"]; got != accountsHolderID {
		t.Fatalf("body holderId = %v, want %q", got, accountsHolderID)
	}

	if account.HolderID == nil || *account.HolderID != accountsHolderID {
		t.Fatalf("decoded holderId = %v, want %q", account.HolderID, accountsHolderID)
	}
}

// TestAccountsFacade_CreateRefusesTheHolderLinkOnV1 proves the v1 create never
// reaches the wire with a holder: the server answers 201 and stores no link, so
// sending it would hand the caller a success and an account owned by nobody.
func TestAccountsFacade_CreateRefusesTheHolderLinkOnV1(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `"}`))
	}))
	defer srv.Close()

	_, err := newTestAccountsFacade(t, srv).
		Create(context.Background(), accountsOrgID, accountsLedgerID, newHolderAccountInput())
	if err == nil {
		t.Fatal("v1 Create must refuse a holder link, got success")
	}

	if reached {
		t.Fatal("v1 Create must refuse before touching the wire")
	}

	if !sdkerrors.IsValidationError(err) {
		t.Fatalf("refusal must be a validation error, got: %v", err)
	}

	for _, want := range []string{"holderId", "V2"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal must mention %q, got: %v", want, err)
		}
	}
}

// TestAccountsFacade_CreateWithoutAHolderStillReachesV1 pins the refusal to the
// holder field alone: an ordinary v1 create is untouched by it.
func TestAccountsFacade_CreateWithoutAHolderStillReachesV1(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + accountsAcctID + `"}`))
	}))
	defer srv.Close()

	_, err := newTestAccountsFacade(t, srv).Create(context.Background(), accountsOrgID, accountsLedgerID,
		models.NewCreateAccountInput("Borrower position", "BRL", "deposit"))
	if err != nil {
		t.Fatalf("v1 Create without a holder: %v", err)
	}

	if !reached {
		t.Fatal("v1 Create without a holder must reach the wire")
	}
}
