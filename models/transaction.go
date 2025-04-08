package models

import (
	"fmt"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/pkg/validation/core"
)

// Transaction represents a transaction in the Midaz Ledger.
// A transaction is a financial event that affects one or more accounts
// through a series of operations (debits and credits).
//
// Transactions are the core financial records in the Midaz system, representing
// the movement of assets between accounts. Each transaction consists of one or more
// operations (debits and credits) that must balance (sum to zero) for each asset type.
//
// Transactions can be in different states as indicated by their Status field:
//   - PENDING: The transaction is created but not yet committed
//   - COMPLETED: The transaction is committed and has affected account balances
//   - FAILED: The transaction processing failed
//   - CANCELED: The transaction was canceled before being committed
//
// Example usage:
//
//	// Accessing transaction details
//	fmt.Printf("Transaction ID: %s\n", transaction.ID)
//	fmt.Printf("Amount: %d (scale: %d)\n", transaction.Amount, transaction.Scale)
//	fmt.Printf("Asset: %s\n", transaction.AssetCode)
//	fmt.Printf("Status: %s\n", transaction.Status)
//	fmt.Printf("Created: %s\n", transaction.CreatedAt.Format(time.RFC3339))
//
//	// Iterating through operations
//	for i, op := range transaction.Operations {
//	    fmt.Printf("Operation %d: %s %s %d (scale: %d) on account %s\n",
//	        i+1, op.Type, op.Amount.AssetCode, op.Amount.Value, op.Amount.Scale, op.AccountID)
//	}
//
//	// Accessing metadata
//	if reference, ok := transaction.Metadata["reference"].(string); ok {
//	    fmt.Printf("Reference: %s\n", reference)
//	}
type Transaction struct {
	// ID is the unique identifier for the transaction
	// This is a system-generated UUID that uniquely identifies the transaction
	ID string `json:"id"`

	// Template is an optional identifier for the transaction template used
	// Templates can be used to create standardized transactions with predefined
	// structures and validation rules
	Template string `json:"template,omitempty"`

	// Amount is the numeric value of the transaction
	// This represents the total value of the transaction as a fixed-point integer
	// The actual amount is calculated as Amount / 10^Scale
	Amount int64 `json:"amount"`

	// Scale represents the decimal precision for the amount
	// For example, a scale of 2 means the amount is in cents (100 = $1.00)
	Scale int64 `json:"scale"`

	// AssetCode identifies the currency or asset type for this transaction
	// Common examples include "USD", "EUR", "BTC", etc.
	AssetCode string `json:"assetCode"`

	// Status indicates the current processing status of the transaction
	// See the Status enum for possible values (PENDING, COMPLETED, FAILED, CANCELED)
	Status Status `json:"status"`

	// LedgerID identifies the ledger this transaction belongs to
	// A ledger is a collection of accounts and transactions within an organization
	LedgerID string `json:"ledgerId"`

	// OrganizationID identifies the organization this transaction belongs to
	// An organization is the top-level entity that owns ledgers and accounts
	OrganizationID string `json:"organizationId"`

	// Operations contains the individual debit and credit operations
	// Each operation represents a single accounting entry (debit or credit)
	// The sum of all operations for each asset must balance to zero
	Operations []Operation `json:"operations,omitempty"`

	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is the timestamp when the transaction was created
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the timestamp when the transaction was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// DeletedAt is the timestamp when the transaction was deleted, if applicable
	// This field is only set if the transaction has been soft-deleted
	DeletedAt *time.Time `json:"deletedAt,omitempty"`

	// ExternalID is an optional identifier for linking to external systems
	// This can be used to correlate transactions with records in other systems
	// and to prevent duplicate transactions
	ExternalID string `json:"externalId,omitempty"`

	// Description is a human-readable description of the transaction
	// This should provide context about the purpose or nature of the transaction
	Description string `json:"description,omitempty"`
}

// DSLAmount represents an amount with a value, scale, and asset code for DSL transactions.
// This is aligned with the lib-commons Amount structure.
type DSLAmount struct {
	// Value is the numeric value of the amount
	Value int64 `json:"value"`

	// Scale represents the decimal precision for the amount
	Scale int64 `json:"scale"`

	// Asset is the asset code for the amount
	Asset string `json:"asset,omitempty"`
}

