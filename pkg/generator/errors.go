package generator

import "errors"

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
