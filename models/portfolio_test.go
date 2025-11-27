package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCreatePortfolioInput(t *testing.T) {
	tests := []struct {
		name     string
		entityID string
		nameVal  string
	}{
		{
			name:     "valid input with normal values",
			entityID: "entity-123",
			nameVal:  "My Portfolio",
		},
		{
			name:     "valid input with special characters",
			entityID: "entity_abc-123",
			nameVal:  "Portfolio & Co. (2024)",
		},
		{
			name:     "valid input with unicode characters",
			entityID: "entity-unicode-456",
			nameVal:  "Portafolio Financiero",
		},
		{
			name:     "valid input with long name",
			entityID: "entity-long-name",
			nameVal:  "This is a very long portfolio name that might be used in some edge cases for testing purposes",
		},
		{
			name:     "empty values - factory allows but validation fails",
			entityID: "",
			nameVal:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreatePortfolioInput(tt.entityID, tt.nameVal)

			assert.NotNil(t, input)
			assert.Equal(t, tt.entityID, input.EntityID)
			assert.Equal(t, tt.nameVal, input.Name)
			assert.Empty(t, input.Status.Code)
			assert.Nil(t, input.Metadata)
		})
	}
}

func TestCreatePortfolioInput_WithStatus(t *testing.T) {
	tests := []struct {
		name   string
		status Status
	}{
		{
			name:   "active status",
			status: NewStatus("ACTIVE"),
		},
		{
			name:   "pending status",
			status: NewStatus("PENDING"),
		},
		{
			name:   "inactive status",
			status: NewStatus("INACTIVE"),
		},
		{
			name:   "status with description",
			status: WithStatusDescription(NewStatus("ACTIVE"), "Portfolio is active"),
		},
		{
			name:   "empty status",
			status: Status{},
		},
		{
			name:   "custom status code",
			status: NewStatus("CUSTOM_STATUS"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreatePortfolioInput("entity-123", "Test Portfolio")
			result := input.WithStatus(tt.status)

			assert.Same(t, input, result, "WithStatus should return same pointer for chaining")
			assert.Equal(t, tt.status.Code, input.Status.Code)

			if tt.status.Description != nil {
				assert.Equal(t, *tt.status.Description, *input.Status.Description)
			}
		})
	}
}

func TestCreatePortfolioInput_WithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "nil metadata",
			metadata: nil,
		},
		{
			name:     "empty metadata",
			metadata: map[string]any{},
		},
		{
			name: "single key-value pair",
			metadata: map[string]any{
				"key": "value",
			},
		},
		{
			name: "multiple key-value pairs",
			metadata: map[string]any{
				"department": "finance",
				"region":     "north-america",
				"priority":   "high",
			},
		},
		{
			name: "mixed types in metadata",
			metadata: map[string]any{
				"string_key": "string_value",
				"int_key":    42,
				"float_key":  3.14,
				"bool_key":   true,
				"nil_key":    nil,
				"array_key":  []string{"a", "b", "c"},
				"nested_key": map[string]any{"inner": "value"},
			},
		},
		{
			name: "metadata with special characters in keys",
			metadata: map[string]any{
				"key-with-dash":       "value1",
				"key_with_underscore": "value2",
				"key.with.dot":        "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreatePortfolioInput("entity-123", "Test Portfolio")
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result, "WithMetadata should return same pointer for chaining")
			assert.Equal(t, tt.metadata, input.Metadata)
		})
	}
}

