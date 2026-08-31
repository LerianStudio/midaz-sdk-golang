package midaz_test

import (
	"reflect"
	"testing"

	midaz "github.com/LerianStudio/midaz-sdk-golang/v6"
	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

// --- from types_contract_test.go ---

// TestTypeAliasesAreIdentical verifies that the Batch 1G top-level type aliases
// preserve type identity with their counterparts in the models package. If any
// of these assertions fail, it means a re-export was downgraded from a type
// alias (`type X = Y`) to a distinct named type (`type X Y`), which silently
// breaks interface assignments and method-set inheritance.
//
// We use reflect.TypeOf(zero value) and compare. Two type aliases that point to
// the same underlying type produce identical reflect.Type values.
func TestTypeAliasesAreIdentical(t *testing.T) {
	cases := []struct {
		name   string
		alias  any
		source any
	}{
		// Resource entities.
		{"Account", midaz.Account{}, models.Account{}},
		{"AccountType", midaz.AccountType{}, models.AccountType{}},
		{"Asset", midaz.Asset{}, models.Asset{}},
		{"AssetRate", midaz.AssetRate{}, models.AssetRate{}},
		{"Balance", midaz.Balance{}, models.Balance{}},
		{"Holder", midaz.Holder{}, models.Holder{}},
		{"Ledger", midaz.Ledger{}, models.Ledger{}},
		{"MetadataIndex", midaz.MetadataIndex{}, models.MetadataIndex{}},
		{"Operation", midaz.Operation{}, models.Operation{}},
		{"OperationRoute", midaz.OperationRoute{}, models.OperationRoute{}},
		{"Organization", midaz.Organization{}, models.Organization{}},
		{"Portfolio", midaz.Portfolio{}, models.Portfolio{}},
		{"Segment", midaz.Segment{}, models.Segment{}},
		{"Transaction", midaz.Transaction{}, models.Transaction{}},
		{"TransactionRoute", midaz.TransactionRoute{}, models.TransactionRoute{}},

		// Create inputs.
		{"CreateAccountInput", midaz.CreateAccountInput{}, models.CreateAccountInput{}},
		{"CreateAccountTypeInput", midaz.CreateAccountTypeInput{}, models.CreateAccountTypeInput{}},
		{"CreateAssetInput", midaz.CreateAssetInput{}, models.CreateAssetInput{}},
		{"CreateAssetRateInput", midaz.CreateAssetRateInput{}, models.CreateAssetRateInput{}},
		{"CreateBalanceInput", midaz.CreateBalanceInput{}, models.CreateBalanceInput{}},
		{"CreateHolderInput", midaz.CreateHolderInput{}, models.CreateHolderInput{}},
		{"CreateLedgerInput", midaz.CreateLedgerInput{}, models.CreateLedgerInput{}},
		{"CreateMetadataIndexInput", midaz.CreateMetadataIndexInput{}, models.CreateMetadataIndexInput{}},
		{"CreateOperationRouteInput", midaz.CreateOperationRouteInput{}, models.CreateOperationRouteInput{}},
		{"CreateOrganizationInput", midaz.CreateOrganizationInput{}, models.CreateOrganizationInput{}},
		{"CreatePortfolioInput", midaz.CreatePortfolioInput{}, models.CreatePortfolioInput{}},
		{"CreateSegmentInput", midaz.CreateSegmentInput{}, models.CreateSegmentInput{}},
		{"CreateTransactionInput", midaz.CreateTransactionInput{}, models.CreateTransactionInput{}},
		{"CreateTransactionRouteInput", midaz.CreateTransactionRouteInput{}, models.CreateTransactionRouteInput{}},

		// Update inputs.
		{"UpdateAccountInput", midaz.UpdateAccountInput{}, models.UpdateAccountInput{}},
		{"UpdateAccountTypeInput", midaz.UpdateAccountTypeInput{}, models.UpdateAccountTypeInput{}},
		{"UpdateAssetInput", midaz.UpdateAssetInput{}, models.UpdateAssetInput{}},
		{"UpdateBalanceInput", midaz.UpdateBalanceInput{}, models.UpdateBalanceInput{}},
		{"UpdateHolderInput", midaz.UpdateHolderInput{}, models.UpdateHolderInput{}},
		{"UpdateLedgerInput", midaz.UpdateLedgerInput{}, models.UpdateLedgerInput{}},
		{"UpdateOperationInput", midaz.UpdateOperationInput{}, models.UpdateOperationInput{}},
		{"UpdateOperationRouteInput", midaz.UpdateOperationRouteInput{}, models.UpdateOperationRouteInput{}},
		{"UpdateOrganizationInput", midaz.UpdateOrganizationInput{}, models.UpdateOrganizationInput{}},
		{"UpdatePortfolioInput", midaz.UpdatePortfolioInput{}, models.UpdatePortfolioInput{}},
		{"UpdateSegmentInput", midaz.UpdateSegmentInput{}, models.UpdateSegmentInput{}},
		{"UpdateTransactionInput", midaz.UpdateTransactionInput{}, models.UpdateTransactionInput{}},
		{"UpdateTransactionRouteInput", midaz.UpdateTransactionRouteInput{}, models.UpdateTransactionRouteInput{}},

		// Transaction sub-DTOs.
		{"AmountInput", midaz.AmountInput{}, models.AmountInput{}},
		{"DistributeInput", midaz.DistributeInput{}, models.DistributeInput{}},
		{"FromToInput", midaz.FromToInput{}, models.FromToInput{}},
		{"SendInput", midaz.SendInput{}, models.SendInput{}},
		{"SourceInput", midaz.SourceInput{}, models.SourceInput{}},

		// Pagination & lists.
		{"Pagination", midaz.Pagination{}, models.Pagination{}},

		// Common.
		{"Status", midaz.Status{}, models.Status{}},
		{"Address", midaz.Address{}, models.Address{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			aliasType := reflect.TypeOf(tc.alias)
			sourceType := reflect.TypeOf(tc.source)
			if aliasType != sourceType {
				t.Errorf("midaz.%s and models.%s are not the same type:\n  midaz.%s  = %v\n  models.%s = %v",
					tc.name, tc.name, tc.name, aliasType, tc.name, sourceType)
			}
		})
	}
}

// TestGenericListResponseAlias verifies the parameterized generic alias works.
// Go 1.24+ supports generic type aliases; ListResponse[T] is the only generic
// in the alias surface today.
func TestGenericListResponseAlias(t *testing.T) {
	var alias midaz.ListResponse[models.Account]
	var source models.ListResponse[models.Account]

	if reflect.TypeOf(alias) != reflect.TypeOf(source) {
		t.Errorf("midaz.ListResponse[models.Account] and models.ListResponse[models.Account] are not identical")
	}

	// Verify mutual assignability — the canonical proof of type identity.
	alias = source
	source = alias
	_ = alias
	_ = source
}

// TestAliasesUsableInUserFlow simulates a typical user code path that mixes
// midaz.* and models.* types. With type aliases, the values flow through
// without conversions or boxing.
func TestAliasesUsableInUserFlow(t *testing.T) {
	// User writes a function that takes a midaz.CreateAccountInput.
	// Behind the scenes this is models.CreateAccountInput, so we can
	// pass it to functions that expect either form.
	input := midaz.CreateAccountInput{
		Name:      "Test Account",
		AssetCode: "USD",
		Type:      "deposit",
	}

	// Pass to a function expecting the models version: works because of alias identity.
	acceptModelsInput := func(in models.CreateAccountInput) string { return in.Name }
	if got := acceptModelsInput(input); got != "Test Account" {
		t.Errorf("alias did not pass cleanly to models.CreateAccountInput consumer: got %q", got)
	}

	// And the reverse direction.
	modelsInput := models.CreateAccountInput{Name: "From models", AssetCode: "EUR", Type: "deposit"}
	acceptMidazInput := func(in midaz.CreateAccountInput) string { return in.Name }
	if got := acceptMidazInput(modelsInput); got != "From models" {
		t.Errorf("models did not pass cleanly to midaz.CreateAccountInput consumer: got %q", got)
	}
}
