package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCRMAndResponseModelsCreateHolderInputOmitsUnsetOptionalFields(t *testing.T) {
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

func TestCRMAndResponseModelsCreateTransactionInputDoesNotMarshalHeaderOnlyFields(t *testing.T) {
	input := NewCreateTransactionInput("USD", "10.00")
	input.IdempotencyKey = "idem-123"

	data, err := json.Marshal(input)
	require.NoError(t, err)

	assert.NotContains(t, string(data), "IdempotencyKey")
	assert.NotContains(t, string(data), "idempotency")
}

func TestCRMAndResponseModelsListResponseUnmarshalHTTPPaginationEnvelope(t *testing.T) {
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

func TestCRMAndResponseModelsNilUnmarshalReceiversReturnErrors(t *testing.T) {
	var pagination *Pagination
	require.ErrorContains(t, pagination.UnmarshalJSON([]byte(`{"limit":10}`)), "receiver cannot be nil")

	var list *ListResponse[string]
	require.ErrorContains(t, list.UnmarshalJSON([]byte(`{"items":[]}`)), "receiver cannot be nil")
}

// TestListOptionsNonAlignedOffsetDoesNotEmitPage existed in v2 to
// pin down the silent-conversion rule (offset 15, limit 10 → unaligned →
// drop offset, do NOT synthesize page=2). v3 deletes models.ListOptions
// entirely; per-entity typed Opts (PageListOpts, CursorListOpts) have no
// Offset field, so the misalignment scenario is structurally impossible.
// The test is intentionally removed — its invariant is encoded in the
// type system rather than enforced at runtime.

func TestCRMAndResponseModelsLedgerSettingsExplicitFalseSerializes(t *testing.T) {
	data, err := json.Marshal(NewUpdateLedgerSettingsInput().WithValidateRoutes(false))
	require.NoError(t, err)

	assert.JSONEq(t, `{"accounting":{"validateRoutes":false}}`, string(data))
}

// TestErrorResponseJSONContract historically pinned down the
// JSON shape of the legacy models.ErrorResponse public type. v3
// Batch 8E removed that type — the canonical SDK error shape is
// pkg/errors.Error, populated at the transport boundary by
// ErrorFromHTTPResponseWithDetails. The wire format compatibility
// is now covered by entities.parseErrorResponse, exercised by
// TestErrorRedactionEnvelopeRedaction and the http error-mapping tests.

// TestLegacyListWrappersMarshalEmptyItems existed in v2 to pin down
// JSON empty-items behavior on the legacy list wrapper types. v3 Batch 5C
// deleted AssetRatesResponse; Batch 5F deletes models.Accounts and
// models.Operations. All list responses now ride the unified
// ListResponse[T] generic, covered by TestListResponseZeroValueMarshalUsesEmptyItems
// in model_test.go.

// TestCRMAndResponseModelsUpdateAliasRelatedPartiesReplaceOnRepeatedBuilderCalls pins
// down the documented contract: repeated WithRelatedParties calls REPLACE
// rather than append. Builders with replace semantics are idempotent;
// "I called this twice with two values, why are there four?" was a
// recurring bug report under the previous append behavior.
func TestCRMAndResponseModelsUpdateAliasRelatedPartiesReplaceOnRepeatedBuilderCalls(t *testing.T) {
	first := &RelatedParty{Document: "DOC-1", Name: "A", Role: RelatedPartyRolePrimaryHolder, StartDate: "2026-01-01"}
	second := &RelatedParty{Document: "DOC-2", Name: "B", Role: RelatedPartyRoleResponsibleParty, StartDate: "2026-01-01"}

	input := NewUpdateAliasInput().
		WithRelatedParties([]*RelatedParty{first}).
		WithRelatedParties([]*RelatedParty{second})

	require.Len(t, input.RelatedParties, 1, "second WithRelatedParties call must replace, not append")
	assert.Equal(t, "DOC-2", input.RelatedParties[0].Document)
}

func TestCRMAndResponseModelsUUIDValidatedBodyFields(t *testing.T) {
	badID := "not-a-uuid"
	account := NewCreateAccountInput("Account", "USD", "deposit").WithPortfolioID(badID)
	require.ErrorContains(t, account.Validate(), "portfolioId must be a valid UUID")

	organization := NewCreateOrganizationInput("Org", "DOC-EXAMPLE").WithMetadata(map[string]any{"ok": true})
	organization.ParentOrganizationID = &badID
	require.ErrorContains(t, organization.Validate(), "parentOrganizationId must be a valid UUID")
}

// TestCRMAndResponseModelsUpdateInputsOmitUnsetMetadata locks in the RFC 7396
// merge-patch invariant: when a caller leaves Metadata unset, the
// PATCH body MUST NOT emit "metadata":null. Emitting null would
// silently wipe server-side metadata. UpdateTransactionRouteInput
// in particular regressed this in pre-v3 by lacking both omitempty
// and a custom MarshalJSON guard.
func TestCRMAndResponseModelsUpdateInputsOmitUnsetMetadata(t *testing.T) {
	t.Run("UpdateTransactionRouteInput", func(t *testing.T) {
		data, err := json.Marshal(&UpdateTransactionRouteInput{Title: "Settlement"})
		require.NoError(t, err)
		assert.NotContains(t, string(data), `"metadata"`,
			"unset metadata must NOT be emitted on PATCH (RFC 7396 wipe risk): %s", string(data))
	})
}
