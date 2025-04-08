// Package libintegration provides internal conversion between SDK models and lib-commons models.
// This package is internal and not intended for direct use by SDK users.
//
// The main purpose of this package is to isolate all direct dependencies on lib-commons
// from the public SDK API. This allows us to maintain compatibility with backend services
// while presenting a clean, self-contained API to SDK users.
package libintegration

import (
	libTransaction "github.com/LerianStudio/lib-commons/commons/transaction"
	"github.com/LerianStudio/midaz-sdk-golang/models"
)

// FromTransactionDSLInput converts an SDK TransactionDSLInput to a lib-commons Transaction.
// This is used internally to prepare API requests.
func FromTransactionDSLInput(input *models.TransactionDSLInput) *libTransaction.Transaction {
	if input == nil {
		return nil
	}

	// Create a new lib-commons Transaction
	transaction := &libTransaction.Transaction{
		ChartOfAccountsGroupName: input.ChartOfAccountsGroupName,
		Description:              input.Description,
		Code:                     input.Code,
		Pending:                  input.Pending,
		Metadata:                 input.Metadata,
	}

	// Convert Send
	if input.Send != nil {
		transaction.Send = convertSend(input.Send)
	}

	return transaction
}

// ToTransactionDSLInput converts a lib-commons Transaction to an SDK TransactionDSLInput.
// This is used internally to process API responses.
func ToTransactionDSLInput(t *libTransaction.Transaction) *models.TransactionDSLInput {
	if t == nil {
		return nil
	}

	// Create a new TransactionDSLInput
	input := &models.TransactionDSLInput{
		ChartOfAccountsGroupName: t.ChartOfAccountsGroupName,
		Description:              t.Description,
		Code:                     t.Code,
		Pending:                  t.Pending,
		Metadata:                 t.Metadata,
	}

	// Convert Send from lib-commons to SDK model
	input.Send = &models.DSLSend{
		Asset: t.Send.Asset,
		Value: t.Send.Value,
		Scale: t.Send.Scale,
	}

	// Convert Source
	if len(t.Send.Source.From) > 0 {
		input.Send.Source = &models.DSLSource{
			Remaining: t.Send.Source.Remaining,
		}

		// Convert From
		for _, from := range t.Send.Source.From {
			dslFrom := convertLibFromToToSDK(from)
			input.Send.Source.From = append(input.Send.Source.From, dslFrom)
		}
	}

	// Convert Distribute
	if len(t.Send.Distribute.To) > 0 {
		input.Send.Distribute = &models.DSLDistribute{
			Remaining: t.Send.Distribute.Remaining,
		}

		// Convert To
		for _, to := range t.Send.Distribute.To {
			dslTo := convertLibFromToToSDK(to)
			input.Send.Distribute.To = append(input.Send.Distribute.To, dslTo)
		}
	}

	return input
}

// FromTransaction converts an SDK Transaction to a lib-commons Transaction.
func FromTransaction(t *models.Transaction) *libTransaction.Transaction {
	if t == nil {
		return nil
	}

	// Create a new lib-commons Transaction
	lt := &libTransaction.Transaction{
		Send: libTransaction.Send{
			Asset: t.AssetCode,
			Value: t.Amount,
			Scale: t.Scale,
			Source: libTransaction.Source{
				From: make([]libTransaction.FromTo, 0),
			},
			Distribute: libTransaction.Distribute{
				To: make([]libTransaction.FromTo, 0),
			},
		},
		Metadata: t.Metadata,
	}

	// Convert Operations
	for _, op := range t.Operations {
		fromTo := libTransaction.FromTo{
			Account: op.AccountID,
			Amount: &libTransaction.Amount{
				Value: op.Amount.Value,
				Scale: int64(op.Amount.Scale),
				Asset: op.Amount.AssetCode,
			},
		}

		if op.AccountAlias != nil {
			fromTo.Description = *op.AccountAlias
		}

		if op.Type == string(models.OperationTypeDebit) {
			lt.Send.Source.From = append(lt.Send.Source.From, fromTo)
		} else {
			lt.Send.Distribute.To = append(lt.Send.Distribute.To, fromTo)
		}
	}

	return lt
}

// ToTransaction converts a lib-commons Transaction to an SDK Transaction.
func ToTransaction(lt *libTransaction.Transaction, id string) *models.Transaction {
	if lt == nil {
		return nil
	}

	transaction := &models.Transaction{
		ID:          id,
		Description: lt.Description,
		AssetCode:   lt.Send.Asset,
		Amount:      lt.Send.Value,
		Scale:       lt.Send.Scale,
		Metadata:    lt.Metadata,
	}

	return transaction
}

// Private helper functions

func convertSend(send *models.DSLSend) libTransaction.Send {
	libSend := libTransaction.Send{
		Asset: send.Asset,
		Value: send.Value,
		Scale: send.Scale,
	}

	// Convert Source
	if send.Source != nil {
		libSend.Source = convertSource(send.Source)
	}

	// Convert Distribute
	if send.Distribute != nil {
		libSend.Distribute = convertDistribute(send.Distribute)
	}

	return libSend
}

