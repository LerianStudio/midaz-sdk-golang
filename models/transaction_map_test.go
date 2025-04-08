package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransactionDSLInputToMap(t *testing.T) {
	// Create a test transaction
	input := &TransactionDSLInput{
		ChartOfAccountsGroupName: "test-group",
		Description:              "Test Transaction",
		Code:                     "TEST",
		Pending:                  true,
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
		Send: &DSLSend{
			Asset: "USD",
			Value: 1000,
			Scale: 2,
			Source: &DSLSource{
				Remaining: "remaining-source",
				From: []DSLFromTo{
					{
						Account: "account-1",
						Amount: &DSLAmount{
							Asset: "USD",
							Value: 1000,
							Scale: 2,
						},
					},
				},
			},
			Distribute: &DSLDistribute{
				Remaining: "remaining-dest",
				To: []DSLFromTo{
					{
						Account: "account-2",
						Amount: &DSLAmount{
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
	txMap := input.ToTransactionMap()

	// Serialize to JSON for easier verification
	jsonData, err := json.Marshal(txMap)
	assert.NoError(t, err)

	// Deserialize to verify
	var decoded map[string]interface{}
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, "Test Transaction", decoded["description"])
	assert.Equal(t, map[string]interface{}{"test": "metadata"}, decoded["metadata"])
	assert.Equal(t, "test-group", decoded["chartOfAccountsGroupName"])
	assert.Equal(t, "TEST", decoded["code"])
	assert.Equal(t, true, decoded["pending"])

	// Verify send structure
	send, ok := decoded["send"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "USD", send["asset"])
	assert.Equal(t, float64(1000), send["value"])
	assert.Equal(t, float64(2), send["scale"])

	// Verify source
	source, ok := send["source"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "remaining-source", source["remaining"])
	fromList, ok := source["from"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, fromList, 1)
	from, ok := fromList[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "account-1", from["account"])

	// Verify distribute
	distribute, ok := send["distribute"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "remaining-dest", distribute["remaining"])
	toList, ok := distribute["to"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, toList, 1)
	to, ok := toList[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "account-2", to["account"])
}

func TestFromTransactionMap(t *testing.T) {
	// Create a test map
	txMap := map[string]interface{}{
		"chartOfAccountsGroupName": "test-group",
		"description":              "Test Transaction",
		"code":                     "TEST",
		"pending":                  true,
		"metadata": map[string]interface{}{
			"test": "metadata",
		},
		"send": map[string]interface{}{
			"asset": "USD",
			"value": float64(1000),
			"scale": float64(2),
			"source": map[string]interface{}{
				"remaining": "remaining-source",
				"from": []interface{}{
					map[string]interface{}{
						"account": "account-1",
						"amount": map[string]interface{}{
							"asset": "USD",
							"value": float64(1000),
							"scale": float64(2),
						},
					},
				},
			},
			"distribute": map[string]interface{}{
				"remaining": "remaining-dest",
				"to": []interface{}{
					map[string]interface{}{
						"account": "account-2",
						"amount": map[string]interface{}{
							"asset": "USD",
							"value": float64(1000),
							"scale": float64(2),
						},
					},
				},
			},
		},
	}

	// Convert from map
	input := FromTransactionMap(txMap)

	// Verify basic fields
	assert.Equal(t, "test-group", input.ChartOfAccountsGroupName)
	assert.Equal(t, "Test Transaction", input.Description)
	assert.Equal(t, "TEST", input.Code)
	assert.Equal(t, true, input.Pending)
	assert.Equal(t, map[string]interface{}{"test": "metadata"}, input.Metadata)

	// Verify Send
	assert.NotNil(t, input.Send)
	assert.Equal(t, "USD", input.Send.Asset)
	assert.Equal(t, int64(1000), input.Send.Value)
	assert.Equal(t, int64(2), input.Send.Scale)

	// Verify Source
	assert.NotNil(t, input.Send.Source)
	assert.Equal(t, "remaining-source", input.Send.Source.Remaining)
	assert.Len(t, input.Send.Source.From, 1)
	assert.Equal(t, "account-1", input.Send.Source.From[0].Account)
	assert.NotNil(t, input.Send.Source.From[0].Amount)
	assert.Equal(t, int64(1000), input.Send.Source.From[0].Amount.Value)

	// Verify Distribute
	assert.NotNil(t, input.Send.Distribute)
	assert.Equal(t, "remaining-dest", input.Send.Distribute.Remaining)
	assert.Len(t, input.Send.Distribute.To, 1)
	assert.Equal(t, "account-2", input.Send.Distribute.To[0].Account)
	assert.NotNil(t, input.Send.Distribute.To[0].Amount)
	assert.Equal(t, int64(1000), input.Send.Distribute.To[0].Amount.Value)
}

func TestTransactionToTransactionMap(t *testing.T) {
	// Create a test Transaction
	tx := &Transaction{
		ID:          "tx-123",
		Description: "Test Transaction",
		AssetCode:   "USD",
		Amount:      1000,
		Scale:       2,
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
		Operations: []Operation{
			{
				ID:        "op-1",
				Type:      "debit",
				AccountID: "account-1",
				Amount: Amount{
					Value:     1000,
					Scale:     2,
					AssetCode: "USD",
				},
			},
			{
				ID:        "op-2",
				Type:      "credit",
				AccountID: "account-2",
				Amount: Amount{
					Value:     1000,
					Scale:     2,
					AssetCode: "USD",
				},
			},
		},
	}

	// Convert to map
	txMap := tx.ToTransactionMap()

	// Verify basic fields
	assert.Equal(t, "Test Transaction", txMap["description"])
	assert.Equal(t, map[string]interface{}{"test": "metadata"}, txMap["metadata"])

	// Verify send structure
	send, ok := txMap["send"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "USD", send["asset"])
	assert.Equal(t, int64(1000), send["value"])
	assert.Equal(t, int64(2), send["scale"])

	// Convert to JSON and back to handle type conversions
	jsonData, err := json.Marshal(txMap)
	assert.NoError(t, err)

	var decoded map[string]interface{}
	err = json.Unmarshal(jsonData, &decoded)
	assert.NoError(t, err)

	// Verify source through JSON path
	send = decoded["send"].(map[string]interface{})
	source, ok := send["source"].(map[string]interface{})
	assert.True(t, ok, "send should have a source")

	fromEntries, ok := source["from"].([]interface{})
	assert.True(t, ok, "source should have a from array")
	assert.Len(t, fromEntries, 1, "from should have 1 entry")

	fromEntry := fromEntries[0].(map[string]interface{})
	assert.Equal(t, "account-1", fromEntry["account"])

	// Verify distribute
	distribute, ok := send["distribute"].(map[string]interface{})
	assert.True(t, ok, "send should have a distribute")

	toEntries, ok := distribute["to"].([]interface{})
	assert.True(t, ok, "distribute should have a to array")
	assert.Len(t, toEntries, 1, "to should have 1 entry")

	toEntry := toEntries[0].(map[string]interface{})
	assert.Equal(t, "account-2", toEntry["account"])
}