// DSLFromTo represents a source or destination in a DSL transaction.
// This is aligned with the lib-commons FromTo structure.
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
// This is aligned with the lib-commons Source structure.
type DSLSource struct {
	// Remaining is an optional remaining account
	Remaining string `json:"remaining,omitempty"`

	// From is a collection of source accounts and amounts
	From []DSLFromTo `json:"from"`
}

// DSLDistribute represents the distribution of a DSL transaction.
// This is aligned with the lib-commons Distribute structure.
type DSLDistribute struct {
	// Remaining is an optional remaining account
	Remaining string `json:"remaining,omitempty"`

	// To is a collection of destination accounts and amounts
	To []DSLFromTo `json:"to"`
}

// DSLSend represents the send operation in a DSL transaction.
// This is aligned with the lib-commons Send structure.
type DSLSend struct {
	// Asset identifies the currency or asset type for this transaction
	Asset string `json:"asset"`

	// Value is the numeric value of the transaction
	Value int64 `json:"value"`

	// Scale represents the decimal precision for the amount
	Scale int64 `json:"scale"`

	// Source specifies where the funds come from
	Source *DSLSource `json:"source,omitempty"`

	// Distribute specifies where the funds go to
	Distribute *DSLDistribute `json:"distribute,omitempty"`
}

// TransactionDSLInput represents the input for creating a transaction using DSL.
// This is aligned with the lib-commons Transaction structure.
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
	return ref.Account
}

// GetAsset returns the asset code for the transaction
func (input *TransactionDSLInput) GetAsset() string {
	if input.Send == nil {
		return ""
	}

	return input.Send.Asset
}

// GetValue returns the amount value for the transaction
func (input *TransactionDSLInput) GetValue() float64 {
	if input.Send == nil {
		return 0
	}

	return float64(input.Send.Value)
}

// GetSourceAccounts returns the source accounts for the transaction
func (input *TransactionDSLInput) GetSourceAccounts() []validation.AccountReference {
	var accounts []validation.AccountReference

	if input.Send != nil && input.Send.Source != nil {
		for _, from := range input.Send.Source.From {
			accounts = append(accounts, &DSLAccountRef{Account: from.Account})
		}
	}

	return accounts
}

// GetDestinationAccounts returns the destination accounts for the transaction
func (input *TransactionDSLInput) GetDestinationAccounts() []validation.AccountReference {
	var accounts []validation.AccountReference

	if input.Send != nil && input.Send.Distribute != nil {
		for _, to := range input.Send.Distribute.To {
			accounts = append(accounts, &DSLAccountRef{Account: to.Account})
		}
	}

	return accounts
}

// GetMetadata returns the metadata for the transaction
func (input *TransactionDSLInput) GetMetadata() map[string]any {
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
	Value      int64  `json:"value"`
	Scale      int64  `json:"scale"`
	ExternalID string `json:"externalId"`
}

// Validate checks that the DSLSend meets all validation requirements.
func (send *DSLSend) Validate() error {
	// Validate required fields
	if send.Asset == "" {
		return fmt.Errorf("asset is required")
	}

	// Validate asset code
	if err := core.ValidateAssetCode(send.Asset); err != nil {
		return err
	}

	if send.Value <= 0 {
		return fmt.Errorf("value must be greater than 0")
	}

	if send.Scale < 0 || send.Scale > 18 {
		return fmt.Errorf("scale must be between 0 and 18")
	}

	// Validate source
	if err := send.validateSource(); err != nil {
		return err
	}

	// Validate distribute
	if err := send.validateDistribute(); err != nil {
		return err
	}

	return nil
}

// validateSource validates the source part of a DSLSend
func (send *DSLSend) validateSource() error {
	if send.Source == nil || len(send.Source.From) == 0 {
		return fmt.Errorf("source.from must contain at least one entry")
	}

	for i, from := range send.Source.From {
		if from.Account == "" {
			return fmt.Errorf("source.from[%d].account is required", i)
		}

		if err := send.validateExternalAccount(from.Account, i, "source.from"); err != nil {
			return err
		}
	}

	return nil
}