func TestCreatePortfolioInput_Validate(t *testing.T) {
	tests := []struct {
		name        string
		entityID    string
		nameVal     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid input",
			entityID:    "entity-123",
			nameVal:     "My Portfolio",
			expectError: false,
		},
		{
			name:        "missing name",
			entityID:    "entity-123",
			nameVal:     "",
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name:        "missing entityID",
			entityID:    "",
			nameVal:     "My Portfolio",
			expectError: true,
			errorMsg:    "entityID is required",
		},
		{
			name:        "missing both name and entityID - name error first",
			entityID:    "",
			nameVal:     "",
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name:        "whitespace only name",
			entityID:    "entity-123",
			nameVal:     "   ",
			expectError: false, // current implementation does not trim whitespace
		},
		{
			name:        "whitespace only entityID",
			entityID:    "   ",
			nameVal:     "My Portfolio",
			expectError: false, // current implementation does not trim whitespace
		},
		{
			name:        "valid input with special characters",
			entityID:    "entity_abc-123",
			nameVal:     "Portfolio & Co.",
			expectError: false,
		},
		{
			name:        "valid input with unicode",
			entityID:    "entity-unicode",
			nameVal:     "Portafolio Empresarial",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreatePortfolioInput(tt.entityID, tt.nameVal)
			err := input.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreatePortfolioInput_MethodChaining(t *testing.T) {
	metadata := map[string]any{
		"department": "finance",
		"priority":   "high",
	}
	status := NewStatus("ACTIVE")

	input := NewCreatePortfolioInput("entity-123", "Test Portfolio").
		WithStatus(status).
		WithMetadata(metadata)

	assert.Equal(t, "entity-123", input.EntityID)
	assert.Equal(t, "Test Portfolio", input.Name)
	assert.Equal(t, "ACTIVE", input.Status.Code)
	assert.Equal(t, metadata, input.Metadata)
	require.NoError(t, input.Validate())
}

func TestNewUpdatePortfolioInput(t *testing.T) {
	input := NewUpdatePortfolioInput()

	assert.NotNil(t, input)
	assert.Empty(t, input.Name)
	assert.Empty(t, input.Status.Code)
	assert.Nil(t, input.Metadata)
}

func TestUpdatePortfolioInput_WithName(t *testing.T) {
	tests := []struct {
		name    string
		nameVal string
	}{
		{
			name:    "normal name",
			nameVal: "Updated Portfolio",
		},
		{
			name:    "empty name",
			nameVal: "",
		},
		{
			name:    "name with special characters",
			nameVal: "Portfolio & Co. (2024)",
		},
		{
			name:    "name with unicode",
			nameVal: "Portafolio Actualizado",
		},
		{
			name:    "very long name",
			nameVal: "This is an extremely long portfolio name that might be used to test the limits of the system and ensure it can handle edge cases properly",
		},
		{
			name:    "name with whitespace",
			nameVal: "   Spaced Portfolio   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdatePortfolioInput()
			result := input.WithName(tt.nameVal)

			assert.Same(t, input, result, "WithName should return same pointer for chaining")
			assert.Equal(t, tt.nameVal, input.Name)
		})
	}
}

func TestUpdatePortfolioInput_WithStatus(t *testing.T) {
	tests := []struct {
		name   string
		status Status
	}{
		{
			name:   "active status",
			status: NewStatus("ACTIVE"),
		},
		{
			name:   "inactive status",
			status: NewStatus("INACTIVE"),
		},
		{
			name:   "pending status",
			status: NewStatus("PENDING"),
		},
		{
			name:   "closed status",
			status: NewStatus("CLOSED"),
		},
		{
			name:   "status with description",
			status: WithStatusDescription(NewStatus("ACTIVE"), "Portfolio reactivated"),
		},
		{
			name:   "empty status",
			status: Status{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdatePortfolioInput()
			result := input.WithStatus(tt.status)

			assert.Same(t, input, result, "WithStatus should return same pointer for chaining")
			assert.Equal(t, tt.status.Code, input.Status.Code)

			if tt.status.Description != nil {
				assert.NotNil(t, input.Status.Description)
				assert.Equal(t, *tt.status.Description, *input.Status.Description)
			}
		})
	}
}

func TestUpdatePortfolioInput_WithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "nil metadata",
			metadata: nil,
		},
		{
			name:     "empty metadata",
			metadata: map[string]any{},
		},
		{
			name: "single key-value pair",
			metadata: map[string]any{
				"updated_by": "admin",
			},
		},
		{
			name: "multiple key-value pairs",
			metadata: map[string]any{
				"updated_by":   "admin",
				"updated_at":   "2024-01-15",
				"update_count": 5,
			},
		},
		{
			name: "replace existing metadata",
			metadata: map[string]any{
				"completely": "new",
				"metadata":   "values",
			},
		},
		{
			name: "metadata with nested objects",
			metadata: map[string]any{
				"audit": map[string]any{
					"user":   "admin",
					"action": "update",
					"timestamp": map[string]any{
						"date": "2024-01-15",
						"time": "10:30:00",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdatePortfolioInput()
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result, "WithMetadata should return same pointer for chaining")
			assert.Equal(t, tt.metadata, input.Metadata)
		})
	}
}

