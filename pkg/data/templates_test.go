package data

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
)

func TestStrPtr(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"simple string", "hello"},
		{"string with spaces", "hello world"},
		{"string with special chars", "test@123!"},
		{"unicode string", "\u4e16\u754c"},
		{"long string", "this is a very long string that could be used as an alias or identifier"},
		{"string with newlines", "line1\nline2"},
		{"string with tabs", "col1\tcol2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := StrPtr(tt.input)
			assert.NotNil(t, ptr)
			assert.Equal(t, tt.input, *ptr)
		})
	}
}

func TestStrPtrReturnsNewPointer(t *testing.T) {
	// Verify that StrPtr returns a new pointer each time
	s := "test"
	ptr1 := StrPtr(s)
	ptr2 := StrPtr(s)

	// Both should have the same value
	assert.Equal(t, *ptr1, *ptr2)

	// But they should be different pointers
	assert.NotSame(t, ptr1, ptr2)
}

func TestStrPtrWithEmptyString(t *testing.T) {
	ptr := StrPtr("")
	assert.NotNil(t, ptr)
	assert.Equal(t, "", *ptr)
}

func TestOrgTemplateStructFromTemplates(t *testing.T) {
	org := OrgTemplate{
		LegalName: "Test Legal Name",
		TradeName: "Test Trade Name",
		TaxID:     "12-3456789",
		Address:   models.NewAddress("123 Main St", "12345", "Test City", "TS", "US"),
		Status:    models.NewStatus(models.StatusActive),
		Metadata:  map[string]any{"key": "value"},
		Industry:  "technology",
		Size:      "medium",
	}

	assert.Equal(t, "Test Legal Name", org.LegalName)
	assert.Equal(t, "Test Trade Name", org.TradeName)
	assert.Equal(t, "12-3456789", org.TaxID)
	assert.Equal(t, "123 Main St", org.Address.Line1)
	assert.Equal(t, "12345", org.Address.ZipCode)
	assert.Equal(t, "Test City", org.Address.City)
	assert.Equal(t, "TS", org.Address.State)
	assert.Equal(t, "US", org.Address.Country)
	assert.Equal(t, models.StatusActive, org.Status.Code)
	assert.Equal(t, "value", org.Metadata["key"])
	assert.Equal(t, "technology", org.Industry)
	assert.Equal(t, "medium", org.Size)
}

func TestAssetTemplateStructFromTemplates(t *testing.T) {
	asset := AssetTemplate{
		Name:     "Test Asset",
		Type:     "currency",
		Code:     "TST",
		Scale:    2,
		Metadata: map[string]any{"symbol": "$"},
	}

	assert.Equal(t, "Test Asset", asset.Name)
	assert.Equal(t, "currency", asset.Type)
	assert.Equal(t, "TST", asset.Code)
	assert.Equal(t, 2, asset.Scale)
	assert.Equal(t, "$", asset.Metadata["symbol"])
}

func TestAccountTemplateStruct(t *testing.T) {
	alias := "test_alias"
	parentID := "parent-123"
	portfolioID := "portfolio-456"
	segmentID := "segment-789"
	entityID := "entity-000"
	typeKey := "CHECKING"

	acc := AccountTemplate{
		Name:            "Test Account",
		Type:            "deposit",
		Status:          models.NewStatus(models.StatusActive),
		Alias:           &alias,
		ParentAccountID: &parentID,
		PortfolioID:     &portfolioID,
		SegmentID:       &segmentID,
		EntityID:        &entityID,
		AccountTypeKey:  &typeKey,
		Metadata:        map[string]any{"key": "value"},
	}

	assert.Equal(t, "Test Account", acc.Name)
	assert.Equal(t, "deposit", acc.Type)
	assert.Equal(t, models.StatusActive, acc.Status.Code)
	assert.Equal(t, "test_alias", *acc.Alias)
	assert.Equal(t, "parent-123", *acc.ParentAccountID)
	assert.Equal(t, "portfolio-456", *acc.PortfolioID)
	assert.Equal(t, "segment-789", *acc.SegmentID)
	assert.Equal(t, "entity-000", *acc.EntityID)
	assert.Equal(t, "CHECKING", *acc.AccountTypeKey)
	assert.Equal(t, "value", acc.Metadata["key"])
}

func TestAccountTemplateStructWithNilOptionalFields(t *testing.T) {
	acc := AccountTemplate{
		Name:            "Test Account",
		Type:            "deposit",
		Status:          models.NewStatus(models.StatusActive),
		Alias:           nil,
		ParentAccountID: nil,
		PortfolioID:     nil,
		SegmentID:       nil,
		EntityID:        nil,
		AccountTypeKey:  nil,
		Metadata:        nil,
	}

	assert.Equal(t, "Test Account", acc.Name)
	assert.Equal(t, "deposit", acc.Type)
	assert.Nil(t, acc.Alias)
	assert.Nil(t, acc.ParentAccountID)
	assert.Nil(t, acc.PortfolioID)
	assert.Nil(t, acc.SegmentID)
	assert.Nil(t, acc.EntityID)
	assert.Nil(t, acc.AccountTypeKey)
	assert.Nil(t, acc.Metadata)
}

func TestLedgerTemplateStruct(t *testing.T) {
	ledger := LedgerTemplate{
		Name:     "Test Ledger",
		Status:   models.NewStatus(models.StatusActive),
		Metadata: map[string]any{"region": "US"},
	}

	assert.Equal(t, "Test Ledger", ledger.Name)
	assert.Equal(t, models.StatusActive, ledger.Status.Code)
	assert.Equal(t, "US", ledger.Metadata["region"])
}

