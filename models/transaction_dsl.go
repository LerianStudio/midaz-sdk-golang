package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation/core"
	"github.com/shopspring/decimal"
)

// DSLAmount represents an amount with a value and asset code for DSL transactions.
// This is aligned with the Midaz DSL amount contract.
type DSLAmount struct {
	// Value is the exact decimal value of the amount.
	Value any `json:"value"`

	// Asset is the asset code for the amount
	Asset string `json:"asset,omitempty"`
}

// DSLFromTo represents a source or destination in a DSL transaction.
// This is aligned with the Midaz DSL source/destination contract.
type DSLFromTo struct {
	// Account is the identifier of the account
	Account string `json:"account"`

	// Amount specifies the amount details if applicable
	Amount *DSLAmount `json:"amount,omitempty"`

	// Share is the sharing configuration
	Share *Share `json:"share,omitempty"`

	// Remaining is an optional remaining account
	Remaining string `json:"remaining,omitempty"`

	// Rate is the exchange rate configuration
	Rate *Rate `json:"rate,omitempty"`

	// Description is a human-readable description
	Description string `json:"description,omitempty"`

	// ChartOfAccounts is the chart of accounts code
	ChartOfAccounts string `json:"chartOfAccounts,omitempty"`

	// Metadata contains additional custom data
	Metadata map[string]any `json:"metadata,omitempty"`
}

// DSLSource represents the source of a DSL transaction.
// This is aligned with the Midaz DSL source contract.
type DSLSource struct {
	// Remaining is an optional remaining account
	Remaining string `json:"remaining,omitempty"`

	// From is a collection of source accounts and amounts
	From []DSLFromTo `json:"from"`
}

// DSLDistribute represents the distribution of a DSL transaction.
// This is aligned with the Midaz DSL distribution contract.
type DSLDistribute struct {
	// Remaining is an optional remaining account
	Remaining string `json:"remaining,omitempty"`

	// To is a collection of destination accounts and amounts
	To []DSLFromTo `json:"to"`
}

// DSLSend represents the send operation in a DSL transaction.
// This is aligned with the Midaz DSL send contract.
type DSLSend struct {
	// Asset identifies the currency or asset type for this transaction
	Asset string `json:"asset"`

	// Value is the exact decimal value of the transaction.
	Value any `json:"value"`

	// Source specifies where the funds come from
	Source *DSLSource `json:"source,omitempty"`

	// Distribute specifies where the funds go to
	Distribute *DSLDistribute `json:"distribute,omitempty"`
}

// TransactionDSLInput represents the input for creating a transaction using DSL.
// This is aligned with the Midaz DSL transaction contract.
type TransactionDSLInput struct {
	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Description provides a human-readable description of the transaction
	Description string `json:"description,omitempty"`

	// Send contains the sending configuration
	Send *DSLSend `json:"send,omitempty"`

	// Metadata contains additional custom data for the transaction
	Metadata map[string]any `json:"metadata,omitempty"`

	// Code is a custom transaction code for categorization
	Code string `json:"code,omitempty"`

	// Pending indicates whether the transaction requires explicit commitment
	Pending bool `json:"pending,omitempty"`
}

// DSLAccountRef is a helper struct to implement the AccountReference interface
type DSLAccountRef struct {
	Account string
}

// GetAccount returns the account identifier
func (ref *DSLAccountRef) GetAccount() string {
	if ref == nil {
		return ""
	}

	return ref.Account
}

// GetAsset returns the asset code for the transaction
func (input *TransactionDSLInput) GetAsset() string {
	if input == nil || input.Send == nil {
		return ""
	}

	return input.Send.Asset
}

// GetValue returns the exact decimal amount text for the transaction.
func (input *TransactionDSLInput) GetValue() string {
	if input == nil || input.Send == nil {
		return ""
	}

	return decimalStringFromAny(input.Send.Value)
}

