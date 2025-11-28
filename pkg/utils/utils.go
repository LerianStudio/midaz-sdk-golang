package utils //nolint:revive // utils is an established SDK package name for backward compatibility

import (
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"
	"github.com/google/uuid"
)

// NewUUID returns a new UUID string.
func NewUUID() string {
	return uuid.New().String()
}

// IsValidUUID validates UUID format using the SDK validation helpers.
func IsValidUUID(s string) bool {
	return validation.IsValidUUID(s)
}

// ValidateMetadata proxies to the SDK validation to enforce constraints.
func ValidateMetadata(m map[string]any) error {
	return validation.ValidateMetadata(m)
}