// validateDistribute validates the distribute part of a DSLSend
func (send *DSLSend) validateDistribute() error {
	if send.Distribute == nil || len(send.Distribute.To) == 0 {
		return fmt.Errorf("distribute.to must contain at least one entry")
	}

	for i, to := range send.Distribute.To {
		if to.Account == "" {
			return fmt.Errorf("distribute.to[%d].account is required", i)
		}

		if err := send.validateExternalAccount(to.Account, i, "distribute.to"); err != nil {
			return err
		}
	}

	return nil
}

// validateExternalAccount validates an external account reference
func (send *DSLSend) validateExternalAccount(account string, index int, location string) error {
	if account == "" || account[0] != '@' {
		return nil
	}

	// For external accounts, check if they match the expected format
	if !core.ExternalAccountPattern.MatchString(account) {
		return fmt.Errorf("invalid external account format in %s[%d]: %s", location, index, account)
	}

	// Check if the asset code in the external account matches the transaction asset
	matches := core.ExternalAccountPattern.FindStringSubmatch(account)
	if len(matches) > 1 && matches[1] != send.Asset {
		return fmt.Errorf("asset code mismatch in %s[%d]: transaction uses %s but external account uses %s",
			location, index, send.Asset, matches[1])
	}

	return nil
}

// Validate checks if the TransactionDSLInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
func (input *TransactionDSLInput) Validate() error {
	// Validate send
	if input.Send == nil {
		return fmt.Errorf("send is required")
	}

	// Validate send operation
	if err := input.Send.Validate(); err != nil {
		return fmt.Errorf("invalid send operation: %w", err)
	}

	// Validate string length constraints
	if len(input.ChartOfAccountsGroupName) > 256 {
		return fmt.Errorf("chartOfAccountsGroupName must be at most 256 characters")
	}

	if len(input.Description) > 256 {
		return fmt.Errorf("description must be at most 256 characters")
	}

	// Validate transaction code
	if input.Code != "" {
		if err := core.ValidateTransactionCode(input.Code); err != nil {
			return err
		}
	}

	// Validate metadata if present
	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}

