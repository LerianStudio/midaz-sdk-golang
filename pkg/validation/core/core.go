// Package core provides fundamental validation utilities for the Midaz SDK.
//
// This package contains primitive validation functions that don't depend on
// any model structures, making it usable by both the models package and the
// validation package without creating circular dependencies.
package core

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

const (
	defaultMaxMetadataSize      = 4096
	defaultMaxStringLength      = 2000
	defaultMaxAddressLineLength = 256
	defaultMaxZipCodeLength     = 20
	defaultMaxCityLength        = 100
	defaultMaxStateLength       = 100
	maxLegacyAddressLineLength  = 100
	maxMetadataKeyLength        = 100
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
		MaxMetadataSize:      defaultMaxMetadataSize,
		MaxStringLength:      defaultMaxStringLength,
		MaxAddressLineLength: defaultMaxAddressLineLength,
		MaxZipCodeLength:     defaultMaxZipCodeLength,
		MaxCityLength:        defaultMaxCityLength,
		MaxStateLength:       defaultMaxStateLength,
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
// (e.g. "@external/USD", "@external/BRL"). The captured asset code is bounded
// to 3-4 uppercase letters to match the Midaz backend's strict ISO-4217-style
// contract — widening this to {1,100} broke "looks like an external alias but
// has the wrong shape" error messages and let through nonsense like
// "@external/foobarbaz".
var ExternalAccountPattern = regexp.MustCompile(`^@external/([A-Z]{3,4})$`)

// AccountAliasPattern is the regex pattern for account aliases.
//
// Midaz aliases support letters, digits, underscores, hyphens, dots, colons,
// and an optional leading "@". The 50-character cap matches the historic
// strict shape; the previous 100-character cap was too permissive and routinely
// let test fixtures through that the backend would later reject.
//
// Example aliases that match: "@treasury_checking", "@user.balance:USD",
// "savings-account-2024", "@alice".
var AccountAliasPattern = regexp.MustCompile(`^@?[a-zA-Z0-9_.:-]{1,50}$`)

// AssetCodePattern is the regex pattern for asset codes (e.g. "USD", "BRL",
// "USDT"). The 3-4 uppercase-letter bound matches ISO 4217 currency codes
// and the most common stablecoin tickers; broader inputs are rejected at the
// SDK boundary so we surface the error close to the call site instead of
// letting the backend reply with a generic 400.
var AssetCodePattern = regexp.MustCompile(`^[A-Z]{3,4}$`)

// TransactionCodePattern is the regex pattern for transaction codes
var TransactionCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

// ValidateAssetCode checks if an asset code is valid.
// Asset codes must be 3-4 uppercase letters (e.g., USD, EUR, BTC, USDT).
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
// Account aliases may include letters, numbers, underscores, hyphens, dots,
// colons, and an optional leading @, up to 50 characters total.
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
		return fmt.Errorf("invalid account alias format: %s (must contain only letters, numbers, underscores, hyphens, dots, colons, and an optional leading @; max 50 chars)", alias)
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

		if len(key) > maxMetadataKeyLength {
			return fmt.Errorf("metadata key '%s' must be at most 100 characters", key)
		}

		// MongoDB reserves '.' as a path separator in dotted keys and any '$'
		// prefix as an operator (e.g. $set, $inc). Allowing either through
		// here would let unsanitized metadata escape into a path/operator
		// position once the backend persists it. Reject both at the SDK
		// boundary so the failure is surfaced at the source.
		if strings.Contains(key, ".") || strings.HasPrefix(key, "$") {
			return fmt.Errorf("metadata key '%s' must not contain '.' or start with '$' (reserved by storage layer)", key)
		}

		if err := validateMetadataValue(key, value); err != nil {
			return err
		}
	}

	return nil
}

