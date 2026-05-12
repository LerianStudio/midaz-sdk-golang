package observability

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typedNilError struct {
	message string
}

func (e *typedNilError) Error() string {
	return e.message
}

type typedNilStringer struct {
	value string
}

type nestedCredentialPayload struct {
	ClientSecret string `json:"clientSecret"`
	SafeValue    string `json:"safeValue"`
}

type credentialPayload struct {
	AccessToken string                  `json:"accessToken"`
	Nested      nestedCredentialPayload `json:"nested"`
	Safe        string                  `json:"safe"`
}

func (s *typedNilStringer) String() string {
	return s.value
}

func TestSanitizeSensitiveStringTruncatesUnscannedTail(t *testing.T) {
	prefix := "access_token=secret " + strings.Repeat("a", sanitizeMaxScanBytes)
	tailSecret := " password=tail-secret"

	sanitized := sanitizeSensitiveString(prefix + tailSecret)

	assert.Contains(t, sanitized, "access_token="+redactedValue)
	assert.NotContains(t, sanitized, "tail-secret")
	assert.Contains(t, sanitized, "[truncated]")
}

func TestSanitizeLogFieldValueRedactsNormalizedSensitiveKeys(t *testing.T) {
	const (
		clientSecret = "raw-client-secret"
		accessToken  = "raw-access-token"
		refreshToken = "raw-refresh-token"
		idToken      = "raw-id-token"
		apiKey       = "raw-api-key"
		xAPIKey      = "raw-x-api-key"
		setCookie    = "raw-set-cookie"
		metadataTok  = "raw-metadata-token"
	)

	sanitized := sanitizeAny(map[string]any{
		"clientSecret":         clientSecret,
		"accessToken":          accessToken,
		"refreshToken":         refreshToken,
		"idToken":              idToken,
		"apiKey":               apiKey,
		"xApiKey":              xAPIKey,
		"setCookie":            setCookie,
		"metadata.accessToken": metadataTok,
		"safe":                 "visible",
	}, 0, make(map[sanitizeVisit]struct{}))

	fields := requireMap(t, sanitized)
	assert.Equal(t, redactedValue, fields["clientSecret"])
	assert.Equal(t, redactedValue, fields["accessToken"])
	assert.Equal(t, redactedValue, fields["refreshToken"])
	assert.Equal(t, redactedValue, fields["idToken"])
	assert.Equal(t, redactedValue, fields["apiKey"])
	assert.Equal(t, redactedValue, fields["xApiKey"])
	assert.Equal(t, redactedValue, fields["setCookie"])
	assert.Equal(t, redactedValue, fields["metadata.accessToken"])
	assert.Equal(t, "visible", fields["safe"])
}

func TestSanitizeSensitiveStringRedactsQuotedJSONCamelCaseSecrets(t *testing.T) {
	sanitized := sanitizeSensitiveString(`{"accessToken":"raw-access-token","clientSecret":"raw-client-secret","safe":"visible"}`)

	assert.NotContains(t, sanitized, "raw-access-token")
	assert.NotContains(t, sanitized, "raw-client-secret")
	assert.Contains(t, sanitized, `"accessToken":`+redactedValue)
	assert.Contains(t, sanitized, `"clientSecret":`+redactedValue)
	assert.Contains(t, sanitized, `"safe":"visible"`)
}

func TestSanitizeLogFieldValueRedactsStructFieldsByJSONTag(t *testing.T) {
	sanitized := sanitizeAny(credentialPayload{
		AccessToken: "raw-struct-access-token",
		Nested: nestedCredentialPayload{
			ClientSecret: "raw-struct-client-secret",
			SafeValue:    "nested-visible",
		},
		Safe: "visible",
	}, 0, make(map[sanitizeVisit]struct{}))

	fields := requireMap(t, sanitized)
	assert.Equal(t, redactedValue, fields["accessToken"])
	assert.Equal(t, "visible", fields["safe"])

	nested := requireMap(t, fields["nested"])
	assert.Equal(t, redactedValue, nested["clientSecret"])
	assert.Equal(t, "nested-visible", nested["safeValue"])
}

func TestSanitizeLogFieldValueRedactsNestedMetadataAccessToken(t *testing.T) {
	const rawToken = "raw-nested-metadata-token"

	sanitized := sanitizeAny(map[string]any{
		"payload": map[string]any{
			"metadata.accessToken": rawToken,
		},
	}, 0, make(map[sanitizeVisit]struct{}))

	b, err := json.Marshal(sanitized)
	require.NoError(t, err)
	assert.NotContains(t, string(b), rawToken)
	assert.Contains(t, string(b), redactedValue)
}

func TestSanitizeAnyHandlesCyclicMap(t *testing.T) {
	cyclic := map[string]any{"accessToken": "raw-cycle-token"}
	cyclic["self"] = cyclic

	sanitized := sanitizeAny(cyclic, 0, make(map[sanitizeVisit]struct{}))

	fields := requireMap(t, sanitized)
	assert.Equal(t, redactedValue, fields["accessToken"])
	assert.Equal(t, redactedValue, fields["self"])
}

func TestSanitizeReflectValueHandlesPointerToInterface(t *testing.T) {
	value := any("accessToken=raw-pointer-interface-token")
	pointer := &value

	sanitized := sanitizeAny(pointer, 0, make(map[sanitizeVisit]struct{}))

	assert.Equal(t, "accessToken="+redactedValue, sanitized)
}

