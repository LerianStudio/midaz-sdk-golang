// Package core provides fundamental validation utilities for the Midaz SDK.
//
// This package contains primitive validation functions that don't depend on
// any model structures, making it usable by both the models package and the
// validation package without creating circular dependencies.
package core

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/LerianStudio/lib-commons/commons"
)

// ValidationConfig represents options for the validation behavior
type ValidationConfig struct {
	// MaxMetadataSize defines the maximum size of metadata in bytes
	MaxMetadataSize int

	// MaxStringLength defines the maximum length for string fields in metadata
	MaxStringLength int

	// MaxAddressLineLength defines the maximum length for address lines
	MaxAddressLineLength int

	// MaxZipCodeLength defines the maximum length for zip codes
	MaxZipCodeLength int

	// MaxCityLength defines the maximum length for city names
	MaxCityLength int

	// MaxStateLength defines the maximum length for state names
	MaxStateLength int

	// StrictMode enables or disables additional validation checks
	StrictMode bool
}

// ValidationOption is a function type for configuring a ValidationConfig
type ValidationOption func(*ValidationConfig) error

// DefaultValidationConfig returns a config with default values
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MaxMetadataSize:      4096,
		MaxStringLength:      256,
		MaxAddressLineLength: 100,
		MaxZipCodeLength:     20,
		MaxCityLength:        100,
		MaxStateLength:       100,
		StrictMode:           false,
	}
}

// WithMaxMetadataSize sets the maximum size for metadata
func WithMaxMetadataSize(size int) ValidationOption {
	return func(c *ValidationConfig) error {
		if size <= 0 {
			return fmt.Errorf("max metadata size must be positive, got %d", size)
		}

		c.MaxMetadataSize = size

		return nil
	}
}

// WithMaxStringLength sets the maximum length for string fields in metadata
func WithMaxStringLength(length int) ValidationOption {
	return func(c *ValidationConfig) error {
		if length <= 0 {
			return fmt.Errorf("max string length must be positive, got %d", length)
		}

		c.MaxStringLength = length

		return nil
	}
}

// WithMaxAddressLineLength sets the maximum length for address lines
func WithMaxAddressLineLength(length int) ValidationOption {
	return func(c *ValidationConfig) error {
		if length <= 0 {
			return fmt.Errorf("max address line length must be positive, got %d", length)
		}

		c.MaxAddressLineLength = length

		return nil
	}
}

// WithMaxZipCodeLength sets the maximum length for zip codes
func WithMaxZipCodeLength(length int) ValidationOption {
	return func(c *ValidationConfig) error {
		if length <= 0 {
			return fmt.Errorf("max zip code length must be positive, got %d", length)
		}

		c.MaxZipCodeLength = length

		return nil
	}
}

// WithMaxCityLength sets the maximum length for city names
func WithMaxCityLength(length int) ValidationOption {
	return func(c *ValidationConfig) error {
		if length <= 0 {
			return fmt.Errorf("max city length must be positive, got %d", length)
		}

		c.MaxCityLength = length

		return nil
	}
}

// WithMaxStateLength sets the maximum length for state names
func WithMaxStateLength(length int) ValidationOption {
	return func(c *ValidationConfig) error {
		if length <= 0 {
			return fmt.Errorf("max state length must be positive, got %d", length)
		}

		c.MaxStateLength = length

		return nil
	}
}

// WithStrictMode enables or disables strict validation mode
func WithStrictMode(strict bool) ValidationOption {
	return func(c *ValidationConfig) error {
		c.StrictMode = strict
		return nil
	}
}

// NewValidationConfig creates a validation config with the provided options
func NewValidationConfig(options ...ValidationOption) (*ValidationConfig, error) {
	config := DefaultValidationConfig()

	for _, option := range options {
		if err := option(config); err != nil {
			return nil, fmt.Errorf("failed to apply validation option: %w", err)
		}
	}

	return config, nil
}

// ExternalAccountPattern is the regex pattern for external account references
var ExternalAccountPattern = regexp.MustCompile(`^@external/([A-Z]{3,4})$`)

// AccountAliasPattern is the regex pattern for account aliases
var AccountAliasPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// AssetCodePattern is the regex pattern for asset codes
var AssetCodePattern = regexp.MustCompile(`^[A-Z]{3,4}$`)