// ToTransactionMap converts a TransactionDSLInput to a map that can be used for API requests.
// This replaces the previous direct lib-commons conversion.
func (input *TransactionDSLInput) ToTransactionMap() map[string]interface{} {
	if input == nil {
		return nil
	}

	// Create base transaction map
	transaction := map[string]interface{}{
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

// sendToMap converts the DSLSend to a map for API requests
func (input *TransactionDSLInput) sendToMap() map[string]interface{} {
	if input.Send == nil {
		return nil
	}

	send := map[string]interface{}{
		"asset": input.Send.Asset,
		"value": input.Send.Value,
		"scale": input.Send.Scale,
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
func (input *TransactionDSLInput) sourceToMap() map[string]interface{} {
	if input.Send.Source == nil {
		return nil
	}

	source := map[string]interface{}{}

	// Add Remaining if present
	if input.Send.Source.Remaining != "" {
		source["remaining"] = input.Send.Source.Remaining
	}

	// Convert From accounts
	if len(input.Send.Source.From) > 0 {
		fromList := make([]map[string]interface{}, 0, len(input.Send.Source.From))
		for _, from := range input.Send.Source.From {
			fromMap := fromToToMap(from)
			fromList = append(fromList, fromMap)
		}
		source["from"] = fromList
	}

	return source
}

// distributeToMap converts DSLDistribute to a map for API requests
func (input *TransactionDSLInput) distributeToMap() map[string]interface{} {
	if input.Send.Distribute == nil {
		return nil
	}

	distribute := map[string]interface{}{}

	// Add Remaining if present
	if input.Send.Distribute.Remaining != "" {
		distribute["remaining"] = input.Send.Distribute.Remaining
	}

	// Convert To accounts
	if len(input.Send.Distribute.To) > 0 {
		toList := make([]map[string]interface{}, 0, len(input.Send.Distribute.To))
		for _, to := range input.Send.Distribute.To {
			toMap := fromToToMap(to)
			toList = append(toList, toMap)
		}
		distribute["to"] = toList
	}

	return distribute
}

// fromToToMap converts a DSLFromTo to a map for API requests
func fromToToMap(from DSLFromTo) map[string]interface{} {
	fromMap := map[string]interface{}{
		"account": from.Account,
	}

	// Add Amount if present
	if from.Amount != nil {
		fromMap["amount"] = map[string]interface{}{
			"asset": from.Amount.Asset,
			"value": from.Amount.Value,
			"scale": from.Amount.Scale,
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
		fromMap["share"] = map[string]interface{}{
			"percentage":             from.Share.Percentage,
			"percentageOfPercentage": from.Share.PercentageOfPercentage,
		}
	}

	// Add Rate if present
	if from.Rate != nil {
		fromMap["rate"] = map[string]interface{}{
			"from":       from.Rate.From,
			"to":         from.Rate.To,
			"value":      from.Rate.Value,
			"scale":      from.Rate.Scale,
			"externalId": from.Rate.ExternalID,
		}
	}

	return fromMap
}

// FromTransactionMap converts a map from the API to a TransactionDSLInput.
// This replaces the previous direct lib-commons conversion.
func FromTransactionMap(data map[string]interface{}) *TransactionDSLInput {
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
	if sendMap, ok := data["send"].(map[string]interface{}); ok {
		input.Send = extractSend(sendMap)
	}

	return input
}

// extractSend converts a map to DSLSend
func extractSend(data map[string]interface{}) *DSLSend {
	if data == nil {
		return nil
	}

	send := &DSLSend{}

	// Extract basic fields
	send.Asset = getStringFromMap(data, "asset")

	// Extract numeric values
	if value, ok := data["value"].(float64); ok {
		send.Value = int64(value)
	}

	if scale, ok := data["scale"].(float64); ok {
		send.Scale = int64(scale)
	}

	// Extract Source
	if sourceMap, ok := data["source"].(map[string]interface{}); ok {
		send.Source = extractSource(sourceMap)
	}

	// Extract Distribute
	if distMap, ok := data["distribute"].(map[string]interface{}); ok {
		send.Distribute = extractDistribute(distMap)
	}

	return send
}

// extractSource converts a map to DSLSource
func extractSource(data map[string]interface{}) *DSLSource {
	if data == nil {
		return nil
	}

	source := &DSLSource{
		Remaining: getStringFromMap(data, "remaining"),
		From:      []DSLFromTo{},
	}

	// Extract From entries
	if fromList, ok := data["from"].([]interface{}); ok {
		for _, item := range fromList {
			if fromMap, ok := item.(map[string]interface{}); ok {
				fromEntry := extractFromTo(fromMap)
				source.From = append(source.From, fromEntry)
			}
		}
	}

	return source
}

// extractDistribute converts a map to DSLDistribute
func extractDistribute(data map[string]interface{}) *DSLDistribute {
	if data == nil {
		return nil
	}

	distribute := &DSLDistribute{
		Remaining: getStringFromMap(data, "remaining"),
		To:        []DSLFromTo{},
	}

	// Extract To entries
	if toList, ok := data["to"].([]interface{}); ok {
		for _, item := range toList {
			if toMap, ok := item.(map[string]interface{}); ok {
				toEntry := extractFromTo(toMap)
				distribute.To = append(distribute.To, toEntry)
			}
		}
	}

	return distribute
}

// extractFromTo converts a map to DSLFromTo
func extractFromTo(data map[string]interface{}) DSLFromTo {
	if data == nil {
		return DSLFromTo{}
	}

	from := DSLFromTo{
		Account:         getStringFromMap(data, "account"),
		Remaining:       getStringFromMap(data, "remaining"),
		Description:     getStringFromMap(data, "description"),
		ChartOfAccounts: getStringFromMap(data, "chartOfAccounts"),
		Metadata:        getMetadataFromMap(data),
	}

	// Extract Amount
	if amountMap, ok := data["amount"].(map[string]interface{}); ok {
		amount := &DSLAmount{
			Asset: getStringFromMap(amountMap, "asset"),
		}

		// Extract numeric values
		if value, ok := amountMap["value"].(float64); ok {
			amount.Value = int64(value)
		}

		if scale, ok := amountMap["scale"].(float64); ok {
			amount.Scale = int64(scale)
		}

		from.Amount = amount
	}

	// Extract Share
	if shareMap, ok := data["share"].(map[string]interface{}); ok {
		share := &Share{}

		if percentage, ok := shareMap["percentage"].(float64); ok {
			share.Percentage = int64(percentage)
		}

		if percentageOfPercentage, ok := shareMap["percentageOfPercentage"].(float64); ok {
			share.PercentageOfPercentage = int64(percentageOfPercentage)
		}

		from.Share = share
	}

	// Extract Rate
	if rateMap, ok := data["rate"].(map[string]interface{}); ok {
		rate := &Rate{
			From:       getStringFromMap(rateMap, "from"),
			To:         getStringFromMap(rateMap, "to"),
			ExternalID: getStringFromMap(rateMap, "externalId"),
		}

		if value, ok := rateMap["value"].(float64); ok {
			rate.Value = int64(value)
		}

		if scale, ok := rateMap["scale"].(float64); ok {
			rate.Scale = int64(scale)
		}

		from.Rate = rate
	}

	return from
}

// Helper functions for extracting values from maps

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// getMetadataFromMap safely extracts metadata from a map
func getMetadataFromMap(m map[string]interface{}) map[string]any {
	if val, ok := m["metadata"].(map[string]interface{}); ok {
		return val
	}
	return nil
}

// CreateTransactionInput is the input for creating a transaction.
// This structure contains all the fields needed to create a new transaction.
//
// CreateTransactionInput is used with the TransactionsService.CreateTransaction method
// to create new transactions in the standard format (as opposed to the DSL format).
// It allows for specifying the transaction details including operations, metadata,
// and other properties.
//
// When creating a transaction, the following rules apply:
//   - The transaction must be balanced (total debits must equal total credits for each asset)
//   - Each operation must specify an account, type (debit or credit), amount, and asset code
//   - The transaction can be created as pending (requiring explicit commitment later)
//   - External IDs and idempotency keys can be used to prevent duplicate transactions
//
// Example - Creating a simple payment transaction:
//
//	// Create a payment transaction with two operations (debit and credit)
//	input := &models.CreateTransactionInput{
//	    Description: "Payment for invoice #123",
//	    AssetCode:   "USD",
//	    Amount:      10000,
//	    Scale:       2, // $100.00
//	    Operations: []models.CreateOperationInput{
//	        {
//	            // Debit the customer's account (decrease balance)
//	            Type:        "debit",
//	            AccountID:   "acc-123", // Customer account ID
//	            AccountAlias: stringPtr("customer:john.doe"), // Optional alias
//	            Amount:      10000,
//	            AssetCode:   "USD",
//	            Scale:       2,
//	        },
//	        {
//	            // Credit the revenue account (increase balance)
//	            Type:        "credit",
//	            AccountID:   "acc-456", // Revenue account ID
//	            AccountAlias: stringPtr("revenue:payments"), // Optional alias
//	            Amount:      10000,
//	            AssetCode:   "USD",
//	            Scale:       2,
//	        },
//	    },
//	    Metadata: map[string]any{
//	        "invoice_id": "inv-123",
//	        "customer_id": "cust-456",
//	    },
//	    ExternalID: "payment-inv123-20230401",
//	}
//
// Example - Creating a pending transaction:
//
//	// Create a pending transaction that requires explicit commitment
//	input := &models.CreateTransactionInput{
//	    Description: "Large transfer pending approval",
//	    AssetCode:   "USD",
//	    Amount:      100000,
//	    Scale:       2, // $1,000.00
//	    Operations: []models.CreateOperationInput{
//	        // Debit operation
//	        {
//	            Type:        "debit",
//	            AccountID:   "acc-789", // Source account ID
//	            Amount:      100000,
//	            AssetCode:   "USD",
//	            Scale:       2,
//	        },
//	        // Credit operation
//	        {
//	            Type:        "credit",
//	            AccountID:   "acc-012", // Target account ID
//	            Amount:      100000,
//	            AssetCode:   "USD",
//	            Scale:       2,
//	        },
//	    },
//	    Pending: true, // Create as pending, requiring explicit commitment
//	    Metadata: map[string]any{
//	        "requires_approval": true,
//	        "approval_level": "manager",
//	    },
//	}
//
//	// Later, after approval:
//	// client.Transactions.CommitTransaction(ctx, orgID, ledgerID, tx.ID)
//
// Helper function for creating string pointers:
//
//	func stringPtr(s string) *string {
//	    return &s
//	}
type CreateTransactionInput struct {
	// Template is an optional identifier for the transaction template to use
	// Templates can be used to create standardized transactions with predefined
	// structures and validation rules
	Template string `json:"template,omitempty"`

	// Amount is the numeric value of the transaction
	// This represents the total value of the transaction as a fixed-point integer
	// The actual amount is calculated as Amount / 10^Scale
	Amount int64 `json:"amount"`

	// Scale represents the decimal precision for the amount
	// For example, a scale of 2 means the amount is in cents (100 = $1.00)
	Scale int64 `json:"scale"`

	// AssetCode identifies the currency or asset type for this transaction
	// Common examples include "USD", "EUR", "BTC", etc.
	AssetCode string `json:"assetCode"`

	// Operations contains the individual debit and credit operations
	// Each operation represents a single accounting entry (debit or credit)
	// The sum of all operations for each asset must balance to zero
	Operations []CreateOperationInput `json:"operations,omitempty"`

	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	// This is used when integrating with traditional accounting systems
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Description is a human-readable description of the transaction
	// This should provide context about the purpose or nature of the transaction
	Description string `json:"description,omitempty"`

	// ExternalID is an optional identifier for linking to external systems
	// This can be used to correlate transactions with records in other systems
	// and to prevent duplicate transactions
	ExternalID string `json:"externalId,omitempty"`

	// Pending indicates whether the transaction should be created in a pending state
	// Pending transactions require explicit commitment before they affect account balances
	Pending bool `json:"pending,omitempty"`

	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	// If a transaction with the same idempotency key already exists, that transaction
	// will be returned instead of creating a new one
	IdempotencyKey string `json:"idempotencyKey,omitempty"`

	// Send contains the source and distribution information for the transaction
	// This is an alternative to using Operations and provides a more structured way
	// to define the transaction flow
	Send *SendInput `json:"send,omitempty"`
}

// SendInput represents the send information for a transaction.
// This structure contains the source and distribution details for a transaction.
type SendInput struct {
	// Asset identifies the currency or asset type for this transaction
	Asset string `json:"asset"`

	// Value is the numeric value of the transaction
	Value int64 `json:"value"`

	// Scale represents the decimal precision for the amount
	Scale int64 `json:"scale"`

	// Source contains the source accounts for the transaction
	Source *SourceInput `json:"source"`

	// Distribute contains the destination accounts for the transaction
	Distribute *DistributeInput `json:"distribute"`
}

// SourceInput represents the source information for a transaction.
// This structure contains the source accounts for a transaction.
type SourceInput struct {
	// From contains the list of source accounts and amounts
	From []FromToInput `json:"from"`
}

// DistributeInput represents the distribution information for a transaction.
// This structure contains the destination accounts for a transaction.
type DistributeInput struct {
	// To contains the list of destination accounts and amounts
	To []FromToInput `json:"to"`
}

// FromToInput represents a single source or destination account in a transaction.
// This structure contains the account and amount details.
type FromToInput struct {
	// Account identifies the account affected by this operation
	Account string `json:"account"`

	// Amount specifies the amount details for this operation
	Amount AmountInput `json:"amount"`
}

// AmountInput represents the amount details for an operation.
// This structure contains the value, scale, and asset code for an amount.
type AmountInput struct {
	// Asset identifies the currency or asset type for this amount
	Asset string `json:"asset"`

	// Value is the numeric value of the amount
	Value int64 `json:"value"`

	// Scale represents the decimal precision for the amount
	Scale int64 `json:"scale"`
}

// Validate checks that the CreateTransactionInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *CreateTransactionInput) Validate() error {
	if input.Amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}

	if input.Scale < 0 || input.Scale > 18 {
		return fmt.Errorf("scale must be between 0 and 18")
	}

	if input.AssetCode == "" {
		return fmt.Errorf("assetCode is required")
	}

	// Validate asset code
	if err := core.ValidateAssetCode(input.AssetCode); err != nil {
		return err
	}

	if len(input.Operations) == 0 && input.Send == nil {
		return fmt.Errorf("either operations or send must be provided")
	}

	// If Operations is provided, validate each operation
	if len(input.Operations) > 0 {
		// Validate operations
		for i, op := range input.Operations {
			if err := op.Validate(); err != nil {
				return fmt.Errorf("invalid operation at index %d: %w", i, err)
			}
		}
	}

	// If Send is provided, validate it
	if input.Send != nil {
		if err := input.Send.Validate(); err != nil {
			return fmt.Errorf("invalid send: %w", err)
		}
	}

	return nil
}

// Validate checks that the SendInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *SendInput) Validate() error {
	// Validate asset code
	if input.Asset == "" {
		return fmt.Errorf("asset is required")
	}

	// Validate value
	if input.Value <= 0 {
		return fmt.Errorf("value must be greater than zero")
	}

	// Validate scale
	if input.Scale < 0 || input.Scale > 18 {
		return fmt.Errorf("scale must be between 0 and 18")
	}

	// Validate source
	if input.Source == nil {
		return fmt.Errorf("source is required")
	}
	if err := input.Source.Validate(); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Validate distribute
	if input.Distribute == nil {
		return fmt.Errorf("distribute is required")
	}
	if err := input.Distribute.Validate(); err != nil {
		return fmt.Errorf("invalid distribute: %w", err)
	}

	return nil
}

// Validate checks that the SourceInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *SourceInput) Validate() error {
	// Validate from
	if len(input.From) == 0 {
		return fmt.Errorf("from is required")
	}

	// Validate each from
	for i, from := range input.From {
		if err := from.Validate(); err != nil {
			return fmt.Errorf("invalid from at index %d: %w", i, err)
		}
	}

	return nil
}

// Validate checks that the DistributeInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *DistributeInput) Validate() error {
	// Validate to
	if len(input.To) == 0 {
		return fmt.Errorf("to is required")
	}

	// Validate each to
	for i, to := range input.To {
		if err := to.Validate(); err != nil {
			return fmt.Errorf("invalid to at index %d: %w", i, err)
		}
	}

	return nil
}

