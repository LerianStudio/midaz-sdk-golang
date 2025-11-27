package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreateAssetInput(t *testing.T) {
	tests := []struct {
		name         string
		assetName    string
		assetCode    string
		wantName     string
		wantCode     string
		wantType     string
		wantMetadata map[string]any
	}{
		{
			name:         "creates input with name and code",
			assetName:    "US Dollar",
			assetCode:    "USD",
			wantName:     "US Dollar",
			wantCode:     "USD",
			wantType:     "",
			wantMetadata: nil,
		},
		{
			name:         "creates input with empty values",
			assetName:    "",
			assetCode:    "",
			wantName:     "",
			wantCode:     "",
			wantType:     "",
			wantMetadata: nil,
		},
		{
			name:         "creates input with long name",
			assetName:    "Very Long Asset Name That Describes The Asset In Great Detail",
			assetCode:    "LONGCODE",
			wantName:     "Very Long Asset Name That Describes The Asset In Great Detail",
			wantCode:     "LONGCODE",
			wantType:     "",
			wantMetadata: nil,
		},
		{
			name:         "creates input with special characters in name",
			assetName:    "Brazilian Real (BRL)",
			assetCode:    "BRL",
			wantName:     "Brazilian Real (BRL)",
			wantCode:     "BRL",
			wantType:     "",
			wantMetadata: nil,
		},
		{
			name:         "creates input with unicode characters",
			assetName:    "Yen",
			assetCode:    "JPY",
			wantName:     "Yen",
			wantCode:     "JPY",
			wantType:     "",
			wantMetadata: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput(tt.assetName, tt.assetCode)

			assert.NotNil(t, input)
			assert.Equal(t, tt.wantName, input.Name)
			assert.Equal(t, tt.wantCode, input.Code)
			assert.Equal(t, tt.wantType, input.Type)
			assert.Nil(t, input.Metadata)
		})
	}
}

func TestCreateAssetInput_WithType(t *testing.T) {
	tests := []struct {
		name      string
		assetType string
		wantType  string
	}{
		{
			name:      "sets currency type",
			assetType: "currency",
			wantType:  "currency",
		},
		{
			name:      "sets cryptocurrency type",
			assetType: "cryptocurrency",
			wantType:  "cryptocurrency",
		},
		{
			name:      "sets commodity type",
			assetType: "commodity",
			wantType:  "commodity",
		},
		{
			name:      "sets stock type",
			assetType: "stock",
			wantType:  "stock",
		},
		{
			name:      "sets empty type",
			assetType: "",
			wantType:  "",
		},
		{
			name:      "sets custom type",
			assetType: "custom_asset_type",
			wantType:  "custom_asset_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", "TST")
			result := input.WithType(tt.assetType)

			assert.Same(t, input, result, "WithType should return same pointer for chaining")
			assert.Equal(t, tt.wantType, input.Type)
		})
	}
}

func TestCreateAssetInput_WithStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		wantStatus Status
	}{
		{
			name:       "sets active status",
			status:     NewStatus("ACTIVE"),
			wantStatus: Status{Code: "ACTIVE"},
		},
		{
			name:       "sets inactive status",
			status:     NewStatus("INACTIVE"),
			wantStatus: Status{Code: "INACTIVE"},
		},
		{
			name:       "sets pending status",
			status:     NewStatus("PENDING"),
			wantStatus: Status{Code: "PENDING"},
		},
		{
			name:       "sets empty status",
			status:     NewStatus(""),
			wantStatus: Status{Code: ""},
		},
		{
			name:       "sets status with description",
			status:     WithStatusDescription(NewStatus("ACTIVE"), "Asset is active"),
			wantStatus: Status{Code: "ACTIVE", Description: stringPtr("Asset is active")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", "TST")
			result := input.WithStatus(tt.status)

			assert.Same(t, input, result, "WithStatus should return same pointer for chaining")
			assert.Equal(t, tt.wantStatus.Code, input.Status.Code)
			if tt.wantStatus.Description != nil {
				assert.NotNil(t, input.Status.Description)
				assert.Equal(t, *tt.wantStatus.Description, *input.Status.Description)
			}
		})
	}
}

