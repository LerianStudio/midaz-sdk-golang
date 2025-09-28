package generator

import (
	"context"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	data "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/data"
)

// AccountNode represents a hierarchical account template tree.
type AccountNode struct {
	Template data.AccountTemplate
	Children []AccountNode
}

// AccountHierarchyGenerator creates account hierarchies.
type AccountHierarchyGenerator struct {
	accGen AccountGenerator
}

func NewAccountHierarchyGenerator(accGen AccountGenerator) *AccountHierarchyGenerator {
	return &AccountHierarchyGenerator{accGen: accGen}
}

// GenerateTree creates a hierarchy of accounts. Parent accounts are created first; children
// receive ParentAccountID automatically. Returns all created accounts in pre-order.
func (h *AccountHierarchyGenerator) GenerateTree(
	ctx context.Context,
	orgID, ledgerID, assetCode string,
	nodes []AccountNode,
) ([]*models.Account, error) {
	if h.accGen == nil {
		return nil, fmt.Errorf("account generator not initialized")
	}

	var out []*models.Account

	for _, n := range nodes {
		created, err := h.createNode(ctx, orgID, ledgerID, assetCode, nil, n)
		if err != nil {
			return nil, err
		}

		out = append(out, created...)
	}

	return out, nil
}

func (h *AccountHierarchyGenerator) createNode(
	ctx context.Context,
	orgID, ledgerID, assetCode string,
	parentID *string,
	node AccountNode,
) ([]*models.Account, error) {
	// Copy template to avoid mutating caller data
	t := node.Template
	if parentID != nil {
		t.ParentAccountID = parentID
	}

	acc, err := h.accGen.Generate(ctx, orgID, ledgerID, assetCode, t)
	if err != nil {
		return nil, err
	}

	created := []*models.Account{acc}
	// Recurse
	for _, child := range node.Children {
		c, err := h.createNode(ctx, orgID, ledgerID, assetCode, &acc.ID, child)
		if err != nil {
			return nil, err
		}

		created = append(created, c...)
	}

	return created, nil
}
