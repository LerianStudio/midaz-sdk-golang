// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package validation_test

import (
	"encoding/json"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFieldError_RedactsSensitiveValues covers Audit C4: the
// safeValue helper now consults [sdkerrors.IsSensitiveFieldName]
// (the shared predicate) rather than only checking for "metadata"
// substrings, so credentials in any sensitive field name are
// replaced with "<redacted>" before rendering.
func TestFieldError_RedactsSensitiveValues(t *testing.T) {
	cases := []struct {
		name       string
		field      string
		value      any
		mustRedact bool
	}{
		{name: "password", field: "password", value: "hunter2", mustRedact: true},
		{name: "client_secret", field: "client_secret", value: "cs_supersecret_42", mustRedact: true},
		{name: "apiKey camelCase", field: "apiKey", value: "ak_live_999", mustRedact: true},
		{name: "X-API-Key", field: "X-API-Key", value: "ak_test_42", mustRedact: true},
		{name: "Authorization", field: "Authorization", value: "Bearer eyJ.tok.sig", mustRedact: true},
		{name: "document", field: "document", value: "12345678900", mustRedact: true},
		{name: "cpf", field: "cpf", value: "111.222.333-44", mustRedact: true},
		{name: "creditCard", field: "creditCard", value: "4111111111111111", mustRedact: true},
		{name: "metadata.user.password", field: "metadata.user.password", value: "secret", mustRedact: true},
		{name: "refresh_token", field: "refresh_token", value: "rt_xyz", mustRedact: true},
		{name: "name (non-sensitive)", field: "name", value: "Alice", mustRedact: false},
		{name: "amount (non-sensitive)", field: "amount", value: 100, mustRedact: false},
		{name: "asset_code (non-sensitive)", field: "assetCode", value: "USD", mustRedact: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fe := &validation.FieldError{
				Field:   tc.field,
				Value:   tc.value,
				Message: "validation failed",
			}

			rendered := fe.Error()

			if tc.mustRedact {
				// "<redacted>" is the placeholder used when the field
				// name is sensitive; we accept either that or the
				// general [REDACTED] marker emitted by the regex pass.
				assert.True(t,
					contains(rendered, "<redacted>") || contains(rendered, "[REDACTED]"),
					"value for %q must be redacted, got: %q", tc.field, rendered)
				assert.NotContains(t, rendered, fmtVal(tc.value),
					"raw value for sensitive field %q must not appear in output", tc.field)
			} else {
				assert.NotContains(t, rendered, "<redacted>",
					"non-sensitive field %q should NOT trip the redactor", tc.field)
			}
		})
	}
}

// TestFieldErrors_RenderingNoLeak covers Audit C7: the FieldErrors
// collection's Error() must always pipe its composed string through
// the package-level redactor. Even if a caller stuffs a credential
// into Message directly (bypassing safeValue), the rendered output
// cannot leak it.
func TestFieldErrors_RenderingNoLeak(t *testing.T) {
	t.Run("credential in message is redacted at collection level", func(t *testing.T) {
		var errs validation.FieldErrors
		errs.Append("username", "validation failed: password=hunter2")
		errs.Append("token", "got authorization=Bearer eyJ.tok.sig")

		rendered := errs.Error()

		assert.NotContains(t, rendered, "hunter2",
			"FieldErrors.Error() must redact credentials in messages")
		assert.NotContains(t, rendered, "eyJ.tok.sig")
		assert.Contains(t, rendered, "[REDACTED]")
	})

	t.Run("rich-context entries also pass through redactor", func(t *testing.T) {
		var errs validation.FieldErrors
		errs.AppendWith("apiKey", "must be a valid key",
			validation.Value("ak_live_supersecret"),
			validation.Constraint("format"),
		)

		rendered := errs.Error()
		assert.NotContains(t, rendered, "ak_live_supersecret",
			"sensitive field value must not appear in error output")
	})
}

// TestFieldError_JSONMarshal_NoLeak covers Audit C4: a naive
// json.Marshal(*FieldError) must not surface Value, Message, or
// Suggestions — those are json:"-".
func TestFieldError_JSONMarshal_NoLeak(t *testing.T) {
	fe := &validation.FieldError{
		Field:       "password",
		Value:       "hunter2",
		Message:     "Authorization: Bearer eyJ.tok.sig is invalid",
		Code:        "INVALID_PASSWORD",
		Constraint:  "format",
		Suggestions: []string{"use a stronger password=hunter999"},
	}

	raw, err := json.Marshal(fe)
	require.NoError(t, err)

	out := string(raw)

	assert.NotContains(t, out, "hunter2", "Value must be json:\"-\"")
	assert.NotContains(t, out, "eyJ.tok.sig", "Message must be json:\"-\"")
	assert.NotContains(t, out, "hunter999", "Suggestions must be json:\"-\"")

	// Safe projection still readable.
	assert.Contains(t, out, `"field":"password"`)
	assert.Contains(t, out, `"code":"INVALID_PASSWORD"`)
	assert.Contains(t, out, `"constraint":"format"`)
}

// TestFieldErrors_Is_NarrowSentinel covers Audit M12: narrow Code-bearing
// sentinels (ErrAssetMismatch, ErrAccountEligibility) no longer
// over-match. Only field errors carrying the same Code count.
func TestFieldErrors_Is_NarrowSentinel(t *testing.T) {
	t.Run("broad ErrValidation matches any non-empty FieldErrors", func(t *testing.T) {
		var fe validation.FieldErrors
		fe.Append("name", "is required")

		require.ErrorIs(t, fe.OrNil(), sdkerrors.ErrValidation,
			"broad ErrValidation must match any FieldErrors")
	})

	t.Run("narrow ErrAssetMismatch only matches when Code aligns", func(t *testing.T) {
		var fe validation.FieldErrors
		fe.Append("name", "is required") // generic, no Code

		// Without a matching Code, the narrow sentinel must NOT match.
		assert.NotErrorIs(t, fe.OrNil(), sdkerrors.ErrAssetMismatch,
			"narrow sentinel must not over-match generic FieldErrors")
	})

	t.Run("narrow ErrAssetMismatch matches when a field carries the Code", func(t *testing.T) {
		var fe validation.FieldErrors
		fe.AppendWith("assetCode", "USD/EUR mismatch",
			validation.Code(string(sdkerrors.CodeAssetMismatch)),
		)

		assert.ErrorIs(t, fe.OrNil(), sdkerrors.ErrAssetMismatch,
			"narrow sentinel must match when a field error has the same Code")
	})
}

// TestFieldError_TruncationAfterRedaction covers the corner case from
// the low-priority cleanup list: tokens in a long string must be
// redacted BEFORE truncation, not after, otherwise a token sitting in
// the truncated suffix could bleed through if the truncation cut it
// short.
func TestFieldError_TruncationAfterRedaction(t *testing.T) {
	long := "noise: " + repeat("a", 200) + " token=secret_xyz_value"

	fe := &validation.FieldError{
		Field:   "data",
		Value:   long,
		Message: "boom",
	}

	rendered := fe.Error()
	assert.NotContains(t, rendered, "secret_xyz_value",
		"redaction must happen before truncation so tail-end credentials are not exposed")
}

// helpers

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func fmtVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}

	return ""
}

func repeat(s string, n int) string {
	return strings.Repeat(s, n)
}
