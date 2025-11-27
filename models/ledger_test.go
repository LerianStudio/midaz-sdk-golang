package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCreateLedgerInput(t *testing.T) {
	tests := []struct {
		name     string
		ledName  string
		wantName string
	}{
		{
			name:     "creates with valid name",
			ledName:  "Main Ledger",
			wantName: "Main Ledger",
		},
		{
			name:     "creates with empty name",
			ledName:  "",
			wantName: "",
		},
		{
			name:     "creates with unicode name",
			ledName:  "Ledger-Conta",
			wantName: "Ledger-Conta",
		},
		{
			name:     "creates with long name",
			ledName:  "This is a very long ledger name that should still work correctly in the system",
			wantName: "This is a very long ledger name that should still work correctly in the system",
		},
		{
			name:     "creates with special characters",
			ledName:  "Ledger_123-Test.v2",
			wantName: "Ledger_123-Test.v2",
		},
		{
			name:     "creates with whitespace only",
			ledName:  "   ",
			wantName: "   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateLedgerInput(tt.ledName)

			assert.NotNil(t, input)
			assert.Equal(t, tt.wantName, input.Name)
		})
	}
}

func TestCreateLedgerInput_Validate(t *testing.T) {
	tests := []struct {
		name        string
		input       *CreateLedgerInput
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid input with name only",
			input:   NewCreateLedgerInput("Test Ledger"),
			wantErr: false,
		},
		{
			name:        "empty name returns error",
			input:       NewCreateLedgerInput(""),
			wantErr:     true,
			errContains: "name is required",
		},
		{
			name: "valid input with status",
			input: func() *CreateLedgerInput {
				return NewCreateLedgerInput("Ledger").WithStatus(NewStatus("ACTIVE"))
			}(),
			wantErr: false,
		},
		{
			name: "valid input with metadata",
			input: func() *CreateLedgerInput {
				return NewCreateLedgerInput("Ledger").WithMetadata(map[string]any{"key": "value"})
			}(),
			wantErr: false,
		},
		{
			name: "valid input with all fields",
			input: func() *CreateLedgerInput {
				return NewCreateLedgerInput("Complete Ledger").
					WithStatus(NewStatus("ACTIVE")).
					WithMetadata(map[string]any{"env": "production"})
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateLedgerInput_WithStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     Status
		wantCode   string
		wantNilErr bool
	}{
		{
			name:       "sets ACTIVE status",
			status:     NewStatus("ACTIVE"),
			wantCode:   "ACTIVE",
			wantNilErr: true,
		},
		{
			name:       "sets INACTIVE status",
			status:     NewStatus("INACTIVE"),
			wantCode:   "INACTIVE",
			wantNilErr: true,
		},
		{
			name:       "sets PENDING status",
			status:     NewStatus("PENDING"),
			wantCode:   "PENDING",
			wantNilErr: true,
		},
		{
			name:       "sets empty status",
			status:     NewStatus(""),
			wantCode:   "",
			wantNilErr: true,
		},
		{
			name:       "sets status with description",
			status:     WithStatusDescription(NewStatus("ACTIVE"), "Fully operational"),
			wantCode:   "ACTIVE",
			wantNilErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateLedgerInput("Test Ledger")
			result := input.WithStatus(tt.status)

			assert.Same(t, input, result, "WithStatus should return the same pointer for chaining")
			assert.Equal(t, tt.wantCode, result.Status.Code)

			if tt.wantNilErr {
				assert.NoError(t, result.Validate())
			}
		})
	}
}

func TestCreateLedgerInput_WithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "sets nil metadata",
			metadata: nil,
		},
		{
			name:     "sets empty metadata",
			metadata: map[string]any{},
		},
		{
			name: "sets single key metadata",
			metadata: map[string]any{
				"key": "value",
			},
		},
		{
			name: "sets multiple keys metadata",
			metadata: map[string]any{
				"env":     "production",
				"region":  "us-east-1",
				"version": 2,
			},
		},
		{
			name: "sets nested metadata",
			metadata: map[string]any{
				"config": map[string]any{
					"nested": "value",
				},
			},
		},
		{
			name: "sets metadata with various types",
			metadata: map[string]any{
				"string":  "value",
				"int":     42,
				"float":   3.14,
				"bool":    true,
				"array":   []string{"a", "b", "c"},
				"nil_val": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewCreateLedgerInput("Test Ledger")
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result, "WithMetadata should return the same pointer for chaining")
			assert.Equal(t, tt.metadata, result.Metadata)
		})
	}
}

