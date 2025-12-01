package generator

import (
	"context"
	"errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAccountGenerator struct {
	generateFunc func(ctx context.Context, orgID, ledgerID, assetCode string, template data.AccountTemplate) (*models.Account, error)
	callCount    int
	callOrder    []string
}

func (m *mockAccountGenerator) Generate(ctx context.Context, orgID, ledgerID, assetCode string, template data.AccountTemplate) (*models.Account, error) {
	m.callCount++

	m.callOrder = append(m.callOrder, template.Name)
	if m.generateFunc != nil {
		return m.generateFunc(ctx, orgID, ledgerID, assetCode, template)
	}

	return &models.Account{ID: "acc-" + template.Name, Name: template.Name}, nil
}

func (*mockAccountGenerator) GenerateBatch(_ context.Context, _, _, _ string, _ []data.AccountTemplate) ([]*models.Account, error) {
	return nil, nil
}

func TestNewAccountHierarchyGenerator(t *testing.T) {
	t.Run("Create with nil account generator", func(t *testing.T) {
		gen := NewAccountHierarchyGenerator(nil)
		assert.NotNil(t, gen)
	})

	t.Run("Create with account generator", func(t *testing.T) {
		mockAccGen := &mockAccountGenerator{}
		gen := NewAccountHierarchyGenerator(mockAccGen)
		assert.NotNil(t, gen)
	})
}

func TestAccountHierarchyGenerator_GenerateTree_NilAccountGenerator(t *testing.T) {
	gen := NewAccountHierarchyGenerator(nil)

	nodes := []AccountNode{
		{Template: data.AccountTemplate{Name: "Root"}},
	}

	_, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account generator not initialized")
}

func TestAccountHierarchyGenerator_GenerateTree_EmptyNodes(t *testing.T) {
	mockAccGen := &mockAccountGenerator{}
	gen := NewAccountHierarchyGenerator(mockAccGen)

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", []AccountNode{})
	require.NoError(t, err)
	assert.Empty(t, results)
	assert.Zero(t, mockAccGen.callCount)
}

func TestAccountHierarchyGenerator_GenerateTree_SingleNode(t *testing.T) {
	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, _, _, _ string, template data.AccountTemplate) (*models.Account, error) {
			return &models.Account{
				ID:   "acc-root",
				Name: template.Name,
			}, nil
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{
				Name: "Root Account",
				Type: "deposit",
			},
		},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "acc-root", results[0].ID)
	assert.Equal(t, 1, mockAccGen.callCount)
}

func TestAccountHierarchyGenerator_GenerateTree_WithChildren(t *testing.T) {
	var capturedTemplates []data.AccountTemplate

	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, _, _, _ string, template data.AccountTemplate) (*models.Account, error) {
			capturedTemplates = append(capturedTemplates, template)

			return &models.Account{
				ID:   "acc-" + template.Name,
				Name: template.Name,
			}, nil
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{
				Name: "Parent",
				Type: "deposit",
			},
			Children: []AccountNode{
				{
					Template: data.AccountTemplate{
						Name: "Child1",
						Type: "deposit",
					},
				},
				{
					Template: data.AccountTemplate{
						Name: "Child2",
						Type: "deposit",
					},
				},
			},
		},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 3, mockAccGen.callCount)

	assert.Nil(t, capturedTemplates[0].ParentAccountID)

	assert.NotNil(t, capturedTemplates[1].ParentAccountID)
	assert.Equal(t, "acc-Parent", *capturedTemplates[1].ParentAccountID)

	assert.NotNil(t, capturedTemplates[2].ParentAccountID)
	assert.Equal(t, "acc-Parent", *capturedTemplates[2].ParentAccountID)
}

func TestAccountHierarchyGenerator_GenerateTree_DeepHierarchy(t *testing.T) {
	mockAccGen := &mockAccountGenerator{}
	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{Name: "Level1"},
			Children: []AccountNode{
				{
					Template: data.AccountTemplate{Name: "Level2"},
					Children: []AccountNode{
						{
							Template: data.AccountTemplate{Name: "Level3"},
							Children: []AccountNode{
								{Template: data.AccountTemplate{Name: "Level4"}},
							},
						},
					},
				},
			},
		},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)
	assert.Len(t, results, 4)
	assert.Equal(t, 4, mockAccGen.callCount)

	assert.Equal(t, []string{"Level1", "Level2", "Level3", "Level4"}, mockAccGen.callOrder)
}

func TestAccountHierarchyGenerator_GenerateTree_MultipleRoots(t *testing.T) {
	mockAccGen := &mockAccountGenerator{}
	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{Template: data.AccountTemplate{Name: "Root1"}},
		{Template: data.AccountTemplate{Name: "Root2"}},
		{Template: data.AccountTemplate{Name: "Root3"}},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)
	assert.Len(t, results, 3)
	assert.Equal(t, 3, mockAccGen.callCount)
}

