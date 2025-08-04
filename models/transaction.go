package models

import (
	"fmt"
	"strconv"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation/core"
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
//	    fmt.Printf("Operation %d: %s %s %s on account %s\n",
//	        i+1, op.Type, op.AssetCode, op.Amount, op.AccountID)
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

	// Amount is the numeric value of the transaction as a decimal string
	// This represents the total value of the transaction (e.g., "100.50" for $100.50)
	Amount string `json:"amount"`

	// AssetCode identifies the currency or asset type for this transaction
	// Common examples include "USD", "EUR", "BTC", etc.
	AssetCode string `json:"assetCode"`

	// Route is the transaction route identifier that defines the overall flow
	// of the transaction, including the structure of operations to be executed
	Route string `json:"route,omitempty"`

	// Status indicates the current processing status of the transaction
	// See the Status enum for possible values (PENDING, COMPLETED, FAILED, CANCELED)
	Status Status `json:"status"`

	// ChartOfAccountsGroupName specifies the chart of accounts group to use
	// This categorizes the transaction under a specific group for accounting purposes
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName,omitempty"`

	// Source contains the list of source account aliases used in this transaction
	// These are the accounts from which funds are debited
	Source []string `json:"source,omitempty"`

	// Destination contains the list of destination account aliases used in this transaction
	// These are the accounts to which funds are credited
	Destination []string `json:"destination,omitempty"`

	// Pending indicates whether the transaction is in a pending state
	// Pending transactions require explicit commitment before affecting account balances
	Pending bool `json:"pending,omitempty"`

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

// DSLAmount represents an amount with a value and asset code for DSL transactions.
// This is aligned with the lib-commons Amount structure.
type DSLAmount struct {
	// Value is the numeric value of the amount as a decimal string
	Value string `json:"value"`

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

	// Value is the numeric value of the transaction as a decimal string
	Value string `json:"value"`

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

	// Convert string value to float64
	value, err := strconv.ParseFloat(input.Send.Value, 64)
	if err != nil {
		return 0
	}
	return value
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
	Value      string `json:"value"`
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

	if send.Value == "" || send.Value == "0" {
		return fmt.Errorf("value must be greater than 0")
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

// sendToMap converts the DSLSend to a map for API requests
func (input *TransactionDSLInput) sendToMap() map[string]any {
	if input.Send == nil {
		return nil
	}

	send := map[string]any{
		"asset": input.Send.Asset,
		"value": input.Send.Value,
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
		"account": from.Account,
	}

	// Add Amount if present
	if from.Amount != nil {
		fromMap["amount"] = map[string]any{
			"asset": from.Amount.Asset,
			"value": from.Amount.Value,
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
			"value":      from.Rate.Value,
			"externalId": from.Rate.ExternalID,
		}
	}

	return fromMap
}

// FromTransactionMap converts a map from the API to a TransactionDSLInput.
// This replaces the previous direct lib-commons conversion.
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

	// Extract numeric values
	if value, ok := data["value"].(string); ok {
		send.Value = value
	} else if value, ok := data["value"].(float64); ok {
		send.Value = fmt.Sprintf("%.2f", value)
	}

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
		Account:         getStringFromMap(data, "account"),
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

		// Extract numeric values
		if value, ok := amountMap["value"].(string); ok {
			amount.Value = value
		} else if value, ok := amountMap["value"].(float64); ok {
			amount.Value = fmt.Sprintf("%.2f", value)
		}

		from.Amount = amount
	}

	// Extract Share
	if shareMap, ok := data["share"].(map[string]any); ok {
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
	if rateMap, ok := data["rate"].(map[string]any); ok {
		rate := &Rate{
			From:       getStringFromMap(rateMap, "from"),
			To:         getStringFromMap(rateMap, "to"),
			ExternalID: getStringFromMap(rateMap, "externalId"),
		}

		if value, ok := rateMap["value"].(float64); ok {
			rate.Value = fmt.Sprintf("%.2f", value)
		}

		from.Rate = rate
	}

	return from
}

// Helper functions for extracting values from maps

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]any, key string) string {
	if val, ok := m[key].(string); ok {
		return val
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
	// Note: This is used for SDK logic but not sent in the API request
	Template string

	// Amount is the numeric value of the transaction as a decimal string
	// This represents the total value of the transaction (e.g., "100.50" for $100.50)
	// Note: This is used for validation but sent within the Send structure
	Amount string

	// AssetCode identifies the currency or asset type for this transaction
	// Common examples include "USD", "EUR", "BTC", etc.
	// Note: This is used for validation but sent within the Send structure
	AssetCode string

	// Operations contains the individual debit and credit operations
	// Each operation represents a single accounting entry (debit or credit)
	// The sum of all operations for each asset must balance to zero
	// Note: Operations are an alternative to Send and should not be serialized when Send is used
	Operations []CreateOperationInput

	// ChartOfAccountsGroupName is REQUIRED by the API specification
	// This categorizes the transaction under a specific chart of accounts group
	ChartOfAccountsGroupName string `json:"chartOfAccountsGroupName"`

	// Description is a human-readable description of the transaction (REQUIRED by API)
	// This should provide context about the purpose or nature of the transaction
	Description string `json:"description"`

	// Pending indicates whether the transaction should be created in a pending state
	// Pending transactions require explicit commitment before affecting account balances
	Pending bool `json:"pending,omitempty"`

	// Route is the transaction route identifier (optional)
	// This defines the overall flow of the transaction structure
	Route string `json:"route,omitempty"`

	// Metadata contains additional custom data for the transaction
	// This can be used to store application-specific information
	// such as references to external systems, tags, or other contextual data
	Metadata map[string]any `json:"metadata,omitempty"`

	// ExternalID is an optional identifier for linking to external systems
	// This can be used to correlate transactions with records in other systems
	// and to prevent duplicate transactions
	// Note: This is handled separately, not in request body for Send format
	ExternalID string

	// IdempotencyKey is a client-generated key to ensure transaction uniqueness
	// If a transaction with the same idempotency key already exists, that transaction
	// will be returned instead of creating a new one
	// Note: This is sent as a header (X-Idempotency), not in the request body
	IdempotencyKey string

	// Send contains the source and distribution information for the transaction (REQUIRED by API)
	// This is an alternative to using Operations and provides a more structured way
	// to define the transaction flow
	Send *SendInput `json:"send"`
}

// SendInput represents the send information for a transaction.
// This structure contains the source and distribution details for a transaction.
type SendInput struct {
	// Asset identifies the currency or asset type for this transaction
	Asset string `json:"asset"`

	// Value is the numeric value of the transaction as a decimal string
	Value string `json:"value"`

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

	// Route is the operation route identifier for this operation (optional)
	// This links the operation to a specific routing rule
	Route string `json:"route,omitempty"`

	// Description provides additional context for this operation (optional)
	Description string `json:"description,omitempty"`

	// ChartOfAccounts specifies the chart of accounts for this operation (optional)
	ChartOfAccounts string `json:"chartOfAccounts,omitempty"`

	// AccountAlias provides an alternative account identifier (optional)
	AccountAlias string `json:"accountAlias,omitempty"`

	// Metadata contains additional custom data for this operation
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AmountInput represents the amount details for an operation.
// This structure contains the value and asset code for an amount.
type AmountInput struct {
	// Asset identifies the currency or asset type for this amount
	Asset string `json:"asset"`

	// Value is the numeric value of the amount as a decimal string
	Value string `json:"value"`
}

// Validate checks that the CreateTransactionInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *CreateTransactionInput) Validate() error {
	if input.Amount == "" || input.Amount == "0" {
		return fmt.Errorf("amount must be greater than zero")
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

// NewCreateTransactionInput creates a new CreateTransactionInput with required fields.
// This constructor ensures that all mandatory fields are provided when creating a transaction input.
func NewCreateTransactionInput(assetCode string, amount string) *CreateTransactionInput {
	return &CreateTransactionInput{
		AssetCode: assetCode,
		Amount:    amount,
	}
}

// WithDescription sets the description.
// This adds a human-readable description to the transaction.
func (input *CreateTransactionInput) WithDescription(description string) *CreateTransactionInput {
	input.Description = description
	return input
}

// WithMetadata sets the metadata.
// This adds custom key-value data to the transaction.
func (input *CreateTransactionInput) WithMetadata(metadata map[string]any) *CreateTransactionInput {
	input.Metadata = metadata
	return input
}

// WithExternalID sets the external ID.
// This links the transaction to external systems.
func (input *CreateTransactionInput) WithExternalID(externalID string) *CreateTransactionInput {
	input.ExternalID = externalID
	return input
}

// WithOperations sets the operations list.
// This defines the individual debit and credit operations.
func (input *CreateTransactionInput) WithOperations(operations []CreateOperationInput) *CreateTransactionInput {
	input.Operations = operations
	return input
}

// WithSend sets the send structure.
// This provides an alternative way to define transaction flow.
func (input *CreateTransactionInput) WithSend(send *SendInput) *CreateTransactionInput {
	input.Send = send
	return input
}

// Validate checks that the SendInput meets all validation requirements.
// It returns an error if any of the validation checks fail.
func (input *SendInput) Validate() error {
	// Validate asset code
	if input.Asset == "" {
		return fmt.Errorf("asset is required")
	}

	// Validate value
	if input.Value == "" || input.Value == "0" {
		return fmt.Errorf("value must be greater than zero")
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
	if input.Value == "" || input.Value == "0" {
		return fmt.Errorf("value must be greater than zero")
	}

	return nil
}

// ToLibTransaction converts a CreateTransactionInput to a lib-commons transaction.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *CreateTransactionInput) ToLibTransaction() map[string]any {
	if input == nil {
		return nil
	}

	// Create a map to hold the transaction data
	tx := map[string]any{}

	// Add chart of accounts group name if provided (required by API)
	if input.ChartOfAccountsGroupName != "" {
		tx["chartOfAccountsGroupName"] = input.ChartOfAccountsGroupName
	}

	// Only add description if provided (required by API)
	if input.Description != "" {
		tx["description"] = input.Description
	}

	// Add pending field if set
	if input.Pending {
		tx["pending"] = input.Pending
	}

	// Add route if provided
	if input.Route != "" {
		tx["route"] = input.Route
	}

	// Add send information if present (required by API)
	if input.Send != nil {
		tx["send"] = input.Send.ToMap()
	}

	// Only add metadata if provided
	if len(input.Metadata) > 0 {
		tx["metadata"] = input.Metadata
	}

	return tx
}

// ToMap converts a SendInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *SendInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	send := map[string]any{
		"asset": input.Asset,
		"value": input.Value, // API expects value as string
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
func (input *SourceInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	source := map[string]any{}

	// Add from information if present
	if len(input.From) > 0 {
		fromList := make([]map[string]any, 0, len(input.From))
		for _, from := range input.From {
			fromList = append(fromList, from.ToMap())
		}
		source["from"] = fromList
	}

	return source
}

// ToMap converts a DistributeInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *DistributeInput) ToMap() map[string]any {
	if input == nil {
		return nil
	}

	distribute := map[string]any{}

	// Add to information if present
	if len(input.To) > 0 {
		toList := make([]map[string]any, 0, len(input.To))
		for _, to := range input.To {
			toList = append(toList, to.ToMap())
		}
		distribute["to"] = toList
	}

	return distribute
}

// ToMap converts a FromToInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input FromToInput) ToMap() map[string]any {
	fromTo := map[string]any{
		"accountAlias": input.Account, // API expects accountAlias, not account
	}

	// Add amount information
	fromTo["amount"] = input.Amount.ToMap()

	// Add route information if provided
	if input.Route != "" {
		fromTo["route"] = input.Route
	}

	return fromTo
}

// ToMap converts an AmountInput to a map.
// This is used internally by the SDK to convert the input to the format expected by the backend.
func (input *AmountInput) ToMap() map[string]any {
	return map[string]any{
		"asset": input.Asset,
		"value": input.Value, // API expects value as string
	}
}

// ToTransactionMap converts an SDK Transaction to a map for API requests.
// This method is used internally to prepare data for the backend API.
func (t *Transaction) ToTransactionMap() map[string]any {
	if t == nil {
		return nil
	}

	transaction := map[string]any{
		"description": t.Description,
		"metadata":    t.Metadata,
	}

	// Build send structure
	send := map[string]any{
		"asset": t.AssetCode,
		"value": t.Amount,
	}

	// Source (debits)
	source := map[string]any{}
	fromEntries := []map[string]any{}

	// Distribute (credits)
	distribute := map[string]any{}
	toEntries := []map[string]any{}

	// Convert Operations
	for _, op := range t.Operations {
		entry := map[string]any{
			"account": op.AccountID,
			"amount": map[string]any{
				"value": op.Amount,
				"asset": op.AssetCode,
			},
		}

		// Add alias as description if present
		if op.AccountAlias != "" {
			entry["description"] = op.AccountAlias
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

// NewUpdateTransactionInput creates a new UpdateTransactionInput.
// This constructor initializes an empty update input that can be customized
// using the With* methods for a fluent API experience.
//
// Returns:
//   - A pointer to the newly created UpdateTransactionInput
func NewUpdateTransactionInput() *UpdateTransactionInput {
	return &UpdateTransactionInput{}
}

// WithMetadata sets the metadata.
// This method allows updating the custom metadata associated with the transaction.
//
// Parameters:
//   - metadata: A map of key-value pairs to store as transaction metadata
//
// Returns:
//   - A pointer to the modified UpdateTransactionInput for method chaining
func (input *UpdateTransactionInput) WithMetadata(metadata map[string]any) *UpdateTransactionInput {
	input.Metadata = metadata
	return input
}

// WithDescription sets the description.
// This method allows updating the human-readable description of the transaction.
//
// Parameters:
//   - description: The new description for the transaction
//
// Returns:
//   - A pointer to the modified UpdateTransactionInput for method chaining
func (input *UpdateTransactionInput) WithDescription(description string) *UpdateTransactionInput {
	input.Description = description
	return input
}

// WithExternalID sets the external ID.
// This method allows setting or updating the external system identifier for the transaction.
//
// Parameters:
//   - externalID: The external identifier for linking to other systems
//
// Returns:
//   - A pointer to the modified UpdateTransactionInput for method chaining
func (input *UpdateTransactionInput) WithExternalID(externalID string) *UpdateTransactionInput {
	input.ExternalID = externalID
	return input
}
