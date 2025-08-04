// Package format provides formatting utilities for the Midaz SDK.
// It handles formatting of amounts, dates, times, durations, transactions,
// and other complex data types for display purposes.
package format

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
)

// Common formatting options and configurations

// DurationOption defines a function that configures DurationOptions
type DurationOption func(*DurationOptions) error

// DurationOptions contains options for formatting durations
type DurationOptions struct {
	// Precision is the number of decimal places to show for seconds
	Precision int
	// UseShortUnits determines whether to use short unit names (s, ms) vs long (seconds, milliseconds)
	UseShortUnits bool
	// MaxComponents is the maximum number of components to show (e.g., 2 would show hours and minutes, but not seconds)
	MaxComponents int
}

// DefaultDurationOptions returns the default options for duration formatting
func DefaultDurationOptions() *DurationOptions {
	return &DurationOptions{
		Precision:     2,
		UseShortUnits: true,
		MaxComponents: 2,
	}
}

// WithPrecision sets the number of decimal places to show for seconds
func WithPrecision(precision int) DurationOption {
	return func(o *DurationOptions) error {
		if precision < 0 {
			return fmt.Errorf("precision cannot be negative: %d", precision)
		}

		o.Precision = precision

		return nil
	}
}

// WithShortUnits configures whether to use short unit names
func WithShortUnits(useShort bool) DurationOption {
	return func(o *DurationOptions) error {
		o.UseShortUnits = useShort
		return nil
	}
}

// WithMaxComponents sets the maximum number of components to show
func WithMaxComponents(max int) DurationOption {
	return func(o *DurationOptions) error {
		if max <= 0 {
			return fmt.Errorf("max components must be positive: %d", max)
		}

		o.MaxComponents = max

		return nil
	}
}

// DateTimeOption defines a function that configures DateTimeOptions
type DateTimeOption func(*DateTimeOptions) error

// DateTimeOptions contains options for formatting dates and times
type DateTimeOptions struct {
	// Format to use for the date/time - if empty, defaults are used based on each function
	Format string
	// DefaultOnZero determines whether to return a default value for zero time
	DefaultOnZero bool
	// DefaultValue is the string to return when time is zero and DefaultOnZero is true
	DefaultValue string
	// UseUTC determines whether to convert time to UTC before formatting
	UseUTC bool
}

// DefaultDateTimeOptions returns the default options for date/time formatting
func DefaultDateTimeOptions() *DateTimeOptions {
	return &DateTimeOptions{
		Format:        "",
		DefaultOnZero: true,
		DefaultValue:  "N/A",
		UseUTC:        false,
	}
}

// WithFormat sets a custom format string for date/time formatting
func WithFormat(format string) DateTimeOption {
	return func(o *DateTimeOptions) error {
		if format == "" {
			return errors.New("format cannot be empty")
		}

		o.Format = format

		return nil
	}
}

// WithDateOnly configures the formatter to use date-only format
func WithDateOnly() DateTimeOption {
	return func(o *DateTimeOptions) error {
		o.Format = "2006-01-02"
		return nil
	}
}

// WithTimeOnly configures the formatter to use time-only format
func WithTimeOnly() DateTimeOption {
	return func(o *DateTimeOptions) error {
		o.Format = "15:04:05"
		return nil
	}
}

// WithISO8601 configures the formatter to use ISO8601 format
func WithISO8601() DateTimeOption {
	return func(o *DateTimeOptions) error {
		o.Format = time.RFC3339
		return nil
	}
}

// WithDefaultOnZero configures whether to return a default value for zero time
func WithDefaultOnZero(enabled bool) DateTimeOption {
	return func(o *DateTimeOptions) error {
		o.DefaultOnZero = enabled
		return nil
	}
}

// WithDefaultValue sets the default value to return when time is zero
func WithDefaultValue(value string) DateTimeOption {
	return func(o *DateTimeOptions) error {
		o.DefaultValue = value
		return nil
	}
}

// WithUTC configures whether to convert time to UTC before formatting
func WithUTC(useUTC bool) DateTimeOption {
	return func(o *DateTimeOptions) error {
		o.UseUTC = useUTC
		return nil
	}
}

// AmountOption defines a function that configures AmountOptions
type AmountOption func(*AmountOptions) error

