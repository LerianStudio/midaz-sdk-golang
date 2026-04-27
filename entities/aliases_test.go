package entities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasesEntity_CreateAlias_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/holders/holder%2F1/aliases", r.URL.EscapedPath())
		assert.Equal(t, "org-123", r.Header.Get("X-Organization-Id"))

		var body models.CreateAliasInput
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "ledger-123", body.LedgerID)
		assert.Equal(t, "account-123", body.AccountID)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"ledgerId":"ledger-123","accountId":"account-123"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewAliasesEntity(server.Client(), "token", map[string]string{"crm": server.URL}).(*aliasesEntity)
	alias, err := service.CreateAlias(context.Background(), "org-123", "holder/1", &models.CreateAliasInput{LedgerID: "ledger-123", AccountID: "account-123"})

	require.NoError(t, err)
	require.NotNil(t, alias)
	assert.Equal(t, "ledger-123", *alias.LedgerID)
	assert.Equal(t, "account-123", *alias.AccountID)
}

func TestAliasesEntity_UpdateAlias_OmitsNilFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/holders/holder-1/aliases/alias-1", r.URL.Path)

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		assert.Equal(t, map[string]any{"risk": "low"}, body["metadata"])
		assert.NotContains(t, body, "bankingDetails")
		assert.NotContains(t, body, "regulatoryFields")
		assert.NotContains(t, body, "relatedParties")

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"ledgerId":"ledger-123","accountId":"account-123","metadata":{"risk":"low"}}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewAliasesEntity(server.Client(), "token", map[string]string{"crm": server.URL}).(*aliasesEntity)
	alias, err := service.UpdateAlias(context.Background(), "org-123", "holder-1", "alias-1", &models.UpdateAliasInput{Metadata: map[string]any{"risk": "low"}})

	require.NoError(t, err)
	require.NotNil(t, alias)
	assert.Equal(t, "low", alias.Metadata["risk"])
}

func TestAliasesEntity_ValidationErrors(t *testing.T) {
	service := NewAliasesEntity(http.DefaultClient, "token", map[string]string{"crm": "https://crm.example.com/v1"}).(*aliasesEntity)

	_, err := service.CreateAlias(context.Background(), "org-123", "holder-1", &models.CreateAliasInput{AccountID: "account-123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledgerId is required")

	_, err = service.UpdateAlias(context.Background(), "org-123", "holder-1", "alias-1", &models.UpdateAliasInput{RelatedParties: []*models.RelatedParty{{Role: "INVALID"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "document is required")
}

func TestAliasesEntity_DeleteRelatedParty_EscapesAllIDs(t *testing.T) {
	entity := &aliasesEntity{baseURLs: map[string]string{"crm": "https://crm.example.com/v1"}}
	endpoint := entity.aliasURL("holder/1", "alias/2")

	assert.Equal(t, "https://crm.example.com/v1/holders/holder%2F1/aliases/alias%2F2", endpoint)
}

func TestAliasesEntity_ListGetDelete_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "org-123", r.Header.Get("X-Organization-Id"))
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aliases":
			assert.Equal(t, "holder-123", r.URL.Query().Get("holder_id"))

			_, err := w.Write([]byte(`{"items":[{"ledgerId":"ledger-123","accountId":"account-123"}],"limit":10,"page":1}`))
			assert.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/holders/holder-123/aliases/alias-123":
			assert.Equal(t, "true", r.URL.Query().Get("include_deleted"))

			_, err := w.Write([]byte(`{"ledgerId":"ledger-123","accountId":"account-123"}`))
			assert.NoError(t, err)
		case r.Method == http.MethodDelete && r.URL.Path == "/holders/holder-123/aliases/alias-123":
			assert.Equal(t, "true", r.URL.Query().Get("hard_delete"))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/holders/holder-123/aliases/alias-123/related-parties/party-123":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := NewAliasesEntity(server.Client(), "token", map[string]string{"crm": server.URL}).(*aliasesEntity)
	list, err := service.ListAliases(context.Background(), "org-123", models.NewListOptions().WithHolderID("holder-123"))
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	alias, err := service.GetAlias(context.Background(), "org-123", "holder-123", "alias-123", true)
	require.NoError(t, err)
	assert.Equal(t, "ledger-123", *alias.LedgerID)

	require.NoError(t, service.DeleteAlias(context.Background(), "org-123", "holder-123", "alias-123", true))
	require.NoError(t, service.DeleteRelatedParty(context.Background(), "org-123", "holder-123", "alias-123", "party-123"))
}