// TransactionCodePattern is the regex pattern for transaction codes
var TransactionCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

// ValidateAssetCode checks if an asset code is valid.
// Asset codes should be 3-4 uppercase letters (e.g., USD, EUR, BTC).
//
// Example:
//
//	if err := core.ValidateAssetCode("USD"); err != nil {
//	    log.Fatal(err)
//	}
func ValidateAssetCode(assetCode string) error {
	if assetCode == "" {
		return errors.New("asset code is required")
	}

	if !AssetCodePattern.MatchString(assetCode) {
		return fmt.Errorf("invalid asset code format: %s (must be 3-4 uppercase letters)", assetCode)
	}

	return nil
}

// ValidateAccountAlias checks if an account alias is valid.
// Account aliases should be alphanumeric with optional underscores and hyphens.
//
// Example:
//
//	if err := core.ValidateAccountAlias("savings_account"); err != nil {
//	    log.Fatal(err)
//	}
func ValidateAccountAlias(alias string) error {
	if alias == "" {
		return errors.New("account alias cannot be empty")
	}

	if !AccountAliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid account alias format: %s (must be alphanumeric with optional underscores and hyphens, max 50 chars)", alias)
	}

	return nil
}

// ValidateTransactionCode checks if a transaction code is valid.
// Transaction codes should be alphanumeric with optional underscores and hyphens.
//
// Example:
//
//	if err := core.ValidateTransactionCode("TX_123456"); err != nil {
//	    log.Fatal(err)
//	}
func ValidateTransactionCode(code string) error {
	if code == "" {
		return errors.New("transaction code cannot be empty")
	}

	if !TransactionCodePattern.MatchString(code) {
		return fmt.Errorf("invalid transaction code format: %s (must be alphanumeric with optional underscores and hyphens, max 100 chars)", code)
	}

	return nil
}

// ValidateMetadata checks if transaction metadata is valid.
// This function verifies that metadata values are of supported types.
//
// Example:
//
//	metadata := map[string]any{
//	    "reference": "inv123",
//	    "amount": 100.50,
//	    "customer_id": 12345,
//	}
//	if err := core.ValidateMetadata(metadata); err != nil {
//	    log.Fatal(err)
//	}
func ValidateMetadata(metadata map[string]any) error {
	if metadata == nil {
		return nil // Empty metadata is valid
	}

	for key, value := range metadata {
		if key == "" {
			return errors.New("metadata keys cannot be empty")
		}

		if err := validateMetadataValue(key, value); err != nil {
			return err
		}
	}

	return nil
}

// validateMetadataValue validates a single metadata value and handles nested structures
func validateMetadataValue(key string, value any) error {
	if !isValidMetadataValueType(value) {
		return fmt.Errorf("invalid metadata value type for key '%s': %T (must be string, number, boolean, or nil)", key, value)
	}

	// Check for nested maps
	if nestedMap, ok := value.(map[string]any); ok {
		if err := ValidateMetadata(nestedMap); err != nil {
			return fmt.Errorf("invalid nested metadata at key '%s': %w", key, err)
		}
	}

	// Check for arrays
	if array, ok := value.([]any); ok {
		if err := validateMetadataArray(key, array); err != nil {
			return err
		}
	}

	return nil
}

// validateMetadataArray validates an array of metadata values
func validateMetadataArray(key string, array []any) error {
	for i, item := range array {
		if !isValidMetadataValueType(item) {
			return fmt.Errorf("invalid metadata array item type at index %d for key '%s': %T (must be string, number, boolean, or nil)", i, key, item)
		}

		// Check for nested maps in arrays
		if nestedMap, ok := item.(map[string]any); ok {
			if err := ValidateMetadata(nestedMap); err != nil {
				return fmt.Errorf("invalid nested metadata in array at key '%s', index %d: %w", key, i, err)
			}
		}
	}

	return nil
}

// isValidMetadataValueType checks if a value is of a type supported in metadata
func isValidMetadataValueType(value any) bool {
	switch value.(type) {
	case string, int, int32, int64, float32, float64, bool, nil, map[string]any, []any:
		return true
	default:
		return false
	}
}