func TestCreateAssetInput_WithMetadata(t *testing.T) {
	tests := []struct {
		name         string
		metadata     map[string]any
		wantMetadata map[string]any
	}{
		{
			name:         "sets nil metadata",
			metadata:     nil,
			wantMetadata: nil,
		},
		{
			name:         "sets empty metadata",
			metadata:     map[string]any{},
			wantMetadata: map[string]any{},
		},
		{
			name: "sets single key metadata",
			metadata: map[string]any{
				"region": "US",
			},
			wantMetadata: map[string]any{
				"region": "US",
			},
		},
		{
			name: "sets multiple keys metadata",
			metadata: map[string]any{
				"region":   "US",
				"category": "fiat",
				"priority": 1,
			},
			wantMetadata: map[string]any{
				"region":   "US",
				"category": "fiat",
				"priority": 1,
			},
		},
		{
			name: "sets metadata with different value types",
			metadata: map[string]any{
				"string_val": "test",
				"int_val":    42,
				"float_val":  3.14,
				"bool_val":   true,
				"array_val":  []string{"a", "b", "c"},
				"map_val":    map[string]string{"nested": "value"},
			},
			wantMetadata: map[string]any{
				"string_val": "test",
				"int_val":    42,
				"float_val":  3.14,
				"bool_val":   true,
				"array_val":  []string{"a", "b", "c"},
				"map_val":    map[string]string{"nested": "value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", "TST")
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result, "WithMetadata should return same pointer for chaining")
			assert.Equal(t, tt.wantMetadata, input.Metadata)
		})
	}
}

func TestCreateAssetInput_Validate(t *testing.T) {
	tests := []struct {
		name      string
		input     *CreateAssetInput
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid input with name and code",
			input:     NewCreateAssetInput("US Dollar", "USD"),
			wantError: false,
		},
		{
			name:      "valid input with all fields",
			input:     NewCreateAssetInput("US Dollar", "USD").WithType("currency").WithStatus(NewStatus("ACTIVE")),
			wantError: false,
		},
		{
			name:      "missing name",
			input:     NewCreateAssetInput("", "USD"),
			wantError: true,
			errorMsg:  "name is required",
		},
		{
			name:      "missing code",
			input:     NewCreateAssetInput("US Dollar", ""),
			wantError: true,
			errorMsg:  "code is required",
		},
		{
			name:      "missing both name and code",
			input:     NewCreateAssetInput("", ""),
			wantError: true,
			errorMsg:  "name is required",
		},
		{
			name:      "whitespace only name",
			input:     NewCreateAssetInput("   ", "USD"),
			wantError: false, // Current implementation doesn't trim whitespace
		},
		{
			name:      "whitespace only code",
			input:     NewCreateAssetInput("US Dollar", "   "),
			wantError: false, // Current implementation doesn't trim whitespace
		},
		{
			name:      "name with leading/trailing spaces",
			input:     NewCreateAssetInput(" US Dollar ", "USD"),
			wantError: false,
		},
		{
			name:      "code with leading/trailing spaces",
			input:     NewCreateAssetInput("US Dollar", " USD "),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.wantError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateAssetInput_MethodChaining(t *testing.T) {
	metadata := map[string]any{
		"region":   "US",
		"category": "fiat",
	}

	input := NewCreateAssetInput("US Dollar", "USD").
		WithType("currency").
		WithStatus(NewStatus("ACTIVE")).
		WithMetadata(metadata)

	assert.Equal(t, "US Dollar", input.Name)
	assert.Equal(t, "USD", input.Code)
	assert.Equal(t, "currency", input.Type)
	assert.Equal(t, "ACTIVE", input.Status.Code)
	assert.Equal(t, metadata, input.Metadata)
}

func TestNewUpdateAssetInput(t *testing.T) {
	input := NewUpdateAssetInput()

	assert.NotNil(t, input)
	assert.Empty(t, input.Name)
	assert.True(t, IsStatusEmpty(input.Status))
	assert.Nil(t, input.Metadata)
}

func TestUpdateAssetInput_WithName(t *testing.T) {
	tests := []struct {
		name     string
		newName  string
		wantName string
	}{
		{
			name:     "sets new name",
			newName:  "Updated Asset Name",
			wantName: "Updated Asset Name",
		},
		{
			name:     "sets empty name",
			newName:  "",
			wantName: "",
		},
		{
			name:     "sets name with special characters",
			newName:  "Asset Name (Updated)",
			wantName: "Asset Name (Updated)",
		},
		{
			name:     "sets name with unicode",
			newName:  "Updated Asset",
			wantName: "Updated Asset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateAssetInput()
			result := input.WithName(tt.newName)

			assert.Same(t, input, result, "WithName should return same pointer for chaining")
			assert.Equal(t, tt.wantName, input.Name)
		})
	}
}

func TestUpdateAssetInput_WithStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		wantStatus Status
	}{
		{
			name:       "sets active status",
			status:     NewStatus("ACTIVE"),
			wantStatus: Status{Code: "ACTIVE"},
		},
		{
			name:       "sets inactive status",
			status:     NewStatus("INACTIVE"),
			wantStatus: Status{Code: "INACTIVE"},
		},
		{
			name:       "sets empty status",
			status:     NewStatus(""),
			wantStatus: Status{Code: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateAssetInput()
			result := input.WithStatus(tt.status)

			assert.Same(t, input, result, "WithStatus should return same pointer for chaining")
			assert.Equal(t, tt.wantStatus.Code, input.Status.Code)
		})
	}
}

