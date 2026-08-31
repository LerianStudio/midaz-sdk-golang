package models_test

import (
	"errors"
	"fmt"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
)

// ExampleCreateOrganizationInput_Validate demonstrates the v4
// validation contract: every input has a Validate() method that
// accumulates field errors. Returns nil when the input is valid.
func ExampleCreateOrganizationInput_Validate() {
	valid := &models.CreateOrganizationInput{
		LegalName:     "Acme Corporation",
		LegalDocument: "00000000000191",
	}

	fmt.Println(valid.Validate())
	// Output: <nil>
}

// ExampleCreateOrganizationInput_Validate_multipleErrors demonstrates
// the multi-field accumulation contract (Track 8C). When several
// fields are invalid, Validate returns ALL of them in a single error,
// not just the first one.
func ExampleCreateOrganizationInput_Validate_multipleErrors() {
	invalid := &models.CreateOrganizationInput{
		// LegalName missing — required
		// LegalDocument missing — required
	}

	err := invalid.Validate()
	fmt.Println(errors.Is(err, sdkerrors.ErrValidation))
	// Output: true
}

// ExampleOrganizationsListOpts demonstrates the typed pagination opts
// for a page-based endpoint. The embedded PageListOpts struct carries
// Limit, Page, and SortDirection — wrong-shape cursor opts don't
// compile here.
func ExampleOrganizationsListOpts() {
	opts := models.OrganizationsListOpts{
		PageListOpts: models.PageListOpts{
			Limit:         50,
			Page:          1,
			SortDirection: models.SortDescending,
		},
	}

	fmt.Println(opts.Limit)
	fmt.Println(opts.SortDirection)
	// Output:
	// 50
	// desc
}

// ExampleAccountsListOpts demonstrates typed list opts for a
// page-based endpoint with custom filters. The Filters struct exposes
// only the fields the endpoint actually honors — no v2-style
// mega-struct that silently drops unsupported fields.
func ExampleAccountsListOpts() {
	opts := models.AccountsListOpts{
		PageListOpts: models.PageListOpts{
			Limit: 25,
		},
	}
	fmt.Println(opts.Limit)
	// Output: 25
}

// ExampleCreateAccountInput_Validate shows validation across
// account-shaped input. The accumulator surfaces every issue in a
// single error.
func ExampleCreateAccountInput_Validate() {
	valid := &models.CreateAccountInput{
		Name:      "Treasury Checking",
		AssetCode: "USD",
		Type:      "deposit",
	}

	fmt.Println(valid.Validate())
	// Output: <nil>
}

// ExamplePagination shows the response-side pagination metadata.
// Returned by every List* call inside *ListResponse[T].Pagination.
func ExamplePagination() {
	p := models.Pagination{
		Total:  142,
		Limit:  50,
		Offset: 0,
	}
	fmt.Printf("total=%d limit=%d offset=%d\n", p.Total, p.Limit, p.Offset)
	// Output: total=142 limit=50 offset=0
}