// GetSourceAccounts returns the source accounts for the transaction
func (input *TransactionDSLInput) GetSourceAccounts() []validation.AccountReference {
	var accounts []validation.AccountReference

	if input != nil && input.Send != nil && input.Send.Source != nil {
		for _, from := range input.Send.Source.From {
			accounts = append(accounts, &DSLAccountRef{Account: from.Account})
		}
	}

	return accounts
}

// GetDestinationAccounts returns the destination accounts for the transaction
func (input *TransactionDSLInput) GetDestinationAccounts() []validation.AccountReference {
	var accounts []validation.AccountReference

	if input != nil && input.Send != nil && input.Send.Distribute != nil {
		for _, to := range input.Send.Distribute.To {
			accounts = append(accounts, &DSLAccountRef{Account: to.Account})
		}
	}

	return accounts
}

// GetMetadata returns the metadata for the transaction
func (input *TransactionDSLInput) GetMetadata() map[string]any {
	if input == nil {
		return nil
	}

	return input.Metadata
}

// Share represents the sharing configuration for a transaction.
type Share struct {
	Percentage             int64 `json:"percentage"`
	PercentageOfPercentage int64 `json:"percentageOfPercentage,omitempty"`
}

// Rate represents an exchange rate configuration.
type Rate struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Value      any    `json:"value"`
	ExternalID string `json:"externalId"`
}

// Validate checks that the DSLSend meets all validation requirements.
// Field-level violations are accumulated so callers see every problem
// in a single error.
func (send *DSLSend) Validate() error {
	if send == nil {
		return errors.New("send is required")
	}

	var errs validation.FieldErrors

	if send.Asset == "" {
		errs.Append("asset", "is required")
	} else if err := core.ValidateAssetCode(send.Asset); err != nil {
		errs.Append("asset", err.Error())
	}

	if err := validatePositiveDecimalString(send.Value, "value"); err != nil {
		errs.Append("value", err.Error())
	}

	send.appendSourceErrors(&errs)
	send.appendDistributeErrors(&errs)

	return errs.OrNil()
}

// appendSourceErrors accumulates source.from violations onto errs.
func (send *DSLSend) appendSourceErrors(errs *validation.FieldErrors) {
	if send.Source == nil || len(send.Source.From) == 0 {
		errs.Append("source.from", "must contain at least one entry")
		return
	}

	for i, from := range send.Source.From {
		if from.Account == "" {
			errs.Append(fmt.Sprintf("source.from[%d].account", i), "is required")
			continue
		}

		send.appendExternalAccountError(errs, from.Account, i, "source.from")
	}
}

// appendDistributeErrors accumulates distribute.to violations onto errs.
func (send *DSLSend) appendDistributeErrors(errs *validation.FieldErrors) {
	if send.Distribute == nil || len(send.Distribute.To) == 0 {
		errs.Append("distribute.to", "must contain at least one entry")
		return
	}

	for i, to := range send.Distribute.To {
		if to.Account == "" {
			errs.Append(fmt.Sprintf("distribute.to[%d].account", i), "is required")
			continue
		}

		send.appendExternalAccountError(errs, to.Account, i, "distribute.to")
	}
}

// appendExternalAccountError validates an external account reference
// and accumulates any violation onto errs.
func (send *DSLSend) appendExternalAccountError(errs *validation.FieldErrors, account string, index int, location string) {
	if account == "" || account[0] != '@' {
		return
	}

	if !core.ExternalAccountPattern.MatchString(account) {
		errs.Append(fmt.Sprintf("%s[%d]", location, index), fmt.Sprintf("invalid external account format: %s", account))
		return
	}

	matches := core.ExternalAccountPattern.FindStringSubmatch(account)
	if len(matches) > 1 && matches[1] != send.Asset {
		errs.Append(fmt.Sprintf("%s[%d]", location, index),
			fmt.Sprintf("asset code mismatch: transaction uses %s but external account uses %s", send.Asset, matches[1]))
	}
}

