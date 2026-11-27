package errors_test

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Subtest names carry the scenario names of the partner-refusal-classification
// spec so each scenario can be traced to its test.

// libAuthActorDenialBody builds the body lib-auth answers for an actor refusal
// (`auth/middleware/actor.go`, actorDenial): a code / title / message object.
func libAuthActorDenialBody(code, title, message string) []byte {
	return fmt.Appendf(nil, `{"code":%q,"title":%q,"message":%q}`, code, title, message)
}

// libAuthPartnerRefusals are the four lib-auth actor refusals (lib-auth
// v5.22.0), in code order, with the reason each one maps to.
var libAuthPartnerRefusals = []struct {
	code    string
	title   string
	message string
	reason  sdkerrors.PartnerRefusalReason
}{
	{
		code:    "AUT-1016",
		title:   "Partner Of Another Tenant",
		message: "the partner this request was made on behalf of belongs to another tenant",
		reason:  "other_tenant",
	},
	{
		code:    "AUT-1017",
		title:   "Partner Suspended",
		message: "the partner this request was made on behalf of is suspended",
		reason:  "suspended",
	},
	{
		code:    "AUT-1018",
		title:   "Partner Credential No Longer Valid",
		message: "the partner this request was made on behalf of is outside its validity period or its credential is no longer valid",
		reason:  "no_longer_valid",
	},
	{
		code:    "AUT-1019",
		title:   "Partner Not Allowed",
		message: "the partner this request was made on behalf of is not allowed to perform it",
		reason:  "not_allowed",
	},
}

// decodedError decodes a server error body the way the facade does.
func decodedError(status int, code string) error {
	return sdkerrors.DecodeProblemJSON(status, libAuthActorDenialBody(code, "Refused", "refused"), "req-1")
}

func TestIsPartnerRefusal(t *testing.T) {
	t.Run("recusa-de-partner-reconhecida", func(t *testing.T) {
		for _, refusal := range libAuthPartnerRefusals {
			t.Run(refusal.code, func(t *testing.T) {
				err := sdkerrors.DecodeProblemJSON(http.StatusForbidden,
					libAuthActorDenialBody(refusal.code, refusal.title, refusal.message), "req-1")

				assert.True(t, sdkerrors.IsPartnerRefusal(err))
			})
		}
	})

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "recusa-da-propria-credencial-nao-e-partner/AUT-1020", err: decodedError(http.StatusForbidden, "AUT-1020"), want: false},
		{name: "credencial-direta-de-partner-fora/AUT-1009", err: decodedError(http.StatusUnauthorized, "AUT-1009"), want: false},
		{name: "credencial-direta-de-partner-fora/AUT-1010", err: decodedError(http.StatusUnauthorized, "AUT-1010"), want: false},
		{
			name: "erro-embrulhado-reconhecido",
			err:  fmt.Errorf("create transaction: %w", decodedError(http.StatusForbidden, "AUT-1019")),
			want: true,
		},
		{name: "erro-sem-codigo-ou-nulo/nil", err: nil, want: false},
		{name: "erro-sem-codigo-ou-nulo/not-an-sdk-error", err: stderrors.New("AUT-1019"), want: false},
		{
			name: "erro-sem-codigo-ou-nulo/sdk-error-without-api-code",
			err:  sdkerrors.ErrorFromHTTPResponse(http.StatusForbidden, "req-1", "forbidden", "", "", ""),
			want: false,
		},
		{name: "sufixo-igual-de-outro-servico-nao-casa/CRM-1016", err: decodedError(http.StatusForbidden, "CRM-1016"), want: false},
		{name: "sufixo-igual-de-outro-servico-nao-casa/1016", err: decodedError(http.StatusForbidden, "1016"), want: false},
		{name: "status-nao-condiciona/401", err: decodedError(http.StatusUnauthorized, "AUT-1017"), want: true},
		{name: "status-nao-condiciona/409", err: decodedError(http.StatusConflict, "AUT-1017"), want: true},
		{name: "status-nao-condiciona/500", err: decodedError(http.StatusInternalServerError, "AUT-1017"), want: true},
		{name: "whole code surrounded by spaces", err: &sdkerrors.Error{APICode: " AUT-1016 "}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sdkerrors.IsPartnerRefusal(tt.err))
		})
	}
}

