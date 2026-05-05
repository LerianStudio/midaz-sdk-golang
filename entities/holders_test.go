package entities

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHoldersEntity_CreateHolder_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/holders", r.URL.Path)
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))

		var body models.CreateHolderInput
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Jane Doe", body.Name)
		assert.Equal(t, "12345678900", body.Document)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"Jane Doe"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewHoldersEntity(server.Client(), "token", map[string]string{"crm": server.URL}).(*holdersEntity)
	holderType := "NATURAL_PERSON"
	holder, err := service.CreateHolder(context.Background(), crmOrgID, &models.CreateHolderInput{Type: &holderType, Name: "Jane Doe", Document: "12345678900"})

	require.NoError(t, err)
	require.NotNil(t, holder)
	assert.NotNil(t, holder.ID)
	assert.Equal(t, "Jane Doe", *holder.Name)
}

func TestHoldersEntity_UpdateHolder_OmitsNilFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/holders/"+crmHolderID, r.URL.Path)

		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "Jane Updated", body["name"])
		assert.NotContains(t, body, "externalId")
		assert.NotContains(t, body, "addresses")
		assert.NotContains(t, body, "contact")
		assert.NotContains(t, body, "metadata")

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"Jane Updated"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	name := "Jane Updated"
	service := NewHoldersEntity(server.Client(), "token", map[string]string{"crm": server.URL}).(*holdersEntity)
	holder, err := service.UpdateHolder(context.Background(), crmOrgID, crmHolderID, &models.UpdateHolderInput{Name: &name})

	require.NoError(t, err)
	require.NotNil(t, holder)
	assert.Equal(t, "Jane Updated", *holder.Name)
}

func TestHoldersEntity_ValidationErrors(t *testing.T) {
	service := NewHoldersEntity(http.DefaultClient, "token", map[string]string{"crm": "https://crm.example.com/v1"}).(*holdersEntity)

	_, err := service.CreateHolder(context.Background(), crmOrgID, &models.CreateHolderInput{Name: "Jane", Document: "123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")

	_, err = service.UpdateHolder(context.Background(), crmOrgID, crmHolderID, &models.UpdateHolderInput{Metadata: map[string]any{"": "bad"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metadata")
}

func TestHoldersEntity_URLFlagsAndEscaping(t *testing.T) {
	entity := &holdersEntity{baseURLs: map[string]string{"crm": "https://crm.example.com/v1"}}

	assert.Equal(t, "https://crm.example.com/v1/holders/a%2Fb", entity.buildURL("a/b"))
}

func TestHoldersEntity_ListGetDelete_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/holders":
			assert.Equal(t, "external-123", r.URL.Query().Get("external_id"))

			_, err := w.Write([]byte(`{"items":[{"id":"550e8400-e29b-41d4-a716-446655440000","name":"Jane Doe"}],"limit":10,"page":1}`))
			assert.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/holders/"+crmHolderID:
			assert.Equal(t, "true", r.URL.Query().Get("include_deleted"))

			_, err := w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"Jane Doe"}`))
			assert.NoError(t, err)
		case r.Method == http.MethodDelete && r.URL.Path == "/holders/"+crmHolderID:
			assert.Equal(t, "true", r.URL.Query().Get("hard_delete"))
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := NewHoldersEntity(server.Client(), "token", map[string]string{"crm": server.URL}).(*holdersEntity)
	list, err := service.ListHolders(context.Background(), crmOrgID, models.NewListOptions().WithExternalID("external-123"))
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, 10, list.Pagination.Limit)
	assert.Equal(t, 1, list.Pagination.Page)
	assert.Equal(t, 1, list.Pagination.ItemCount)

	holder, err := service.GetHolder(context.Background(), crmOrgID, crmHolderID, true)
	require.NoError(t, err)
	assert.Equal(t, "Jane Doe", *holder.Name)

	require.NoError(t, service.DeleteHolder(context.Background(), crmOrgID, crmHolderID, true))
}