func convertSource(source *models.DSLSource) libTransaction.Source {
	libSource := libTransaction.Source{
		Remaining: source.Remaining,
	}

	// Convert From
	for _, from := range source.From {
		libFrom := convertFromTo(from)
		libSource.From = append(libSource.From, libFrom)
	}

	return libSource
}

func convertDistribute(distribute *models.DSLDistribute) libTransaction.Distribute {
	libDistribute := libTransaction.Distribute{
		Remaining: distribute.Remaining,
	}

	// Convert To
	for _, to := range distribute.To {
		libTo := convertFromTo(to)
		libDistribute.To = append(libDistribute.To, libTo)
	}

	return libDistribute
}

func convertFromTo(from models.DSLFromTo) libTransaction.FromTo {
	libFrom := libTransaction.FromTo{
		Account:         from.Account,
		Remaining:       from.Remaining,
		Description:     from.Description,
		ChartOfAccounts: from.ChartOfAccounts,
		Metadata:        from.Metadata,
	}

	// Convert Amount
	if from.Amount != nil {
		libFrom.Amount = &libTransaction.Amount{
			Asset: from.Amount.Asset,
			Value: from.Amount.Value,
			Scale: from.Amount.Scale,
		}
	}

	// Convert Share
	if from.Share != nil {
		libFrom.Share = &libTransaction.Share{
			Percentage:             from.Share.Percentage,
			PercentageOfPercentage: from.Share.PercentageOfPercentage,
		}
	}

	// Convert Rate
	if from.Rate != nil {
		libFrom.Rate = &libTransaction.Rate{
			From:       from.Rate.From,
			To:         from.Rate.To,
			Value:      from.Rate.Value,
			Scale:      from.Rate.Scale,
			ExternalID: from.Rate.ExternalID,
		}
	}

	return libFrom
}

func convertLibFromToToSDK(from libTransaction.FromTo) models.DSLFromTo {
	dslFrom := models.DSLFromTo{
		Account:         from.Account,
		Remaining:       from.Remaining,
		Description:     from.Description,
		ChartOfAccounts: from.ChartOfAccounts,
		Metadata:        from.Metadata,
	}

	// Convert Amount
	if from.Amount != nil {
		dslFrom.Amount = &models.DSLAmount{
			Asset: from.Amount.Asset,
			Value: from.Amount.Value,
			Scale: from.Amount.Scale,
		}
	}

	// Convert Share
	if from.Share != nil {
		dslFrom.Share = &models.Share{
			Percentage:             from.Share.Percentage,
			PercentageOfPercentage: from.Share.PercentageOfPercentage,
		}
	}

	// Convert Rate
	if from.Rate != nil {
		dslFrom.Rate = &models.Rate{
			From:       from.Rate.From,
			To:         from.Rate.To,
			Value:      from.Rate.Value,
			Scale:      from.Rate.Scale,
			ExternalID: from.Rate.ExternalID,
		}
	}

	return dslFrom
}

// ToSDKMap converts a lib-commons transaction to a map for the SDK
func ToSDKMap(lt *libTransaction.Transaction) map[string]interface{} {
	if lt == nil {
		return nil
	}

	tx := map[string]interface{}{
		"description": lt.Description,
		"metadata":    lt.Metadata,
	}

	// Add chart of accounts group name if present
	if lt.ChartOfAccountsGroupName != "" {
		tx["chartOfAccountsGroupName"] = lt.ChartOfAccountsGroupName
	}

	// Add pending flag if true
	if lt.Pending {
		tx["pending"] = lt.Pending
	}

	// Add send information if present
	send := map[string]interface{}{
		"asset": lt.Send.Asset,
		"value": lt.Send.Value,
		"scale": lt.Send.Scale,
	}

	// Source
	if len(lt.Send.Source.From) > 0 {
		source := map[string]interface{}{}

		fromList := make([]map[string]interface{}, 0, len(lt.Send.Source.From))
		for _, from := range lt.Send.Source.From {
			fromMap := convertLibFromToToMap(from)
			fromList = append(fromList, fromMap)
		}
		source["from"] = fromList

		if lt.Send.Source.Remaining != "" {
			source["remaining"] = lt.Send.Source.Remaining
		}

		send["source"] = source
	}

	// Distribute
	if len(lt.Send.Distribute.To) > 0 {
		distribute := map[string]interface{}{}

		toList := make([]map[string]interface{}, 0, len(lt.Send.Distribute.To))
		for _, to := range lt.Send.Distribute.To {
			toMap := convertLibFromToToMap(to)
			toList = append(toList, toMap)
		}
		distribute["to"] = toList

		if lt.Send.Distribute.Remaining != "" {
			distribute["remaining"] = lt.Send.Distribute.Remaining
		}

		send["distribute"] = distribute
	}

	tx["send"] = send

	return tx
}

// Helper to convert libTransaction.FromTo to map
func convertLibFromToToMap(from libTransaction.FromTo) map[string]interface{} {
	fromMap := map[string]interface{}{
		"account": from.Account,
	}

	// Add amount if present
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

	// Add share if present
	if from.Share != nil {
		fromMap["share"] = map[string]interface{}{
			"percentage":             from.Share.Percentage,
			"percentageOfPercentage": from.Share.PercentageOfPercentage,
		}
	}

	// Add rate if present
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
