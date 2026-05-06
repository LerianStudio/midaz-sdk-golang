package errors_test

import (
	"errors"
	"net/http"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typedNilError struct{}

func (*typedNilError) Error() string {
	panic("typed nil Error method must not be called")
}

func TestSlice3Redaction_RedactsPIIAndFinancialFields(t *testing.T) {
	raw := errors.New("document=12345678900 legal_document=98765432100 external_id=ext-123 banking_details_account=000111 banking_details_iban=IBAN123 metadata.secret=value related_party_document=11122233344 regulatory_fields_participant_document=55566677788")

	outputs := []string{
		sdkerrors.FormatErrorForDisplay(raw),
		sdkerrors.FormatErrorDetails(raw),
		sdkerrors.FormatOperationError(raw, "CreateHolder"),
		sdkerrors.FormatUnifiedTransactionError(raw, "CreateHolder"),
	}

	for _, output := range outputs {
		assert.NotContains(t, output, "12345678900")
		assert.NotContains(t, output, "98765432100")
		assert.NotContains(t, output, "ext-123")
		assert.NotContains(t, output, "000111")
		assert.NotContains(t, output, "IBAN123")
		assert.NotContains(t, output, "value")
		assert.NotContains(t, output, "11122233344")
		assert.NotContains(t, output, "55566677788")
		assert.Contains(t, output, "[REDACTED]")
	}
}

func TestSlice3Constructors_WithTypedNilError_DoNotPanic(t *testing.T) {
	var typedNil *typedNilError

	var err error = typedNil

	constructors := []struct {
		name      string
		construct func() *sdkerrors.Error
	}{
		{name: "validation", construct: func() *sdkerrors.Error { return sdkerrors.NewValidationError("op", "invalid", err) }},
		{name: "invalid input", construct: func() *sdkerrors.Error { return sdkerrors.NewInvalidInputError("op", err) }},
		{name: "authentication", construct: func() *sdkerrors.Error { return sdkerrors.NewAuthenticationError("op", "auth", err) }},
		{name: "authorization", construct: func() *sdkerrors.Error { return sdkerrors.NewAuthorizationError("op", "forbidden", err) }},
		{name: "cancellation", construct: func() *sdkerrors.Error { return sdkerrors.NewCancellationError("op", err) }},
		{name: "network", construct: func() *sdkerrors.Error { return sdkerrors.NewNetworkError("op", err) }},
		{name: "internal", construct: func() *sdkerrors.Error { return sdkerrors.NewInternalError("op", err) }},
		{name: "unprocessable", construct: func() *sdkerrors.Error { return sdkerrors.NewUnprocessableError("op", "resource", err) }},
		{name: "insufficient balance", construct: func() *sdkerrors.Error { return sdkerrors.NewInsufficientBalanceError("op", "acc-1", err) }},
		{name: "account eligibility", construct: func() *sdkerrors.Error { return sdkerrors.NewAccountEligibilityError("op", "acc-1", err) }},
	}

	for _, tt := range constructors {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				got := tt.construct()
				require.NotNil(t, got)
				assert.NoError(t, got.Err)
			})
		})
	}
}

func TestSlice3GetErrorDetails_ExposesStructuredAPIEnvelope(t *testing.T) {
	err := &sdkerrors.Error{
		Category:   sdkerrors.CategoryValidation,
		Code:       sdkerrors.CodeValidation,
		APICode:    "MIDAZ-0042",
		Title:      "Validation failed",
		Message:    "invalid payload",
		EntityType: "account",
		Fields:     []string{"document"},
		Details:    map[string]any{"reason": "invalid document"},
		StatusCode: http.StatusBadRequest,
		RequestID:  "req-123",
	}

	details := sdkerrors.GetErrorDetails(err)

	assert.Equal(t, "MIDAZ-0042", details.APICode)
	assert.Equal(t, "Validation failed", details.Title)
	assert.Equal(t, "account", details.EntityType)
	assert.Equal(t, []string{"document"}, details.Fields)
	assert.Equal(t, "invalid document", details.Details["reason"])
	assert.Equal(t, "req-123", details.RequestID)
}