func TestUpdatePortfolioInput_Validate(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*UpdatePortfolioInput)
		expectError bool
	}{
		{
			name:        "empty input is valid",
			setupFunc:   func(_ *UpdatePortfolioInput) {},
			expectError: false,
		},
		{
			name: "only name set",
			setupFunc: func(input *UpdatePortfolioInput) {
				input.WithName("New Name")
			},
			expectError: false,
		},
		{
			name: "only status set",
			setupFunc: func(input *UpdatePortfolioInput) {
				input.WithStatus(NewStatus("ACTIVE"))
			},
			expectError: false,
		},
		{
			name: "only metadata set",
			setupFunc: func(input *UpdatePortfolioInput) {
				input.WithMetadata(map[string]any{"key": "value"})
			},
			expectError: false,
		},
		{
			name: "all fields set",
			setupFunc: func(input *UpdatePortfolioInput) {
				input.WithName("Updated Portfolio").
					WithStatus(NewStatus("ACTIVE")).
					WithMetadata(map[string]any{"key": "value"})
			},
			expectError: false,
		},
		{
			name: "empty name is valid for update",
			setupFunc: func(input *UpdatePortfolioInput) {
				input.WithName("")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdatePortfolioInput()
			tt.setupFunc(input)

			err := input.Validate()

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdatePortfolioInput_MethodChaining(t *testing.T) {
	metadata := map[string]any{
		"updated_by": "admin",
		"priority":   "critical",
	}
	status := WithStatusDescription(NewStatus("ACTIVE"), "Reactivated")

	input := NewUpdatePortfolioInput().
		WithName("Updated Portfolio Name").
		WithStatus(status).
		WithMetadata(metadata)

	assert.Equal(t, "Updated Portfolio Name", input.Name)
	assert.Equal(t, "ACTIVE", input.Status.Code)
	assert.NotNil(t, input.Status.Description)
	assert.Equal(t, "Reactivated", *input.Status.Description)
	assert.Equal(t, metadata, input.Metadata)
	require.NoError(t, input.Validate())
}

func TestPortfolioInputs_StatusEnumValues(t *testing.T) {
	commonStatuses := []string{
		"ACTIVE",
		"INACTIVE",
		"PENDING",
		"CLOSED",
		"BLOCKED",
		"SUSPENDED",
	}

	t.Run("CreatePortfolioInput accepts all common statuses", func(t *testing.T) {
		for _, statusCode := range commonStatuses {
			input := NewCreatePortfolioInput("entity-123", "Test Portfolio").
				WithStatus(NewStatus(statusCode))

			assert.Equal(t, statusCode, input.Status.Code)
			require.NoError(t, input.Validate())
		}
	})

	t.Run("UpdatePortfolioInput accepts all common statuses", func(t *testing.T) {
		for _, statusCode := range commonStatuses {
			input := NewUpdatePortfolioInput().
				WithStatus(NewStatus(statusCode))

			assert.Equal(t, statusCode, input.Status.Code)
			require.NoError(t, input.Validate())
		}
	})
}

func TestCreatePortfolioInput_EmbeddedMmodelFields(t *testing.T) {
	input := NewCreatePortfolioInput("entity-test", "Embedded Test")

	assert.Equal(t, "entity-test", input.EntityID)
	assert.Equal(t, "Embedded Test", input.Name)

	input.WithStatus(NewStatus("ACTIVE"))
	assert.Equal(t, "ACTIVE", input.Status.Code)

	metadata := map[string]any{"key": "value"}
	input.WithMetadata(metadata)
	assert.Equal(t, metadata, input.Metadata)
}

func TestUpdatePortfolioInput_EmbeddedMmodelFields(t *testing.T) {
	input := NewUpdatePortfolioInput()

	input.WithName("Embedded Update")
	assert.Equal(t, "Embedded Update", input.Name)

	input.WithStatus(NewStatus("INACTIVE"))
	assert.Equal(t, "INACTIVE", input.Status.Code)

	metadata := map[string]any{"updated": true}
	input.WithMetadata(metadata)
	assert.Equal(t, metadata, input.Metadata)
}

func TestCreatePortfolioInput_ValidationOrder(t *testing.T) {
	input := NewCreatePortfolioInput("", "")
	err := input.Validate()

	require.Error(t, err)
	assert.Equal(t, "name is required", err.Error())

	input2 := NewCreatePortfolioInput("entity-123", "")
	err2 := input2.Validate()

	require.Error(t, err2)
	assert.Equal(t, "name is required", err2.Error())

	input3 := NewCreatePortfolioInput("", "Valid Name")
	err3 := input3.Validate()

	require.Error(t, err3)
	assert.Equal(t, "entityID is required", err3.Error())
}

func TestPortfolioInput_MetadataImmutability(t *testing.T) {
	t.Run("CreatePortfolioInput metadata reference", func(t *testing.T) {
		originalMetadata := map[string]any{
			"key": "original",
		}

		input := NewCreatePortfolioInput("entity-123", "Test").
			WithMetadata(originalMetadata)

		originalMetadata["key"] = "modified"

		assert.Equal(t, "modified", input.Metadata["key"],
			"Metadata is passed by reference, modifications affect the input")
	})

	t.Run("UpdatePortfolioInput metadata reference", func(t *testing.T) {
		originalMetadata := map[string]any{
			"key": "original",
		}

		input := NewUpdatePortfolioInput().
			WithMetadata(originalMetadata)

		originalMetadata["key"] = "modified"

		assert.Equal(t, "modified", input.Metadata["key"],
			"Metadata is passed by reference, modifications affect the input")
	})
}

func TestPortfolioInput_NilSafety(t *testing.T) {
	t.Run("CreatePortfolioInput with nil metadata is safe", func(t *testing.T) {
		input := NewCreatePortfolioInput("entity-123", "Test")

		assert.Nil(t, input.Metadata)

		input.WithMetadata(nil)
		assert.Nil(t, input.Metadata)

		require.NoError(t, input.Validate())
	})

	t.Run("UpdatePortfolioInput with nil metadata is safe", func(t *testing.T) {
		input := NewUpdatePortfolioInput()

		assert.Nil(t, input.Metadata)

		input.WithMetadata(nil)
		assert.Nil(t, input.Metadata)

		require.NoError(t, input.Validate())
	})
}

func TestPortfolioInput_StatusDescriptionPointer(t *testing.T) {
	t.Run("CreatePortfolioInput status description", func(t *testing.T) {
		input := NewCreatePortfolioInput("entity-123", "Test")

		assert.Nil(t, input.Status.Description)

		statusWithDesc := WithStatusDescription(NewStatus("ACTIVE"), "Test description")
		input.WithStatus(statusWithDesc)

		assert.NotNil(t, input.Status.Description)
		assert.Equal(t, "Test description", *input.Status.Description)
	})

	t.Run("UpdatePortfolioInput status description", func(t *testing.T) {
		input := NewUpdatePortfolioInput()

		assert.Nil(t, input.Status.Description)

		statusWithDesc := WithStatusDescription(NewStatus("INACTIVE"), "Deactivated by admin")
		input.WithStatus(statusWithDesc)

		assert.NotNil(t, input.Status.Description)
		assert.Equal(t, "Deactivated by admin", *input.Status.Description)
	})
}

func TestPortfolioInput_EmptyStringVsNil(t *testing.T) {
	t.Run("empty string name fails validation", func(t *testing.T) {
		input := NewCreatePortfolioInput("entity-123", "")
		err := input.Validate()
		require.Error(t, err)
		assert.Equal(t, "name is required", err.Error())
	})

	t.Run("empty string entityID fails validation", func(t *testing.T) {
		input := NewCreatePortfolioInput("", "Valid Name")
		err := input.Validate()
		require.Error(t, err)
		assert.Equal(t, "entityID is required", err.Error())
	})

	t.Run("empty status code is allowed", func(t *testing.T) {
		input := NewCreatePortfolioInput("entity-123", "Valid Name").
			WithStatus(Status{})
		err := input.Validate()
		require.NoError(t, err)
		assert.Empty(t, input.Status.Code)
	})
}

func TestPortfolioInput_CompleteWorkflow(t *testing.T) {
	t.Run("complete create workflow", func(t *testing.T) {
		createInput := NewCreatePortfolioInput("entity-abc-123", "Investment Portfolio")
		createInput.WithStatus(WithStatusDescription(NewStatus("ACTIVE"), "Initial creation"))
		createInput.WithMetadata(map[string]any{
			"created_by":  "system",
			"category":    "investments",
			"risk_level":  "medium",
			"target_date": "2025-12-31",
		})

		require.NoError(t, createInput.Validate())
		assert.Equal(t, "entity-abc-123", createInput.EntityID)
		assert.Equal(t, "Investment Portfolio", createInput.Name)
		assert.Equal(t, "ACTIVE", createInput.Status.Code)
		assert.Equal(t, "Initial creation", *createInput.Status.Description)
		assert.Len(t, createInput.Metadata, 4)
	})

	t.Run("complete update workflow", func(t *testing.T) {
		updateInput := NewUpdatePortfolioInput()
		updateInput.WithName("Renamed Investment Portfolio")
		updateInput.WithStatus(WithStatusDescription(NewStatus("INACTIVE"), "Temporarily suspended"))
		updateInput.WithMetadata(map[string]any{
			"updated_by":     "admin",
			"reason":         "quarterly review",
			"review_date":    "2024-03-15",
			"previous_state": "ACTIVE",
		})

		require.NoError(t, updateInput.Validate())
		assert.Equal(t, "Renamed Investment Portfolio", updateInput.Name)
		assert.Equal(t, "INACTIVE", updateInput.Status.Code)
		assert.Equal(t, "Temporarily suspended", *updateInput.Status.Description)
		assert.Len(t, updateInput.Metadata, 4)
	})
}
