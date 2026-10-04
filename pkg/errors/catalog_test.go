package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogCodePredicates(t *testing.T) {
	cases := []struct {
		name    string
		apiCode string
		pred    func(error) bool
		want    bool
	}{
		{"skip-not-permitted match", "0490", IsSkipNotPermitted, true},
		{"skip-not-permitted miss", "0491", IsSkipNotPermitted, false},
		{"holder-required match", "0491", IsHolderRequired, true},
		{"holder-required miss", "0490", IsHolderRequired, false},
		{"holder-not-found match", "CRM-0006", IsHolderNotFound, true},
		{"holder-not-found miss (unmodifiable 0006)", "0006", IsHolderNotFound, false},
		{"fee low bound 0179", "0179", IsFeeError, true},
		{"fee high bound 0233", "0233", IsFeeError, true},
		{"fee mid 0186", "0186", IsFeeError, true},
		{"fee below range 0178", "0178", IsFeeError, false},
		{"fee above range 0234", "0234", IsFeeError, false},
		{"fee prefixed wire form", "LEDGER-0200", IsFeeError, true},
		{"fee non-numeric code", "CRM-0006", IsFeeError, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.pred(&Error{APICode: c.apiCode}))
		})
	}
}

// TestIsTransactionReservationDenied decodes the problem+json body midaz v4.2
// returns: every 422 Tracer refusal matches; the 503 unavailability codes do not.
func TestIsTransactionReservationDenied(t *testing.T) {
	cases := []struct {
		code   string
		status int
		want   bool
	}{
		{"0177", 422, true}, // usage limit exceeded
		{"0531", 422, true}, // rule asked for review or could not be evaluated
		{"0532", 422, true}, // Tracer rejected the reservation request
		{"0535", 422, true}, // rule denied
		{"0178", 503, false},
		{"0536", 503, false},
		{"0200", 422, false}, // fee engine
	}

	for _, c := range cases {
		t.Run(c.code, func(t *testing.T) {
			body := fmt.Sprintf(`{"type":"about:blank","title":"Tracer","status":%d,"detail":"refused","code":%q}`, c.status, c.code)
			assert.Equal(t, c.want, IsTransactionReservationDenied(DecodeProblemJSON(c.status, []byte(body), "req")))
		})
	}
}

func TestCatalogPredicatesNilSafe(t *testing.T) {
	for _, pred := range []func(error) bool{
		IsSkipNotPermitted, IsTransactionReservationDenied, IsHolderRequired, IsHolderNotFound, IsFeeError,
		IsFeatureNotAvailable,
	} {
		assert.False(t, pred(nil))
	}
}

// TestFeatureNotAvailableDistinctFromNotFound is the money-path-adjacent
// guard: the encryption legacy-mode 404 must be recognizable via
// IsFeatureNotAvailable WITHOUT a generic NotFound satisfying it.
func TestFeatureNotAvailableDistinctFromNotFound(t *testing.T) {
	generic := &Error{Category: CategoryNotFound, StatusCode: 404}
	require.True(t, IsNotFoundError(generic))
	assert.False(t, IsFeatureNotAvailable(generic),
		"a generic NotFound must NOT be classified FeatureNotAvailable")

	tagged := MarkFeatureNotAvailable(generic)
	assert.True(t, IsFeatureNotAvailable(tagged),
		"a tagged legacy-mode 404 must be FeatureNotAvailable")
	assert.True(t, IsNotFoundError(tagged),
		"the tagged 404 must still be NotFound (underlying *Error preserved via errors.As)")

	// MarkFeatureNotAvailable is a no-op on non-NotFound and on nil.
	nonNF := &Error{Category: CategoryValidation, StatusCode: 422}
	assert.False(t, IsFeatureNotAvailable(MarkFeatureNotAvailable(nonNF)))
	assert.NoError(t, MarkFeatureNotAvailable(nil))
}