// Validate checks that the FromToInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *FromToInput) Validate() error {
	// Validate account
	if input.Account == "" {
		return fmt.Errorf("account is required")
	}

	// Validate amount
	if err := input.Amount.Validate(); err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}

	return nil
}

// Validate checks that the AmountInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *AmountInput) Validate() error {
	// Validate asset
	if input.Asset == "" {
		return fmt.Errorf("asset is required")
	}

	// Validate value
	if input.Value <= 0 {
		return fmt.Errorf("value must be greater than zero")
	}

	// Validate scale
	if input.Scale < 0 || input.Scale > 18 {
		return fmt.Errorf("scale must be between 0 and 18")
	}

	return nil
}

// ToLibTransaction converts a CreateTransactionInput to a lib-commons transaction.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *CreateTransactionInput) ToLibTransaction() map[string]interface{} {
	if input == nil {
		return nil
	}

	// Create a map to hold the transaction data
	tx := map[string]interface{}{
		"description": input.Description,
		"metadata":    input.Metadata,
	}

	// Add chart of accounts group name if present
	if input.ChartOfAccountsGroupName != "" {
		tx["chartOfAccountsGroupName"] = input.ChartOfAccountsGroupName
	}

	// Add pending flag if true
	if input.Pending {
		tx["pending"] = input.Pending
	}

	// Add send information if present
	if input.Send != nil {
		tx["send"] = input.Send.ToMap()
	}

	return tx
}

