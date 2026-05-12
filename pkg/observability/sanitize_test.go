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