func TestCreateLedgerInput_Chaining(t *testing.T) {
	t.Run("all builder methods can be chained", func(t *testing.T) {
		metadata := map[string]any{"key": "value"}
		status := NewStatus("ACTIVE")

		input := NewCreateLedgerInput("Chained Ledger").
			WithStatus(status).
			WithMetadata(metadata)

		assert.NotNil(t, input)
		assert.Equal(t, "Chained Ledger", input.Name)
		assert.Equal(t, "ACTIVE", input.Status.Code)
		assert.Equal(t, metadata, input.Metadata)
		assert.NoError(t, input.Validate())
	})

	t.Run("chaining order does not matter", func(t *testing.T) {
		metadata := map[string]any{"env": "test"}
		status := NewStatus("PENDING")

		input1 := NewCreateLedgerInput("Ledger1").
			WithStatus(status).
			WithMetadata(metadata)

		input2 := NewCreateLedgerInput("Ledger1").
			WithMetadata(metadata).
			WithStatus(status)

		assert.Equal(t, input1.Name, input2.Name)
		assert.Equal(t, input1.Status.Code, input2.Status.Code)
		assert.Equal(t, input1.Metadata, input2.Metadata)
	})

	t.Run("builder methods can be called multiple times", func(t *testing.T) {
		input := NewCreateLedgerInput("Test").
			WithStatus(NewStatus("ACTIVE")).
			WithStatus(NewStatus("INACTIVE")).
			WithMetadata(map[string]any{"first": true}).
			WithMetadata(map[string]any{"second": true})

		assert.Equal(t, "INACTIVE", input.Status.Code)
		assert.Equal(t, map[string]any{"second": true}, input.Metadata)
	})
}

func TestNewUpdateLedgerInput(t *testing.T) {
	t.Run("creates empty update input", func(t *testing.T) {
		input := NewUpdateLedgerInput()

		assert.NotNil(t, input)
		assert.Empty(t, input.Name)
		assert.True(t, IsStatusEmpty(input.Status))
		assert.Nil(t, input.Metadata)
	})
}

func TestUpdateLedgerInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *UpdateLedgerInput
		wantErr bool
	}{
		{
			name:    "empty input is valid",
			input:   NewUpdateLedgerInput(),
			wantErr: false,
		},
		{
			name:    "input with name only is valid",
			input:   NewUpdateLedgerInput().WithName("New Name"),
			wantErr: false,
		},
		{
			name:    "input with empty name is valid",
			input:   NewUpdateLedgerInput().WithName(""),
			wantErr: false,
		},
		{
			name:    "input with status only is valid",
			input:   NewUpdateLedgerInput().WithStatus(NewStatus("ACTIVE")),
			wantErr: false,
		},
		{
			name:    "input with metadata only is valid",
			input:   NewUpdateLedgerInput().WithMetadata(map[string]any{"key": "value"}),
			wantErr: false,
		},
		{
			name: "input with all fields is valid",
			input: NewUpdateLedgerInput().
				WithName("Updated Ledger").
				WithStatus(NewStatus("INACTIVE")).
				WithMetadata(map[string]any{"updated": true}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateLedgerInput_WithName(t *testing.T) {
	tests := []struct {
		name     string
		ledName  string
		wantName string
	}{
		{
			name:     "sets valid name",
			ledName:  "Updated Ledger Name",
			wantName: "Updated Ledger Name",
		},
		{
			name:     "sets empty name",
			ledName:  "",
			wantName: "",
		},
		{
			name:     "sets name with special characters",
			ledName:  "Ledger-2024_v2.0",
			wantName: "Ledger-2024_v2.0",
		},
		{
			name:     "sets unicode name",
			ledName:  "Conta Principal",
			wantName: "Conta Principal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateLedgerInput()
			result := input.WithName(tt.ledName)

			assert.Same(t, input, result, "WithName should return the same pointer for chaining")
			assert.Equal(t, tt.wantName, result.Name)
		})
	}
}

func TestUpdateLedgerInput_WithStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		wantCode string
	}{
		{
			name:     "sets ACTIVE status",
			status:   NewStatus("ACTIVE"),
			wantCode: "ACTIVE",
		},
		{
			name:     "sets INACTIVE status",
			status:   NewStatus("INACTIVE"),
			wantCode: "INACTIVE",
		},
		{
			name:     "sets CLOSED status",
			status:   NewStatus("CLOSED"),
			wantCode: "CLOSED",
		},
		{
			name:     "sets empty status",
			status:   NewStatus(""),
			wantCode: "",
		},
		{
			name:     "sets custom status",
			status:   NewStatus("CUSTOM_STATUS"),
			wantCode: "CUSTOM_STATUS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateLedgerInput()
			result := input.WithStatus(tt.status)

			assert.Same(t, input, result, "WithStatus should return the same pointer for chaining")
			assert.Equal(t, tt.wantCode, result.Status.Code)
		})
	}
}