// ToMap converts a SendInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *SendInput) ToMap() map[string]interface{} {
	if input == nil {
		return nil
	}

	send := map[string]interface{}{
		"asset": input.Asset,
		"value": input.Value,
		"scale": input.Scale,
	}

	// Add source information if present
	if input.Source != nil {
		send["source"] = input.Source.ToMap()
	}

	// Add distribute information if present
	if input.Distribute != nil {
		send["distribute"] = input.Distribute.ToMap()
	}

	return send
}

// ToMap converts a SourceInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *SourceInput) ToMap() map[string]interface{} {
	if input == nil {
		return nil
	}

	source := map[string]interface{}{}

	// Add from information if present
	if len(input.From) > 0 {
		fromList := make([]map[string]interface{}, 0, len(input.From))
		for _, from := range input.From {
			fromList = append(fromList, from.ToMap())
		}
		source["from"] = fromList
	}

	return source
}

// ToMap converts a DistributeInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *DistributeInput) ToMap() map[string]interface{} {
	if input == nil {
		return nil
	}

	distribute := map[string]interface{}{}

	// Add to information if present
	if len(input.To) > 0 {
		toList := make([]map[string]interface{}, 0, len(input.To))
		for _, to := range input.To {
			toList = append(toList, to.ToMap())
		}
		distribute["to"] = toList
	}

	return distribute
}