func TestGetPartnerRefusalReason(t *testing.T) {
	t.Run("motivo-por-codigo", func(t *testing.T) {
		for _, refusal := range libAuthPartnerRefusals {
			t.Run(refusal.code, func(t *testing.T) {
				err := sdkerrors.DecodeProblemJSON(http.StatusForbidden,
					libAuthActorDenialBody(refusal.code, refusal.title, refusal.message), "req-1")

				reason, ok := sdkerrors.GetPartnerRefusalReason(err)

				assert.True(t, ok)
				assert.Equal(t, refusal.reason, reason)
			})
		}
	})

	tests := []struct {
		name string
		err  error
	}{
		{name: "motivo-ausente-quando-nao-e-recusa/AUT-1020", err: decodedError(http.StatusForbidden, "AUT-1020")},
		{name: "motivo-ausente-quando-nao-e-recusa/nil", err: nil},
		{name: "motivo-ausente-quando-nao-e-recusa/not-an-sdk-error", err: stderrors.New("AUT-1016")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, ok := sdkerrors.GetPartnerRefusalReason(tt.err)

			assert.False(t, ok)
			assert.Empty(t, reason)
		})
	}
}

// TestPartnerRefusalConstants pins the constant values to lib-auth
// `auth/middleware/actor.go` (v5.22.0) and to the documented reason strings.
func TestPartnerRefusalConstants(t *testing.T) {
	assert.Equal(t, "AUT-1016", sdkerrors.APICodePartnerOtherTenant)
	assert.Equal(t, "AUT-1017", sdkerrors.APICodePartnerSuspended)
	assert.Equal(t, "AUT-1018", sdkerrors.APICodePartnerNoLongerValid)
	assert.Equal(t, "AUT-1019", sdkerrors.APICodePartnerNotAllowed)

	assert.Equal(t, sdkerrors.PartnerRefusalReason("other_tenant"), sdkerrors.PartnerRefusalOtherTenant)
	assert.Equal(t, sdkerrors.PartnerRefusalReason("suspended"), sdkerrors.PartnerRefusalSuspended)
	assert.Equal(t, sdkerrors.PartnerRefusalReason("no_longer_valid"), sdkerrors.PartnerRefusalNoLongerValid)
	assert.Equal(t, sdkerrors.PartnerRefusalReason("not_allowed"), sdkerrors.PartnerRefusalNotAllowed)
}

// TestPartnerRefusalKeepsExistingClassification guards that recognizing a
// partner refusal changes nothing in how the SDK already classifies the error.
func TestPartnerRefusalKeepsExistingClassification(t *testing.T) {
	t.Run("classificacao-existente-inalterada", func(t *testing.T) {
		refusal := libAuthPartnerRefusals[3]
		require.Equal(t, "AUT-1019", refusal.code)

		err := sdkerrors.DecodeProblemJSON(http.StatusForbidden,
			libAuthActorDenialBody(refusal.code, refusal.title, refusal.message), "req-1")

		var sdkErr *sdkerrors.Error
		require.True(t, stderrors.As(err, &sdkErr))

		assert.Equal(t, sdkerrors.ErrorCode("permission_error"), sdkErr.Code)
		assert.Equal(t, sdkerrors.ErrorCategory("authorization"), sdkErr.Category)
		assert.Equal(t, http.StatusForbidden, sdkErr.StatusCode)
		assert.Equal(t, "AUT-1019", sdkErr.APICode)
		assert.Equal(t, "Partner Not Allowed", sdkErr.Title)
		assert.True(t, sdkerrors.IsAuthorizationError(err))
		assert.False(t, sdkerrors.IsAuthenticationError(err))
	})
}
