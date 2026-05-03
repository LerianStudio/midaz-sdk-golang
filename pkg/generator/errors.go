package generator

import (
	"errors"
	"fmt"
)

// errorsJoin aggregates multiple errors using errors.Join.
// Returns nil when no errors, or the single error when only one.
func errorsJoin(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}

	if len(errs) == 1 {
		return errs[0]
	}

	return errors.Join(errs...)
}

func errNilGenerated(entity string) error {
	return fmt.Errorf("%s generation returned nil response", entity)
}