// Validate checks if the TransactionDSLInput meets the validation requirements.
// All field-level violations are accumulated and returned together.
func (input *TransactionDSLInput) Validate() error {
	if input == nil {
		return errors.New("transaction DSL input is required")
	}

	var errs validation.FieldErrors

	switch {
	case strings.TrimSpace(input.ChartOfAccountsGroupName) == "":
		errs.Append("chartOfAccountsGroupName", "is required")
	case len(input.ChartOfAccountsGroupName) > 256:
		errs.Append("chartOfAccountsGroupName", "must be at most 256 characters")
	}

	if input.Send == nil {
		errs.Append("send", "is required")
	} else if err := input.Send.Validate(); err != nil {
		errs.Append("send", "invalid send operation: "+err.Error())
	}

	if len(input.Description) > maxTransactionDescriptionLength {
		errs.Append("description", "must be at most 256 characters")
	}

	if input.Code != "" {
		if err := core.ValidateTransactionCode(input.Code); err != nil {
			errs.Append("code", err.Error())
		}
	}

	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			errs.Append("metadata", "invalid: "+err.Error())
		}
	}

	return errs.OrNil()
}

// ToTransactionMap converts a TransactionDSLInput to a map that can be used for API requests.
// This replaces the previous direct internal payload conversion.
func (input *TransactionDSLInput) ToTransactionMap() map[string]any {
	if input == nil {
		return nil
	}

	// Create base transaction map
	transaction := map[string]any{
		"description": input.Description,
		"metadata":    input.Metadata,
	}

	// Add optional fields if present
	if input.ChartOfAccountsGroupName != "" {
		transaction["chartOfAccountsGroupName"] = input.ChartOfAccountsGroupName
	}

	if input.Code != "" {
		transaction["code"] = input.Code
	}

	if input.Pending {
		transaction["pending"] = input.Pending
	}

	// Add Send information if present
	if input.Send != nil {
		transaction["send"] = input.sendToMap()
	}

	return transaction
}

// RenderDSL renders the transaction input to Midaz DSL syntax.
func (input *TransactionDSLInput) RenderDSL() ([]byte, error) {
	if input == nil {
		return nil, errors.New("transaction DSL input is required")
	}

	if err := input.Validate(); err != nil {
		return nil, err
	}

	var builder strings.Builder
	builder.WriteString("(transaction V1")

	if input.ChartOfAccountsGroupName != "" {
		if err := validateDSLToken("chartOfAccountsGroupName", input.ChartOfAccountsGroupName); err != nil {
			return nil, err
		}

		builder.WriteString(" (chart-of-accounts-group-name ")
		builder.WriteString(input.ChartOfAccountsGroupName)
		builder.WriteString(")")
	}

	if input.Description != "" {
		builder.WriteString(" (description ")
		builder.WriteString(quoteDSLString(input.Description))
		builder.WriteString(")")
	}

	if input.Code != "" {
		if err := validateDSLToken("code", input.Code); err != nil {
			return nil, err
		}

		builder.WriteString(" (code ")
		builder.WriteString(input.Code)
		builder.WriteString(")")
	}

	if input.Pending {
		builder.WriteString(" (pending true)")
	}

	if len(input.Metadata) > 0 {
		metadataDSL, err := renderDSLMetadata(input.Metadata)
		if err != nil {
			return nil, err
		}

		builder.WriteByte(' ')
		builder.WriteString(metadataDSL)
	}

	sendDSL, err := renderDSLSend(input.Send)
	if err != nil {
		return nil, err
	}

	builder.WriteByte(' ')
	builder.WriteString(sendDSL)
	builder.WriteByte(')')

	return []byte(builder.String()), nil
}

