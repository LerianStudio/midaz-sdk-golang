package observability

import (
	"regexp"
	"strings"
)

const (
	redactedValue = "[REDACTED]"

	// sanitizeMaxScanBytes caps the slice we scan for sensitive tokens. Very
	// large response bodies are not a typical source of credentials and the
	// regex passes are O(n); truncating the scan window keeps log redaction
	// bounded under pathological input. The original (untruncated) string is
	// still returned to the caller — only the scanning cost is capped.
	sanitizeMaxScanBytes = 8 * 1024
)

var (
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(access[_-]?token|api[_-]?key|apikey|auth[_-]?token|password|secret|token|refresh[_-]?token|x-idempotency|idempotency-key|x-midaz-auto-idempotency)(\s*[=:]\s*)([^\s&;,]+)`)
	bearerTokenPattern         = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]+`)
)

// sanitizeSensitiveString redacts secrets from value. It is hot-pathed by the
// fast preflight checks below — the regex engine is only invoked when the
// input plausibly contains a sensitive marker.
//
// Fast path:
//   - empty string → return as-is.
//   - no '=' AND no ':' AND no case-insensitive "bearer" → return as-is.
//
// Slow path: scan up to sanitizeMaxScanBytes for the assignment + bearer
// patterns. If we redacted within the scan window we return the redacted
// prefix concatenated with the untouched tail; otherwise we return value
// unchanged. This avoids quadratic regex behavior on multi-megabyte
// response bodies that callers occasionally pass through here.
func sanitizeSensitiveString(value string) string {
	if value == "" {
		return ""
	}

	if !mayContainSensitiveToken(value) {
		return value
	}

	scan := value
	tail := ""
	if len(scan) > sanitizeMaxScanBytes {
		scan = value[:sanitizeMaxScanBytes]
		tail = value[sanitizeMaxScanBytes:]
	}

	sanitized := sensitiveAssignmentPattern.ReplaceAllString(scan, `${1}${2}`+redactedValue)
	sanitized = bearerTokenPattern.ReplaceAllString(sanitized, `${1}`+redactedValue)

	if tail == "" {
		return sanitized
	}

	return sanitized + tail
}

// mayContainSensitiveToken returns true when the string is plausibly carrying
// a credential payload. Both regex patterns require either an '=' / ':'
// assignment separator or the literal "bearer" prefix; if none of those
// markers are present we know the regex engine cannot match. Skipping the
// regex pass on every benign log line is the dominant win here — the byte
// scans are O(n) with very small constants.
func mayContainSensitiveToken(value string) bool {
	if strings.IndexByte(value, '=') >= 0 {
		return true
	}

	if strings.IndexByte(value, ':') >= 0 {
		return true
	}

	// Case-insensitive search for "bearer" without allocating a lower-cased
	// copy of the whole string. EqualFold over a sliding window would be
	// slower than ToLower for typical sizes, so we accept one allocation
	// here — sanitizeSensitiveString is itself a slow path triggered only
	// when one of the byte markers is missing.
	return strings.Contains(strings.ToLower(value), "bearer")
}