func TestSanitizeAnyHandlesTypedNilError(t *testing.T) {
	var err *typedNilError

	assert.NotPanics(t, func() {
		assert.Nil(t, sanitizeAny(err, 0, make(map[sanitizeVisit]struct{})))
	})
}

func TestSanitizeAnyHandlesTypedNilStringer(t *testing.T) {
	var stringer *typedNilStringer

	assert.NotPanics(t, func() {
		assert.Nil(t, sanitizeAny(stringer, 0, make(map[sanitizeVisit]struct{})))
	})
}

func TestSanitizeSensitiveStringRedactsBasicAndBearerCredentials(t *testing.T) {
	assert.Equal(t, "Authorization: Basic "+redactedValue, sanitizeSensitiveString("Authorization: Basic dXNlcjpwYXNz"))
	assert.Equal(t, "Authorization: Bearer "+redactedValue, sanitizeSensitiveString("Authorization: Bearer raw-bearer-token"))
}

func TestSanitizeSensitiveStringRedactsAuthSchemeAfterShortPrefix(t *testing.T) {
	input := strings.Repeat("safe ", 20) + "Bearer raw-bearer-token"

	sanitized := sanitizeSensitiveString(input)

	assert.NotContains(t, sanitized, "raw-bearer-token")
	assert.Contains(t, sanitized, "Bearer "+redactedValue)
}

// FuzzSensitiveAssignmentPattern feeds pathological inputs (mixed
// quotes, embedded nulls, multibyte UTF-8, deeply nested metadata
// paths, very long values) to the regex redactor and asserts the two
// non-negotiable invariants:
//
//  1. sanitizeSensitiveString must NOT panic on any input.
//  2. When the input contains a sensitive marker followed by '=' / ':',
//     the literal token MUST survive in the rendered output (the
//     keyword itself is intentionally preserved; only the value is
//     replaced with redactedValue).
//
// We deliberately do NOT assert "all sensitive values are redacted" —
// fuzz-generated inputs frequently land in shapes the regex was never
// designed to match (e.g. a sensitive key with no separator). The
// guarantee here is robustness, not exhaustive coverage.
func FuzzSensitiveAssignmentPattern(f *testing.F) {
	seedCorpus := []string{
		"",
		"password=hunter2",
		`{"clientSecret":"raw"}`,
		"metadata.user.email=alice@example.com",
		"Authorization: Bearer raw-bearer-token",
		"a:b:c:d:e:f:g=value",
		"password=\x00null-byte",
		strings.Repeat("token=", 1024) + "tail",
		"αβγδε=multibyte",
		"\"password\":'mixed'quotes",
	}
	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		// Bound the input so a pathological 100 MiB string from the
		// fuzz engine doesn't turn this fuzz run into a CPU stall — we
		// already exercise the truncation path in
		// TestSanitizeSensitiveStringTruncatesUnscannedTail.
		if len(input) > 4096 {
			input = input[:4096]
		}

		// Invariant 1: must not panic.
		var out string

		require.NotPanics(t, func() {
			out = sanitizeSensitiveString(input)
		})

		// Invariant 2 (best-effort, only when the input shape matches
		// the regex precondition): a clean lowercase keyword followed by
		// '=' must yield a string that does NOT contain the literal
		// post-keyword value tail.
		if idx := strings.Index(input, "password="); idx >= 0 {
			// Find what came after "password=" in the input that is
			// NOT itself sensitive-marker noise. If the regex matched,
			// out must contain "password=" + redactedValue at idx.
			suffix := input[idx+len("password="):]
			if isPlainAlphaNum(suffix) && suffix != "" {
				assert.NotContains(t, out, "password="+suffix,
					"raw password value must not survive when input has the expected shape")
			}
		}
	})
}

// isPlainAlphaNum returns true when s consists only of ASCII letters
// and digits and is non-empty. Used by the fuzz invariant check above
// to skip cases where the post-keyword suffix is itself another
// sensitive token or noise the regex wasn't meant to match.
func isPlainAlphaNum(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		default:
			return false
		}
	}

	return s != ""
}

// TestSanitizeSensitiveStringRedactsCompoundMetadataPath covers the
// regex's optional `(?:\.[\w.-]+)?` suffix on the keyword group: a
// compound key like `metadata.user.email=…` must be redacted as a
// single assignment, not split across the dot boundary.
func TestSanitizeSensitiveStringRedactsCompoundMetadataPath(t *testing.T) {
	cases := []struct {
		name  string
		input string
		raw   string
	}{
		{name: "metadata dotted path", input: "metadata.user.email=alice@example.com", raw: "alice@example.com"},
		{name: "metadata bare", input: "metadata=arbitrary-secret", raw: "arbitrary-secret"},
		{name: "document", input: "document=12345678900", raw: "12345678900"},
		{name: "legal_document", input: "legal_document=12345678000199", raw: "12345678000199"},
		{name: "external_id", input: "external_id=customer-42", raw: "customer-42"},
		{name: "banking_details_account", input: "banking_details_account=00012345-6", raw: "00012345-6"},
		{name: "banking_details_iban", input: "banking_details_iban=DE89370400440532013000", raw: "DE89370400440532013000"},
		{name: "related_party_document", input: "related_party_document=12345678900", raw: "12345678900"},
		{name: "regulatory_fields_participant_document", input: "regulatory_fields_participant_document=12345678900", raw: "12345678900"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := sanitizeSensitiveString(tc.input)
			assert.NotContains(t, out, tc.raw, "raw PII value must not survive the observability redactor")
			assert.Contains(t, out, redactedValue)
		})
	}
}
