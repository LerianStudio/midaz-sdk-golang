package validation

import (
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/helpers"
)

// IsValidUUID checks if a string is a valid UUID.
func IsValidUUID(s string) bool {
	return helpers.IsValidUUID(s)
}

// IsValidAmount checks if an amount value is valid for a given scale.
func IsValidAmount(amount int64, scale int64) bool {
	return helpers.IsValidAmount(amount, scale)
}

// IsValidExternalAccountID checks if an account ID is a valid external account ID.
func IsValidExternalAccountID(id string) bool {
	return helpers.IsValidExternalAccountID(id)
}

// IsValidAuthToken checks if a token has a valid format.
func IsValidAuthToken(token string) bool {
	return helpers.IsValidAuthToken(token)
}
