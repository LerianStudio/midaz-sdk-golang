package midaz

import (
	"reflect"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
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
		{"Account", Account{}, models.Account{}},
		{"AccountType", AccountType{}, models.AccountType{}},
		{"Alias", Alias{}, models.Alias{}},
		{"Asset", Asset{}, models.Asset{}},
		{"AssetRate", AssetRate{}, models.AssetRate{}},
		{"Balance", Balance{}, models.Balance{}},
		{"Holder", Holder{}, models.Holder{}},
		{"Ledger", Ledger{}, models.Ledger{}},
		{"MetadataIndex", MetadataIndex{}, models.MetadataIndex{}},
		{"Operation", Operation{}, models.Operation{}},
		{"OperationRoute", OperationRoute{}, models.OperationRoute{}},
		{"Organization", Organization{}, models.Organization{}},
		{"Portfolio", Portfolio{}, models.Portfolio{}},
		{"Segment", Segment{}, models.Segment{}},
		{"Transaction", Transaction{}, models.Transaction{}},
		{"TransactionRoute", TransactionRoute{}, models.TransactionRoute{}},

		// Create inputs.
		{"CreateAccountInput", CreateAccountInput{}, models.CreateAccountInput{}},
		{"CreateAccountTypeInput", CreateAccountTypeInput{}, models.CreateAccountTypeInput{}},
		{"CreateAliasInput", CreateAliasInput{}, models.CreateAliasInput{}},
		{"CreateAssetInput", CreateAssetInput{}, models.CreateAssetInput{}},
		{"CreateAssetRateInput", CreateAssetRateInput{}, models.CreateAssetRateInput{}},
		{"CreateBalanceInput", CreateBalanceInput{}, models.CreateBalanceInput{}},
		{"CreateHolderInput", CreateHolderInput{}, models.CreateHolderInput{}},
		{"CreateLedgerInput", CreateLedgerInput{}, models.CreateLedgerInput{}},
		{"CreateMetadataIndexInput", CreateMetadataIndexInput{}, models.CreateMetadataIndexInput{}},
		{"CreateOperationInput", CreateOperationInput{}, models.CreateOperationInput{}},
		{"CreateOperationRouteInput", CreateOperationRouteInput{}, models.CreateOperationRouteInput{}},
		{"CreateOrganizationInput", CreateOrganizationInput{}, models.CreateOrganizationInput{}},
		{"CreatePortfolioInput", CreatePortfolioInput{}, models.CreatePortfolioInput{}},
		{"CreateSegmentInput", CreateSegmentInput{}, models.CreateSegmentInput{}},
		{"CreateTransactionInput", CreateTransactionInput{}, models.CreateTransactionInput{}},
		{"CreateTransactionRouteInput", CreateTransactionRouteInput{}, models.CreateTransactionRouteInput{}},

		// Update inputs.
		{"UpdateAccountInput", UpdateAccountInput{}, models.UpdateAccountInput{}},
		{"UpdateAccountTypeInput", UpdateAccountTypeInput{}, models.UpdateAccountTypeInput{}},
		{"UpdateAliasInput", UpdateAliasInput{}, models.UpdateAliasInput{}},
		{"UpdateAssetInput", UpdateAssetInput{}, models.UpdateAssetInput{}},
		{"UpdateBalanceInput", UpdateBalanceInput{}, models.UpdateBalanceInput{}},
		{"UpdateHolderInput", UpdateHolderInput{}, models.UpdateHolderInput{}},
		{"UpdateLedgerInput", UpdateLedgerInput{}, models.UpdateLedgerInput{}},
		{"UpdateOperationInput", UpdateOperationInput{}, models.UpdateOperationInput{}},
		{"UpdateOperationRouteInput", UpdateOperationRouteInput{}, models.UpdateOperationRouteInput{}},
		{"UpdateOrganizationInput", UpdateOrganizationInput{}, models.UpdateOrganizationInput{}},
		{"UpdatePortfolioInput", UpdatePortfolioInput{}, models.UpdatePortfolioInput{}},
		{"UpdateSegmentInput", UpdateSegmentInput{}, models.UpdateSegmentInput{}},
		{"UpdateTransactionInput", UpdateTransactionInput{}, models.UpdateTransactionInput{}},
		{"UpdateTransactionRouteInput", UpdateTransactionRouteInput{}, models.UpdateTransactionRouteInput{}},

		// Transaction sub-DTOs.
		{"AmountInput", AmountInput{}, models.AmountInput{}},
		{"DistributeInput", DistributeInput{}, models.DistributeInput{}},
		{"FromToInput", FromToInput{}, models.FromToInput{}},
		{"SendInput", SendInput{}, models.SendInput{}},
		{"SourceInput", SourceInput{}, models.SourceInput{}},

		// Pagination & lists.
		{"Pagination", Pagination{}, models.Pagination{}},

		// Common.
		{"Status", Status{}, models.Status{}},
		{"Address", Address{}, models.Address{}},
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
	var alias ListResponse[models.Account]
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
	input := CreateAccountInput{
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
	acceptMidazInput := func(in CreateAccountInput) string { return in.Name }
	if got := acceptMidazInput(modelsInput); got != "From models" {
		t.Errorf("models did not pass cleanly to midaz.CreateAccountInput consumer: got %q", got)
	}
}