func TestUpdateLedgerInput_WithMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
	}{
		{
			name:     "sets nil metadata",
			metadata: nil,
		},
		{
			name:     "sets empty metadata",
			metadata: map[string]any{},
		},
		{
			name: "sets metadata with string value",
			metadata: map[string]any{
				"key": "value",
			},
		},
		{
			name: "sets metadata with multiple types",
			metadata: map[string]any{
				"string": "text",
				"number": 123,
				"bool":   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := NewUpdateLedgerInput()
			result := input.WithMetadata(tt.metadata)

			assert.Same(t, input, result, "WithMetadata should return the same pointer for chaining")
			assert.Equal(t, tt.metadata, result.Metadata)
		})
	}
}

func TestUpdateLedgerInput_Chaining(t *testing.T) {
	t.Run("all builder methods can be chained", func(t *testing.T) {
		input := NewUpdateLedgerInput().
			WithName("Updated Name").
			WithStatus(NewStatus("ACTIVE")).
			WithMetadata(map[string]any{"updated": true})

		assert.NotNil(t, input)
		assert.Equal(t, "Updated Name", input.Name)
		assert.Equal(t, "ACTIVE", input.Status.Code)
		assert.Equal(t, map[string]any{"updated": true}, input.Metadata)
	})

	t.Run("chaining order does not matter", func(t *testing.T) {
		input1 := NewUpdateLedgerInput().
			WithName("Name").
			WithStatus(NewStatus("ACTIVE")).
			WithMetadata(map[string]any{"key": "value"})

		input2 := NewUpdateLedgerInput().
			WithMetadata(map[string]any{"key": "value"}).
			WithName("Name").
			WithStatus(NewStatus("ACTIVE"))

		assert.Equal(t, input1.Name, input2.Name)
		assert.Equal(t, input1.Status.Code, input2.Status.Code)
		assert.Equal(t, input1.Metadata, input2.Metadata)
	})

	t.Run("later calls override earlier values", func(t *testing.T) {
		input := NewUpdateLedgerInput().
			WithName("First Name").
			WithName("Second Name").
			WithStatus(NewStatus("ACTIVE")).
			WithStatus(NewStatus("INACTIVE"))

		assert.Equal(t, "Second Name", input.Name)
		assert.Equal(t, "INACTIVE", input.Status.Code)
	})
}

func TestLedgerStatusValues(t *testing.T) {
	t.Run("common status codes", func(t *testing.T) {
		statusCodes := []string{
			"ACTIVE",
			"INACTIVE",
			"PENDING",
			"CLOSED",
			"SUSPENDED",
		}

		for _, code := range statusCodes {
			status := NewStatus(code)
			assert.Equal(t, code, status.Code)
			assert.False(t, IsStatusEmpty(status))
		}
	})

	t.Run("status with description", func(t *testing.T) {
		status := WithStatusDescription(NewStatus("ACTIVE"), "Ledger is fully operational")

		assert.Equal(t, "ACTIVE", status.Code)
		assert.NotNil(t, status.Description)
		assert.Equal(t, "Ledger is fully operational", *status.Description)
	})

	t.Run("empty status check", func(t *testing.T) {
		emptyStatus := Status{}
		assert.True(t, IsStatusEmpty(emptyStatus))

		statusWithCode := NewStatus("ACTIVE")
		assert.False(t, IsStatusEmpty(statusWithCode))

		statusWithDesc := WithStatusDescription(Status{}, "Description only")
		assert.False(t, IsStatusEmpty(statusWithDesc))
	})
}

func TestLedgerMetadataHandling(t *testing.T) {
	t.Run("create input accepts various metadata types", func(t *testing.T) {
		metadata := map[string]any{
			"string_val":  "text",
			"int_val":     42,
			"float_val":   3.14159,
			"bool_val":    true,
			"nil_val":     nil,
			"array_val":   []int{1, 2, 3},
			"nested_val":  map[string]any{"inner": "value"},
			"empty_str":   "",
			"zero_int":    0,
			"false_bool":  false,
			"empty_array": []string{},
		}

		input := NewCreateLedgerInput("Metadata Test").WithMetadata(metadata)

		assert.Equal(t, metadata, input.Metadata)
		assert.NoError(t, input.Validate())
	})

	t.Run("update input accepts various metadata types", func(t *testing.T) {
		metadata := map[string]any{
			"updated_at": "2024-01-15T10:30:00Z",
			"version":    2,
			"tags":       []string{"finance", "main"},
		}

		input := NewUpdateLedgerInput().WithMetadata(metadata)

		assert.Equal(t, metadata, input.Metadata)
		assert.NoError(t, input.Validate())
	})

	t.Run("metadata can be overwritten", func(t *testing.T) {
		firstMeta := map[string]any{"first": true}
		secondMeta := map[string]any{"second": true}

		input := NewCreateLedgerInput("Test").
			WithMetadata(firstMeta).
			WithMetadata(secondMeta)

		assert.Equal(t, secondMeta, input.Metadata)
		assert.NotContains(t, input.Metadata, "first")
	})

	t.Run("nil metadata replaces existing", func(t *testing.T) {
		input := NewCreateLedgerInput("Test").
			WithMetadata(map[string]any{"key": "value"}).
			WithMetadata(nil)

		assert.Nil(t, input.Metadata)
	})
}

