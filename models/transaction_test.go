package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newValidSendInput(value float64) *SendInput {
	asset := "USD"

	return &SendInput{
		Asset: asset,
		Value: value,
		Source: &SourceInput{From: []FromToInput{{
			Account: "source-account",
			Amount:  AmountInput{Asset: asset, Value: value},
		}}},
		Distribute: &DistributeInput{To: []FromToInput{{
			Account: "dest-account",
			Amount:  AmountInput{Asset: asset, Value: value},
		}}},
	}
}

// =============================================================================
// CreateTransactionInput Tests
// =============================================================================

func TestNewCreateTransactionInput(t *testing.T) {
	tests := []struct {
		name      string
		assetCode string
		amount    float64
		wantAsset string
		wantAmt   float64
	}{
		{
			name:      "valid USD transaction",
			assetCode: "USD",
			amount:    100.50,
			wantAsset: "USD",
			wantAmt:   100.50,
		},
		{
			name:      "valid BRL transaction",
			assetCode: "BRL",
			amount:    1000,
			wantAsset: "BRL",
			wantAmt:   1000,
		},
		{
			name:      "valid BTC transaction",
			assetCode: "BTC",
			amount:    0.00001,
			wantAsset: "BTC",
			wantAmt:   0.00001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateTransactionInput(tt.assetCode, tt.amount)
			require.NotNil(t, input.Send)
			assert.Equal(t, tt.wantAsset, input.Send.Asset)
			assert.Equal(t, decimalStringFromAny(tt.wantAmt), input.Send.Value)
		})
	}
}

func TestCreateTransactionInputExternalIDIsNeverSerialized(t *testing.T) {
	input := NewCreateTransactionInput("USD", "10.00").WithExternalID("external-id-is-client-only")
	input.Send.Source = &SourceInput{From: []FromToInput{{Account: "source", Amount: AmountInput{Asset: "USD", Value: "10.00"}}}}
	input.Send.Distribute = &DistributeInput{To: []FromToInput{{Account: "destination", Amount: AmountInput{Asset: "USD", Value: "10.00"}}}}

	body, err := json.Marshal(input)
	require.NoError(t, err)

	require.NotContains(t, string(body), "externalId")
	require.NotContains(t, string(body), "ExternalID")
	require.NotContains(t, string(body), "external-id-is-client-only")
}

func TestCreateTransactionInput_Validate(t *testing.T) {
	validSend := newValidSendInput(100)

	tests := []struct {
		name    string
		input   *CreateTransactionInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid with send",
			input: &CreateTransactionInput{
				Send: validSend,
			},
			wantErr: false,
		},
		{
			name: "valid with optional fields",
			input: &CreateTransactionInput{
				Description:              "Payment",
				ChartOfAccountsGroupName: "ASSETS",
				Metadata:                 map[string]any{"key": "value"},
				Send:                     validSend,
			},
			wantErr: false,
		},
		{
			name:    "missing send",
			input:   &CreateTransactionInput{Description: "No send"},
			wantErr: true,
			errMsg:  "send is required",
		},
		{
			name: "invalid send - zero value",
			input: &CreateTransactionInput{
				Send: &SendInput{
					Asset:      "USD",
					Value:      0,
					Source:     validSend.Source,
					Distribute: validSend.Distribute,
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "invalid send - missing asset",
			input: &CreateTransactionInput{
				Send: &SendInput{
					Asset:      "",
					Value:      100,
					Source:     validSend.Source,
					Distribute: validSend.Distribute,
				},
			},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name: "invalid send - missing source",
			input: &CreateTransactionInput{
				Send: &SendInput{
					Asset:      "USD",
					Value:      100,
					Source:     nil,
					Distribute: validSend.Distribute,
				},
			},
			wantErr: true,
			errMsg:  "source is required",
		},
		{
			name: "invalid send - missing source",
			input: &CreateTransactionInput{
				Send: &SendInput{
					Asset:      "USD",
					Value:      100,
					Source:     nil,
					Distribute: validSend.Distribute,
				},
			},
			wantErr: true,
			errMsg:  "source is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateTransactionInput_ValidateBalancedFixedAmounts(t *testing.T) {
	t.Run("rejects unbalanced source and distribute totals", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", "100.00").WithSend(&SendInput{
			Asset: "USD",
			Value: "100.00",
			Source: &SourceInput{From: []FromToInput{{
				Account: "source-account",
				Amount:  AmountInput{Asset: "USD", Value: "100.00"},
			}}},
			Distribute: &DistributeInput{To: []FromToInput{{
				Account: "dest-account",
				Amount:  AmountInput{Asset: "USD", Value: "90.00"},
			}}},
		})

		require.ErrorContains(t, input.Validate(), "source and distribute totals must match")
	})

	t.Run("rejects send value that differs from balanced entries", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", "100.00").WithSend(&SendInput{
			Asset: "USD",
			Value: "100.00",
			Source: &SourceInput{From: []FromToInput{{
				Account: "source-account",
				Amount:  AmountInput{Asset: "USD", Value: "90.00"},
			}}},
			Distribute: &DistributeInput{To: []FromToInput{{
				Account: "dest-account",
				Amount:  AmountInput{Asset: "USD", Value: "90.00"},
			}}},
		})

		require.ErrorContains(t, input.Validate(), "value must equal source and distribute totals")
	})
}

