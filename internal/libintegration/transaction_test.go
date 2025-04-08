package libintegration

import (
	"encoding/json"
	"testing"

	libTransaction "github.com/LerianStudio/lib-commons/commons/transaction"
	"github.com/LerianStudio/midaz-sdk-golang/models"
	"github.com/stretchr/testify/assert"
)

func TestFromTransactionDSLInput(t *testing.T) {
	// Create a test transaction
	input := &models.TransactionDSLInput{
		ChartOfAccountsGroupName: "test-group",
		Description:              "Test Transaction",
		Code:                     "TEST",
		Pending:                  true,
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
		Send: &models.DSLSend{
			Asset: "USD",
			Value: 1000,
			Scale: 2,
			Source: &models.DSLSource{
				Remaining: "remaining-source",
				From: []models.DSLFromTo{
					{
						Account: "account-1",
						Amount: &models.DSLAmount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
			Distribute: &models.DSLDistribute{
				Remaining: "remaining-dest",
				To: []models.DSLFromTo{
					{
						Account: "account-2",
						Amount: &models.DSLAmount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
		},
	}

	// Convert to lib-commons
	libTx := FromTransactionDSLInput(input)

	// Verify basic fields
	assert.Equal(t, input.ChartOfAccountsGroupName, libTx.ChartOfAccountsGroupName)
	assert.Equal(t, input.Description, libTx.Description)
	assert.Equal(t, input.Code, libTx.Code)
	assert.Equal(t, input.Pending, libTx.Pending)
	assert.Equal(t, input.Metadata, libTx.Metadata)

	// Verify Send
	assert.Equal(t, input.Send.Asset, libTx.Send.Asset)
	assert.Equal(t, input.Send.Value, libTx.Send.Value)
	assert.Equal(t, input.Send.Scale, libTx.Send.Scale)

	// Verify Source
	assert.Equal(t, input.Send.Source.Remaining, libTx.Send.Source.Remaining)
	assert.Len(t, libTx.Send.Source.From, 1)
	assert.Equal(t, input.Send.Source.From[0].Account, libTx.Send.Source.From[0].Account)
	assert.Equal(t, input.Send.Source.From[0].Amount.Value, libTx.Send.Source.From[0].Amount.Value)

	// Verify Distribute
	assert.Equal(t, input.Send.Distribute.Remaining, libTx.Send.Distribute.Remaining)
	assert.Len(t, libTx.Send.Distribute.To, 1)
	assert.Equal(t, input.Send.Distribute.To[0].Account, libTx.Send.Distribute.To[0].Account)
	assert.Equal(t, input.Send.Distribute.To[0].Amount.Value, libTx.Send.Distribute.To[0].Amount.Value)
}

func TestToTransactionDSLInput(t *testing.T) {
	// Create a test lib-commons transaction
	libTx := &libTransaction.Transaction{
		ChartOfAccountsGroupName: "test-group",
		Description:              "Test Transaction",
		Code:                     "TEST",
		Pending:                  true,
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
		Send: libTransaction.Send{
			Asset: "USD",
			Value: 1000,
			Scale: 2,
			Source: libTransaction.Source{
				Remaining: "remaining-source",
				From: []libTransaction.FromTo{
					{
						Account: "account-1",
						Amount: &libTransaction.Amount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
			Distribute: libTransaction.Distribute{
				Remaining: "remaining-dest",
				To: []libTransaction.FromTo{
					{
						Account: "account-2",
						Amount: &libTransaction.Amount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
		},
	}

	// Convert to SDK transaction
	input := ToTransactionDSLInput(libTx)

	// Verify basic fields
	assert.Equal(t, libTx.ChartOfAccountsGroupName, input.ChartOfAccountsGroupName)
	assert.Equal(t, libTx.Description, input.Description)
	assert.Equal(t, libTx.Code, input.Code)
	assert.Equal(t, libTx.Pending, input.Pending)
	assert.Equal(t, libTx.Metadata, input.Metadata)

	// Verify Send
	assert.Equal(t, libTx.Send.Asset, input.Send.Asset)
	assert.Equal(t, libTx.Send.Value, input.Send.Value)
	assert.Equal(t, libTx.Send.Scale, input.Send.Scale)

	// Verify Source
	assert.Equal(t, libTx.Send.Source.Remaining, input.Send.Source.Remaining)
	assert.Len(t, input.Send.Source.From, 1)
	assert.Equal(t, libTx.Send.Source.From[0].Account, input.Send.Source.From[0].Account)
	assert.Equal(t, libTx.Send.Source.From[0].Amount.Value, input.Send.Source.From[0].Amount.Value)

	// Verify Distribute
	assert.Equal(t, libTx.Send.Distribute.Remaining, input.Send.Distribute.Remaining)
	assert.Len(t, input.Send.Distribute.To, 1)
	assert.Equal(t, libTx.Send.Distribute.To[0].Account, input.Send.Distribute.To[0].Account)
	assert.Equal(t, libTx.Send.Distribute.To[0].Amount.Value, input.Send.Distribute.To[0].Amount.Value)
}

func TestSDKMapConversion(t *testing.T) {
	// Test converting lib-commons Transaction to map
	libTx := &libTransaction.Transaction{
		ChartOfAccountsGroupName: "test-group",
		Description:              "Test Transaction",
		Code:                     "TEST",
		Pending:                  true,
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
		Send: libTransaction.Send{
			Asset: "USD",
			Value: 1000,
			Scale: 2,
			Source: libTransaction.Source{
				From: []libTransaction.FromTo{
					{
						Account: "account-1",
						Amount: &libTransaction.Amount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
			Distribute: libTransaction.Distribute{
				To: []libTransaction.FromTo{
					{
						Account: "account-2",
						Amount: &libTransaction.Amount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
		},
	}

	// Convert to map
	txMap := ToSDKMap(libTx)

	// Serialize to JSON
	jsonData, err := json.Marshal(txMap)
	assert.NoError(t, err)

	// Deserialize to verify
	var decoded map[string]interface{}
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "Test Transaction", decoded["description"])
	assert.Equal(t, map[string]interface{}{"test": "metadata"}, decoded["metadata"])

	// Verify send structure
	send, ok := decoded["send"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "USD", send["asset"])
	assert.Equal(t, float64(1000), send["value"])
	assert.Equal(t, float64(2), send["scale"])

	// Verify source
	source, ok := send["source"].(map[string]interface{})
	assert.True(t, ok)
	fromList, ok := source["from"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, fromList, 1)

	// Verify distribute
	distribute, ok := send["distribute"].(map[string]interface{})
	assert.True(t, ok)
	toList, ok := distribute["to"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, toList, 1)
}
