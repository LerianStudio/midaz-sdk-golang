package helpers

import (
	"regexp"
	"strings"
)

// IsValidUUID checks if a string is a valid UUID.
func IsValidUUID(s string) bool {
	if s == "" {
		return false
	}

	// UUID format: 8-4-4-4-12 hexadecimal digits
	r := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	return r.MatchString(strings.ToLower(s))
}

// IsValidAmount checks if an amount value is valid for a given scale.
func IsValidAmount(amount int64, scale int64) bool {
	// Amount should not be negative
	if amount < 0 {
		return false
	}

	// Scale should be between 0 and 18
	if scale < 0 || scale > 18 {
		return false
	}

	return true
}

// IsValidExternalAccountID checks if an account ID is a valid external account ID.
func IsValidExternalAccountID(id string) bool {
	if id == "" {
		return false
	}

	// External accounts start with @external/
	return strings.HasPrefix(id, "@external/")
}

// IsValidAuthToken checks if a token has a valid format.
// This is a simple placeholder implementation; in a real-world scenario,
// you might perform more sophisticated validation.
func IsValidAuthToken(token string) bool {
	// Check if token is empty
	if token == "" {
		return false
	}

	// Check minimum length
	if len(token) < 8 {
		return false
	}

	return true
}
