package entities

import (
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v5/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestRequirePathIDsRejectsOddArguments covers the branch that fires when a
// caller inside this package passes an unpaired name/value list.
//
// It is a programming mistake, not a caller mistake, so it must not report as a
// validation failure: an SDK user cannot fix it, and classifying it as
// "your input was bad" would send them looking at their own ids. It reports as
// an internal error instead, which is the honest classification and the one
// that shows up as an SDK bug rather than a customer bug.
func TestRequirePathIDsRejectsOddArguments(t *testing.T) {
	err := requirePathIDs("Balances.GetBalance", "organizationID", "org-1", "balanceID")

	require.Error(t, err)
	require.False(t, sdkerrors.IsValidationError(err),
		"an unpaired argument list is an SDK defect, not a caller mistake, got %v", err)
	require.Contains(t, err.Error(), "name/value pairs must be even")
}