func renderDSLSend(send *DSLSend) (string, error) {
	if err := validateDSLToken("send.asset", send.Asset); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("(send ")
	builder.WriteString(send.Asset)
	builder.WriteByte(' ')

	value, err := formatDSLDecimal(decimalStringFromAny(send.Value))
	if err != nil {
		return "", fmt.Errorf("invalid send.value: %w", err)
	}

	builder.WriteString(value)

	if send.Source != nil {
		sourceDSL, err := renderDSLSource(send.Source)
		if err != nil {
			return "", err
		}

		builder.WriteByte(' ')
		builder.WriteString(sourceDSL)
	}

	if send.Distribute != nil {
		distributeDSL, err := renderDSLDistribute(send.Distribute)
		if err != nil {
			return "", err
		}

		builder.WriteByte(' ')
		builder.WriteString(distributeDSL)
	}

	builder.WriteByte(')')

	return builder.String(), nil
}

func renderDSLSource(source *DSLSource) (string, error) {
	var builder strings.Builder
	builder.WriteString("(source")

	if source.Remaining != "" {
		if err := validateDSLToken("source.remaining", source.Remaining); err != nil {
			return "", err
		}

		builder.WriteString(" :")
		builder.WriteString(source.Remaining)
	}

	for _, from := range source.From {
		entryDSL, err := renderDSLFromTo("from", from)
		if err != nil {
			return "", err
		}

		builder.WriteByte(' ')
		builder.WriteString(entryDSL)
	}

	builder.WriteByte(')')

	return builder.String(), nil
}

func renderDSLDistribute(distribute *DSLDistribute) (string, error) {
	var builder strings.Builder
	builder.WriteString("(distribute")

	if distribute.Remaining != "" {
		if err := validateDSLToken("distribute.remaining", distribute.Remaining); err != nil {
			return "", err
		}

		builder.WriteString(" :")
		builder.WriteString(distribute.Remaining)
	}

	for _, to := range distribute.To {
		entryDSL, err := renderDSLFromTo("to", to)
		if err != nil {
			return "", err
		}

		builder.WriteByte(' ')
		builder.WriteString(entryDSL)
	}

	builder.WriteByte(')')

	return builder.String(), nil
}

func renderDSLFromTo(kind string, entry DSLFromTo) (string, error) {
	if err := validateDSLToken(kind+".account", entry.Account); err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteByte('(')
	builder.WriteString(kind)
	builder.WriteByte(' ')
	builder.WriteString(entry.Account)

	if err := renderDSLFromToValue(&builder, kind, entry); err != nil {
		return "", err
	}

	if err := renderDSLRate(&builder, kind, entry.Rate); err != nil {
		return "", err
	}

	if entry.Description != "" {
		builder.WriteString(" (description ")
		builder.WriteString(quoteDSLString(entry.Description))
		builder.WriteByte(')')
	}

	if entry.ChartOfAccounts != "" {
		if err := validateDSLToken(kind+".chartOfAccounts", entry.ChartOfAccounts); err != nil {
			return "", err
		}

		builder.WriteString(" (chart-of-accounts ")
		builder.WriteString(entry.ChartOfAccounts)
		builder.WriteByte(')')
	}

	if len(entry.Metadata) > 0 {
		metadataDSL, err := renderDSLMetadata(entry.Metadata)
		if err != nil {
			return "", err
		}

		builder.WriteByte(' ')
		builder.WriteString(metadataDSL)
	}

	builder.WriteByte(')')

	return builder.String(), nil
}

func renderDSLFromToValue(builder *strings.Builder, kind string, entry DSLFromTo) error {
	switch {
	case entry.Amount != nil:
		return renderDSLAmount(builder, kind, entry.Amount)
	case entry.Share != nil:
		renderDSLShare(builder, entry.Share)
	case entry.Remaining != "":
		return renderDSLRemaining(builder, kind, entry.Remaining)
	}

	return nil
}

func renderDSLAmount(builder *strings.Builder, kind string, amount *DSLAmount) error {
	if err := validateDSLToken(kind+".amount.asset", amount.Asset); err != nil {
		return err
	}

	builder.WriteString(" :amount ")
	builder.WriteString(amount.Asset)
	builder.WriteByte(' ')

	value, err := formatDSLDecimal(decimalStringFromAny(amount.Value))
	if err != nil {
		return fmt.Errorf("invalid %s.amount.value: %w", kind, err)
	}

	builder.WriteString(value)

	return nil
}

