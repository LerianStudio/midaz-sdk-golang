package entities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/sdkctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasesEntity_CreateAlias_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/holders/"+crmHolderID+"/aliases", r.URL.EscapedPath())
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))

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

	service := newAliasesEntity(server.Client(), map[string]string{"crm": server.URL}).(*aliasesEntity)
	alias, err := service.CreateAlias(context.Background(), crmOrgID, crmHolderID, &models.CreateAliasInput{LedgerID: "ledger-123", AccountID: "account-123"})

	require.NoError(t, err)
	require.NotNil(t, alias)
	assert.Equal(t, "ledger-123", *alias.LedgerID)
	assert.Equal(t, "account-123", *alias.AccountID)
}

func TestAliasesEntity_UpdateAlias_OmitsNilFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/holders/"+crmHolderID+"/aliases/"+crmAliasID, r.URL.Path)

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

	service := newAliasesEntity(server.Client(), map[string]string{"crm": server.URL}).(*aliasesEntity)
	alias, err := service.UpdateAlias(context.Background(), crmOrgID, crmHolderID, crmAliasID, &models.UpdateAliasInput{Metadata: map[string]any{"risk": "low"}})

	require.NoError(t, err)
	require.NotNil(t, alias)
	assert.Equal(t, "low", alias.Metadata["risk"])
}

func TestAliasesEntity_ValidationErrors(t *testing.T) {
	service := newAliasesEntity(http.DefaultClient, map[string]string{"crm": "https://crm.example.com/v1"}).(*aliasesEntity)

	_, err := service.CreateAlias(context.Background(), crmOrgID, crmHolderID, &models.CreateAliasInput{AccountID: "account-123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ledgerId is required")

	_, err = service.UpdateAlias(context.Background(), crmOrgID, crmHolderID, crmAliasID, &models.UpdateAliasInput{RelatedParties: []*models.RelatedParty{{Role: "INVALID"}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "document is required")
}

func TestAliasesEntity_DeleteRelatedParty_EscapesAllIDs(t *testing.T) {
	entity := &aliasesEntity{serviceEntity: serviceEntity{baseURLs: map[string]string{"crm": "https://crm.example.com/v1"}}}
	endpoint := entity.aliasURL("holder/1", "alias/2")

	assert.Equal(t, "https://crm.example.com/v1/holders/holder%2F1/aliases/alias%2F2", endpoint)
}

func TestAliasesEntity_ListGetDelete_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/aliases":
			assert.Equal(t, crmHolderID, r.URL.Query().Get("holder_id"))

			_, err := w.Write([]byte(`{"items":[{"ledgerId":"ledger-123","accountId":"account-123"}],"limit":10,"page":1}`))
			assert.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/holders/"+crmHolderID+"/aliases/"+crmAliasID:
			assert.Equal(t, "true", r.URL.Query().Get("include_deleted"))

			_, err := w.Write([]byte(`{"ledgerId":"ledger-123","accountId":"account-123"}`))
			assert.NoError(t, err)
		case r.Method == http.MethodDelete && r.URL.Path == "/holders/"+crmHolderID+"/aliases/"+crmAliasID:
			assert.Equal(t, "true", r.URL.Query().Get("hard_delete"))
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/holders/"+crmHolderID+"/aliases/"+crmAliasID+"/related-parties/"+crmRelatedPartyID:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := newAliasesEntity(server.Client(), map[string]string{"crm": server.URL}).(*aliasesEntity)
	list, err := service.ListAliases(context.Background(), crmOrgID, models.AliasesListOpts{
		Filters: models.AliasesFilters{HolderID: crmHolderID},
	})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, 10, list.Pagination.Limit)
	assert.Equal(t, 1, list.Pagination.Page)
	assert.Equal(t, 1, list.Pagination.ItemCount)

	getCtx := sdkctx.WithIncludeDeleted(context.Background(), true)
	alias, err := service.GetAlias(getCtx, crmOrgID, crmHolderID, crmAliasID)
	require.NoError(t, err)
	assert.Equal(t, "ledger-123", *alias.LedgerID)

	deleteCtx := sdkctx.WithHardDelete(context.Background(), true)
	require.NoError(t, service.DeleteAlias(deleteCtx, crmOrgID, crmHolderID, crmAliasID))
	require.NoError(t, service.DeleteRelatedParty(context.Background(), crmOrgID, crmHolderID, crmAliasID, crmRelatedPartyID))
}
