package models

import (
	"strings"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCreateAnnotationInput(t *testing.T) {
	input := NewCreateAnnotationInput("Test annotation", newValidSendInput(1))

	assert.NotNil(t, input)
	assert.Equal(t, "Test annotation", input.Description)
}

func TestNewCreateAnnotationInput_BackwardCompatibleWithoutSend(t *testing.T) {
	input := NewCreateAnnotationInput("Test annotation")

	assert.NotNil(t, input)
	assert.Equal(t, "Test annotation", input.Description)
	assert.Nil(t, input.Send)
}

func TestCreateTransactionInput_ValidateTransactionDate(t *testing.T) {
	validDates := []string{
		"2021-01-01T00:00:00Z",
		"2021-01-01T00:00:00.000Z",
		"2021-01-01T00:00:00.000000001Z",
	}

	for _, transactionDate := range validDates {
		t.Run(transactionDate, func(t *testing.T) {
			input := NewCreateTransactionInput("USD", 1).
				WithSend(newValidSendInput(1)).
				WithTransactionDate(transactionDate)

			require.NoError(t, input.Validate())
		})
	}

	t.Run("rejects unsupported space separated date", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 1).
			WithSend(newValidSendInput(1)).
			WithTransactionDate("2021-01-01 00:00:00")

		require.ErrorContains(t, input.Validate(), "transactionDate must be RFC3339")
	})

	t.Run("rejects timezone naive date", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 1).
			WithSend(newValidSendInput(1)).
			WithTransactionDate("2021-01-01")

		require.ErrorContains(t, input.Validate(), "transactionDate must be RFC3339")
	})

	t.Run("rejects future date", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 1).
			WithSend(newValidSendInput(1)).
			WithTransactionDate("2999-01-01T00:00:00Z")

		require.ErrorContains(t, input.Validate(), "transactionDate cannot be in the future")
	})

	t.Run("rejects pending custom date", func(t *testing.T) {
		input := NewCreateTransactionInput("USD", 1).
			WithSend(newValidSendInput(1)).
			WithPending(true).
			WithTransactionDate("2021-01-01T00:00:00Z")

		require.ErrorContains(t, input.Validate(), "pending transactions cannot have a custom transactionDate")
	})
}

func TestCreateAnnotationInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateAnnotationInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid annotation",
			input: &CreateAnnotationInput{
				Description: "Monthly reconciliation note",
				Send:        newValidSendInput(1),
			},
			wantErr: false,
		},
		{
			name: "with all optional fields",
			input: &CreateAnnotationInput{
				Description:              "Full annotation",
				ChartOfAccountsGroupName: "NOTES",
				Code:                     "ANN-001",
				Metadata:                 map[string]any{"author": "system"},
				Send:                     newValidSendInput(1),
			},
			wantErr: false,
		},
		{
			name: "missing description is allowed",
			input: &CreateAnnotationInput{
				Description: "",
				Send:        newValidSendInput(1),
			},
			wantErr: false,
		},
		{
			name: "send is optional for metadata-only annotations",
			input: &CreateAnnotationInput{
				Description: "Annotation-only note",
			},
			wantErr: false,
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

func TestCreateAnnotationInput_WithMethods(t *testing.T) {
	t.Run("WithCode", func(t *testing.T) {
		input := NewCreateAnnotationInput("Test", newValidSendInput(1))
		result := input.WithCode("ANN-001")

		assert.Equal(t, "ANN-001", result.Code)
		assert.Same(t, input, result)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		input := NewCreateAnnotationInput("Test", newValidSendInput(1))
		metadata := map[string]any{"note_type": "audit"}
		result := input.WithMetadata(metadata)

		assert.Equal(t, metadata, result.Metadata)
		assert.Same(t, input, result)
	})

	t.Run("WithChartOfAccountsGroupName", func(t *testing.T) {
		input := NewCreateAnnotationInput("Test", newValidSendInput(1))
		input.ChartOfAccountsGroupName = "ANNOTATIONS"
		result := input

		assert.Equal(t, "ANNOTATIONS", result.ChartOfAccountsGroupName)
		assert.Same(t, input, result)
	})

	t.Run("chained methods", func(t *testing.T) {
		input := NewCreateAnnotationInput("Audit note", newValidSendInput(1)).
			WithCode("AUD-001").
			WithMetadata(map[string]any{"auditor": "external"})
		input.ChartOfAccountsGroupName = "AUDIT"

		assert.Equal(t, "Audit note", input.Description)
		assert.Equal(t, "AUD-001", input.Code)
		assert.Equal(t, "AUDIT", input.ChartOfAccountsGroupName)
		assert.Equal(t, map[string]any{"auditor": "external"}, input.Metadata)
	})
}

// =============================================================================
// TransactionDSLInput Tests
// =============================================================================

func TestTransactionDSLInput_Validate(t *testing.T) {
	validSend := &DSLSend{
		Asset: "USD",
		Value: 100,
		Source: &DSLSource{
			From: []DSLFromTo{
				{Account: "source-acc"},
			},
		},
		Distribute: &DSLDistribute{
			To: []DSLFromTo{
				{Account: "dest-acc"},
			},
		},
	}

	tests := []struct {
		name    string
		input   *TransactionDSLInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid DSL input",
			input: &TransactionDSLInput{
				ChartOfAccountsGroupName: "TRANSFERS",
				Send:                     validSend,
			},
			wantErr: false,
		},
		{
			name: "with all optional fields",
			input: &TransactionDSLInput{
				ChartOfAccountsGroupName: "TRANSFERS",
				Description:              "Test transfer",
				Code:                     "TX_001",
				Metadata:                 map[string]any{"ref": "123"},
				Pending:                  true,
				Send:                     validSend,
			},
			wantErr: false,
		},
		{
			name:    "missing send",
			input:   &TransactionDSLInput{ChartOfAccountsGroupName: "TRANSFERS"},
			wantErr: true,
			errMsg:  "send is required",
		},
		{
			name:    "missing chart of accounts group name",
			input:   &TransactionDSLInput{Send: validSend},
			wantErr: true,
			errMsg:  "chartOfAccountsGroupName is required",
		},
		{
			name: "description too long",
			input: &TransactionDSLInput{
				ChartOfAccountsGroupName: "TRANSFERS",
				Description:              strings.Repeat("a", 257),
				Send:                     validSend,
			},
			wantErr: true,
			errMsg:  "description must be at most 256 characters",
		},
		{
			name: "chart of accounts group name too long",
			input: &TransactionDSLInput{
				ChartOfAccountsGroupName: strings.Repeat("a", 257),
				Send:                     validSend,
			},
			wantErr: true,
			errMsg:  "chartOfAccountsGroupName must be at most 256 characters",
		},
		{
			name: "invalid transaction code",
			input: &TransactionDSLInput{
				ChartOfAccountsGroupName: "TRANSFERS",
				Code:                     "invalid code with spaces!",
				Send:                     validSend,
			},
			wantErr: true,
			errMsg:  "invalid transaction code format",
		},
		{
			name: "invalid metadata",
			input: &TransactionDSLInput{
				ChartOfAccountsGroupName: "TRANSFERS",
				Metadata:                 map[string]any{"": "empty key"},
				Send:                     validSend,
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

// TestTransactionDSLInput_Validate_AccumulatesAllFieldErrors mirrors
// the Track 8C accumulation contract used across the rest of the
// models. When multiple fields are invalid, Validate must surface ALL
// of them in a single call rather than short-circuiting on the first.
func TestTransactionDSLInput_Validate_AccumulatesAllFieldErrors(t *testing.T) {
	input := &TransactionDSLInput{
		// chartOfAccountsGroupName missing — required
		Description: strings.Repeat("a", 257), // too long
		Code:        "invalid code with spaces!",
		Metadata:    map[string]any{"": "empty key"},
		// Send missing — required
	}

	err := input.Validate()
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "chartOfAccountsGroupName is required", "in: %q", msg)
	assert.Contains(t, msg, "send is required", "in: %q", msg)
	assert.Contains(t, msg, "description must be at most 256 characters", "in: %q", msg)
	assert.Contains(t, msg, "invalid transaction code format", "in: %q", msg)
	assert.Contains(t, msg, "metadata keys cannot be empty", "in: %q", msg)

	var fe *validation.FieldErrors
	require.ErrorAs(t, err, &fe)
	assert.GreaterOrEqual(t, fe.Len(), 5, "expected >=5 field errors, got %d: %s", fe.Len(), msg)
}

// TestDSLSend_Validate_AccumulatesAllFieldErrors locks in that the
// Send sub-validator collects every problem instead of returning on
// the first one.
func TestDSLSend_Validate_AccumulatesAllFieldErrors(t *testing.T) {
	send := &DSLSend{
		Asset:      "",  // required
		Value:      0,   // must be positive
		Source:     nil, // source.from missing
		Distribute: nil, // distribute.to missing
	}

	err := send.Validate()
	require.Error(t, err)

	msg := err.Error()
	assert.Contains(t, msg, "asset is required", "in: %q", msg)
	assert.Contains(t, msg, "value must be greater than zero", "in: %q", msg)
	assert.Contains(t, msg, "source.from must contain at least one entry", "in: %q", msg)
	assert.Contains(t, msg, "distribute.to must contain at least one entry", "in: %q", msg)

	var fe *validation.FieldErrors
	require.ErrorAs(t, err, &fe)
	assert.GreaterOrEqual(t, fe.Len(), 4, "expected >=4 field errors, got %d: %s", fe.Len(), msg)
}

func TestTransactionDSLInput_RenderDSL(t *testing.T) {
	validSend := &DSLSend{
		Asset: "USD",
		Value: 100,
		Source: &DSLSource{From: []DSLFromTo{{
			Account: "source-acc",
			Amount:  &DSLAmount{Asset: "USD", Value: 100},
		}}},
		Distribute: &DSLDistribute{To: []DSLFromTo{{
			Account: "dest-acc",
			Amount:  &DSLAmount{Asset: "USD", Value: 100},
		}}},
	}

	input := &TransactionDSLInput{
		ChartOfAccountsGroupName: "TRANSFERS",
		Send:                     validSend,
	}

	rendered, err := input.RenderDSL()
	require.NoError(t, err)
	assert.Equal(t, `(transaction V1 (chart-of-accounts-group-name TRANSFERS) (send USD 100|0 (source (from source-acc :amount USD 100|0)) (distribute (to dest-acc :amount USD 100|0))))`, string(rendered))
}

func TestTransactionDSLInput_RenderDSL_RejectsFractionalValues(t *testing.T) {
	input := &TransactionDSLInput{
		ChartOfAccountsGroupName: "TRANSFERS",
		Send: &DSLSend{
			Asset: "USD",
			Value: "100.50",
			Source: &DSLSource{From: []DSLFromTo{{
				Account: "source-acc",
				Amount:  &DSLAmount{Asset: "USD", Value: 100},
			}}},
			Distribute: &DSLDistribute{To: []DSLFromTo{{
				Account: "dest-acc",
				Amount:  &DSLAmount{Asset: "USD", Value: 100},
			}}},
		},
	}

	_, err := input.RenderDSL()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fractional DSL values are not supported")
}

func TestTransactionDSLInput_GetMethods(t *testing.T) {
	t.Run("GetAsset with nil Send", func(t *testing.T) {
		input := &TransactionDSLInput{}
		assert.Empty(t, input.GetAsset())
	})

	t.Run("GetAsset with Send", func(t *testing.T) {
		input := &TransactionDSLInput{
			Send: &DSLSend{Asset: "USD"},
		}
		assert.Equal(t, "USD", input.GetAsset())
	})

	t.Run("GetValue with nil Send", func(t *testing.T) {
		input := &TransactionDSLInput{}
		assert.Empty(t, input.GetValue())
	})

	t.Run("GetValue with valid Send", func(t *testing.T) {
		input := &TransactionDSLInput{
			Send: &DSLSend{Value: 100.50},
		}
		assert.Equal(t, "100.5", input.GetValue())
	})

	t.Run("GetValue with zero value", func(t *testing.T) {
		input := &TransactionDSLInput{
			Send: &DSLSend{Value: 0},
		}
		assert.Equal(t, "0", input.GetValue())
	})

	t.Run("GetSourceAccounts with nil Send", func(t *testing.T) {
		input := &TransactionDSLInput{}
		accounts := input.GetSourceAccounts()
		assert.Nil(t, accounts)
	})

	t.Run("GetSourceAccounts with accounts", func(t *testing.T) {
		input := &TransactionDSLInput{
			Send: &DSLSend{
				Source: &DSLSource{
					From: []DSLFromTo{
						{Account: "acc-1"},
						{Account: "acc-2"},
					},
				},
			},
		}
		accounts := input.GetSourceAccounts()
		assert.Len(t, accounts, 2)
		assert.Equal(t, "acc-1", accounts[0].GetAccount())
		assert.Equal(t, "acc-2", accounts[1].GetAccount())
	})

	t.Run("GetDestinationAccounts with nil Send", func(t *testing.T) {
		input := &TransactionDSLInput{}
		accounts := input.GetDestinationAccounts()
		assert.Nil(t, accounts)
	})

	t.Run("GetDestinationAccounts with accounts", func(t *testing.T) {
		input := &TransactionDSLInput{
			Send: &DSLSend{
				Distribute: &DSLDistribute{
					To: []DSLFromTo{
						{Account: "dest-1"},
						{Account: "dest-2"},
					},
				},
			},
		}
		accounts := input.GetDestinationAccounts()
		assert.Len(t, accounts, 2)
		assert.Equal(t, "dest-1", accounts[0].GetAccount())
		assert.Equal(t, "dest-2", accounts[1].GetAccount())
	})

	t.Run("GetMetadata", func(t *testing.T) {
		metadata := map[string]any{"key": "value"}
		input := &TransactionDSLInput{Metadata: metadata}
		assert.Equal(t, metadata, input.GetMetadata())
	})
}

func TestTransactionDSLInput_ToTransactionMap(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		var input *TransactionDSLInput

		result := input.ToTransactionMap()
		assert.Nil(t, result)
	})

	t.Run("complete input", func(t *testing.T) {
		input := &TransactionDSLInput{
			ChartOfAccountsGroupName: "TRANSFERS",
			Description:              "Test transfer",
			Code:                     "TX_001",
			Pending:                  true,
			Metadata:                 map[string]any{"ref": "123"},
			Send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					Remaining: "remaining-acc",
					From: []DSLFromTo{
						{Account: "source"},
					},
				},
				Distribute: &DSLDistribute{
					Remaining: "remaining-dest",
					To: []DSLFromTo{
						{Account: "dest"},
					},
				},
			},
		}
		result := input.ToTransactionMap()

		assert.Equal(t, "TRANSFERS", result["chartOfAccountsGroupName"])
		assert.Equal(t, "Test transfer", result["description"])
		assert.Equal(t, "TX_001", result["code"])
		assert.Equal(t, true, result["pending"])
		assert.Equal(t, map[string]any{"ref": "123"}, result["metadata"])
		assert.NotNil(t, result["send"])
	})

	t.Run("minimal input", func(t *testing.T) {
		input := &TransactionDSLInput{
			Description: "Minimal",
		}
		result := input.ToTransactionMap()

		assert.Equal(t, "Minimal", result["description"])
		_, hasChartOfAccounts := result["chartOfAccountsGroupName"]
		_, hasCode := result["code"]
		_, hasPending := result["pending"]

		assert.False(t, hasChartOfAccounts)
		assert.False(t, hasCode)
		assert.False(t, hasPending)
	})
}

