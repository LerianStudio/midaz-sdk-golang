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

// Operation type constants for transaction operations.
const (
	opTypeDebit  = "DEBIT"
	opTypeCredit = "CREDIT"
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
func WithMaxComponents(maxComponents int) DurationOption {
	return func(o *DurationOptions) error {
		if maxComponents <= 0 {
			return fmt.Errorf("max components must be positive: %d", maxComponents)
		}

		o.MaxComponents = maxComponents

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

// Amount converts a numeric amount and scale to a human-readable string representation.
// For example, an amount of 12345 with scale 2 becomes "123.45".
//
// Example:
//
//	formattedAmount := format.Amount(12345, 2)
//	// Result: "123.45"
func Amount(amount int64, scale int) string {
	// AmountWithOptions with no options always succeeds
	result, _ := AmountWithOptions(amount, scale) //nolint:errcheck // default options never fail
	return result
}

// AmountWithOptions formats an amount with the given options.
func AmountWithOptions(amount int64, scale int, opts ...AmountOption) (string, error) {
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

// Currency formats a currency amount with the given scale and currency code.
// For backward compatibility, this calls CurrencyWithOptions with default options.
func Currency(amount int64, scale int64, currencyCode string) string {
	// CurrencyWithOptions with no options always succeeds
	result, _ := CurrencyWithOptions(amount, scale, currencyCode) //nolint:errcheck // default options never fail
	return result
}

// CurrencyWithOptions formats a currency amount with the given options.
func CurrencyWithOptions(amount int64, scale int64, currencyCode string, opts ...AmountOption) (string, error) {
	// First format the amount with the given options
	formattedAmount, err := AmountWithOptions(amount, int(scale), opts...)
	if err != nil {
		return "", fmt.Errorf("failed to format amount: %w", err)
	}

	// Then append the currency code
	return formattedAmount + " " + currencyCode, nil
}

// DateTime formats a time.Time value in a human-readable format.
// For backward compatibility, this calls DateTimeWithOptions with default options.
func DateTime(t time.Time) string {
	// DateTimeWithOptions with no options always succeeds
	result, _ := DateTimeWithOptions(t) //nolint:errcheck // default options never fail
	return result
}

// DateTimeWithOptions formats a time.Time value with the given options.
func DateTimeWithOptions(t time.Time, opts ...DateTimeOption) (string, error) {
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

// Date formats a time.Time value as a date only.
// For backward compatibility, this calls DateTimeWithOptions with date-only format.
func Date(t time.Time) string {
	// WithDateOnly option always returns nil error
	result, _ := DateTimeWithOptions(t, WithDateOnly()) //nolint:errcheck // option never fails
	return result
}

// Time formats a time.Time value as a time only.
// For backward compatibility, this calls DateTimeWithOptions with time-only format.
func Time(t time.Time) string {
	// WithTimeOnly option always returns nil error
	result, _ := DateTimeWithOptions(t, WithTimeOnly()) //nolint:errcheck // option never fails
	return result
}

// ISODate formats a time.Time as an ISO date string (YYYY-MM-DD).
//
// Example:
//
//	isoDate := format.ISODate(time.Now())
//	// Result: "2025-04-02"
//
// For backward compatibility, this calls DateTimeWithOptions with date-only format.
func ISODate(t time.Time) string {
	// Options always return nil error
	result, _ := DateTimeWithOptions(t, WithDateOnly(), WithUTC(true)) //nolint:errcheck // options never fail
	return result
}

// ISODateTime formats a time.Time as an ISO date-time string (YYYY-MM-DDThh:mm:ssZ).
//
// Example:
//
//	isoDateTime := format.ISODateTime(time.Now())
//	// Result: "2025-04-02T15:04:05Z"
//
// For backward compatibility, this calls DateTimeWithOptions with ISO8601 format.
func ISODateTime(t time.Time) string {
	// Options always return nil error
	result, _ := DateTimeWithOptions(t, WithISO8601(), WithUTC(true)) //nolint:errcheck // options never fail
	return result
}

// Duration formats a duration in a human-readable format.
// For backward compatibility, this calls DurationWithOptions with default options.
func Duration(d time.Duration) string {
	// DurationWithOptions with no options always succeeds
	result, _ := DurationWithOptions(d) //nolint:errcheck // default options never fail
	return result
}

// DurationWithOptions formats a duration with the given options.
func DurationWithOptions(d time.Duration, opts ...DurationOption) (string, error) {
	options, err := applyDurationOptions(opts...)
	if err != nil {
		return "", err
	}

	// Format based on duration magnitude
	if result := formatSubSecondDuration(d, options); result != "" {
		return result, nil
	}

	if result := formatSecondDuration(d, options); result != "" {
		return result, nil
	}

	// For longer durations, break into components
	return formatLongDuration(d, options), nil
}

// applyDurationOptions applies all duration formatting options
func applyDurationOptions(opts ...DurationOption) (*DurationOptions, error) {
	options := DefaultDurationOptions()

	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, fmt.Errorf("failed to apply duration option: %w", err)
		}
	}

	return options, nil
}

// formatSubSecondDuration handles durations less than one second
func formatSubSecondDuration(d time.Duration, options *DurationOptions) string {
	if d >= time.Millisecond {
		return ""
	}

	unitStr := getTimeUnit("μs", "microseconds", options.UseShortUnits)

	return fmt.Sprintf("%d%s", d.Microseconds(), unitStr)
}

// formatSecondDuration handles durations in the second range
func formatSecondDuration(d time.Duration, options *DurationOptions) string {
	if d < time.Millisecond {
		return ""
	}

	if d < time.Second {
		unitStr := getTimeUnit("ms", "milliseconds", options.UseShortUnits)
		return fmt.Sprintf("%d%s", d.Milliseconds(), unitStr)
	}

	if d < time.Minute {
		unitStr := getTimeUnit("s", "seconds", options.UseShortUnits)
		formatStr := "%." + fmt.Sprintf("%d", options.Precision) + "f%s"

		return fmt.Sprintf(formatStr, float64(d)/float64(time.Second), unitStr)
	}

	return ""
}

// formatLongDuration handles durations longer than one minute
func formatLongDuration(d time.Duration, options *DurationOptions) string {
	components := buildDurationComponents(d, options)

	if len(components) == 0 {
		// Add zero seconds component if no other components
		secondSuffix := getTimeUnit("s", "seconds", options.UseShortUnits)
		components = append(components, fmt.Sprintf("0%s", secondSuffix))
	}

	return strings.Join(components, " ")
}

// buildDurationComponents creates formatted duration components
func buildDurationComponents(d time.Duration, options *DurationOptions) []string {
	components := make([]string, 0, options.MaxComponents)

	hours := d / time.Hour
	d = d % time.Hour

	minutes := d / time.Minute
	d = d % time.Minute

	seconds := d / time.Second

	// Add components in order of significance
	components = addDurationComponent(components, int64(hours), "h", "hours", options)
	components = addDurationComponent(components, int64(minutes), "m", "minutes", options)
	components = addDurationComponent(components, int64(seconds), "s", "seconds", options)

	return components
}

// addDurationComponent adds a duration component if it has a non-zero value and doesn't exceed max components
func addDurationComponent(components []string, value int64, shortUnit, longUnit string, options *DurationOptions) []string {
	if len(components) >= options.MaxComponents || value == 0 {
		return components
	}

	unitStr := getTimeUnit(shortUnit, longUnit, options.UseShortUnits)
	component := fmt.Sprintf("%d%s", value, unitStr)

	return append(components, component)
}

// getTimeUnit returns the appropriate time unit string based on formatting preferences
func getTimeUnit(shortUnit, longUnit string, useShort bool) string {
	if useShort {
		return shortUnit
	}

	return " " + longUnit
}

// Transaction creates a user-friendly summary of a transaction.
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
//	summary := format.Transaction(tx)
//	fmt.Println(summary)
//	// Result: "Transfer: 100.00 USD from savings to checking (Completed)"
func Transaction(tx *models.Transaction) string {
	// TransactionWithOptions with no options always succeeds
	result, _ := TransactionWithOptions(tx) //nolint:errcheck // default options never fail
	return result
}

// TransactionWithOptions formats a transaction with the given options.
func TransactionWithOptions(tx *models.Transaction, opts ...TransactionOption) (string, error) {
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
		// DateTimeWithOptions with no options always succeeds
		timestampStr, _ := DateTimeWithOptions(tx.CreatedAt) //nolint:errcheck // default options never fail
		summary += fmt.Sprintf("%s - ", timestampStr)
	}

	// Add transaction type and amount
	summary += fmt.Sprintf("%s: %s %s", txType, amountStr, tx.AssetCode)

	// Get status string, using custom mapping if available
	statusStr := resolveStatusString(tx.Status.Code, options.CustomStatusMapping)

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

// resolveStatusString resolves a status code to a display string.
func resolveStatusString(statusCode string, customMapping map[string]string) string {
	if statusCode == "" {
		return ""
	}

	if mappedStatus, exists := customMapping[statusCode]; exists {
		return mappedStatus
	}

	return capitalizeStatusCode(statusCode)
}

// capitalizeStatusCode capitalizes the first letter and lowercases the rest.
func capitalizeStatusCode(code string) string {
	if len(code) == 0 {
		return code
	}

	return strings.ToUpper(code[:1]) + strings.ToLower(code[1:])
}

// determineTransactionType analyzes a transaction to determine its type.
func determineTransactionType(tx *models.Transaction) string {
	if len(tx.Operations) == 0 {
		return "Transaction"
	}

	hasExternal, hasInternal := analyzeOperationPatterns(tx.Operations)

	return resolveTransactionType(hasExternal, hasInternal, tx.Operations[0])
}

// analyzeOperationPatterns checks operations for external and internal account patterns.
func analyzeOperationPatterns(operations []models.Operation) (hasExternal, hasInternal bool) {
	for _, op := range operations {
		if isExternalAccount(op) {
			hasExternal = true
		} else {
			hasInternal = true
		}
	}

	return hasExternal, hasInternal
}

// resolveTransactionType determines the transaction type based on operation patterns.
func resolveTransactionType(hasExternal, hasInternal bool, firstOp models.Operation) string {
	if hasExternal && hasInternal {
		if isExternalAccount(firstOp) {
			return "Deposit"
		}

		return "Withdrawal"
	}

	if hasInternal && !hasExternal {
		return "Transfer"
	}

	return "Transaction"
}

// extractAccountsFromOperations extracts a summary of the accounts involved in a transaction.
func extractAccountsFromOperations(operations []models.Operation) string {
	if len(operations) == 0 {
		return ""
	}

	fromAccounts, toAccounts := categorizeOperationAccounts(operations)

	return formatAccountsSummary(fromAccounts, toAccounts)
}

func categorizeOperationAccounts(operations []models.Operation) (fromAccounts, toAccounts []string) {
	fromAccounts = []string{}
	toAccounts = []string{}

	for _, op := range operations {
		if isExternalAccount(op) {
			continue
		}

		accountRef := getAccountReference(op)

		switch op.Type {
		case opTypeDebit:
			fromAccounts = append(fromAccounts, accountRef)
		case opTypeCredit:
			toAccounts = append(toAccounts, accountRef)
		}
	}

	return fromAccounts, toAccounts
}

func isExternalAccount(op models.Operation) bool {
	return op.AccountAlias != "" && strings.HasPrefix(op.AccountAlias, "@external/")
}

func getAccountReference(op models.Operation) string {
	if op.AccountAlias != "" {
		return op.AccountAlias
	}

	return op.AccountID
}

func formatAccountsSummary(fromAccounts, toAccounts []string) string {
	result := formatAccountList(fromAccounts, "from")

	toResult := formatAccountList(toAccounts, "to")
	if toResult != "" {
		if result != "" {
			result += " "
		}

		result += toResult
	}

	return result
}

func formatAccountList(accounts []string, prefix string) string {
	if len(accounts) == 0 {
		return ""
	}

	if len(accounts) == 1 {
		return prefix + " " + accounts[0]
	}

	return fmt.Sprintf("%s multiple accounts (%d)", prefix, len(accounts))
}

// Backward compatibility aliases - deprecated, use the shorter names instead

// FormatAmount is deprecated, use Amount instead.
var FormatAmount = Amount

// FormatAmountWithOptions is deprecated, use AmountWithOptions instead.
var FormatAmountWithOptions = AmountWithOptions

// FormatCurrency is deprecated, use Currency instead.
var FormatCurrency = Currency

// FormatCurrencyWithOptions is deprecated, use CurrencyWithOptions instead.
var FormatCurrencyWithOptions = CurrencyWithOptions

// FormatDateTime is deprecated, use DateTime instead.
var FormatDateTime = DateTime

// FormatDateTimeWithOptions is deprecated, use DateTimeWithOptions instead.
var FormatDateTimeWithOptions = DateTimeWithOptions

// FormatDate is deprecated, use Date instead.
var FormatDate = Date

// FormatTime is deprecated, use Time instead.
var FormatTime = Time

// FormatISODate is deprecated, use ISODate instead.
var FormatISODate = ISODate

// FormatISODateTime is deprecated, use ISODateTime instead.
var FormatISODateTime = ISODateTime

// FormatDuration is deprecated, use Duration instead.
var FormatDuration = Duration

// FormatDurationWithOptions is deprecated, use DurationWithOptions instead.
var FormatDurationWithOptions = DurationWithOptions

// FormatTransaction is deprecated, use Transaction instead.
var FormatTransaction = Transaction

// FormatTransactionWithOptions is deprecated, use TransactionWithOptions instead.
var FormatTransactionWithOptions = TransactionWithOptions