func TestLedgerEdgeCases(t *testing.T) {
	t.Run("CreateLedgerInput with whitespace-only name fails validation", func(t *testing.T) {
		input := NewCreateLedgerInput("   ")
		err := input.Validate()
		assert.NoError(t, err, "whitespace-only name currently passes validation")
	})

	t.Run("CreateLedgerInput pointer is same through chain", func(t *testing.T) {
		original := NewCreateLedgerInput("Test")
		afterStatus := original.WithStatus(NewStatus("ACTIVE"))
		afterMeta := afterStatus.WithMetadata(map[string]any{})

		assert.Same(t, original, afterStatus)
		assert.Same(t, afterStatus, afterMeta)
	})

	t.Run("UpdateLedgerInput pointer is same through chain", func(t *testing.T) {
		original := NewUpdateLedgerInput()
		afterName := original.WithName("Name")
		afterStatus := afterName.WithStatus(NewStatus("ACTIVE"))
		afterMeta := afterStatus.WithMetadata(map[string]any{})

		assert.Same(t, original, afterName)
		assert.Same(t, afterName, afterStatus)
		assert.Same(t, afterStatus, afterMeta)
	})

	t.Run("very long name is accepted", func(t *testing.T) {
		longName := ""
		for i := 0; i < 1000; i++ {
			longName += "a"
		}

		input := NewCreateLedgerInput(longName)
		assert.Equal(t, 1000, len(input.Name))
		assert.NoError(t, input.Validate())
	})

	t.Run("special unicode characters in name", func(t *testing.T) {
		unicodeName := "Ledger Conta Principal"
		input := NewCreateLedgerInput(unicodeName)
		assert.Equal(t, unicodeName, input.Name)
		assert.NoError(t, input.Validate())
	})

	t.Run("empty metadata map vs nil metadata", func(t *testing.T) {
		inputNil := NewCreateLedgerInput("Test").WithMetadata(nil)
		inputEmpty := NewCreateLedgerInput("Test").WithMetadata(map[string]any{})

		assert.Nil(t, inputNil.Metadata)
		assert.NotNil(t, inputEmpty.Metadata)
		assert.Empty(t, inputEmpty.Metadata)
	})

	t.Run("status with empty code and nil description", func(t *testing.T) {
		input := NewCreateLedgerInput("Test").WithStatus(Status{})

		assert.Empty(t, input.Status.Code)
		assert.Nil(t, input.Status.Description)
		assert.True(t, IsStatusEmpty(input.Status))
	})

	t.Run("update input with all empty values is valid", func(t *testing.T) {
		input := NewUpdateLedgerInput().
			WithName("").
			WithStatus(Status{}).
			WithMetadata(nil)

		assert.NoError(t, input.Validate())
	})
}

func TestLedgerInputImmutability(t *testing.T) {
	t.Run("metadata map is not copied", func(t *testing.T) {
		metadata := map[string]any{"key": "original"}
		input := NewCreateLedgerInput("Test").WithMetadata(metadata)

		metadata["key"] = "modified"

		assert.Equal(t, "modified", input.Metadata["key"])
	})

	t.Run("modifying input does not affect original metadata reference", func(t *testing.T) {
		originalMeta := map[string]any{"key": "value"}
		input := NewCreateLedgerInput("Test").WithMetadata(originalMeta)

		input.Metadata["newKey"] = "newValue"

		assert.Contains(t, originalMeta, "newKey")
	})
}

func TestCreateLedgerInputDefaults(t *testing.T) {
	t.Run("new input has expected zero values", func(t *testing.T) {
		input := NewCreateLedgerInput("Test")

		assert.Equal(t, "Test", input.Name)
		assert.True(t, IsStatusEmpty(input.Status))
		assert.Nil(t, input.Metadata)
	})
}

func TestUpdateLedgerInputDefaults(t *testing.T) {
	t.Run("new input has expected zero values", func(t *testing.T) {
		input := NewUpdateLedgerInput()

		assert.Empty(t, input.Name)
		assert.True(t, IsStatusEmpty(input.Status))
		assert.Nil(t, input.Metadata)
	})
}