// =============================================================================
// DSLSend Tests
// =============================================================================

func TestDSLSend_Validate(t *testing.T) {
	tests := []struct {
		name    string
		send    *DSLSend
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid send",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: false,
		},
		{
			name: "missing asset",
			send: &DSLSend{
				Asset: "",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "asset is required",
		},
		{
			name: "invalid asset format",
			send: &DSLSend{
				Asset: "invalid",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "invalid asset code format",
		},
		{
			name: "missing value",
			send: &DSLSend{
				Asset: "USD",
				Value: 0,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "zero value",
			send: &DSLSend{
				Asset: "USD",
				Value: 0,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "value must be greater than zero",
		},
		{
			name: "missing source",
			send: &DSLSend{
				Asset:  "USD",
				Value:  100,
				Source: nil,
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "source.from must contain at least one entry",
		},
		{
			name: "empty source from",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "source.from must contain at least one entry",
		},
		{
			name: "missing distribute",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: nil,
			},
			wantErr: true,
			errMsg:  "distribute.to must contain at least one entry",
		},
		{
			name: "empty distribute to",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{},
				},
			},
			wantErr: true,
			errMsg:  "distribute.to must contain at least one entry",
		},
		{
			name: "source from missing account",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: ""}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "source.from[0].account is required",
		},
		{
			name: "distribute to missing account",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "acc-1"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: ""}},
				},
			},
			wantErr: true,
			errMsg:  "distribute.to[0].account is required",
		},
		{
			name: "valid external account in source",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "@external/USD"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: false,
		},
		{
			name: "external account asset mismatch",
			send: &DSLSend{
				Asset: "USD",
				Value: 100,
				Source: &DSLSource{
					From: []DSLFromTo{{Account: "@external/EUR"}},
				},
				Distribute: &DSLDistribute{
					To: []DSLFromTo{{Account: "acc-2"}},
				},
			},
			wantErr: true,
			errMsg:  "asset code mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.send.Validate()
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

// =============================================================================
// DSLAccountRef Tests
// =============================================================================

func TestDSLAccountRef_GetAccount(t *testing.T) {
	ref := &DSLAccountRef{Account: "test-account"}
	assert.Equal(t, "test-account", ref.GetAccount())

	var nilRef *DSLAccountRef
	assert.Empty(t, nilRef.GetAccount())
}

func TestTransactionDSLInput_NilReceiverHelpers(t *testing.T) {
	var input *TransactionDSLInput

	assert.Empty(t, input.GetAsset())
	assert.Empty(t, input.GetValue())
	assert.Empty(t, input.GetSourceAccounts())
	assert.Empty(t, input.GetDestinationAccounts())
	assert.Nil(t, input.GetMetadata())
}

// =============================================================================
// FromTransactionMap Tests
// =============================================================================

func TestFromTransactionMap(t *testing.T) {
	t.Run("nil map returns nil", func(t *testing.T) {
		result := FromTransactionMap(nil)
		assert.Nil(t, result)
	})

	t.Run("empty map", func(t *testing.T) {
		result := FromTransactionMap(map[string]any{})
		assert.NotNil(t, result)
		assert.Empty(t, result.Description)
	})

	t.Run("complete map", func(t *testing.T) {
		data := map[string]any{
			"chartOfAccountsGroupName": "TRANSFERS",
			"description":              "Test transaction",
			"code":                     "TX_001",
			"pending":                  true,
			"metadata": map[string]any{
				"ref": "123",
			},
			"send": map[string]any{
				"asset": "USD",
				"value": "100.50",
				"source": map[string]any{
					"remaining": "rem-acc",
					"from": []any{
						map[string]any{
							"accountAlias": "source-acc",
							"remaining":    "rem-1",
							"description":  "Source description",
							"amount": map[string]any{
								"asset": "USD",
								"value": "100.50",
							},
							"share": map[string]any{
								"percentage":             float64(50),
								"percentageOfPercentage": float64(10),
							},
							"rate": map[string]any{
								"from":       "USD",
								"to":         "EUR",
								"value":      float64(0.85),
								"externalId": "rate-123",
							},
						},
					},
				},
				"distribute": map[string]any{
					"remaining": "dist-rem",
					"to": []any{
						map[string]any{
							"accountAlias": "dest-acc",
						},
					},
				},
			},
		}

		result := FromTransactionMap(data)

		assert.Equal(t, "TRANSFERS", result.ChartOfAccountsGroupName)
		assert.Equal(t, "Test transaction", result.Description)
		assert.Equal(t, "TX_001", result.Code)
		assert.True(t, result.Pending)
		assert.Equal(t, map[string]any{"ref": "123"}, result.Metadata)

		require.NotNil(t, result.Send)
		assert.Equal(t, "USD", result.Send.Asset)
		assert.Equal(t, "100.50", result.Send.Value)

		require.NotNil(t, result.Send.Source)
		assert.Equal(t, "rem-acc", result.Send.Source.Remaining)
		require.Len(t, result.Send.Source.From, 1)
		assert.Equal(t, "source-acc", result.Send.Source.From[0].Account)
		assert.Equal(t, "rem-1", result.Send.Source.From[0].Remaining)
		assert.Equal(t, "Source description", result.Send.Source.From[0].Description)

		require.NotNil(t, result.Send.Source.From[0].Amount)
		assert.Equal(t, "USD", result.Send.Source.From[0].Amount.Asset)
		assert.Equal(t, "100.50", result.Send.Source.From[0].Amount.Value)

		require.NotNil(t, result.Send.Source.From[0].Share)
		assert.Equal(t, int64(50), result.Send.Source.From[0].Share.Percentage)
		assert.Equal(t, int64(10), result.Send.Source.From[0].Share.PercentageOfPercentage)

		require.NotNil(t, result.Send.Source.From[0].Rate)
		assert.Equal(t, "USD", result.Send.Source.From[0].Rate.From)
		assert.Equal(t, "EUR", result.Send.Source.From[0].Rate.To)
		assert.Equal(t, "rate-123", result.Send.Source.From[0].Rate.ExternalID)

		require.NotNil(t, result.Send.Distribute)
		assert.Equal(t, "dist-rem", result.Send.Distribute.Remaining)
		require.Len(t, result.Send.Distribute.To, 1)
		assert.Equal(t, "dest-acc", result.Send.Distribute.To[0].Account)
	})

	t.Run("value as float64", func(t *testing.T) {
		data := map[string]any{
			"send": map[string]any{
				"asset": "USD",
				"value": float64(100.50),
			},
		}

		result := FromTransactionMap(data)
		require.NotNil(t, result.Send)
		assert.Equal(t, "100.5", result.Send.Value)
	})
}

// =============================================================================
// Transaction.ToTransactionMap Tests
// =============================================================================

func TestTransaction_ToTransactionMap(t *testing.T) {
	t.Run("nil transaction returns nil", func(t *testing.T) {
		var tx *Transaction

		result := tx.ToTransactionMap()
		assert.Nil(t, result)
	})

	t.Run("transaction with operations", func(t *testing.T) {
		val50 := newDecimal("50")
		tx := &Transaction{
			Description: "Test transaction",
			AssetCode:   "USD",
			Amount:      "100",
			Metadata:    map[string]any{"ref": "123"},
			Operations: []Operation{
				{AccountID: "acc-1", AccountAlias: "alias-1", Type: string(OperationTypeDebit), Amount: Amount{Value: &val50}, AssetCode: "USD"},
				{AccountID: "acc-2", AccountAlias: "alias-2", Type: string(OperationTypeCredit), Amount: Amount{Value: &val50}, AssetCode: "USD"},
			},
		}

		result := tx.ToTransactionMap()

		assert.Equal(t, "Test transaction", result["description"])
		assert.Equal(t, map[string]any{"ref": "123"}, result["metadata"])

		send, ok := result["send"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "USD", send["asset"])
		assert.Equal(t, "100", send["value"])

		source, ok := send["source"].(map[string]any)
		require.True(t, ok)
		fromList, ok := source["from"].([]map[string]any)
		require.True(t, ok)
		assert.Len(t, fromList, 1)
		assert.Equal(t, "alias-1", fromList[0]["accountAlias"])
		assert.NotContains(t, fromList[0], "account")

		distribute, ok := send["distribute"].(map[string]any)
		require.True(t, ok)
		toList, ok := distribute["to"].([]map[string]any)
		require.True(t, ok)
		assert.Len(t, toList, 1)
		assert.Equal(t, "alias-2", toList[0]["accountAlias"])
		assert.NotContains(t, toList[0], "account")
	})

	// Regression guard for the C1 third-rail bug: ToTransactionMap previously
	// compared op.Type against lowercase "debit" while the Midaz API returns
	// canonical uppercase "DEBIT"/"CREDIT" (per OperationType constants and
	// Operation.Validate). Under the bug, every real DEBIT silently fell into
	// the credit branch — inverting double-entry direction on every transaction.
	//
	// This test explicitly asserts:
	//   - canonical uppercase "DEBIT" routes to source.from
	//   - canonical uppercase "CREDIT" routes to distribute.to
	//   - mixed casing is tolerated (defense in depth via strings.EqualFold)
	t.Run("routes operations by canonical uppercase type", func(t *testing.T) {
		val := newDecimal("100")
		tx := &Transaction{
			AssetCode: "USD",
			Amount:    "100",
			Operations: []Operation{
				{AccountID: "src", AccountAlias: "src-alias", Type: string(OperationTypeDebit), Amount: Amount{Value: &val}, AssetCode: "USD"},
				{AccountID: "dst", AccountAlias: "dst-alias", Type: string(OperationTypeCredit), Amount: Amount{Value: &val}, AssetCode: "USD"},
			},
		}

		result := tx.ToTransactionMap()

		send, ok := result["send"].(map[string]any)
		require.True(t, ok, "send map must be present")

		source, ok := send["source"].(map[string]any)
		require.True(t, ok, "source map must be present (DEBIT must produce source.from)")
		fromEntries, ok := source["from"].([]map[string]any)
		require.True(t, ok, "source.from must be present")
		require.Len(t, fromEntries, 1, "DEBIT op must land in source.from")
		assert.Equal(t, "src-alias", fromEntries[0]["accountAlias"], "the DEBIT op routed to from")

		distribute, ok := send["distribute"].(map[string]any)
		require.True(t, ok, "distribute map must be present (CREDIT must produce distribute.to)")
		toEntries, ok := distribute["to"].([]map[string]any)
		require.True(t, ok, "distribute.to must be present")
		require.Len(t, toEntries, 1, "CREDIT op must land in distribute.to")
		assert.Equal(t, "dst-alias", toEntries[0]["accountAlias"], "the CREDIT op should land in to-entries")
	})

	t.Run("tolerates mixed-case operation types", func(t *testing.T) {
		val := newDecimal("10")
		tx := &Transaction{
			AssetCode: "USD",
			Amount:    "10",
			Operations: []Operation{
				{AccountID: "src", AccountAlias: "src-alias", Type: "Debit", Amount: Amount{Value: &val}, AssetCode: "USD"},
				{AccountID: "dst", AccountAlias: "dst-alias", Type: "credit", Amount: Amount{Value: &val}, AssetCode: "USD"},
			},
		}

		result := tx.ToTransactionMap()

		send, ok := result["send"].(map[string]any)
		require.True(t, ok)
		source, ok := send["source"].(map[string]any)
		require.True(t, ok)
		fromEntries, ok := source["from"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, fromEntries, 1, "mixed-case 'Debit' must still route to source.from")

		distribute, ok := send["distribute"].(map[string]any)
		require.True(t, ok)
		toEntries, ok := distribute["to"].([]map[string]any)
		require.True(t, ok)
		require.Len(t, toEntries, 1, "lowercase 'credit' must still route to distribute.to")
	})
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestGetStringFromMap(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected string
	}{
		{
			name:     "existing string key",
			m:        map[string]any{"key": "value"},
			key:      "key",
			expected: "value",
		},
		{
			name:     "missing key",
			m:        map[string]any{},
			key:      "key",
			expected: "",
		},
		{
			name:     "non-string value",
			m:        map[string]any{"key": 123},
			key:      "key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringFromMap(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetMetadataFromMap(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		expected map[string]any
	}{
		{
			name:     "existing metadata",
			m:        map[string]any{"metadata": map[string]any{"key": "value"}},
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "missing metadata",
			m:        map[string]any{},
			expected: nil,
		},
		{
			name:     "wrong type",
			m:        map[string]any{"metadata": "not a map"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMetadataFromMap(tt.m)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// fromToToMap Tests
// =============================================================================

func TestFromToToMap(t *testing.T) {
	t.Run("basic from/to", func(t *testing.T) {
		from := DSLFromTo{
			Account: "acc-123",
		}
		result := fromToToMap(from)

		assert.Equal(t, "acc-123", result["accountAlias"])
		assert.NotContains(t, result, "account")
	})

	t.Run("with amount", func(t *testing.T) {
		from := DSLFromTo{
			Account: "acc-123",
			Amount: &DSLAmount{
				Asset: "USD",
				Value: 100,
			},
		}
		result := fromToToMap(from)

		amountMap, ok := result["amount"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "USD", amountMap["asset"])
		assert.Equal(t, "100", amountMap["value"])
	})

	t.Run("with share", func(t *testing.T) {
		from := DSLFromTo{
			Account: "acc-123",
			Share: &Share{
				Percentage:             50,
				PercentageOfPercentage: 10,
			},
		}
		result := fromToToMap(from)

		shareMap, ok := result["share"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, int64(50), shareMap["percentage"])
		assert.Equal(t, int64(10), shareMap["percentageOfPercentage"])
	})

	t.Run("with rate", func(t *testing.T) {
		from := DSLFromTo{
			Account: "acc-123",
			Rate: &Rate{
				From:       "USD",
				To:         "EUR",
				Value:      0.85,
				ExternalID: "rate-1",
			},
		}
		result := fromToToMap(from)

		rateMap, ok := result["rate"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "USD", rateMap["from"])
		assert.Equal(t, "EUR", rateMap["to"])
		assert.Equal(t, "0.85", rateMap["value"])
		assert.Equal(t, "rate-1", rateMap["externalId"])
	})

	t.Run("with all optional fields", func(t *testing.T) {
		from := DSLFromTo{
			Account:         "acc-123",
			Remaining:       "rem-acc",
			Description:     "Test description",
			ChartOfAccounts: "ASSETS",
			Metadata:        map[string]any{"key": "value"},
		}
		result := fromToToMap(from)

		assert.Equal(t, "rem-acc", result["remaining"])
		assert.Equal(t, "Test description", result["description"])
		assert.Equal(t, "ASSETS", result["chartOfAccounts"])
		assert.Equal(t, map[string]any{"key": "value"}, result["metadata"])
	})
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestEdgeCases(t *testing.T) {
	t.Run("empty strings in CreateTransactionInput", func(t *testing.T) {
		input := &CreateTransactionInput{}
		err := input.Validate()
		require.Error(t, err)
	})

	t.Run("nil metadata", func(t *testing.T) {
		input := NewUpdateTransactionInput().WithMetadata(nil)
		err := input.Validate()
		require.ErrorContains(t, err, "empty update payload not allowed")
	})

	t.Run("empty metadata map", func(t *testing.T) {
		input := NewUpdateTransactionInput().WithMetadata(map[string]any{})
		err := input.Validate()
		require.NoError(t, err)
	})

	t.Run("deeply nested metadata", func(t *testing.T) {
		metadata := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": "value",
				},
			},
		}
		input := NewUpdateTransactionInput().WithMetadata(metadata)
		err := input.Validate()
		require.Error(t, err)
	})

	t.Run("metadata with various types", func(t *testing.T) {
		metadata := map[string]any{
			"string": "value",
			"int":    42,
			"float":  3.14,
			"bool":   true,
			"null":   nil,
		}
		input := NewUpdateTransactionInput().WithMetadata(metadata)
		err := input.Validate()
		require.NoError(t, err)
	})

	t.Run("boundary values for description length", func(t *testing.T) {
		exactLength := strings.Repeat("a", 256)
		input := NewUpdateTransactionInput().WithDescription(exactLength)
		err := input.Validate()
		require.NoError(t, err)

		overLength := strings.Repeat("a", 257)
		input2 := NewUpdateTransactionInput().WithDescription(overLength)
		err2 := input2.Validate()
		require.Error(t, err2)
	})

	t.Run("boundary values for external ID length", func(t *testing.T) {
		// ExternalID is deprecated and excluded from change-detection. An
		// update payload that ONLY sets ExternalID is treated as empty
		// regardless of length. Tests must combine ExternalID with a
		// real mutation to exercise the validator at all.
		exactLength := strings.Repeat("a", 64)
		input := NewUpdateTransactionInput().
			WithExternalID(exactLength).
			WithDescription("non-empty")
		err := input.Validate()
		require.NoError(t, err)

		overLength := strings.Repeat("a", 65)
		input2 := NewUpdateTransactionInput().
			WithExternalID(overLength).
			WithDescription("non-empty")
		err2 := input2.Validate()
		require.NoError(t, err2)
	})
}

// =============================================================================
// Operation Validation Tests (within CreateTransactionInput)
// =============================================================================

func TestCreateTransactionInput_SendValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   *CreateTransactionInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid send",
			input: &CreateTransactionInput{
				Send: newValidSendInput(100),
			},
			wantErr: false,
		},
		{
			name: "invalid send - missing account",
			input: &CreateTransactionInput{
				Send: &SendInput{
					Asset: "USD",
					Value: 100,
					Source: &SourceInput{From: []FromToInput{{
						Account: "",
						Amount:  AmountInput{Asset: "USD", Value: 100},
					}}},
					Distribute: &DistributeInput{To: []FromToInput{{
						Account: "dest",
						Amount:  AmountInput{Asset: "USD", Value: 100},
					}}},
				},
			},
			wantErr: true,
			// 8C: format is "send invalid: <inner>" via accumulator.
			errMsg: "send invalid",
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
