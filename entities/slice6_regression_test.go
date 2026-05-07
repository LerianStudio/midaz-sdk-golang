package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	crmOrgID          = "550e8400-e29b-41d4-a716-446655440000"
	crmHolderID       = "550e8400-e29b-41d4-a716-446655440001"
	crmAliasID        = "550e8400-e29b-41d4-a716-446655440002"
	crmRelatedPartyID = "550e8400-e29b-41d4-a716-446655440003"
)

func TestSlice6CRMConstructorsCopyTrimAndListNilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))
		assert.NotContains(t, r.URL.Path, "//")
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/holders":
			_, err := w.Write([]byte(`{"items":[],"limit":10,"page":1}`))
			assert.NoError(t, err)
		case "/aliases":
			_, err := w.Write([]byte(`{"items":[],"limit":10,"page":1}`))
			assert.NoError(t, err)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURLs := map[string]string{"crm": server.URL + "/"}
	holders := newHoldersEntity(server.Client(), baseURLs).(*holdersEntity)
	aliases := newAliasesEntity(server.Client(), baseURLs).(*aliasesEntity)
	baseURLs["crm"] = "https://mutated.example.com/v1"

	holdersList, err := holders.ListHolders(nilContext(), "  "+crmOrgID+"  ", models.HoldersListOpts{})
	require.NoError(t, err)
	require.NotNil(t, holdersList.Items)

	aliasesList, err := aliases.ListAliases(nilContext(), crmOrgID, models.AliasesListOpts{})
	require.NoError(t, err)
	require.NotNil(t, aliasesList.Items)
}

func TestSlice6CRMRejectsInvalidScopedIdentifiersBeforeTransport(t *testing.T) {
	called := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	holders := newHoldersEntity(server.Client(), map[string]string{"crm": server.URL}).(*holdersEntity)
	aliases := newAliasesEntity(server.Client(), map[string]string{"crm": server.URL}).(*aliasesEntity)

	_, err := holders.ListHolders(context.Background(), "   ", models.HoldersListOpts{})
	require.ErrorContains(t, err, "organizationID")

	_, err = holders.GetHolder(context.Background(), crmOrgID, "holder-123")
	require.ErrorContains(t, err, "holderID must be a valid UUID")

	_, err = aliases.ListAliases(context.Background(), crmOrgID, models.AliasesListOpts{
		Filters: models.AliasesFilters{HolderID: "holder-123"},
	})
	require.ErrorContains(t, err, "holder_id must be a valid UUID")

	err = aliases.DeleteRelatedParty(context.Background(), crmOrgID, crmHolderID, crmAliasID, "party-123")
	require.ErrorContains(t, err, "relatedPartyID must be a valid UUID")
	assert.False(t, called)
}

func TestSlice6CRMHeadersPreserveOrganizationAndIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))
		assert.Equal(t, "crm-idem", r.Header.Get("X-Idempotency"))
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440010","name":"Jane Doe"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := newHoldersEntity(server.Client(), map[string]string{"crm": server.URL}).(*holdersEntity)

	ctx := sdkctx.WithIdempotencyKey(context.Background(), "crm-idem")
	holderType := models.HolderTypeNaturalPerson

	holder, err := service.CreateHolder(ctx, " "+crmOrgID+" ", &models.CreateHolderInput{
		Type:     &holderType,
		Name:     "Jane Doe",
		Document: "12345678900",
	})
	require.NoError(t, err)
	require.NotNil(t, holder)
}

func TestSlice6NewEntityWithConfigDefaultsMissingCRMURL(t *testing.T) {
	baseURLs := map[string]string{
		"onboarding":  "https://api.example.com/onboarding/v1",
		"transaction": "https://api.example.com/transaction/v1",
	}
	entity, err := NewEntityWithConfig(&mockPluginAuthConfig{httpClient: http.DefaultClient, baseURLs: baseURLs})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/onboarding/v1", entity.baseURLs["crm"])
}

func TestSlice6CRMResultMethodsReturnErrorOnNullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(`null`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	service := newHoldersEntity(server.Client(), map[string]string{"crm": server.URL}).(*holdersEntity)
	_, err := service.GetHolder(context.Background(), crmOrgID, crmHolderID)
	require.ErrorContains(t, err, "null response body")
}
