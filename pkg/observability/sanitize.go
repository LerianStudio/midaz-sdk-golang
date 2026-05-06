package observability

import (
	"regexp"
	"strings"
)

const (
	redactedValue = "[REDACTED]"

	// sanitizeMaxScanBytes caps the slice we scan for sensitive tokens. The
	// regex passes are O(n) but with non-trivial constants; an unbounded
	// scan turns multi-megabyte response bodies into a denial-of-service
	// vector for log redaction. Truncating the scan window keeps redaction
	// bounded under pathological input. The original (untruncated) string
	// is still returned to the caller — only the scanning cost is capped.
	//
	// SAFETY TRADE-OFF — READ BEFORE PASSING LARGE PAYLOADS:
	// Anything past byte 65,536 ships unredacted. The bytes [0, 65536) are
	// scanned and rewritten; bytes [65536, len) are concatenated back to the
	// output verbatim. A bearer token that lives at byte 70,000 of a 200 KB
	// response body will appear in logs in the clear. This is acceptable for
	// the SDK's own use (HTTP error messages, span attributes — all well
	// under the cap) but is NOT a credential firewall for arbitrary payloads.
	//
	// 64 KiB covers typical HTTP error bodies (4xx/5xx JSON responses rarely
	// exceed a few KiB) and the larger upstream payloads we have seen in
	// practice, while keeping a single ReplaceAllString pass well under a
	// millisecond on commodity hardware. Callers that intentionally pass
	// larger blobs through here should redact at the source instead of
	// relying on this best-effort post-hoc scrub.
	sanitizeMaxScanBytes = 64 * 1024
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