// AmountOptions contains options for formatting monetary amounts
type AmountOptions struct {
	// IncludeSymbol determines whether to include a currency symbol
	IncludeSymbol bool
	// SymbolPosition can be "prefix" or "suffix"
	SymbolPosition string
	// ThousandsSeparator is the character used to separate thousands
	ThousandsSeparator string
	// DecimalSeparator is the character used as decimal point
	DecimalSeparator string
}

// DefaultAmountOptions returns the default options for amount formatting
func DefaultAmountOptions() *AmountOptions {
	return &AmountOptions{
		IncludeSymbol:      false,
		SymbolPosition:     "suffix",
		ThousandsSeparator: "",
		DecimalSeparator:   ".",
	}
}

// WithCurrencySymbol configures whether to include a currency symbol and its position
func WithCurrencySymbol(include bool, position string) AmountOption {
	return func(o *AmountOptions) error {
		o.IncludeSymbol = include

		if include && position != "" {
			if position != "prefix" && position != "suffix" {
				return fmt.Errorf("symbol position must be 'prefix' or 'suffix', got '%s'", position)
			}

			o.SymbolPosition = position
		}

		return nil
	}
}

// WithThousandsSeparator sets the thousands separator character
func WithThousandsSeparator(sep string) AmountOption {
	return func(o *AmountOptions) error {
		o.ThousandsSeparator = sep
		return nil
	}
}

// WithDecimalSeparator sets the decimal separator character
func WithDecimalSeparator(sep string) AmountOption {
	return func(o *AmountOptions) error {
		if sep == "" {
			return errors.New("decimal separator cannot be empty")
		}

		o.DecimalSeparator = sep

		return nil
	}
}

// TransactionOption defines a function that configures TransactionOptions
type TransactionOption func(*TransactionOptions) error

// TransactionOptions contains options for formatting transactions
type TransactionOptions struct {
	// IncludeID determines whether to include the transaction ID
	IncludeID bool
	// IncludeTimestamp determines whether to include the transaction timestamp
	IncludeTimestamp bool
	// VerboseAccountInfo determines whether to show detailed account information
	VerboseAccountInfo bool
	// CustomStatusMapping maps status codes to display strings
	CustomStatusMapping map[string]string
}

// DefaultTransactionOptions returns the default options for transaction formatting
func DefaultTransactionOptions() *TransactionOptions {
	return &TransactionOptions{
		IncludeID:           false,
		IncludeTimestamp:    false,
		VerboseAccountInfo:  false,
		CustomStatusMapping: map[string]string{},
	}
}

// WithTransactionID configures whether to include the transaction ID
func WithTransactionID(include bool) TransactionOption {
	return func(o *TransactionOptions) error {
		o.IncludeID = include
		return nil
	}
}

// WithTransactionTimestamp configures whether to include the transaction timestamp
func WithTransactionTimestamp(include bool) TransactionOption {
	return func(o *TransactionOptions) error {
		o.IncludeTimestamp = include
		return nil
	}
}

// WithVerboseAccountInfo configures whether to show detailed account information
func WithVerboseAccountInfo(verbose bool) TransactionOption {
	return func(o *TransactionOptions) error {
		o.VerboseAccountInfo = verbose
		return nil
	}
}

// WithCustomStatusMapping sets custom mappings for transaction status codes
func WithCustomStatusMapping(mapping map[string]string) TransactionOption {
	return func(o *TransactionOptions) error {
		if mapping == nil {
			return errors.New("status mapping cannot be nil")
		}

		o.CustomStatusMapping = mapping

		return nil
	}
}

// FormatAmount converts a numeric amount and scale to a human-readable string representation.
// For example, an amount of 12345 with scale 2 becomes "123.45".
//
// Example:
//
//	formattedAmount := format.FormatAmount(12345, 2)
//	// Result: "123.45"
func FormatAmount(amount int64, scale int) string {
	result, _ := FormatAmountWithOptions(amount, scale)
	return result
}

