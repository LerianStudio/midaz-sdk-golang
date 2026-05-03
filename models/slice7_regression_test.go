package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlice7CreateHolderInputOmitsUnsetOptionalFields(t *testing.T) {
	holderType := HolderTypeNaturalPerson
	data, err := json.Marshal(&CreateHolderInput{
		Type:     &holderType,
		Name:     "Jane Doe",
		Document: "DOC-EXAMPLE-001",
	})
	require.NoError(t, err)

	assert.JSONEq(t, `{"type":"NATURAL_PERSON","name":"Jane Doe","document":"DOC-EXAMPLE-001"}`, string(data))
	assert.NotContains(t, string(data), "externalId")
	assert.NotContains(t, string(data), "addresses")
	assert.NotContains(t, string(data), "contact")
	assert.NotContains(t, string(data), "metadata")
}

func TestSlice7CreateTransactionInputDoesNotMarshalHeaderOnlyFields(t *testing.T) {
	input := NewCreateTransactionInput("USD", "10.00")
	input.IdempotencyKey = "idem-123"
	input.ExternalID = "external-123"

	data, err := json.Marshal(input)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "IdempotencyKey")
	assert.NotContains(t, string(data), "idempotency")
	assert.NotContains(t, string(data), "ExternalID")
	assert.NotContains(t, string(data), "external")
}

func TestSlice7ListResponseUnmarshalHTTPPaginationEnvelope(t *testing.T) {
	var response ListResponse[map[string]any]

	err := json.Unmarshal([]byte(`{
		"items":[{"id":"one"}],
		"http.Pagination":{"limit":10,"page":2,"next_cursor":"next-123"}
	}`), &response)
	require.NoError(t, err)

	assert.Len(t, response.Items, 1)
	assert.Equal(t, 10, response.Pagination.Limit)
	assert.Equal(t, 2, response.Pagination.Page)
	assert.Equal(t, "next-123", response.Pagination.NextCursor)
	assert.Equal(t, 1, response.Pagination.ItemCount)
}

func TestSlice7NilUnmarshalReceiversReturnErrors(t *testing.T) {
	var pagination *Pagination
	require.ErrorContains(t, pagination.UnmarshalJSON([]byte(`{"limit":10}`)), "receiver cannot be nil")

	var list *ListResponse[string]
	require.ErrorContains(t, list.UnmarshalJSON([]byte(`{"items":[]}`)), "receiver cannot be nil")
}

func TestSlice7ListOptionsNonAlignedOffsetDoesNotEmitPage(t *testing.T) {
	params := NewListOptions().WithLimit(10).WithOffset(15).ToQueryParams()

	assert.NotContains(t, params, QueryParamPage)
	assert.NotContains(t, params, QueryParamOffset)
}

func TestSlice7LedgerSettingsExplicitFalseSerializes(t *testing.T) {
	data, err := json.Marshal(NewUpdateLedgerSettingsInput().WithValidateRoutes(false))
	require.NoError(t, err)

	assert.JSONEq(t, `{"accounting":{"validateRoutes":false}}`, string(data))
}

func TestSlice7ErrorResponseJSONContract(t *testing.T) {
	data := []byte(`{"code":"ERR_INVALID_INPUT","title":"Bad Request","message":"validation failed","entityType":"Account","fields":{"type":"must not be external"}}`)

	var response ErrorResponse
	require.NoError(t, json.Unmarshal(data, &response))

	assert.Equal(t, "ERR_INVALID_INPUT", response.Code)
	assert.Equal(t, "Bad Request", response.Title)
	assert.Equal(t, "validation failed", response.Message)
	assert.Equal(t, "Account", response.EntityType)
	assert.Equal(t, "must not be external", response.Fields["type"])
}

func TestSlice7LegacyListWrappersMarshalEmptyItems(t *testing.T) {
	assetRates, err := json.Marshal(AssetRatesResponse{})
	require.NoError(t, err)
	assert.Contains(t, string(assetRates), `"items":[]`)

	accounts, err := json.Marshal(Accounts{})
	require.NoError(t, err)
	assert.Contains(t, string(accounts), `"items":[]`)

	operations, err := json.Marshal(Operations{})
	require.NoError(t, err)
	assert.Contains(t, string(operations), `"items":[]`)
}

// TestSlice7UpdateAliasRelatedPartiesReplaceOnRepeatedBuilderCalls pins
// down the documented contract: repeated WithRelatedParties calls REPLACE
// rather than append. Builders with replace semantics are idempotent;
// "I called this twice with two values, why are there four?" was a
// recurring bug report under the previous append behavior.
func TestSlice7UpdateAliasRelatedPartiesReplaceOnRepeatedBuilderCalls(t *testing.T) {
	first := &RelatedParty{Document: "DOC-1", Name: "A", Role: RelatedPartyRolePrimaryHolder, StartDate: "2026-01-01"}
	second := &RelatedParty{Document: "DOC-2", Name: "B", Role: RelatedPartyRoleResponsibleParty, StartDate: "2026-01-01"}

	input := NewUpdateAliasInput().
		WithRelatedParties([]*RelatedParty{first}).
		WithRelatedParties([]*RelatedParty{second})

	require.Len(t, input.RelatedParties, 1, "second WithRelatedParties call must replace, not append")
	assert.Equal(t, "DOC-2", input.RelatedParties[0].Document)
}

func TestSlice7UUIDValidatedBodyFields(t *testing.T) {
	badID := "not-a-uuid"
	account := NewCreateAccountInput("Account", "USD", "deposit").WithPortfolioID(badID)
	require.ErrorContains(t, account.Validate(), "portfolioId must be a valid UUID")

	organization := NewCreateOrganizationInput("Org", "DOC-EXAMPLE").WithMetadata(map[string]any{"ok": true})
	organization.ParentOrganizationID = &badID
	require.ErrorContains(t, organization.Validate(), "parentOrganizationId must be a valid UUID")
}
