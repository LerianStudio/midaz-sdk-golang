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

func TestMetadataIndexesEntity_CreateMetadataIndex_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/settings/metadata-indexes/entities/transaction", r.URL.Path)

		var body models.CreateMetadataIndexInput
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		assert.Equal(t, "externalId", body.MetadataKey)

		if !assert.NotNil(t, body.Sparse) {
			http.Error(w, "missing sparse", http.StatusBadRequest)
			return
		}

		assert.True(t, *body.Sparse)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"indexName":"metadata.externalId_1","entityName":"transaction","metadataKey":"externalId","sparse":true}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := NewMetadataIndexesEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	result, err := service.CreateMetadataIndex(context.Background(), "transaction", models.NewCreateMetadataIndexInput("externalId").WithSparse(true))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "externalId", result.MetadataKey)
}

func TestCreateMetadataIndexInput_OmitsSparseWhenUnset(t *testing.T) {
	body, err := json.Marshal(models.NewCreateMetadataIndexInput("externalId"))

	require.NoError(t, err)
	assert.NotContains(t, string(body), "sparse")
}

func TestMetadataIndexValidation(t *testing.T) {
	require.NoError(t, models.NewCreateMetadataIndexInput("valid_key1").Validate())
	require.Error(t, models.NewCreateMetadataIndexInput("1bad").Validate())
	require.Error(t, models.NewCreateMetadataIndexInput("bad.key").Validate())
	require.Error(t, models.NewCreateMetadataIndexInput("bad-key").Validate())
	require.Error(t, models.NewCreateMetadataIndexInput("bad key").Validate())
	require.Error(t, (*models.CreateMetadataIndexInput)(nil).Validate())
	assert.Nil(t, (*models.CreateMetadataIndexInput)(nil).WithUnique(true))
}

func TestMetadataIndexesEntity_RejectsInvalidEntityName(t *testing.T) {
	service := NewMetadataIndexesEntity(http.DefaultClient, "token", map[string]string{"transaction": "https://ledger.example.com/v1"})
	_, err := service.CreateMetadataIndex(context.Background(), "invalid_entity", models.NewCreateMetadataIndexInput("tier"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid entityName")
}

func TestMetadataIndexesEntity_ListAndDelete_RequestConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/settings/metadata-indexes":
			assert.Equal(t, "transaction", r.URL.Query().Get("entity_name"))

			_, err := w.Write([]byte(`[{"indexName":"metadata.tier_1","entityName":"transaction","metadataKey":"tier"}]`))
			assert.NoError(t, err)
		case r.Method == http.MethodDelete && r.URL.Path == "/settings/metadata-indexes/entities/transaction/key/tier":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	service := NewMetadataIndexesEntity(server.Client(), "token", map[string]string{"transaction": server.URL})
	indexes, err := service.ListMetadataIndexes(context.Background(), "transaction")
	require.NoError(t, err)
	require.Len(t, indexes, 1)
	assert.Equal(t, "tier", indexes[0].MetadataKey)

	require.NoError(t, service.DeleteMetadataIndex(context.Background(), "transaction", "tier"))
}