// FormatAmountWithOptions formats an amount with the given options.
func FormatAmountWithOptions(amount int64, scale int, opts ...AmountOption) (string, error) {
	// Start with default options
	options := DefaultAmountOptions()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return "", fmt.Errorf("failed to apply amount option: %w", err)
		}
	}

	// Skip decimal handling for scale <= 0
	if scale <= 0 {
		return fmt.Sprintf("%d", amount), nil
	}

	// Handle negative amounts
	negative := amount < 0
	if negative {
		amount = -amount
	}

	// Convert to string and pad with leading zeros if needed
	amountStr := fmt.Sprintf("%d", amount)
	for len(amountStr) <= scale {
		amountStr = "0" + amountStr
	}

	// Split into whole and decimal parts
	decimalPos := len(amountStr) - scale
	wholePart := amountStr[:decimalPos]

	if wholePart == "" {
		wholePart = "0"
	}

	decimalPart := amountStr[decimalPos:]

	// Apply thousands separator if specified
	if options.ThousandsSeparator != "" {
		newWholePart := ""

		for i := len(wholePart); i > 0; i -= 3 {
			start := i - 3
			if start < 0 {
				start = 0
			}

			group := wholePart[start:i]

			if newWholePart != "" {
				newWholePart = group + options.ThousandsSeparator + newWholePart
			} else {
				newWholePart = group
			}
		}

		wholePart = newWholePart
	}

	// Combine with decimal separator
	result := wholePart + options.DecimalSeparator + decimalPart

	// Add negative sign if needed
	if negative {
		result = "-" + result
	}

	return result, nil
}

// FormatCurrency formats a currency amount with the given scale and currency code.
// For backward compatibility, this calls FormatCurrencyWithOptions with default options.
func FormatCurrency(amount int64, scale int64, currencyCode string) string {
	result, _ := FormatCurrencyWithOptions(amount, scale, currencyCode)
	return result
}

// FormatCurrencyWithOptions formats a currency amount with the given options.
func FormatCurrencyWithOptions(amount int64, scale int64, currencyCode string, opts ...AmountOption) (string, error) {
	// First format the amount with the given options
	formattedAmount, err := FormatAmountWithOptions(amount, int(scale), opts...)
	if err != nil {
		return "", fmt.Errorf("failed to format amount: %w", err)
	}

	// Then append the currency code
	return formattedAmount + " " + currencyCode, nil
}

// FormatDateTime formats a time.Time value in a human-readable format.
// For backward compatibility, this calls FormatDateTimeWithOptions with default options.
func FormatDateTime(t time.Time) string {
	result, _ := FormatDateTimeWithOptions(t)
	return result
}

// FormatDateTimeWithOptions formats a time.Time value with the given options.
func FormatDateTimeWithOptions(t time.Time, opts ...DateTimeOption) (string, error) {
	// Start with default options
	options := DefaultDateTimeOptions()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return "", fmt.Errorf("failed to apply date/time option: %w", err)
		}
	}

	// Handle zero time
	if t.IsZero() && options.DefaultOnZero {
		return options.DefaultValue, nil
	}

	// If UTC is requested, convert time
	if options.UseUTC {
		t = t.UTC()
	}

	// Determine format to use
	format := options.Format
	if format == "" {
		format = "2006-01-02 15:04:05"
	}

	return t.Format(format), nil
}

// FormatDate formats a time.Time value as a date only.
// For backward compatibility, this calls FormatDateTimeWithOptions with date-only format.
func FormatDate(t time.Time) string {
	result, _ := FormatDateTimeWithOptions(t, WithDateOnly())
	return result
}

// FormatTime formats a time.Time value as a time only.
// For backward compatibility, this calls FormatDateTimeWithOptions with time-only format.
func FormatTime(t time.Time) string {
	result, _ := FormatDateTimeWithOptions(t, WithTimeOnly())
	return result
}

// FormatISODate formats a time.Time as an ISO date string (YYYY-MM-DD).
//
// Example:
//
//	isoDate := format.FormatISODate(time.Now())
//	// Result: "2025-04-02"
//
// For backward compatibility, this calls FormatDateTimeWithOptions with date-only format.
func FormatISODate(t time.Time) string {
	result, _ := FormatDateTimeWithOptions(t, WithDateOnly(), WithUTC(true))
	return result
}

