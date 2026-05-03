package observability

import "regexp"

const redactedValue = "[REDACTED]"

var (
	sensitiveAssignmentPattern = regexp.MustCompile(`(?i)(access[_-]?token|api[_-]?key|apikey|auth[_-]?token|password|secret|token|refresh[_-]?token|x-idempotency|idempotency-key|x-midaz-auto-idempotency)(\s*[=:]\s*)([^\s&;,]+)`)
	bearerTokenPattern         = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]+`)
)

func sanitizeSensitiveString(value string) string {
	if value == "" {
		return ""
	}

	sanitized := sensitiveAssignmentPattern.ReplaceAllString(value, `${1}${2}`+redactedValue)
	sanitized = bearerTokenPattern.ReplaceAllString(sanitized, `${1}`+redactedValue)

	return sanitized
}
