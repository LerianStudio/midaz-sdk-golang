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

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v4/pkg/errors"
)

const metadataIndexesBase = "/v1/settings/metadata-indexes"

// TestMetadataIndexesFacade_List asserts the global list decodes a bare JSON
// array (no pagination envelope) into []models.MetadataIndex and threads the
// entityName through the entity_name query param.
func TestMetadataIndexesFacade_List(t *testing.T) {
	var gotPath, gotEntity string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotEntity = r.URL.Query().Get("entity_name")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"indexName":"idx_customer","entityName":"account","metadataKey":"customer_id","unique":true,"sparse":false},{"indexName":"idx_region","entityName":"account","metadataKey":"region","unique":false,"sparse":true}]`))
	}))
	defer srv.Close()

	indexes, err := newTestMetadataIndexesFacade(t, srv).List(context.Background(), "account")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotPath != metadataIndexesBase {
		t.Fatalf("path = %q, want %q", gotPath, metadataIndexesBase)
	}
	if gotEntity != "account" {
		t.Fatalf("entity_name = %q, want account", gotEntity)
	}
	if len(indexes) != 2 || indexes[0].MetadataKey != "customer_id" || !indexes[0].Unique || indexes[1].MetadataKey != "region" || !indexes[1].Sparse {
		t.Fatalf("List = %+v", indexes)
	}
}

// TestMetadataIndexesFacade_Create round-trips a create over the entity-scoped
// path, asserting the marshaled input body and single-object decode.
func TestMetadataIndexesFacade_Create(t *testing.T) {
	var m, p, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"indexName":"idx_customer","entityName":"account","metadataKey":"customer_id","unique":true,"sparse":false}`))
	}))
	defer srv.Close()

	idx, err := newTestMetadataIndexesFacade(t, srv).Create(context.Background(), "account",
		models.NewCreateMetadataIndexInput("customer_id").WithUnique(true))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	wantPath := metadataIndexesBase + "/entities/account"
	if m != http.MethodPost || p != wantPath {
		t.Fatalf("create req = %s %s, want POST %s", m, p, wantPath)
	}
	if !strings.Contains(body, `"metadataKey":"customer_id"`) || !strings.Contains(body, `"unique":true`) {
		t.Fatalf("body = %q, want marshaled CreateMetadataIndexInput", body)
	}
	if idx.MetadataKey != "customer_id" || !idx.Unique {
		t.Fatalf("Create returned %+v", idx)
	}
}

// TestMetadataIndexesFacade_Delete asserts the entity+key-scoped delete path and
// a clean nil error on 204.
func TestMetadataIndexesFacade_Delete(t *testing.T) {
	var m, p string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m, p = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := newTestMetadataIndexesFacade(t, srv).Delete(context.Background(), "account", "customer_id"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	wantPath := metadataIndexesBase + "/entities/account/key/customer_id"
	if m != http.MethodDelete || p != wantPath {
		t.Fatalf("delete req = %s %s, want DELETE %s", m, p, wantPath)
	}
}

// TestMetadataIndexesFacade_ErrorDecodes asserts RFC 9457 decode with request-ID.
func TestMetadataIndexesFacade_ErrorDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("X-Request-ID", "req-mi-422")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"code":"LEDGER-0009","title":"Invalid","status":422}`))
	}))
	defer srv.Close()

	_, err := newTestMetadataIndexesFacade(t, srv).Create(context.Background(), "account",
		models.NewCreateMetadataIndexInput("customer_id"))
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("error type = %T, want *errors.Error", err)
	}
	if sdkErr.APICode != "LEDGER-0009" || sdkErr.RequestID != "req-mi-422" {
		t.Fatalf("decoded error = %+v", sdkErr)
	}
}

// TestMetadataIndexesFacade_WriteReplaySafe is the money-path 401-replay guard:
// the write body must survive the auth round tripper's post-401 replay.
func TestMetadataIndexesFacade_WriteReplaySafe(t *testing.T) {
	var attempts int
	var replayed string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":"LEDGER-0001","title":"Unauthorized","status":401}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		replayed = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"indexName":"idx_customer","entityName":"account","metadataKey":"customer_id","unique":true}`))
	}))
	defer srv.Close()

	_, err := newTestMetadataIndexesFacade(t, srv).Create(context.Background(), "account",
		models.NewCreateMetadataIndexInput("customer_id").WithUnique(true))
	if err != nil {
		t.Fatalf("Create with one 401 refresh: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("attempts = %d, want >= 2", attempts)
	}
	if !strings.Contains(replayed, `"metadataKey":"customer_id"`) {
		t.Fatalf("replayed body = %q, want full JSON", replayed)
	}
}

func newTestMetadataIndexesFacade(t *testing.T, srv *httptest.Server) *metadataIndexesFacade {
	t.Helper()
	return newMetadataIndexesFacade(newTestLedgerClient(t, srv))
}