// FormatISODateTime formats a time.Time as an ISO date-time string (YYYY-MM-DDThh:mm:ssZ).
//
// Example:
//
//	isoDateTime := format.FormatISODateTime(time.Now())
//	// Result: "2025-04-02T15:04:05Z"
//
// For backward compatibility, this calls FormatDateTimeWithOptions with ISO8601 format.
func FormatISODateTime(t time.Time) string {
	result, _ := FormatDateTimeWithOptions(t, WithISO8601(), WithUTC(true))
	return result
}

// FormatDuration formats a duration in a human-readable format.
// For backward compatibility, this calls FormatDurationWithOptions with default options.
func FormatDuration(d time.Duration) string {
	result, _ := FormatDurationWithOptions(d)
	return result
}

// FormatDurationWithOptions formats a duration with the given options.
func FormatDurationWithOptions(d time.Duration, opts ...DurationOption) (string, error) {
	// Start with default options
	options := DefaultDurationOptions()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return "", fmt.Errorf("failed to apply duration option: %w", err)
		}
	}

	// Format based on duration magnitude and options
	if d < time.Millisecond {
		unitStr := "μs"
		if !options.UseShortUnits {
			unitStr = " microseconds"
		}

		return fmt.Sprintf("%d%s", d.Microseconds(), unitStr), nil
	} else if d < time.Second {
		unitStr := "ms"
		if !options.UseShortUnits {
			unitStr = " milliseconds"
		}

		return fmt.Sprintf("%d%s", d.Milliseconds(), unitStr), nil
	} else if d < time.Minute {
		unitStr := "s"
		if !options.UseShortUnits {
			unitStr = " seconds"
		}

		formatStr := "%." + fmt.Sprintf("%d", options.Precision) + "f%s"

		return fmt.Sprintf(formatStr, float64(d)/float64(time.Second), unitStr), nil
	}

	// For longer durations, break into components
	components := make([]string, 0, options.MaxComponents)

	// Calculate components
	hours := d / time.Hour
	d = d % time.Hour

	minutes := d / time.Minute
	d = d % time.Minute

	seconds := d / time.Second

	// Add components in order of significance, respecting MaxComponents
	if hours > 0 {
		hourSuffix := "h"
		if !options.UseShortUnits {
			hourSuffix = " hours"
		}

		components = append(components, fmt.Sprintf("%d%s", hours, hourSuffix))
	}

	if len(components) < options.MaxComponents && minutes > 0 {
		minuteSuffix := "m"
		if !options.UseShortUnits {
			minuteSuffix = " minutes"
		}

		components = append(components, fmt.Sprintf("%d%s", minutes, minuteSuffix))
	}

	if len(components) < options.MaxComponents && seconds > 0 {
		secondSuffix := "s"
		if !options.UseShortUnits {
			secondSuffix = " seconds"
		}

		components = append(components, fmt.Sprintf("%d%s", seconds, secondSuffix))
	}

	// If no components added (e.g., duration is 0), add a zero seconds component
	if len(components) == 0 {
		secondSuffix := "s"
		if !options.UseShortUnits {
			secondSuffix = " seconds"
		}

		components = append(components, fmt.Sprintf("0%s", secondSuffix))
	}

	// Join components with spaces
	return strings.Join(components, " "), nil
}

// FormatTransaction creates a user-friendly summary of a transaction.
//
// Example:
//
//	tx := &models.Transaction{
//	    ID: "tx_123456",
//	    Amount: 10000,
//	    Scale: 2,
//	    AssetCode: "USD",
//	    Status: models.Status{Code: "COMPLETED"},
//	    Operations: []models.Operation{
//	        {
//	            Type: "DEBIT",
//	            AccountID: "acc_source",
//	            AccountAlias: ptr.String("savings"),
//	        },
//	        {
//	            Type: "CREDIT",
//	            AccountID: "acc_dest",
//	            AccountAlias: ptr.String("checking"),
//	        },
//	    },
//	}
//	summary := format.FormatTransaction(tx)
//	fmt.Println(summary)
//	// Result: "Transfer: 100.00 USD from savings to checking (Completed)"
func FormatTransaction(tx *models.Transaction) string {
	result, _ := FormatTransactionWithOptions(tx)
	return result
}