// validateMetadataValue validates a single flat metadata value.
func validateMetadataValue(key string, value any) error {
	if !isValidMetadataValueType(value) {
		return fmt.Errorf("invalid metadata value type for key '%s': %T (must be string, number, boolean, array, or nil)", key, value)
	}

	if err := validateFiniteMetadataNumber(key, value); err != nil {
		return err
	}

	if len(fmt.Sprint(value)) > defaultMaxStringLength {
		return fmt.Errorf("metadata value for key '%s' must be at most 2000 characters", key)
	}

	return nil
}

func validateFiniteMetadataNumber(key string, value any) error {
	switch v := value.(type) {
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return fmt.Errorf("metadata value for key '%s' must be finite", key)
		}
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("metadata value for key '%s' must be finite", key)
		}
	}

	return nil
}

// isValidMetadataValueType checks if a value is of a type supported in metadata
func isValidMetadataValueType(value any) bool {
	switch value := value.(type) {
	case string, int, int32, int64, float32, float64, bool, nil:
		return true
	case []any:
		for _, item := range value {
			if !isValidMetadataValueType(item) {
				return false
			}
		}

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

// allowedAccountTypes is the closed set of account types accepted by the
// Midaz backend. The list intentionally includes accounting categories
// (expense, revenue, equity, liability) that the SDK has historically
// surfaced even though the backend's own ValidateAccountType is narrower —
// rejecting them here would break SDK consumers that explicitly model
// chart-of-accounts categories.
var allowedAccountTypes = map[string]struct{}{
	"deposit":     {},
	"savings":     {},
	"loans":       {},
	"marketplace": {},
	"creditCard":  {},
	"expense":     {},
	"revenue":     {},
	"equity":      {},
	"liability":   {},
}

// ValidateAccountType validates that the account type is one of the supported
// account types in the Midaz system. The check is case-sensitive and uses an
// allowlist; this is strict-by-default behavior that callers can rely on at
// the SDK boundary instead of having the backend reject the request later.
//
// Allowed values: deposit, savings, loans, marketplace, creditCard, expense,
// revenue, equity, liability.
func ValidateAccountType(accountType string) error {
	if accountType == "" {
		return errors.New("account type is required")
	}

	if _, ok := allowedAccountTypes[accountType]; !ok {
		return fmt.Errorf(
			"account type must be one of: deposit, savings, loans, marketplace, creditCard, expense, revenue, equity, liability (got %q)",
			accountType,
		)
	}

	return nil
}

// ValidateAssetType validates if the asset type is one of the supported types
// in the Midaz system.
func ValidateAssetType(assetType string) error {
	if assetType == "" {
		return errors.New("asset type is required")
	}

	if _, ok := allowedAssetTypes[strings.ToLower(assetType)]; !ok {
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

	if _, ok := allowedCurrencyCodes[code]; !ok {
		return fmt.Errorf("invalid currency code: %s", code)
	}

	return nil
}

// ValidateCountryCode checks if the country code is valid according to ISO 3166-1 alpha-2.
func ValidateCountryCode(code string) error {
	if code == "" {
		return errors.New("country code cannot be empty")
	}

	if _, ok := allowedCountryCodes[code]; !ok {
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

	if len(address.Line1) > maxLegacyAddressLineLength {
		return errors.New("address line 1 must be at most 100 characters")
	}

	if address.Line2 != nil && len(*address.Line2) > maxLegacyAddressLineLength {
		return errors.New("address line 2 must be at most 100 characters")
	}

	if address.ZipCode == "" {
		return errors.New("zip code is required")
	}

	if len(address.ZipCode) > defaultMaxZipCodeLength {
		return errors.New("zip code must be at most 20 characters")
	}

	if address.City == "" {
		return errors.New("city is required")
	}

	if len(address.City) > defaultMaxCityLength {
		return errors.New("city must be at most 100 characters")
	}

	if address.State == "" {
		return errors.New("state is required")
	}

	if len(address.State) > defaultMaxStateLength {
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