func TestCreateTransactionInput_WithMethods(t *testing.T) {
	t.Run("WithDescription", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 100)
		result := input.WithDescription("Test payment")
		assert.Equal(t, "Test payment", result.Description)
		assert.Same(t, input, result)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 100)
		metadata := map[string]any{"key": "value", "number": 42}
		result := input.WithMetadata(metadata)
		assert.Equal(t, metadata, result.Metadata)
		assert.Same(t, input, result)
	})

	t.Run("WithExternalID", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 100)
		result := input.WithExternalID("ext-123")
		assert.Equal(t, "ext-123", result.ExternalID)
		assert.Same(t, input, result)
	})

	t.Run("WithCode", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 100)
		result := input.WithCode("TX-001")
		assert.Equal(t, "TX-001", result.Code)
		assert.Same(t, input, result)
	})

	t.Run("WithSend", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 100)
		send := newValidSendInput(100)
		result := input.WithSend(send)
		assert.Equal(t, send, result.Send)
		assert.Same(t, input, result)
	})

	t.Run("chained methods", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 100).
			WithDescription("Payment").
			WithCode("TX-CHAIN").
			WithExternalID("ext-1").
			WithMetadata(map[string]any{"ref": "123"})

		assert.Equal(t, "Payment", input.Description)
		assert.Equal(t, "TX-CHAIN", input.Code)
		assert.Equal(t, "ext-1", input.ExternalID)
		assert.Equal(t, map[string]any{"ref": "123"}, input.Metadata)
	})
}

func TestCreateTransactionInput_ToLibTransaction(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		var input *CreateTransactionInput

		result := input.ToLibTransaction()
		assert.Nil(t, result)
	})

	t.Run("basic input", func(t *testing.T) {
		input := &CreateTransactionInput{
			ChartOfAccountsGroupName: "ASSETS",
			Description:              "Test transaction",
			Pending:                  true,
			Route:                    "main-route",
			Metadata:                 map[string]any{"key": "value"},
		}
		result := input.ToLibTransaction()

		assert.Equal(t, "ASSETS", result["chartOfAccountsGroupName"])
		assert.Equal(t, "Test transaction", result["description"])
		assert.Equal(t, true, result["pending"])
		assert.Equal(t, "main-route", result["route"])
		assert.Equal(t, map[string]any{"key": "value"}, result["metadata"])
	})

	t.Run("with send structure", func(t *testing.T) {
		input := &CreateTransactionInput{
			Description: "With send",
			Send: &SendInput{
				Asset: "USD",
				Value: 100,
				Source: &SourceInput{
					From: []FromToInput{
						{Account: "source", Amount: AmountInput{Asset: "USD", Value: 100}},
					},
				},
				Distribute: &DistributeInput{
					To: []FromToInput{
						{Account: "dest", Amount: AmountInput{Asset: "USD", Value: 100}},
					},
				},
			},
		}
		result := input.ToLibTransaction()

		assert.NotNil(t, result["send"])
		sendMap := result["send"].(map[string]any)
		assert.Equal(t, "USD", sendMap["asset"])
		assert.Equal(t, "100", sendMap["value"])
	})

	t.Run("empty optional fields not included", func(t *testing.T) {
		input := &CreateTransactionInput{}
		result := input.ToLibTransaction()

		_, hasChartOfAccounts := result["chartOfAccountsGroupName"]
		_, hasDescription := result["description"]
		_, hasPending := result["pending"]
		_, hasRoute := result["route"]
		_, hasMetadata := result["metadata"]
		_, hasSend := result["send"]

		assert.False(t, hasChartOfAccounts)
		assert.False(t, hasDescription)
		assert.False(t, hasPending)
		assert.False(t, hasRoute)
		assert.False(t, hasMetadata)
		assert.False(t, hasSend)
	})

	t.Run("normalizes legacy operation amount wrappers", func(t *testing.T) {
		value := decimal.NewFromInt(50)
		input := &CreateTransactionInput{
			AssetCode: "USD",
			Amount:    "50",
			Operations: []CreateOperationInput{
				{Type: string(OperationTypeDebit), AccountID: "source", Amount: Amount{Value: &value}, AssetCode: "USD"},
				{Type: string(OperationTypeCredit), AccountID: "dest", Amount: &Amount{Value: &value}, AssetCode: "USD"},
			},
		}

		result := input.ToLibTransaction()
		send, ok := result["send"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "50", send["value"])

		source, ok := send["source"].(map[string]any)
		require.True(t, ok)
		from, ok := source["from"].([]map[string]any)
		require.True(t, ok)
		sourceAmount, ok := from[0]["amount"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "50", sourceAmount["value"])

		distribute, ok := send["distribute"].(map[string]any)
		require.True(t, ok)
		to, ok := distribute["to"].([]map[string]any)
		require.True(t, ok)
		destinationAmount, ok := to[0]["amount"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "50", destinationAmount["value"])
	})
}