// FormatTransactionWithOptions formats a transaction with the given options.
func FormatTransactionWithOptions(tx *models.Transaction, opts ...TransactionOption) (string, error) {
	if tx == nil {
		return "Invalid transaction: nil", nil
	}

	// Start with default options
	options := DefaultTransactionOptions()

	// Apply all provided options
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return "", fmt.Errorf("failed to apply transaction option: %w", err)
		}
	}

	// Determine transaction type based on operations
	txType := determineTransactionType(tx)

	// Use the amount as-is since it's already formatted as a decimal string
	amountStr := tx.Amount

	// Build summary with optional ID prefix
	summary := ""
	if options.IncludeID {
		summary = fmt.Sprintf("%s - ", tx.ID)
	}

	// Add timestamp if requested
	if options.IncludeTimestamp && !tx.CreatedAt.IsZero() {
		timestampStr, _ := FormatDateTimeWithOptions(tx.CreatedAt)
		summary += fmt.Sprintf("%s - ", timestampStr)
	}

	// Add transaction type and amount
	summary += fmt.Sprintf("%s: %s %s", txType, amountStr, tx.AssetCode)

	// Get status string, using custom mapping if available
	statusStr := ""

	if tx.Status.Code != "" {
		if mappedStatus, exists := options.CustomStatusMapping[tx.Status.Code]; exists {
			statusStr = mappedStatus
		} else {
			statusStr = tx.Status.Code
			// Capitalize first letter
			if len(statusStr) > 0 {
				statusStr = strings.ToUpper(statusStr[:1]) + strings.ToLower(statusStr[1:])
			}
		}
	}

	// Add accounts information if available
	if len(tx.Operations) > 0 {
		accountInfo := extractAccountsFromOperations(tx.Operations)

		if accountInfo != "" {
			summary += " " + accountInfo
		}
	}

	// Add status
	summary += fmt.Sprintf(" (%s)", statusStr)

	return summary, nil
}

// determineTransactionType analyzes a transaction to determine its type.
func determineTransactionType(tx *models.Transaction) string {
	// Default type
	txType := "Transaction"

	// Check if we have operations to determine type
	if len(tx.Operations) > 0 {
		// Look for operations with specific patterns
		hasExternal := false
		hasInternal := false

		for _, op := range tx.Operations {
			if op.AccountAlias != nil && strings.HasPrefix(*op.AccountAlias, "@external/") {
				hasExternal = true
			} else {
				hasInternal = true
			}
		}

		// Determine type based on patterns
		if hasExternal && hasInternal {
			// Check first operation to see if it's from external (deposit) or to external (withdrawal)
			if tx.Operations[0].AccountAlias != nil && strings.HasPrefix(*tx.Operations[0].AccountAlias, "@external/") {
				txType = "Deposit"
			} else {
				txType = "Withdrawal"
			}
		} else if hasInternal && !hasExternal {
			txType = "Transfer"
		}
	}

	return txType
}

// extractAccountsFromOperations extracts a summary of the accounts involved in a transaction.
func extractAccountsFromOperations(operations []models.Operation) string {
	if len(operations) == 0 {
		return ""
	}

	fromAccounts := []string{}
	toAccounts := []string{}

	for _, op := range operations {
		// Skip external accounts for cleaner output
		if op.AccountAlias != nil && strings.HasPrefix(*op.AccountAlias, "@external/") {
			continue
		}

		switch op.Type {
		case "DEBIT":
			if op.AccountAlias != nil {
				fromAccounts = append(fromAccounts, *op.AccountAlias)
			} else {
				fromAccounts = append(fromAccounts, op.AccountID)
			}

		case "CREDIT":
			if op.AccountAlias != nil {
				toAccounts = append(toAccounts, *op.AccountAlias)
			} else {
				toAccounts = append(toAccounts, op.AccountID)
			}
		}
	}

	result := ""

	// Format the from accounts
	if len(fromAccounts) > 0 {
		result += "from "

		if len(fromAccounts) == 1 {
			result += fromAccounts[0]
		} else {
			result += fmt.Sprintf("multiple accounts (%d)", len(fromAccounts))
		}
	}

	// Format the to accounts
	if len(toAccounts) > 0 {
		if result != "" {
			result += " "
		}

		result += "to "

		if len(toAccounts) == 1 {
			result += toAccounts[0]
		} else {
			result += fmt.Sprintf("multiple accounts (%d)", len(toAccounts))
		}
	}

	return result
}