func renderDSLShare(builder *strings.Builder, share *Share) {
	builder.WriteString(" :share ")
	builder.WriteString(strconv.FormatInt(share.Percentage, 10))

	if share.PercentageOfPercentage > 0 {
		builder.WriteString(" :of ")
		builder.WriteString(strconv.FormatInt(share.PercentageOfPercentage, 10))
	}
}

func renderDSLRemaining(builder *strings.Builder, kind, remaining string) error {
	if err := validateDSLToken(kind+".remaining", remaining); err != nil {
		return err
	}

	builder.WriteString(" :")
	builder.WriteString(remaining)

	return nil
}

func renderDSLRate(builder *strings.Builder, kind string, rate *Rate) error {
	if rate == nil {
		return nil
	}

	if err := validateDSLRateTokens(kind, rate); err != nil {
		return err
	}

	builder.WriteString(" (rate ")
	builder.WriteString(rate.ExternalID)
	builder.WriteByte(' ')
	builder.WriteString(rate.From)
	builder.WriteString(" -> ")
	builder.WriteString(rate.To)
	builder.WriteByte(' ')

	value, err := formatDSLDecimal(decimalStringFromAny(rate.Value))
	if err != nil {
		return fmt.Errorf("invalid %s.rate.value: %w", kind, err)
	}

	builder.WriteString(value)
	builder.WriteByte(')')

	return nil
}

func validateDSLRateTokens(kind string, rate *Rate) error {
	if err := validateDSLToken(kind+".rate.externalId", rate.ExternalID); err != nil {
		return err
	}

	if err := validateDSLToken(kind+".rate.from", rate.From); err != nil {
		return err
	}

	return validateDSLToken(kind+".rate.to", rate.To)
}

func renderDSLMetadata(metadata map[string]any) (string, error) {
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("(metadata")

	for _, key := range keys {
		if err := validateDSLToken("metadata key", key); err != nil {
			return "", err
		}

		valueToken, err := formatDSLMetadataValue(metadata[key])
		if err != nil {
			return "", fmt.Errorf("invalid metadata value for %q: %w", key, err)
		}

		builder.WriteString(" (")
		builder.WriteString(key)
		builder.WriteByte(' ')
		builder.WriteString(valueToken)
		builder.WriteByte(')')
	}

	builder.WriteByte(')')

	return builder.String(), nil
}

func formatDSLMetadataValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if strings.ContainsAny(v, "()\" \t\n\r") {
			return "", errors.New("string metadata values for DSL cannot contain whitespace, quotes, or parentheses")
		}

		if v == "" {
			return "", errors.New("empty metadata value")
		}

		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return formatDSLDecimal(strconv.FormatFloat(v, 'f', -1, 64))
	default:
		return "", fmt.Errorf("unsupported metadata type %T", value)
	}
}

func quoteDSLString(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")

	return "\"" + escaped + "\""
}

func formatDSLDecimal(value string) (string, error) {
	parsed, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil {
		return "", errors.New("value must be a valid decimal")
	}

	if !parsed.Equal(parsed.Truncate(0)) {
		return "", errors.New("fractional DSL values are not supported by the current Midaz DSL parser")
	}

	return parsed.StringFixed(0) + "|0", nil
}

func validateDSLToken(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", field)
	}

	if strings.ContainsAny(value, "()\"' \t\n\r") {
		return fmt.Errorf("%s contains characters that cannot be safely rendered in DSL", field)
	}

	return nil
}

// sendToMap converts the DSLSend to a map for API requests
func (input *TransactionDSLInput) sendToMap() map[string]any {
	if input.Send == nil {
		return nil
	}

	send := map[string]any{
		"asset": input.Send.Asset,
		"value": decimalStringFromAny(input.Send.Value),
	}

	// Add Source if present
	if input.Send.Source != nil {
		send["source"] = input.sourceToMap()
	}

	// Add Distribute if present
	if input.Send.Distribute != nil {
		send["distribute"] = input.distributeToMap()
	}

	return send
}

