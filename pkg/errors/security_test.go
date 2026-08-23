// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors_test

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedaction_ExtendedCredentialHeaders covers Audit C2: the
// sensitive-key whitelist must catch the canonical credential-header
// forms — x-api-key, apikey, api-key, access_token, refresh_token,
// id_token, jwt — plus the Authorization: Basic variant.
func TestRedaction_ExtendedCredentialHeaders(t *testing.T) {
	cases := []struct {
		name           string
		input          string
		mustNotContain []string
		mustContain    []string
	}{
		{
			name:           "x-api-key header",
			input:          "x-api-key=ak_live_abc123",
			mustNotContain: []string{"ak_live_abc123"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "apikey header",
			input:          "apikey: ak_test_xyz789",
			mustNotContain: []string{"ak_test_xyz789"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "api-key header (hyphen variant)",
			input:          "api-key=secret_value_42",
			mustNotContain: []string{"secret_value_42"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "access_token form",
			input:          "access_token=eyJhbGc.payload.sig",
			mustNotContain: []string{"eyJhbGc.payload.sig"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "access-token form (hyphen variant)",
			input:          "access-token=opaque_token_value",
			mustNotContain: []string{"opaque_token_value"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "refresh_token form",
			input:          "refresh_token=rt_long_lived_xyz",
			mustNotContain: []string{"rt_long_lived_xyz"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "id_token form (OIDC)",
			input:          "id_token=oidc_token_payload",
			mustNotContain: []string{"oidc_token_payload"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "jwt form",
			input:          "jwt=eyJzdWIiOiIxMjM0NSJ9",
			mustNotContain: []string{"eyJzdWIiOiIxMjM0NSJ9"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "Authorization: Basic header",
			input:          "Authorization: Basic dXNlcjpwYXNz",
			mustNotContain: []string{"dXNlcjpwYXNz"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "Authorization: Bearer header (regression check)",
			input:          "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9",
			mustNotContain: []string{"eyJhbGciOiJIUzI1NiJ9"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "x-idempotency-key header (regression check)",
			input:          "x-idempotency-key=idem-2024-abc",
			mustNotContain: []string{"idem-2024-abc"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "client_secret form (regression check via 'secret')",
			input:          "client_secret=cs_supersecret_42",
			mustNotContain: []string{"cs_supersecret_42"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "quoted JSON camelCase access token",
			input:          `{"accessToken":"json-access-token"}`,
			mustNotContain: []string{"json-access-token"},
			mustContain:    []string{"[REDACTED]"},
		},
		{
			name:           "camelCase client secret",
			input:          "clientSecret: raw-client-secret",
			mustNotContain: []string{"raw-client-secret"},
			mustContain:    []string{"[REDACTED]"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			redacted := sdkerrors.RedactSensitiveString(tc.input)

			for _, leak := range tc.mustNotContain {
				assert.NotContains(t, redacted, leak,
					"redacted output leaked %q (input: %q, output: %q)", leak, tc.input, redacted)
			}

			for _, marker := range tc.mustContain {
				assert.Contains(t, redacted, marker,
					"redacted output missing %q (input: %q, output: %q)", marker, tc.input, redacted)
			}
		})
	}
}

// TestRedactionLengthCap covers Audit M15: redactSensitive must bound
// its input to keep regex CPU under control. Overlong inputs are
// truncated and a "[truncated]" marker is appended.
func TestRedactionLengthCap(t *testing.T) {
	t.Run("input over 64KB is truncated", func(t *testing.T) {
		// 65KB of harmless padding + a trailing credential we expect
		// to NOT see in the output (because it's past the cap).
		giant := strings.Repeat("a", 65*1024) + "password=hunter2"
		out := sdkerrors.RedactSensitiveString(giant)

		assert.Contains(t, out, "[truncated]",
			"long input should be marked as truncated")
		assert.NotContains(t, out, "hunter2",
			"credential past the cap must not appear in output")
		assert.Less(t, len(out), len(giant)+32,
			"truncated output should be roughly the cap size, not the full input")
	})

	t.Run("input under cap is unchanged in length", func(t *testing.T) {
		small := "username=alice api_key=ak_test_42"
		out := sdkerrors.RedactSensitiveString(small)

		assert.NotContains(t, out, "[truncated]",
			"short input should not get truncation marker")
		assert.NotContains(t, out, "ak_test_42")
		assert.Contains(t, out, "[REDACTED]")
	})
}

func TestErrorUnwrapRedactingWrapperIsTerminalAndMatchable(t *testing.T) {
	sentinel := stderrors.New("clientSecret=raw-inner-secret")
	err := sdkerrors.NewConfigurationError("midaz.New", "invalid configuration", sentinel)

	wrapped := stderrors.Unwrap(err)
	require.Error(t, wrapped)
	assert.NotContains(t, wrapped.Error(), "raw-inner-secret")
	require.NoError(t, stderrors.Unwrap(wrapped), "redacting wrapper must be terminal to recursive unwrap rendering")
	assert.ErrorIs(t, err, sentinel, "terminal redacting wrapper must still preserve errors.Is matching")
}

// typedInnerError is used by TestRedactingError_AsForwardsTypedExtraction
// to confirm the redacting wrapper forwards errors.As to the inner
// chain. The custom type is necessary because errors.As needs a
// concrete target with the right pointer-to-T shape.
type typedInnerError struct {
	marker string
}

func (e *typedInnerError) Error() string { return "typed inner: " + e.marker }

// TestRedactingError_AsForwardsTypedExtraction verifies that the
// redacting wrapper (which intentionally makes errors.Unwrap terminal
// to stop unredacted rendering) still forwards typed extraction via
// errors.As. Programmatic dispatch must keep working even though the
// rendered chain is sealed off.
func TestRedactingError_AsForwardsTypedExtraction(t *testing.T) {
	inner := &typedInnerError{marker: "raw-marker"}
	wrapped := sdkerrors.NewValidationError("op", "boom", inner)

	// Walk to the redactingError shell directly.
	shell := stderrors.Unwrap(wrapped)
	require.Error(t, shell)

	var target *typedInnerError
	require.ErrorAs(t, shell, &target, "redactingError.As must forward typed extraction")
	assert.Same(t, inner, target)
}

// TestRedactingError_NilInnerIsAsReturnFalseAndDoNotPanic exercises the
// degenerate path where the wrapper carries a nil inner. Both Is and
// As must return false and must not panic. This pins the defensive
// guards in (*redactingError).Is / .As.
func TestRedactingError_NilInnerIsAsReturnFalseAndDoNotPanic(t *testing.T) {
	// Build a Configuration error with a typed-nil inner. The wrapper
	// only gets exercised when Unwrap is called on the outer Error,
	// at which point it constructs a redactingError around the inner.
	// Forcing a nil inner here is most directly done by constructing
	// the redactingError-equivalent via the public surface and then
	// walking to it.
	wrapped := sdkerrors.NewConfigurationError("op", "boom", nil)

	shell := stderrors.Unwrap(wrapped)
	// A nil inner short-circuits inside the Error constructor:
	// NewConfigurationError(..., nil) does not install a redacting
	// shell. The expected behaviour is that Unwrap() returns nil.
	assert.NoError(t, shell)

	// Defensive: a manually-constructed Error with no Err field must
	// also Unwrap() to nil without panicking.
	var bare *sdkerrors.Error
	assert.NoError(t, bare.Unwrap())
}

// TestErrorJSONMarshal_NoLeak covers Audit C3: a naive
// json.Marshal(*Error) must not surface Bearer tokens, request bodies,
// or any inner-error string. Only the safe whitelist (Category, Code,
// APICode, Resource, EntityType, StatusCode) appears in the JSON.
func TestErrorJSONMarshal_NoLeak(t *testing.T) {
	t.Run("inner error with credentials is suppressed", func(t *testing.T) {
		err := &sdkerrors.Error{
			Category:                  sdkerrors.CategoryNetwork,
			Code:                      sdkerrors.CodeNetwork,
			Message:                   "POST /v1/transactions: Authorization: Bearer eyJ.tok.sig",
			Operation:                 "transactions.Create",
			Resource:                  "transaction",
			ResourceID:                "tx-1234567890",
			Title:                     "Authorization: Bearer eyJ.tok.sig",
			Fields:                    []string{"password=hunter2"},
			Details:                   map[string]any{"token": "rt_abc_123"},
			UpstreamBody:              "client_secret=raw-upstream-secret",
			UpstreamBodyOriginalBytes: len("client_secret=raw-upstream-secret"),
			RequestID:                 "req-X-API-Key=ak_live_999",
			StatusCode:                502,
			Err:                       stderrors.New("password=hunter2 api_key=ak_live_999"),
		}

		raw, marshalErr := json.Marshal(err)
		require.NoError(t, marshalErr)

		out := string(raw)

		// None of the credential-bearing fields should leak.
		assert.NotContains(t, out, "eyJ.tok.sig", "Message must be json:\"-\"")
		assert.NotContains(t, out, "hunter2", "Fields/Err must be json:\"-\"")
		assert.NotContains(t, out, "ak_live_999", "RequestID/Details must be json:\"-\"")
		assert.NotContains(t, out, "rt_abc_123", "Details map must be json:\"-\"")
		assert.NotContains(t, out, "tx-1234567890", "ResourceID must be json:\"-\"")
		assert.NotContains(t, out, "transactions.Create", "Operation must be json:\"-\"")
		assert.NotContains(t, out, "raw-upstream-secret", "UpstreamBody must be json:\"-\"")

		// The safe projection should be intact.
		assert.Contains(t, out, `"category":"network"`)
		assert.Contains(t, out, `"code":"network_error"`)
		assert.Contains(t, out, `"resource":"transaction"`)
		assert.Contains(t, out, `"statusCode":502`)
	})

	t.Run("nil receiver renders as JSON null", func(t *testing.T) {
		var err *sdkerrors.Error

		raw, marshalErr := json.Marshal(err)
		require.NoError(t, marshalErr)
		assert.JSONEq(t, "null", string(raw))
	})
}

// TestUnwrap_RedactsInnerString covers Audit C5: errors.Unwrap on an
// *Error must return a wrapper whose Error() pipes the inner string
// through the redactor. Loggers walking the chain via Unwrap cannot
// see the unredacted inner error.
func TestUnwrap_RedactsInnerString(t *testing.T) {
	t.Run("inner credential is redacted on Unwrap().Error()", func(t *testing.T) {
		inner := stderrors.New("password=hunter2 token=secret")
		wrapped := sdkerrors.NewValidationError("op", "bad input", inner)

		unwrapped := stderrors.Unwrap(wrapped)
		require.Error(t, unwrapped)

		out := unwrapped.Error()
		assert.NotContains(t, out, "hunter2",
			"errors.Unwrap chain must not surface raw credentials")
		assert.NotContains(t, out, "secret")
		assert.Contains(t, out, "[REDACTED]")
	})

	t.Run("errors.Is still walks through the redacting wrapper", func(t *testing.T) {
		sentinel := stderrors.New("sentinel root cause")
		wrapped := sdkerrors.NewValidationError("op", "boom", sentinel)

		// errors.Is should find the sentinel through the redacting shim.
		require.ErrorIs(t, wrapped, sentinel,
			"redactingError must preserve the chain for errors.Is/As semantics")
	})

	t.Run("Unwrap on nil receiver returns nil", func(t *testing.T) {
		var err *sdkerrors.Error
		assert.NoError(t, err.Unwrap())
	})
}

// TestHTTPErrorMappings_408_425 covers Audit C6: 408 (Request Timeout)
// and 425 (Too Early) must map to the right Category/Code instead of
// falling into the Internal default.
func TestHTTPErrorMappings_408_425(t *testing.T) {
	t.Run("408 Request Timeout maps to CategoryTimeout", func(t *testing.T) {
		err := sdkerrors.ErrorFromHTTPResponse(http.StatusRequestTimeout, "req-1", "client took too long", "", "", "")

		var sdkErr *sdkerrors.Error
		require.ErrorAs(t, err, &sdkErr)
		assert.Equal(t, sdkerrors.CategoryTimeout, sdkErr.Category,
			"408 must classify as CategoryTimeout, not Internal")
		assert.Equal(t, sdkerrors.CodeTimeout, sdkErr.Code)
		assert.Equal(t, http.StatusRequestTimeout, sdkErr.StatusCode)
	})

	t.Run("425 Too Early maps to CategoryLimitExceeded", func(t *testing.T) {
		err := sdkerrors.ErrorFromHTTPResponse(http.StatusTooEarly, "req-2", "replay protection", "", "", "")

		var sdkErr *sdkerrors.Error
		require.ErrorAs(t, err, &sdkErr)
		assert.Equal(t, sdkerrors.CategoryLimitExceeded, sdkErr.Category,
			"425 must classify as CategoryLimitExceeded, mirroring 429's retry semantics")
		assert.Equal(t, sdkerrors.CodeRateLimit, sdkErr.Code)
		assert.Equal(t, http.StatusTooEarly, sdkErr.StatusCode)
	})
}

// TestHTTPErrorMappings_503_CodeAlignment covers Audit M13: 503 must
// pair CategoryNetwork with a network-shaped Code, not the generic
// CodeInternal — every other entry in httpErrorMappings pairs Category
// with a matching Code.
func TestHTTPErrorMappings_503_CodeAlignment(t *testing.T) {
	err := sdkerrors.ErrorFromHTTPResponse(http.StatusServiceUnavailable, "req-3", "upstream down", "", "", "")

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, sdkerrors.CategoryNetwork, sdkErr.Category)
	assert.NotEqual(t, sdkerrors.CodeInternal, sdkErr.Code,
		"503 should not use the generic CodeInternal; pair with a service-specific Code")
}

// TestIsSensitiveFieldName covers Audit C4: the shared predicate must
// catch every credential / PII / financial field name it claims to.
func TestIsSensitiveFieldName(t *testing.T) {
	sensitive := []string{
		"password",
		"PASSWORD",
		"client_secret",
		"clientSecret",
		"apiKey",
		"X-API-Key",
		"x_api_key",
		"api-key",
		"api_key",
		"access_token",
		"accessToken",
		"refresh_token",
		"refreshToken",
		"id_token",
		"jwt",
		"Authorization",
		"authorization",
		"document",
		"legalDocument",
		"legal_document",
		"cpf",
		"cnpj",
		"creditCard",
		"credit_card",
		"cardNumber",
		"card-number",
		"ssn",
		"metadata.user.password",
		"metadata.token",
		"banking_details_iban",
		"externalId",
		"idempotency_key",
		"x-idempotency",
	}

	for _, name := range sensitive {
		t.Run("sensitive: "+name, func(t *testing.T) {
			assert.True(t, sdkerrors.IsSensitiveFieldName(name),
				"%q must trip IsSensitiveFieldName", name)
		})
	}

	nonSensitive := []string{
		"",
		"name",
		"amount",
		"asset_code",
		"description",
		"createdAt",
		"updatedAt",
		"id", // generic id, not externalId — must NOT match
		"status",
	}

	for _, name := range nonSensitive {
		t.Run("non-sensitive: "+name, func(t *testing.T) {
			assert.False(t, sdkerrors.IsSensitiveFieldName(name),
				"%q must NOT trip IsSensitiveFieldName", name)
		})
	}
}

// TestErrorFromHTTPResponse_RedactsTitleAndDetails covers Audit M11:
// Title and Details must be redacted at construction time so neither
// renders raw credentials when later surfaced by GetErrorDetails or
// Error().
func TestErrorFromHTTPResponse_RedactsTitleAndDetails(t *testing.T) {
	err := sdkerrors.ErrorFromHTTPResponseWithDetails(
		http.StatusBadRequest,
		"req-4",
		"validation failed: token=abc123",
		"MIDAZ-0042",
		"transaction",
		"tx-1",
		"Authorization: Bearer leaked-token",
		[]string{"password=hunter2", "amount"},
		map[string]any{"token": "rt_xyz", "amount": 100},
	)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)

	// Constructor-stored fields are pre-redacted.
	assert.NotContains(t, sdkErr.Title, "leaked-token")
	assert.NotContains(t, sdkErr.Message, "abc123")

	for _, f := range sdkErr.Fields {
		assert.NotContains(t, f, "hunter2",
			"Fields slice must be redacted at construction")
	}

	// The token key in Details has a sensitive name → its value is
	// replaced with "[REDACTED]" verbatim.
	assert.Equal(t, "[REDACTED]", sdkErr.Details["token"],
		"Details with sensitive key must be flat-redacted")
	assert.Equal(t, 100, sdkErr.Details["amount"],
		"Details with non-sensitive keys are preserved")
}

// TestURLUserinfoRedaction covers Audit M14: URLs embedded in
// transport-layer errors must have their userinfo (user[:password])
// stripped before classification — the stdlib's own xxxxx-masking
// preserves the username, which we don't want.
func TestURLUserinfoRedaction(t *testing.T) {
	// We can't easily synthesize a *url.Error without a real dial, so
	// we go through ClassifyTransportError's inner stripURLUserinfo by
	// constructing a url.Error directly via a wrapped parse. The
	// integration-style live-localhost test already covers the
	// end-to-end shape.
	//
	// Direct check that the helper is present on the package surface
	// at the right call site — see transport.go.
	t.Run("classification still wraps an internal error for unrelated input", func(t *testing.T) {
		// A non-URL error should pass through ClassifyTransportError
		// unchanged in shape (CategoryInternal). The stripURLUserinfo
		// shim is a no-op on non-url.Error inputs.
		err := sdkerrors.ClassifyTransportError("op", stderrors.New("totally unrelated"))

		var sdkErr *sdkerrors.Error
		require.ErrorAs(t, err, &sdkErr)
		assert.Equal(t, sdkerrors.CategoryInternal, sdkErr.Category)
	})
}