func TestLedgerTemplateStructWithNilMetadata(t *testing.T) {
	ledger := LedgerTemplate{
		Name:     "Test Ledger",
		Status:   models.NewStatus(models.StatusActive),
		Metadata: nil,
	}

	assert.Equal(t, "Test Ledger", ledger.Name)
	assert.Nil(t, ledger.Metadata)
}

func TestTransactionPatternStructFromTemplates(t *testing.T) {
	pattern := TransactionPattern{
		ChartOfAccountsGroupName: "test_group",
		Description:              "Test transaction pattern",
		DSLTemplate:              "send [USD 100] (source = @a)",
		RequiresCommit:           true,
		IdempotencyKey:           "idem-key-123",
		ExternalID:               "ext-id-456",
		Metadata:                 map[string]any{"pattern": "test"},
	}

	assert.Equal(t, "test_group", pattern.ChartOfAccountsGroupName)
	assert.Equal(t, "Test transaction pattern", pattern.Description)
	assert.Equal(t, "send [USD 100] (source = @a)", pattern.DSLTemplate)
	assert.True(t, pattern.RequiresCommit)
	assert.Equal(t, "idem-key-123", pattern.IdempotencyKey)
	assert.Equal(t, "ext-id-456", pattern.ExternalID)
	assert.Equal(t, "test", pattern.Metadata["pattern"])
}

func TestTransactionPatternStructWithDefaults(t *testing.T) {
	pattern := TransactionPattern{}

	assert.Empty(t, pattern.ChartOfAccountsGroupName)
	assert.Empty(t, pattern.Description)
	assert.Empty(t, pattern.DSLTemplate)
	assert.False(t, pattern.RequiresCommit)
	assert.Empty(t, pattern.IdempotencyKey)
	assert.Empty(t, pattern.ExternalID)
	assert.Nil(t, pattern.Metadata)
}

func TestTemplateStructsZeroValues(t *testing.T) {
	t.Run("OrgTemplate zero value", func(t *testing.T) {
		var org OrgTemplate
		assert.Empty(t, org.LegalName)
		assert.Empty(t, org.TradeName)
		assert.Empty(t, org.TaxID)
		assert.Empty(t, org.Industry)
		assert.Empty(t, org.Size)
		assert.Nil(t, org.Metadata)
	})

	t.Run("AssetTemplate zero value", func(t *testing.T) {
		var asset AssetTemplate
		assert.Empty(t, asset.Name)
		assert.Empty(t, asset.Type)
		assert.Empty(t, asset.Code)
		assert.Equal(t, 0, asset.Scale)
		assert.Nil(t, asset.Metadata)
	})

	t.Run("AccountTemplate zero value", func(t *testing.T) {
		var acc AccountTemplate
		assert.Empty(t, acc.Name)
		assert.Empty(t, acc.Type)
		assert.Nil(t, acc.Alias)
		assert.Nil(t, acc.ParentAccountID)
		assert.Nil(t, acc.PortfolioID)
		assert.Nil(t, acc.SegmentID)
		assert.Nil(t, acc.EntityID)
		assert.Nil(t, acc.AccountTypeKey)
		assert.Nil(t, acc.Metadata)
	})

	t.Run("LedgerTemplate zero value", func(t *testing.T) {
		var ledger LedgerTemplate
		assert.Empty(t, ledger.Name)
		assert.Nil(t, ledger.Metadata)
	})

	t.Run("TransactionPattern zero value", func(t *testing.T) {
		var pattern TransactionPattern
		assert.Empty(t, pattern.ChartOfAccountsGroupName)
		assert.Empty(t, pattern.Description)
		assert.Empty(t, pattern.DSLTemplate)
		assert.False(t, pattern.RequiresCommit)
		assert.Empty(t, pattern.IdempotencyKey)
		assert.Empty(t, pattern.ExternalID)
		assert.Nil(t, pattern.Metadata)
	})
}

func TestStrPtrUsageInTemplates(t *testing.T) {
	// Test that StrPtr works correctly when building templates
	acc := AccountTemplate{
		Name:           "Test Account",
		Type:           "deposit",
		Status:         models.NewStatus(models.StatusActive),
		Alias:          StrPtr("test_alias"),
		AccountTypeKey: StrPtr(AccountTypeKeyChecking),
		Metadata:       map[string]any{},
	}

	assert.NotNil(t, acc.Alias)
	assert.Equal(t, "test_alias", *acc.Alias)
	assert.NotNil(t, acc.AccountTypeKey)
	assert.Equal(t, AccountTypeKeyChecking, *acc.AccountTypeKey)
}

func TestTemplateMetadataTypes(t *testing.T) {
	// Test that metadata can hold various types
	metadata := map[string]any{
		"string_value":  "hello",
		"int_value":     42,
		"float_value":   3.14,
		"bool_value":    true,
		"array_value":   []string{"a", "b", "c"},
		"nested_object": map[string]any{"nested_key": "nested_value"},
	}

	org := OrgTemplate{
		LegalName: "Test",
		Metadata:  metadata,
	}

	assert.Equal(t, "hello", org.Metadata["string_value"])
	assert.Equal(t, 42, org.Metadata["int_value"])
	assert.Equal(t, 3.14, org.Metadata["float_value"])
	assert.Equal(t, true, org.Metadata["bool_value"])
	assert.Equal(t, []string{"a", "b", "c"}, org.Metadata["array_value"])

	nested, ok := org.Metadata["nested_object"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "nested_value", nested["nested_key"])
}