// =============================================================================
// SendInput Tests
// =============================================================================

func TestSendInput_Validate(t *testing.T) {
	validSource := &SourceInput{
		From: []FromToInput{
			{Account: "source-acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	validDistribute := &DistributeInput{
		To: []FromToInput{
			{Account: "dest-acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	tests := []struct {
		name    string
		input   *SendInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid send",
			input: &SendInput{
				Asset:      "USD",
				Value:      100,
				Source:     validSource,
				Distribute: validDistribute,
			},
			wantErr: false,
		},
		{
			name: "missing asset",
			input: &SendInput{
				Asset:      "",
				Value:      100,
				Source:     validSource,
				Distribute: validDistribute,
			},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name: "missing value",
			input: &SendInput{
				Asset:      "USD",
				Value:      0,
				Source:     validSource,
				Distribute: validDistribute,
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "zero value",
			input: &SendInput{
				Asset:      "USD",
				Value:      0,
				Source:     validSource,
				Distribute: validDistribute,
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "missing source",
			input: &SendInput{
				Asset:      "USD",
				Value:      100,
				Source:     nil,
				Distribute: validDistribute,
			},
			wantErr: true,
			errMsg:  "source is required",
		},
		{
			name: "missing distribute",
			input: &SendInput{
				Asset:      "USD",
				Value:      100,
				Source:     validSource,
				Distribute: nil,
			},
			wantErr: true,
			errMsg:  "distribute is required",
		},
		{
			name: "invalid source",
			input: &SendInput{
				Asset: "USD",
				Value: 100,
				Source: &SourceInput{
					From: []FromToInput{},
				},
				Distribute: validDistribute,
			},
			wantErr: true,
			errMsg:  "from is required",
		},
		{
			name: "invalid distribute",
			input: &SendInput{
				Asset:  "USD",
				Value:  100,
				Source: validSource,
				Distribute: &DistributeInput{
					To: []FromToInput{},
				},
			},
			wantErr: true,
			errMsg:  "to is required",
		},
		{
			// Cross-asset money-path guard: a USD send whose distribute leg carries
			// a fixed BRL amount. sumFixedAmountEntries flags the mismatched asset and
			// appendSendBalanceErrors rejects it — the client must not ship a leg whose
			// asset disagrees with the send asset.
			name: "distribute leg asset mismatches send asset",
			input: &SendInput{
				Asset:  "USD",
				Value:  100,
				Source: validSource,
				Distribute: &DistributeInput{
					To: []FromToInput{
						{Account: "dest-acc", Amount: AmountInput{Asset: "BRL", Value: 100}},
					},
				},
			},
			wantErr: true,
			errMsg:  "amount assets must match send asset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSendInput_ToMap(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		var input *SendInput

		result := input.ToMap()
		assert.Nil(t, result)
	})

	t.Run("complete send", func(t *testing.T) {
		input := &SendInput{
			Asset: "USD",
			Value: 100,
			Source: &SourceInput{
				From: []FromToInput{
					{Account: "source", Amount: AmountInput{Asset: "USD", Value: 100}},
				},
			},
			Distribute: &DistributeInput{
				To: []FromToInput{
					{Account: "dest", Amount: AmountInput{Asset: "USD", Value: 100}},
				},
			},
		}
		result := input.ToMap()

		assert.Equal(t, "USD", result["asset"])
		assert.Equal(t, "100", result["value"])
		assert.NotNil(t, result["source"])
		assert.NotNil(t, result["distribute"])
	})
}

// =============================================================================
// SourceInput Tests
// =============================================================================

func TestSourceInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *SourceInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid source",
			input: &SourceInput{
				From: []FromToInput{
					{Account: "acc-1", Amount: AmountInput{Asset: "USD", Value: 100}},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple from entries",
			input: &SourceInput{
				From: []FromToInput{
					{Account: "acc-1", Amount: AmountInput{Asset: "USD", Value: 50}},
					{Account: "acc-2", Amount: AmountInput{Asset: "USD", Value: 50}},
				},
			},
			wantErr: false,
		},
		{
			name: "empty from list",
			input: &SourceInput{
				From: []FromToInput{},
			},
			wantErr: true,
			errMsg:  "from is required",
		},
		{
			name: "invalid from entry",
			input: &SourceInput{
				From: []FromToInput{
					{Account: "", Amount: AmountInput{Asset: "USD", Value: 100}},
				},
			},
			wantErr: true,
			errMsg:  "account is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSourceInput_ToMap(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		var input *SourceInput

		result := input.ToMap()
		assert.Nil(t, result)
	})

	t.Run("with from entries", func(t *testing.T) {
		input := &SourceInput{
			From: []FromToInput{
				{Account: "acc-1", Amount: AmountInput{Asset: "USD", Value: 100}},
			},
		}
		result := input.ToMap()

		fromList, ok := result["from"].([]map[string]any)
		require.True(t, ok)
		assert.Len(t, fromList, 1)
	})

	t.Run("empty from list", func(t *testing.T) {
		input := &SourceInput{
			From: []FromToInput{},
		}
		result := input.ToMap()

		_, hasFrom := result["from"]
		assert.False(t, hasFrom)
	})
}

// =============================================================================
// DistributeInput Tests
// =============================================================================

func TestDistributeInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *DistributeInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid distribute",
			input: &DistributeInput{
				To: []FromToInput{
					{Account: "acc-1", Amount: AmountInput{Asset: "USD", Value: 100}},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple to entries",
			input: &DistributeInput{
				To: []FromToInput{
					{Account: "acc-1", Amount: AmountInput{Asset: "USD", Value: 50}},
					{Account: "acc-2", Amount: AmountInput{Asset: "USD", Value: 50}},
				},
			},
			wantErr: false,
		},
		{
			name: "empty to list",
			input: &DistributeInput{
				To: []FromToInput{},
			},
			wantErr: true,
			errMsg:  "to is required",
		},
		{
			name: "invalid to entry",
			input: &DistributeInput{
				To: []FromToInput{
					{Account: "", Amount: AmountInput{Asset: "USD", Value: 100}},
				},
			},
			wantErr: true,
			errMsg:  "account is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDistributeInput_ToMap(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		var input *DistributeInput

		result := input.ToMap()
		assert.Nil(t, result)
	})

	t.Run("with to entries", func(t *testing.T) {
		input := &DistributeInput{
			To: []FromToInput{
				{Account: "acc-1", Amount: AmountInput{Asset: "USD", Value: 100}},
			},
		}
		result := input.ToMap()

		toList, ok := result["to"].([]map[string]any)
		require.True(t, ok)
		assert.Len(t, toList, 1)
	})
}

// =============================================================================
// FromToInput Tests
// =============================================================================

func TestFromToInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *FromToInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid from/to",
			input: &FromToInput{
				Account: "acc-123",
				Amount:  AmountInput{Asset: "USD", Value: 100},
			},
			wantErr: false,
		},
		{
			name: "with optional fields",
			input: &FromToInput{
				Account:         "acc-123",
				Amount:          AmountInput{Asset: "USD", Value: 100},
				Route:           "route-1",
				Description:     "Test description",
				ChartOfAccounts: "ASSETS",
				AccountAlias:    "main-account",
				Metadata:        map[string]any{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "missing account",
			input: &FromToInput{
				Account: "",
				Amount:  AmountInput{Asset: "USD", Value: 100},
			},
			wantErr: true,
			errMsg:  "account is required",
		},
		{
			name: "invalid amount - missing asset",
			input: &FromToInput{
				Account: "acc-123",
				Amount:  AmountInput{Asset: "", Value: 100},
			},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name: "invalid amount - missing value",
			input: &FromToInput{
				Account: "acc-123",
				Amount:  AmountInput{Asset: "USD", Value: 0},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name:    "valid share leg (no amount)",
			input:   &FromToInput{Account: "acc-123", Share: &Share{Percentage: 50}},
			wantErr: false,
		},
		{
			name:    "share percentage below 1",
			input:   &FromToInput{Account: "acc-123", Share: &Share{Percentage: 0}},
			wantErr: true,
			errMsg:  "share.percentage",
		},
		{
			name:    "share percentage above 100",
			input:   &FromToInput{Account: "acc-123", Share: &Share{Percentage: 101}},
			wantErr: true,
			errMsg:  "share.percentage",
		},
		{
			name:    "share percentage negative",
			input:   &FromToInput{Account: "acc-123", Share: &Share{Percentage: -1}},
			wantErr: true,
			errMsg:  "share.percentage",
		},
		{
			name:    "share percentageOfPercentage negative",
			input:   &FromToInput{Account: "acc-123", Share: &Share{Percentage: 50, PercentageOfPercentage: -1}},
			wantErr: true,
			errMsg:  "share.percentageOfPercentage",
		},
		{
			name:    "share percentageOfPercentage above 100",
			input:   &FromToInput{Account: "acc-123", Share: &Share{Percentage: 50, PercentageOfPercentage: 101}},
			wantErr: true,
			errMsg:  "share.percentageOfPercentage",
		},
		{
			name:    "no value mechanism",
			input:   &FromToInput{Account: "acc-123"},
			wantErr: true,
			errMsg:  "one of amount, share, remaining, or rate is required",
		},
		{
			name:    "amount combined with share rejected",
			input:   &FromToInput{Account: "acc-123", Amount: AmountInput{Asset: "USD", Value: 100}, Share: &Share{Percentage: 50}},
			wantErr: true,
			errMsg:  "cannot be combined with share, remaining, or rate",
		},
		{
			name:    "amount combined with remaining rejected",
			input:   &FromToInput{Account: "acc-123", Amount: AmountInput{Asset: "USD", Value: 100}, Remaining: "remaining"},
			wantErr: true,
			errMsg:  "cannot be combined with share, remaining, or rate",
		},
		{
			name:    "amount combined with rate rejected",
			input:   &FromToInput{Account: "acc-123", Amount: AmountInput{Asset: "USD", Value: 100}, Rate: &Rate{From: "USD", To: "BRL", Value: "5.00", ExternalID: "fx-1"}},
			wantErr: true,
			errMsg:  "cannot be combined with share, remaining, or rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFromToInput_ToMap(t *testing.T) {
	t.Run("basic input", func(t *testing.T) {
		input := FromToInput{
			Account: "acc-123",
			Amount:  AmountInput{Asset: "USD", Value: 100},
		}
		result := input.ToMap()

		assert.Equal(t, "acc-123", result["accountAlias"])
		assert.NotNil(t, result["amount"])
	})

	t.Run("with route", func(t *testing.T) {
		input := FromToInput{
			Account: "acc-123",
			Amount:  AmountInput{Asset: "USD", Value: 100},
			Route:   "main-route",
		}
		result := input.ToMap()

		assert.Equal(t, "main-route", result["route"])
	})

	t.Run("rate leg carries rate key", func(t *testing.T) {
		input := FromToInput{
			Account: "acc-123",
			Rate:    &Rate{From: "USD", To: "BRL", Value: "5.00", ExternalID: "fx-1"},
		}
		result := input.ToMap()

		assert.Equal(t, input.Rate, result["rate"], "a rate leg must serialize its rate on the wire")
		assert.Nil(t, result["amount"], "a rate leg must not ship an empty amount alongside it")
	})

	t.Run("remaining leg carries remaining key", func(t *testing.T) {
		input := FromToInput{
			Account:   "acc-123",
			Remaining: "remaining",
		}
		result := input.ToMap()

		assert.Equal(t, "remaining", result["remaining"], "a remaining leg must serialize its remaining token")
		assert.Nil(t, result["amount"], "a remaining leg must not ship an empty amount alongside it")
	})
}

// =============================================================================
// AmountInput Tests
// =============================================================================

func TestAmountInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *AmountInput
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid amount",
			input:   &AmountInput{Asset: "USD", Value: 100},
			wantErr: false,
		},
		{
			name:    "valid decimal amount",
			input:   &AmountInput{Asset: "USD", Value: 100.50},
			wantErr: false,
		},
		{
			name:    "missing asset",
			input:   &AmountInput{Asset: "", Value: 100},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name:    "missing value",
			input:   &AmountInput{Asset: "USD", Value: 0},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name:    "zero value",
			input:   &AmountInput{Asset: "USD", Value: 0},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAmountInput_ToMap(t *testing.T) {
	input := &AmountInput{Asset: "USD", Value: 100.50}
	result := input.ToMap()

	assert.Equal(t, "USD", result["asset"])
	assert.Equal(t, "100.5", result["value"])
}

// =============================================================================
// UpdateTransactionInput Tests
// =============================================================================

func TestNewUpdateTransactionInput(t *testing.T) {
	input := NewUpdateTransactionInput()
	assert.NotNil(t, input)
	assert.Nil(t, input.Metadata)
	assert.Empty(t, input.Description)
	assert.Empty(t, input.ExternalID)
}

func TestUpdateTransactionInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *UpdateTransactionInput
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty input is rejected",
			input:   &UpdateTransactionInput{},
			wantErr: true,
			errMsg:  "empty update payload not allowed",
		},
		{
			name: "valid with metadata",
			input: &UpdateTransactionInput{
				Metadata: map[string]any{"key": "value"},
			},
			wantErr: false,
		},
		{
			name: "valid with description",
			input: &UpdateTransactionInput{
				Description: "Updated description",
			},
			wantErr: false,
		},
		{
			// ExternalID is deprecated and intentionally excluded from
			// hasChanges, so an update payload that ONLY sets ExternalID
			// is treated as empty and rejected. Callers must combine it
			// with at least one of Metadata / Description.
			name: "external ID alone is rejected as empty payload",
			input: &UpdateTransactionInput{
				ExternalID: "ext-456",
			},
			wantErr: true,
			errMsg:  "empty update payload not allowed",
		},
		{
			name: "description too long",
			input: &UpdateTransactionInput{
				Description: strings.Repeat("a", 257),
			},
			wantErr: true,
			errMsg:  "description must not exceed 256 characters",
		},
		{
			// ExternalID is deprecated; even paired with another mutation
			// it is not validated for length. The presence of Description
			// is what makes the payload valid.
			name: "external ID is ignored for validation when paired with description",
			input: &UpdateTransactionInput{
				Description: "non-empty",
				ExternalID:  strings.Repeat("a", 65),
			},
			wantErr: false,
		},
		{
			name: "invalid metadata - empty key",
			input: &UpdateTransactionInput{
				Metadata: map[string]any{"": "value"},
			},
			wantErr: true,
			errMsg:  "metadata keys cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateTransactionInput_WithMethods(t *testing.T) {
	t.Run("WithMetadata", func(t *testing.T) {
		input := NewUpdateTransactionInput()
		metadata := map[string]any{"updated": true}
		result := input.WithMetadata(metadata)

		assert.Equal(t, metadata, result.Metadata)
		assert.Same(t, input, result)
	})

	t.Run("WithDescription", func(t *testing.T) {
		input := NewUpdateTransactionInput()
		result := input.WithDescription("New description")

		assert.Equal(t, "New description", result.Description)
		assert.Same(t, input, result)
	})

	t.Run("WithExternalID", func(t *testing.T) {
		input := NewUpdateTransactionInput()
		result := input.WithExternalID("new-ext-id")

		assert.Equal(t, "new-ext-id", result.ExternalID)
		assert.Same(t, input, result)
	})

	t.Run("chained methods", func(t *testing.T) {
		input := NewUpdateTransactionInput().
			WithDescription("Chained").
			WithExternalID("ext-chain").
			WithMetadata(map[string]any{"chain": true})

		assert.Equal(t, "Chained", input.Description)
		assert.Equal(t, "ext-chain", input.ExternalID)
		assert.Equal(t, map[string]any{"chain": true}, input.Metadata)
	})
}

// =============================================================================
// CreateInflowInput Tests
// =============================================================================

func TestNewCreateInflowInput(t *testing.T) {
	distribute := &DistributeInput{
		To: []FromToInput{
			{Account: "dest-acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	input := NewCreateInflowInput("USD", 100, distribute)

	assert.NotNil(t, input)
	assert.NotNil(t, input.Send)
	assert.Equal(t, "USD", input.Send.Asset)
	assert.Equal(t, "100", input.Send.Value)
	assert.Equal(t, distribute, input.Send.Distribute)
}

func TestCreateInflowInput_Validate(t *testing.T) {
	validDistribute := &DistributeInput{
		To: []FromToInput{
			{Account: "dest-acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	tests := []struct {
		name    string
		input   *CreateInflowInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid inflow",
			input: &CreateInflowInput{
				Send: &SendInflowInput{
					Asset:      "USD",
					Value:      100,
					Distribute: validDistribute,
				},
			},
			wantErr: false,
		},
		{
			name:    "missing send",
			input:   &CreateInflowInput{},
			wantErr: true,
			errMsg:  "send is required",
		},
		{
			name: "missing asset",
			input: &CreateInflowInput{
				Send: &SendInflowInput{
					Asset:      "",
					Value:      100,
					Distribute: validDistribute,
				},
			},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name: "missing value",
			input: &CreateInflowInput{
				Send: &SendInflowInput{
					Asset:      "USD",
					Value:      0,
					Distribute: validDistribute,
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "zero value",
			input: &CreateInflowInput{
				Send: &SendInflowInput{
					Asset:      "USD",
					Value:      0,
					Distribute: validDistribute,
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "missing distribute",
			input: &CreateInflowInput{
				Send: &SendInflowInput{
					Asset:      "USD",
					Value:      100,
					Distribute: nil,
				},
			},
			wantErr: true,
			errMsg:  "distribute.to is required",
		},
		{
			name: "empty distribute.to",
			input: &CreateInflowInput{
				Send: &SendInflowInput{
					Asset: "USD",
					Value: 100,
					Distribute: &DistributeInput{
						To: []FromToInput{},
					},
				},
			},
			wantErr: true,
			errMsg:  "distribute.to is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateInflowInput_WithMethods(t *testing.T) {
	distribute := &DistributeInput{
		To: []FromToInput{
			{Account: "acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	t.Run("WithDescription", func(t *testing.T) {
		input := NewCreateInflowInput("USD", 100, distribute)
		result := input.WithDescription("Deposit")

		assert.Equal(t, "Deposit", result.Description)
		assert.Same(t, input, result)
	})

	t.Run("WithCode", func(t *testing.T) {
		input := NewCreateInflowInput("USD", 100, distribute)
		result := input.WithCode("DEP-001")

		assert.Equal(t, "DEP-001", result.Code)
		assert.Same(t, input, result)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		input := NewCreateInflowInput("USD", 100, distribute)
		metadata := map[string]any{"source": "bank"}
		result := input.WithMetadata(metadata)

		assert.Equal(t, metadata, result.Metadata)
		assert.Same(t, input, result)
	})

	t.Run("WithChartOfAccountsGroupName", func(t *testing.T) {
		input := NewCreateInflowInput("USD", 100, distribute)
		result := input.WithChartOfAccountsGroupName("ASSETS")

		assert.Equal(t, "ASSETS", result.ChartOfAccountsGroupName)
		assert.Same(t, input, result)
	})

	t.Run("WithRoute", func(t *testing.T) {
		input := NewCreateInflowInput("USD", 100, distribute)
		result := input.WithRoute("deposit-route")

		assert.Equal(t, "deposit-route", result.Route)
		assert.Same(t, input, result)
	})

	t.Run("chained methods", func(t *testing.T) {
		input := NewCreateInflowInput("USD", 100, distribute).
			WithDescription("Test deposit").
			WithCode("DEP-002").
			WithChartOfAccountsGroupName("ASSETS").
			WithRoute("main-route").
			WithMetadata(map[string]any{"ref": "123"})

		assert.Equal(t, "Test deposit", input.Description)
		assert.Equal(t, "DEP-002", input.Code)
		assert.Equal(t, "ASSETS", input.ChartOfAccountsGroupName)
		assert.Equal(t, "main-route", input.Route)
		assert.Equal(t, map[string]any{"ref": "123"}, input.Metadata)
	})
}

// =============================================================================
// CreateOutflowInput Tests
// =============================================================================

func TestNewCreateOutflowInput(t *testing.T) {
	source := &SourceInput{
		From: []FromToInput{
			{Account: "source-acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	input := NewCreateOutflowInput("USD", 100, source)

	assert.NotNil(t, input)
	assert.NotNil(t, input.Send)
	assert.Equal(t, "USD", input.Send.Asset)
	assert.Equal(t, "100", input.Send.Value)
	assert.Equal(t, source, input.Send.Source)
}

func TestCreateOutflowInput_Validate(t *testing.T) {
	validSource := &SourceInput{
		From: []FromToInput{
			{Account: "source-acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	tests := []struct {
		name    string
		input   *CreateOutflowInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid outflow",
			input: &CreateOutflowInput{
				Send: &SendOutflowInput{
					Asset:  "USD",
					Value:  100,
					Source: validSource,
				},
			},
			wantErr: false,
		},
		{
			name:    "missing send",
			input:   &CreateOutflowInput{},
			wantErr: true,
			errMsg:  "send is required",
		},
		{
			name: "missing asset",
			input: &CreateOutflowInput{
				Send: &SendOutflowInput{
					Asset:  "",
					Value:  100,
					Source: validSource,
				},
			},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name: "missing value",
			input: &CreateOutflowInput{
				Send: &SendOutflowInput{
					Asset:  "USD",
					Value:  0,
					Source: validSource,
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "zero value",
			input: &CreateOutflowInput{
				Send: &SendOutflowInput{
					Asset:  "USD",
					Value:  0,
					Source: validSource,
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "missing source",
			input: &CreateOutflowInput{
				Send: &SendOutflowInput{
					Asset:  "USD",
					Value:  100,
					Source: nil,
				},
			},
			wantErr: true,
			errMsg:  "source.from is required",
		},
		{
			name: "empty source.from",
			input: &CreateOutflowInput{
				Send: &SendOutflowInput{
					Asset: "USD",
					Value: 100,
					Source: &SourceInput{
						From: []FromToInput{},
					},
				},
			},
			wantErr: true,
			errMsg:  "source.from is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateOutflowInput_WithMethods(t *testing.T) {
	source := &SourceInput{
		From: []FromToInput{
			{Account: "acc", Amount: AmountInput{Asset: "USD", Value: 100}},
		},
	}

	t.Run("WithDescription", func(t *testing.T) {
		input := NewCreateOutflowInput("USD", 100, source)
		result := input.WithDescription("Withdrawal")

		assert.Equal(t, "Withdrawal", result.Description)
		assert.Same(t, input, result)
	})

	t.Run("WithCode", func(t *testing.T) {
		input := NewCreateOutflowInput("USD", 100, source)
		result := input.WithCode("WTH-001")

		assert.Equal(t, "WTH-001", result.Code)
		assert.Same(t, input, result)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		input := NewCreateOutflowInput("USD", 100, source)
		metadata := map[string]any{"destination": "bank"}
		result := input.WithMetadata(metadata)

		assert.Equal(t, metadata, result.Metadata)
		assert.Same(t, input, result)
	})

	t.Run("WithChartOfAccountsGroupName", func(t *testing.T) {
		input := NewCreateOutflowInput("USD", 100, source)
		result := input.WithChartOfAccountsGroupName("LIABILITIES")

		assert.Equal(t, "LIABILITIES", result.ChartOfAccountsGroupName)
		assert.Same(t, input, result)
	})

	t.Run("WithRoute", func(t *testing.T) {
		input := NewCreateOutflowInput("USD", 100, source)
		result := input.WithRoute("withdrawal-route")

		assert.Equal(t, "withdrawal-route", result.Route)
		assert.Same(t, input, result)
	})

	t.Run("chained methods", func(t *testing.T) {
		input := NewCreateOutflowInput("USD", 100, source).
			WithDescription("Test withdrawal").
			WithCode("WTH-002").
			WithChartOfAccountsGroupName("LIABILITIES").
			WithRoute("main-route").
			WithMetadata(map[string]any{"ref": "456"})

		assert.Equal(t, "Test withdrawal", input.Description)
		assert.Equal(t, "WTH-002", input.Code)
		assert.Equal(t, "LIABILITIES", input.ChartOfAccountsGroupName)
		assert.Equal(t, "main-route", input.Route)
		assert.Equal(t, map[string]any{"ref": "456"}, input.Metadata)
	})
}

// =============================================================================
// CreateAnnotationInput Tests
// =============================================================================
