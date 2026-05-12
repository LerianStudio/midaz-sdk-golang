package observability

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggerRedactsNestedStructuredFields(t *testing.T) {
	const (
		bearerSecret  = "bearer-secret-token"
		basicSecret   = "dXNlcjpwYXNz"
		apiKeySecret  = "raw-api-key"
		clientSecret  = "raw-client-secret"
		refreshSecret = "raw-refresh-secret"
		plainSecret   = "raw-plain-secret"
	)

	var output bytes.Buffer
	logger := NewLogger(InfoLevel, &output, nil).With(map[string]any{
		"request": map[string]any{
			"Authorization": "Bearer " + bearerSecret,
			"headers": map[string]string{
				"x-api-key":     apiKeySecret,
				"client_secret": clientSecret,
				"safe":          "value",
			},
			"events": []any{
				"refresh_token=" + refreshSecret,
				map[string]any{"password": plainSecret},
			},
			"schemes": []string{
				"Basic " + basicSecret,
				"Bearer " + bearerSecret,
			},
		},
	})

	logger.Info("processing access_token=message-secret")

	rawLog := output.String()
	assert.NotContains(t, rawLog, bearerSecret)
	assert.NotContains(t, rawLog, basicSecret)
	assert.NotContains(t, rawLog, apiKeySecret)
	assert.NotContains(t, rawLog, clientSecret)
	assert.NotContains(t, rawLog, refreshSecret)
	assert.NotContains(t, rawLog, plainSecret)
	assert.NotContains(t, rawLog, "message-secret")

	entry := decodeLogEntry(t, rawLog)
	request := requireMap(t, entry["request"])
	assert.Equal(t, redactedValue, request["Authorization"])

	headers := requireMap(t, request["headers"])
	assert.Equal(t, redactedValue, headers["x-api-key"])
	assert.Equal(t, redactedValue, headers["client_secret"])
	assert.Equal(t, "value", headers["safe"])

	events := requireSlice(t, request["events"])
	assert.Equal(t, "refresh_token="+redactedValue, events[0])
	eventFields := requireMap(t, events[1])
	assert.Equal(t, redactedValue, eventFields["password"])

	schemes := requireSlice(t, request["schemes"])
	assert.Equal(t, "Basic "+redactedValue, schemes[0])
	assert.Equal(t, "Bearer "+redactedValue, schemes[1])
}

func TestLoggerRedactsTopLevelSensitiveKeysAndPreservesReservedFields(t *testing.T) {
	var output bytes.Buffer
	logger := NewLogger(InfoLevel, &output, nil).With(map[string]any{
		"timestamp":     "evil-timestamp",
		"level":         "evil-level",
		"message":       "evil-message",
		"caller":        "evil-caller",
		"access_token":  "raw-access-token",
		"token":         "raw-token",
		"set-cookie":    "raw-cookie",
		"Authorization": "Basic raw-basic-token",
	})

	logger.Info("safe message with Bearer raw-bearer-token")

	rawLog := output.String()
	assert.NotContains(t, rawLog, "raw-access-token")
	assert.NotContains(t, rawLog, "raw-token")
	assert.NotContains(t, rawLog, "raw-cookie")
	assert.NotContains(t, rawLog, "raw-basic-token")
	assert.NotContains(t, rawLog, "raw-bearer-token")
	assert.NotContains(t, rawLog, "evil-timestamp")
	assert.NotContains(t, rawLog, "evil-level")
	assert.NotContains(t, rawLog, "evil-message")
	assert.NotContains(t, rawLog, "evil-caller")

	entry := decodeLogEntry(t, rawLog)
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "safe message with Bearer "+redactedValue, entry["message"])
	assert.Equal(t, redactedValue, entry["access_token"])
	assert.Equal(t, redactedValue, entry["token"])
	assert.Equal(t, redactedValue, entry["set-cookie"])
	assert.Equal(t, redactedValue, entry["Authorization"])
	assert.NotEmpty(t, entry["timestamp"])
	assert.NotEmpty(t, entry["caller"])
}

func TestSanitizeSensitiveStringRedactsBearerAndBasicAuth(t *testing.T) {
	assert.Equal(t, "Authorization: Bearer "+redactedValue, sanitizeSensitiveString("Authorization: Bearer raw-bearer"))
	assert.Equal(t, "Authorization: Basic "+redactedValue, sanitizeSensitiveString("Authorization: Basic dXNlcjpwYXNz"))
}

func decodeLogEntry(t *testing.T, rawLog string) map[string]any {
	t.Helper()

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(rawLog), &entry))

	return entry
}

func requireMap(t *testing.T, value any) map[string]any {
	t.Helper()

	m, ok := value.(map[string]any)
	require.Truef(t, ok, "expected map[string]any, got %T", value)

	return m
}

func requireSlice(t *testing.T, value any) []any {
	t.Helper()

	s, ok := value.([]any)
	require.Truef(t, ok, "expected []any, got %T", value)

	return s
}