// sourceToMap converts DSLSource to a map for API requests
func (input *TransactionDSLInput) sourceToMap() map[string]any {
	if input.Send.Source == nil {
		return nil
	}

	source := map[string]any{}

	// Add Remaining if present
	if input.Send.Source.Remaining != "" {
		source["remaining"] = input.Send.Source.Remaining
	}

	// Convert From accounts
	if len(input.Send.Source.From) > 0 {
		fromList := make([]map[string]any, 0, len(input.Send.Source.From))

		for _, from := range input.Send.Source.From {
			fromMap := fromToToMap(from)
			fromList = append(fromList, fromMap)
		}

		source["from"] = fromList
	}

	return source
}

// distributeToMap converts DSLDistribute to a map for API requests
func (input *TransactionDSLInput) distributeToMap() map[string]any {
	if input.Send.Distribute == nil {
		return nil
	}

	distribute := map[string]any{}

	// Add Remaining if present
	if input.Send.Distribute.Remaining != "" {
		distribute["remaining"] = input.Send.Distribute.Remaining
	}

	// Convert To accounts
	if len(input.Send.Distribute.To) > 0 {
		toList := make([]map[string]any, 0, len(input.Send.Distribute.To))

		for _, to := range input.Send.Distribute.To {
			toMap := fromToToMap(to)
			toList = append(toList, toMap)
		}

		distribute["to"] = toList
	}

	return distribute
}

// fromToToMap converts a DSLFromTo to a map for API requests
func fromToToMap(from DSLFromTo) map[string]any {
	fromMap := map[string]any{
		"accountAlias": from.Account,
	}

	// Add Amount if present
	if from.Amount != nil {
		fromMap["amount"] = map[string]any{
			"asset": from.Amount.Asset,
			"value": decimalStringFromAny(from.Amount.Value),
		}
	}

	// Add other fields if present
	if from.Remaining != "" {
		fromMap["remaining"] = from.Remaining
	}

	if from.Description != "" {
		fromMap["description"] = from.Description
	}

	if from.ChartOfAccounts != "" {
		fromMap["chartOfAccounts"] = from.ChartOfAccounts
	}

	if from.Metadata != nil {
		fromMap["metadata"] = from.Metadata
	}

	// Add Share if present
	if from.Share != nil {
		fromMap["share"] = map[string]any{
			"percentage":             from.Share.Percentage,
			"percentageOfPercentage": from.Share.PercentageOfPercentage,
		}
	}

	// Add Rate if present
	if from.Rate != nil {
		fromMap["rate"] = map[string]any{
			"from":       from.Rate.From,
			"to":         from.Rate.To,
			"value":      decimalStringFromAny(from.Rate.Value),
			"externalId": from.Rate.ExternalID,
		}
	}

	return fromMap
}

// FromTransactionMap converts a map from the API to a TransactionDSLInput.
// This replaces the previous direct internal payload conversion.
func FromTransactionMap(data map[string]any) *TransactionDSLInput {
	if data == nil {
		return nil
	}

	// Extract basic fields
	input := &TransactionDSLInput{
		ChartOfAccountsGroupName: getStringFromMap(data, "chartOfAccountsGroupName"),
		Description:              getStringFromMap(data, "description"),
		Code:                     getStringFromMap(data, "code"),
		Metadata:                 getMetadataFromMap(data),
	}

	// Extract pending flag
	if pendingVal, ok := data["pending"].(bool); ok {
		input.Pending = pendingVal
	}

	// Extract Send information
	if sendMap, ok := data["send"].(map[string]any); ok {
		input.Send = extractSend(sendMap)
	}

	return input
}

