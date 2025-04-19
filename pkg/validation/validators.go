// Package validation provides validation utilities for the Midaz SDK.
package validation

import (
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
)

// TypeValidator defines the interface for validating asset types
type TypeValidator = core.TypeValidator

// AccountTypeValidator defines the interface for validating account types
type AccountTypeValidator = core.AccountTypeValidator

// CurrencyValidator defines the interface for validating currency codes
type CurrencyValidator = core.CurrencyValidator

// CountryValidator defines the interface for validating country codes
type CountryValidator = core.CountryValidator

// ValidatorProvider defines a provider for all validators
type ValidatorProvider = core.ValidatorProvider