// ToMap converts a FromToInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input FromToInput) ToMap() map[string]interface{} {
	fromTo := map[string]interface{}{
		"account": input.Account,
	}

	// Add amount information
	fromTo["amount"] = input.Amount.ToMap()

	return fromTo
}

// ToMap converts an AmountInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *AmountInput) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"asset": input.Asset,
		"value": input.Value,
		"scale": input.Scale,
	}
}

// ToTransactionMap converts an SDK Transaction to a map for API requests.
// This method is used internally to prepare data for the backend API.
func (t *Transaction) ToTransactionMap() map[string]interface{} {
	if t == nil {
		return nil
	}

	transaction := map[string]interface{}{
		"description": t.Description,
		"metadata":    t.Metadata,
	}

	// Build send structure
	send := map[string]interface{}{
		"asset": t.AssetCode,
		"value": t.Amount,
		"scale": t.Scale,
	}

	// Source (debits)
	source := map[string]interface{}{}
	fromEntries := []map[string]interface{}{}

	// Distribute (credits)
	distribute := map[string]interface{}{}
	toEntries := []map[string]interface{}{}

	// Convert Operations
	for _, op := range t.Operations {
		entry := map[string]interface{}{
			"account": op.AccountID,
			"amount": map[string]interface{}{
				"value": op.Amount.Value,
				"scale": op.Amount.Scale,
				"asset": op.Amount.AssetCode,
			},
		}

		// Add alias as description if present
		if op.AccountAlias != nil {
			entry["description"] = *op.AccountAlias
		}

		// Add to appropriate list based on operation type
		if op.Type == "debit" {
			fromEntries = append(fromEntries, entry)
		} else {
			toEntries = append(toEntries, entry)
		}
	}

	// Add from entries if any exist
	if len(fromEntries) > 0 {
		source["from"] = fromEntries
		send["source"] = source
	}

	// Add to entries if any exist
	if len(toEntries) > 0 {
		distribute["to"] = toEntries
		send["distribute"] = distribute
	}

	// Add send to transaction
	transaction["send"] = send

	return transaction
}