// extractSend converts a map to DSLSend
func extractSend(data map[string]any) *DSLSend {
	if data == nil {
		return nil
	}

	send := &DSLSend{}

	// Extract basic fields
	send.Asset = getStringFromMap(data, "asset")

	send.Value = decimalStringFromAny(data["value"])

	// Extract Source
	if sourceMap, ok := data["source"].(map[string]any); ok {
		send.Source = extractSource(sourceMap)
	}

	// Extract Distribute
	if distMap, ok := data["distribute"].(map[string]any); ok {
		send.Distribute = extractDistribute(distMap)
	}

	return send
}

// extractSource converts a map to DSLSource
func extractSource(data map[string]any) *DSLSource {
	if data == nil {
		return nil
	}

	source := &DSLSource{
		Remaining: getStringFromMap(data, "remaining"),
		From:      []DSLFromTo{},
	}

	// Extract From entries
	if fromList, ok := data["from"].([]any); ok {
		for _, item := range fromList {
			if fromMap, ok := item.(map[string]any); ok {
				fromEntry := extractFromTo(fromMap)
				source.From = append(source.From, fromEntry)
			}
		}
	}

	return source
}

// extractDistribute converts a map to DSLDistribute
func extractDistribute(data map[string]any) *DSLDistribute {
	if data == nil {
		return nil
	}

	distribute := &DSLDistribute{
		Remaining: getStringFromMap(data, "remaining"),
		To:        []DSLFromTo{},
	}

	// Extract To entries
	if toList, ok := data["to"].([]any); ok {
		for _, item := range toList {
			if toMap, ok := item.(map[string]any); ok {
				toEntry := extractFromTo(toMap)
				distribute.To = append(distribute.To, toEntry)
			}
		}
	}

	return distribute
}

// extractFromTo converts a map to DSLFromTo
func extractFromTo(data map[string]any) DSLFromTo {
	if data == nil {
		return DSLFromTo{}
	}

	from := DSLFromTo{
		Account:         firstStringFromMap(data, "accountAlias", "account"),
		Remaining:       getStringFromMap(data, "remaining"),
		Description:     getStringFromMap(data, "description"),
		ChartOfAccounts: getStringFromMap(data, "chartOfAccounts"),
		Metadata:        getMetadataFromMap(data),
	}

	// Extract Amount
	if amountMap, ok := data["amount"].(map[string]any); ok {
		amount := &DSLAmount{
			Asset: getStringFromMap(amountMap, "asset"),
		}

		amount.Value = decimalStringFromAny(amountMap["value"])

		from.Amount = amount
	}

	// Extract Share
	if shareMap, ok := data["share"].(map[string]any); ok {
		share := &Share{}

		if percentage, ok := int64FromAny(shareMap["percentage"]); ok {
			share.Percentage = percentage
		}

		if percentageOfPercentage, ok := int64FromAny(shareMap["percentageOfPercentage"]); ok {
			share.PercentageOfPercentage = percentageOfPercentage
		}

		from.Share = share
	}

	// Extract Rate
	if rateMap, ok := data["rate"].(map[string]any); ok {
		rate := &Rate{
			From:       getStringFromMap(rateMap, "from"),
			To:         getStringFromMap(rateMap, "to"),
			ExternalID: getStringFromMap(rateMap, "externalId"),
		}

		rate.Value = decimalStringFromAny(rateMap["value"])

		from.Rate = rate
	}

	return from
}

func int64FromAny(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		parsed, err := v.Int64()
		if err == nil {
			return parsed, true
		}

		decimalValue, err := strconv.ParseFloat(v.String(), 64)

		return int64(decimalValue), err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return parsed, true
		}

		decimalValue, err := strconv.ParseFloat(strings.TrimSpace(v), 64)

		return int64(decimalValue), err == nil
	default:
		return 0, false
	}
}

// Helper functions for extracting values from maps

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}

	return ""
}

func firstStringFromMap(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := getStringFromMap(m, key); value != "" {
			return value
		}
	}

	return ""
}

// getMetadataFromMap safely extracts metadata from a map
func getMetadataFromMap(m map[string]any) map[string]any {
	if val, ok := m["metadata"].(map[string]any); ok {
		return val
	}

	return nil
}
