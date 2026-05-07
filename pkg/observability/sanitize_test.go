package observability

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSensitiveStringTruncatesUnscannedTail(t *testing.T) {
	prefix := "access_token=secret " + strings.Repeat("a", sanitizeMaxScanBytes)
	tailSecret := " password=tail-secret"

	sanitized := sanitizeSensitiveString(prefix + tailSecret)

	assert.Contains(t, sanitized, "access_token="+redactedValue)
	assert.NotContains(t, sanitized, "tail-secret")
	assert.Contains(t, sanitized, "[truncated]")
}