// UpdateTransactionInput represents the input for updating a transaction.
// This structure contains the fields that can be updated on an existing transaction.
//
// UpdateTransactionInput is used with the TransactionsService.UpdateTransaction method
// to update existing transactions. It allows for updating metadata and other mutable
// properties of a transaction.
//
// Note that not all fields of a transaction can be updated after creation, especially
// for transactions that have already been committed. Typically, only metadata and
// certain status-related fields can be modified.
//
// Example - Updating transaction metadata:
//
//	// Update a transaction's metadata
//	input := &models.UpdateTransactionInput{
//	    Metadata: map[string]any{
//	        "updated_by": "admin",
//	        "approval_status": "approved",
//	        "notes": "Verified and approved by finance team",
//	    },
//	}
//
//	updatedTx, err := client.Transactions.UpdateTransaction(
//	    ctx, orgID, ledgerID, transactionID, input,
//	)
type UpdateTransactionInput struct {
	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// Description is a human-readable description of the transaction
	// This should provide context about the purpose or nature of the transaction
	Description string `json:"description,omitempty"`

	// ExternalID is an optional identifier for linking to external systems
	// This can be used to correlate transactions with records in other systems
	ExternalID string `json:"externalId,omitempty"`
}

// Validate checks if the UpdateTransactionInput meets the validation requirements.
// It returns an error if any of the validation checks fail.
//
// Returns:
//   - error: An error if the input is invalid, nil otherwise
func (input *UpdateTransactionInput) Validate() error {
	// Validate description length if provided
	if input.Description != "" && len(input.Description) > 256 {
		return fmt.Errorf("description must not exceed 256 characters")
	}

	// Validate external ID if provided
	if input.ExternalID != "" && len(input.ExternalID) > 64 {
		return fmt.Errorf("externalId must not exceed 64 characters")
	}

	// Validate metadata if provided
	if input.Metadata != nil {
		if err := core.ValidateMetadata(input.Metadata); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	return nil
}