func TestAccountHierarchyGenerator_GenerateTree_MultipleRootsWithChildren(t *testing.T) {
	mockAccGen := &mockAccountGenerator{}
	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{Name: "Root1"},
			Children: []AccountNode{
				{Template: data.AccountTemplate{Name: "Child1A"}},
			},
		},
		{
			Template: data.AccountTemplate{Name: "Root2"},
			Children: []AccountNode{
				{Template: data.AccountTemplate{Name: "Child2A"}},
				{Template: data.AccountTemplate{Name: "Child2B"}},
			},
		},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, 5, mockAccGen.callCount)
}

func TestAccountHierarchyGenerator_GenerateTree_ErrorOnRootCreation(t *testing.T) {
	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, _, _, _ string, _ data.AccountTemplate) (*models.Account, error) {
			return nil, errors.New("root creation failed")
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{Template: data.AccountTemplate{Name: "Root"}},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root creation failed")
	assert.Nil(t, results)
}

func TestAccountHierarchyGenerator_GenerateTree_ErrorOnChildCreation(t *testing.T) {
	callCount := 0
	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, _, _, _ string, template data.AccountTemplate) (*models.Account, error) {
			callCount++
			if callCount == 2 {
				return nil, errors.New("child creation failed")
			}

			return &models.Account{ID: "acc-" + template.Name, Name: template.Name}, nil
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{Name: "Parent"},
			Children: []AccountNode{
				{Template: data.AccountTemplate{Name: "Child"}},
			},
		},
	}

	results, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "child creation failed")
	assert.Nil(t, results)
}

func TestAccountHierarchyGenerator_GenerateTree_PreservesTemplateFields(t *testing.T) {
	var capturedTemplate data.AccountTemplate

	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, _, _, _ string, template data.AccountTemplate) (*models.Account, error) {
			capturedTemplate = template
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	alias := "test-alias"
	entityID := "entity-123"
	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{
				Name:     "Test Account",
				Type:     "deposit",
				Alias:    &alias,
				EntityID: &entityID,
				Status:   models.NewStatus(models.StatusActive),
				Metadata: map[string]any{"key": "value"},
			},
		},
	}

	_, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)

	assert.Equal(t, "Test Account", capturedTemplate.Name)
	assert.Equal(t, "deposit", capturedTemplate.Type)
	assert.Equal(t, "test-alias", *capturedTemplate.Alias)
	assert.Equal(t, "entity-123", *capturedTemplate.EntityID)
	assert.NotNil(t, capturedTemplate.Status)
	assert.NotNil(t, capturedTemplate.Metadata)
}

func TestAccountHierarchyGenerator_GenerateTree_DoesNotMutateOriginalTemplate(t *testing.T) {
	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, _, _, _ string, _ data.AccountTemplate) (*models.Account, error) {
			return &models.Account{ID: "acc-123"}, nil
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	childTemplate := data.AccountTemplate{
		Name: "Child",
		Type: "deposit",
	}

	nodes := []AccountNode{
		{
			Template: data.AccountTemplate{Name: "Parent"},
			Children: []AccountNode{
				{Template: childTemplate},
			},
		},
	}

	_, err := gen.GenerateTree(context.Background(), "org-123", "ledger-123", "USD", nodes)
	require.NoError(t, err)

	assert.Nil(t, childTemplate.ParentAccountID)
}

func TestAccountNode_Structure(t *testing.T) {
	t.Run("Node with template only", func(t *testing.T) {
		node := AccountNode{
			Template: data.AccountTemplate{
				Name: "Test",
				Type: "deposit",
			},
		}

		assert.Equal(t, "Test", node.Template.Name)
		assert.Nil(t, node.Children)
	})

	t.Run("Node with children", func(t *testing.T) {
		node := AccountNode{
			Template: data.AccountTemplate{Name: "Parent"},
			Children: []AccountNode{
				{Template: data.AccountTemplate{Name: "Child1"}},
				{Template: data.AccountTemplate{Name: "Child2"}},
			},
		}

		assert.Equal(t, "Parent", node.Template.Name)
		assert.Len(t, node.Children, 2)
	})
}

func TestAccountHierarchyGenerator_GenerateTree_VerifyOrgLedgerAssetCode(t *testing.T) {
	var receivedOrgID, receivedLedgerID, receivedAssetCode string

	mockAccGen := &mockAccountGenerator{
		generateFunc: func(_ context.Context, orgID, ledgerID, assetCode string, _ data.AccountTemplate) (*models.Account, error) {
			receivedOrgID = orgID
			receivedLedgerID = ledgerID
			receivedAssetCode = assetCode

			return &models.Account{ID: "acc-123"}, nil
		},
	}

	gen := NewAccountHierarchyGenerator(mockAccGen)

	nodes := []AccountNode{
		{Template: data.AccountTemplate{Name: "Test"}},
	}

	_, err := gen.GenerateTree(context.Background(), "custom-org", "custom-ledger", "BRL", nodes)
	require.NoError(t, err)

	assert.Equal(t, "custom-org", receivedOrgID)
	assert.Equal(t, "custom-ledger", receivedLedgerID)
	assert.Equal(t, "BRL", receivedAssetCode)
}