// ValidateDateRange checks if a date range is valid.
// The start date must not be after the end date.
//
// Example:
//
//	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
//	end := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)
//	if err := core.ValidateDateRange(start, end); err != nil {
//	    log.Fatal(err)
//	}
func ValidateDateRange(start, end time.Time) error {
	// Check if start date is after end date
	if start.After(end) {
		return fmt.Errorf("start date (%s) cannot be after end date (%s)",
			start.Format(time.RFC3339),
			end.Format(time.RFC3339))
	}

	return nil
}

// ValidateAccountType validates if the account type is one of the supported types
// in the Midaz system.
func ValidateAccountType(accountType string) error {
	if accountType == "" {
		return errors.New("account type is required")
	}

	// Use commons.ValidateAccountType to ensure consistency with backend APIs
	if err := commons.ValidateAccountType(accountType); err != nil {
		// Convert the error to a more user-friendly message
		// Create a list of valid types for the error message
		validTypes := []string{"deposit", "savings", "loans", "marketplace", "creditCard"}

		return fmt.Errorf("invalid account type: %s. Valid types are: %s",
			accountType, strings.Join(validTypes, ", "))
	}

	return nil
}

// ValidateAssetType validates if the asset type is one of the supported types
// in the Midaz system.
func ValidateAssetType(assetType string) error {
	if assetType == "" {
		return errors.New("asset type is required")
	}

	// Use commons.ValidateType to ensure consistency with backend APIs
	// Note: commons.ValidateType expects lowercase types, so we convert to lowercase
	if err := commons.ValidateType(strings.ToLower(assetType)); err != nil {
		// Create a list of valid types for the error message
		validTypes := []string{"crypto", "currency", "commodity", "others"}

		return fmt.Errorf("invalid asset type: %s. Valid types are: %s",
			assetType, strings.Join(validTypes, ", "))
	}

	return nil
}

// ValidateCurrencyCode checks if the currency code is valid according to ISO 4217.
func ValidateCurrencyCode(code string) error {
	if code == "" {
		return errors.New("currency code cannot be empty")
	}

	// Use commons.ValidateCurrency to ensure consistency with backend APIs
	if err := commons.ValidateCurrency(code); err != nil {
		return fmt.Errorf("invalid currency code: %s", code)
	}

	return nil
}

// ValidateCountryCode checks if the country code is valid according to ISO 3166-1 alpha-2.
func ValidateCountryCode(code string) error {
	if code == "" {
		return errors.New("country code cannot be empty")
	}

	// Use commons.ValidateCountryAddress to ensure consistency with backend APIs
	if err := commons.ValidateCountryAddress(code); err != nil {
		return fmt.Errorf("invalid country code: %s (must be a valid ISO 3166-1 alpha-2 code)", code)
	}

	return nil
}

// Address is a simplified address structure for validation purposes.
type Address struct {
	Line1   string
	Line2   *string
	ZipCode string
	City    string
	State   string
	Country string
}

// ValidateAddress validates an address structure for completeness and correctness.
func ValidateAddress(address *Address) error {
	if address == nil {
		return errors.New("address cannot be nil")
	}

	if address.Line1 == "" {
		return errors.New("address line 1 is required")
	}

	if len(address.Line1) > 100 {
		return errors.New("address line 1 must be at most 100 characters")
	}

	if address.Line2 != nil && len(*address.Line2) > 100 {
		return errors.New("address line 2 must be at most 100 characters")
	}

	if address.ZipCode == "" {
		return errors.New("zip code is required")
	}

	if len(address.ZipCode) > 20 {
		return errors.New("zip code must be at most 20 characters")
	}

	if address.City == "" {
		return errors.New("city is required")
	}

	if len(address.City) > 100 {
		return errors.New("city must be at most 100 characters")
	}

	if address.State == "" {
		return errors.New("state is required")
	}

	if len(address.State) > 100 {
		return errors.New("state must be at most 100 characters")
	}

	if address.Country == "" {
		return errors.New("country is required")
	}

	// Validate country code
	if err := ValidateCountryCode(address.Country); err != nil {
		return fmt.Errorf("invalid country: %w", err)
	}

	return nil
}