func TestUpdateAssetInput_WithMetadata(t *testing.T) {
	tests := []struct {
		name         string
		metadata     map[string]any
		wantMetadata map[string]any
	}{
		{
			name:         "sets nil metadata",
			metadata:     nil,
			wantMetadata: nil,
		},
		{
			name:         "sets empty metadata",
			metadata:     map[string]any{},
			wantMetadata: map[string]any{},
		},
		{
			name: "sets metadata with values",
			metadata: map[string]any{
				"updated": true,
				"version": 2,
			},
			wantMetadata: map[string]any{
				"updated": true,
				"version": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateAssetInput()
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result, "WithMetadata should return same pointer for chaining")
			assert.Equal(t, tt.wantMetadata, input.Metadata)
		})
	}
}

func TestUpdateAssetInput_Validate(t *testing.T) {
	tests := []struct {
		name      string
		input     *UpdateAssetInput
		wantError bool
	}{
		{
			name:      "empty update input is valid",
			input:     NewUpdateAssetInput(),
			wantError: false,
		},
		{
			name:      "update with name only is valid",
			input:     NewUpdateAssetInput().WithName("New Name"),
			wantError: false,
		},
		{
			name:      "update with status only is valid",
			input:     NewUpdateAssetInput().WithStatus(NewStatus("INACTIVE")),
			wantError: false,
		},
		{
			name:      "update with metadata only is valid",
			input:     NewUpdateAssetInput().WithMetadata(map[string]any{"key": "value"}),
			wantError: false,
		},
		{
			name: "update with all fields is valid",
			input: NewUpdateAssetInput().
				WithName("Updated Name").
				WithStatus(NewStatus("ACTIVE")).
				WithMetadata(map[string]any{"updated": true}),
			wantError: false,
		},
		{
			name:      "update with empty name is valid",
			input:     NewUpdateAssetInput().WithName(""),
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateAssetInput_MethodChaining(t *testing.T) {
	metadata := map[string]any{
		"updated":   true,
		"updatedBy": "admin",
	}

	input := NewUpdateAssetInput().
		WithName("Updated Asset Name").
		WithStatus(NewStatus("INACTIVE")).
		WithMetadata(metadata)

	assert.Equal(t, "Updated Asset Name", input.Name)
	assert.Equal(t, "INACTIVE", input.Status.Code)
	assert.Equal(t, metadata, input.Metadata)
}

func TestCreateAssetInput_AssetCodes(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		wantValid bool
	}{
		{
			name:      "standard currency code USD",
			code:      "USD",
			wantValid: true,
		},
		{
			name:      "standard currency code EUR",
			code:      "EUR",
			wantValid: true,
		},
		{
			name:      "standard currency code BRL",
			code:      "BRL",
			wantValid: true,
		},
		{
			name:      "cryptocurrency code BTC",
			code:      "BTC",
			wantValid: true,
		},
		{
			name:      "cryptocurrency code ETH",
			code:      "ETH",
			wantValid: true,
		},
		{
			name:      "lowercase code",
			code:      "usd",
			wantValid: true, // Current implementation accepts lowercase
		},
		{
			name:      "mixed case code",
			code:      "UsD",
			wantValid: true, // Current implementation accepts mixed case
		},
		{
			name:      "numeric code",
			code:      "840",
			wantValid: true, // ISO 4217 numeric code
		},
		{
			name:      "alphanumeric code",
			code:      "USD1",
			wantValid: true,
		},
		{
			name:      "long code",
			code:      "VERYLONGASSETCODE",
			wantValid: true,
		},
		{
			name:      "single character code",
			code:      "X",
			wantValid: true,
		},
		{
			name:      "code with underscore",
			code:      "USD_TEST",
			wantValid: true, // Current implementation accepts underscores
		},
		{
			name:      "code with hyphen",
			code:      "USD-TEST",
			wantValid: true, // Current implementation accepts hyphens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", tt.code)
			err := input.Validate()

			if tt.wantValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestCreateAssetInput_AssetNames(t *testing.T) {
	tests := []struct {
		name      string
		assetName string
		wantValid bool
	}{
		{
			name:      "standard asset name",
			assetName: "US Dollar",
			wantValid: true,
		},
		{
			name:      "asset name with numbers",
			assetName: "Asset Type 1",
			wantValid: true,
		},
		{
			name:      "asset name with special chars",
			assetName: "Asset (Type)",
			wantValid: true,
		},
		{
			name:      "asset name with unicode",
			assetName: "Yen Currency",
			wantValid: true,
		},
		{
			name:      "very long asset name",
			assetName: "This is a very long asset name that describes the asset in great detail and might exceed reasonable limits",
			wantValid: true,
		},
		{
			name:      "single character name",
			assetName: "A",
			wantValid: true,
		},
		{
			name:      "numeric name",
			assetName: "123",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput(tt.assetName, "TST")
			err := input.Validate()

			if tt.wantValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestCreateAssetInput_MetadataEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "nil metadata",
			metadata: nil,
		},
		{
			name:     "empty metadata map",
			metadata: map[string]any{},
		},
		{
			name: "metadata with empty string key",
			metadata: map[string]any{
				"": "empty key",
			},
		},
		{
			name: "metadata with empty string value",
			metadata: map[string]any{
				"key": "",
			},
		},
		{
			name: "metadata with nil value",
			metadata: map[string]any{
				"key": nil,
			},
		},
		{
			name: "metadata with nested map",
			metadata: map[string]any{
				"nested": map[string]any{
					"level2": map[string]any{
						"level3": "deep value",
					},
				},
			},
		},
		{
			name: "metadata with array",
			metadata: map[string]any{
				"tags": []string{"tag1", "tag2", "tag3"},
			},
		},
		{
			name: "metadata with mixed types",
			metadata: map[string]any{
				"string": "value",
				"int":    123,
				"float":  1.23,
				"bool":   true,
				"null":   nil,
			},
		},
		{
			name: "metadata with special characters in key",
			metadata: map[string]any{
				"key-with-dash":       "value1",
				"key_with_underscore": "value2",
				"key.with.dots":       "value3",
			},
		},
		{
			name: "metadata with unicode key",
			metadata: map[string]any{
				"key": "unicode value",
			},
		},
		{
			name: "metadata with long key",
			metadata: map[string]any{
				"this_is_a_very_long_metadata_key_that_might_exceed_limits": "value",
			},
		},
		{
			name: "metadata with long value",
			metadata: map[string]any{
				"key": "This is a very long metadata value that describes something in great detail and might be used for various purposes in the system such as storing additional information about the asset",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", "TST").WithMetadata(tt.metadata)
			assert.Equal(t, tt.metadata, input.Metadata)

			err := input.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestUpdateAssetInput_MetadataEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "nil metadata clears existing",
			metadata: nil,
		},
		{
			name:     "empty metadata",
			metadata: map[string]any{},
		},
		{
			name: "metadata with complex nested structure",
			metadata: map[string]any{
				"config": map[string]any{
					"settings": map[string]any{
						"enabled": true,
						"options": []int{1, 2, 3},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateAssetInput().WithMetadata(tt.metadata)
			assert.Equal(t, tt.metadata, input.Metadata)

			err := input.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestCreateAssetInput_StatusEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		status Status
	}{
		{
			name:   "empty status code",
			status: NewStatus(""),
		},
		{
			name:   "lowercase status code",
			status: NewStatus("active"),
		},
		{
			name:   "mixed case status code",
			status: NewStatus("Active"),
		},
		{
			name:   "status with very long code",
			status: NewStatus("VERY_LONG_STATUS_CODE_THAT_MIGHT_EXCEED_LIMITS"),
		},
		{
			name:   "status with description",
			status: WithStatusDescription(NewStatus("CUSTOM"), "Custom status description"),
		},
		{
			name:   "status with empty description",
			status: WithStatusDescription(NewStatus("ACTIVE"), ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", "TST").WithStatus(tt.status)
			assert.Equal(t, tt.status.Code, input.Status.Code)

			err := input.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestCreateAssetInput_TypeEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		assetType string
	}{
		{
			name:      "empty type",
			assetType: "",
		},
		{
			name:      "standard type currency",
			assetType: "currency",
		},
		{
			name:      "uppercase type",
			assetType: "CURRENCY",
		},
		{
			name:      "mixed case type",
			assetType: "Currency",
		},
		{
			name:      "type with underscore",
			assetType: "custom_type",
		},
		{
			name:      "type with hyphen",
			assetType: "custom-type",
		},
		{
			name:      "type with numbers",
			assetType: "type123",
		},
		{
			name:      "very long type",
			assetType: "this_is_a_very_long_asset_type_that_describes_the_category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateAssetInput("Test Asset", "TST").WithType(tt.assetType)
			assert.Equal(t, tt.assetType, input.Type)

			err := input.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestCreateAssetInput_OverwriteFields(t *testing.T) {
	t.Run("overwrite type", func(t *testing.T) {
		input := NewCreateAssetInput("Test Asset", "TST").
			WithType("currency").
			WithType("cryptocurrency")

		assert.Equal(t, "cryptocurrency", input.Type)
	})

	t.Run("overwrite status", func(t *testing.T) {
		input := NewCreateAssetInput("Test Asset", "TST").
			WithStatus(NewStatus("ACTIVE")).
			WithStatus(NewStatus("INACTIVE"))

		assert.Equal(t, "INACTIVE", input.Status.Code)
	})

	t.Run("overwrite metadata", func(t *testing.T) {
		input := NewCreateAssetInput("Test Asset", "TST").
			WithMetadata(map[string]any{"key1": "value1"}).
			WithMetadata(map[string]any{"key2": "value2"})

		assert.Equal(t, map[string]any{"key2": "value2"}, input.Metadata)
		_, exists := input.Metadata["key1"]
		assert.False(t, exists, "key1 should not exist after overwrite")
	})
}

func TestUpdateAssetInput_OverwriteFields(t *testing.T) {
	t.Run("overwrite name", func(t *testing.T) {
		input := NewUpdateAssetInput().
			WithName("First Name").
			WithName("Second Name")

		assert.Equal(t, "Second Name", input.Name)
	})

	t.Run("overwrite status", func(t *testing.T) {
		input := NewUpdateAssetInput().
			WithStatus(NewStatus("ACTIVE")).
			WithStatus(NewStatus("INACTIVE"))

		assert.Equal(t, "INACTIVE", input.Status.Code)
	})

	t.Run("overwrite metadata", func(t *testing.T) {
		input := NewUpdateAssetInput().
			WithMetadata(map[string]any{"key1": "value1"}).
			WithMetadata(map[string]any{"key2": "value2"})

		assert.Equal(t, map[string]any{"key2": "value2"}, input.Metadata)
	})
}

func TestCreateAssetInput_UnderlyingStructAccess(t *testing.T) {
	input := NewCreateAssetInput("US Dollar", "USD").
		WithType("currency").
		WithStatus(NewStatus("ACTIVE")).
		WithMetadata(map[string]any{"region": "US"})

	assert.Equal(t, "US Dollar", input.CreateAssetInput.Name)
	assert.Equal(t, "USD", input.CreateAssetInput.Code)
	assert.Equal(t, "currency", input.CreateAssetInput.Type)
	assert.Equal(t, "ACTIVE", input.CreateAssetInput.Status.Code)
	assert.Equal(t, map[string]any{"region": "US"}, input.CreateAssetInput.Metadata)
}

func TestUpdateAssetInput_UnderlyingStructAccess(t *testing.T) {
	input := NewUpdateAssetInput().
		WithName("Updated Name").
		WithStatus(NewStatus("INACTIVE")).
		WithMetadata(map[string]any{"updated": true})

	assert.Equal(t, "Updated Name", input.UpdateAssetInput.Name)
	assert.Equal(t, "INACTIVE", input.UpdateAssetInput.Status.Code)
	assert.Equal(t, map[string]any{"updated": true}, input.UpdateAssetInput.Metadata)
}

func TestAssetTypeAlias(t *testing.T) {
	var asset Asset

	assert.NotNil(t, &asset)
	assert.Empty(t, asset.ID)
	assert.Empty(t, asset.Name)
	assert.Empty(t, asset.Code)
}
