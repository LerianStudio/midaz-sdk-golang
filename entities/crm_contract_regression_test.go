package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	crmOrgID          = "550e8400-e29b-41d4-a716-446655440000"
	crmHolderID       = "550e8400-e29b-41d4-a716-446655440001"
	crmAliasID        = "550e8400-e29b-41d4-a716-446655440002"
	crmRelatedPartyID = "550e8400-e29b-41d4-a716-446655440003"
)

// Epic 5.4: the Holders legacy entity was deleted; this CRM-contract suite now
// covers the surviving CRM entity (Aliases) plus the shared NewEntityWithConfig
// URL defaulting.
func TestCRMContractConstructorsCopyTrimAndListNilContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, crmOrgID, r.Header.Get("X-Organization-Id"))
		assert.NotContains(t, r.URL.Path, "//")
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/aliases":
			_, err := w.Write([]byte(`{"items":[],"limit":10,"page":1}`))
			assert.NoError(t, err)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURLs := map[string]string{"crm": server.URL + "/"}
	aliases := newAliasesEntity(server.Client(), baseURLs).(*aliasesEntity)
	baseURLs["crm"] = "https://mutated.example.com/v1"

	aliasesList, err := aliases.ListAliases(nilContext(), crmOrgID, models.AliasesListOpts{})
	require.NoError(t, err)
	require.NotNil(t, aliasesList.Items)
}

func TestCRMContractRejectsInvalidScopedIdentifiersBeforeTransport(t *testing.T) {
	called := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	aliases := newAliasesEntity(server.Client(), map[string]string{"crm": server.URL}).(*aliasesEntity)

	_, err := aliases.ListAliases(context.Background(), crmOrgID, models.AliasesListOpts{
		Filters: models.AliasesFilters{HolderID: "holder-123"},
	})
	require.ErrorContains(t, err, "holder_id must be a valid UUID")

	err = aliases.DeleteRelatedParty(context.Background(), crmOrgID, crmHolderID, crmAliasID, "party-123")
	require.ErrorContains(t, err, "relatedPartyID must be a valid UUID")
	assert.False(t, called)
}

func TestCRMContractNewEntityWithConfigDefaultsMissingCRMURL(t *testing.T) {
	baseURLs := map[string]string{
		"onboarding":  "https://api.example.com/onboarding/v1",
		"transaction": "https://api.example.com/transaction/v1",
	}
	entity, err := NewEntityWithConfig(&mockPluginAuthConfig{httpClient: http.DefaultClient, baseURLs: baseURLs})
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/onboarding/v1", entity.baseURLs["crm"])
}
