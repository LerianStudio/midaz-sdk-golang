// Package reflectutil contains small reflect-based helpers shared across the
// SDK. It is intentionally placed under pkg/internal so it stays unexported to
// SDK consumers while being importable by every internal package.
package reflectutil

import "reflect"

// IsTypedNil reports whether value is a typed-nil interface — that is, an
// interface value whose dynamic type is non-nil but whose dynamic value is nil
// (e.g. a (*T)(nil) stored in an interface).
//
// IsTypedNil returns false when value is the literal untyped nil interface,
// because the caller should already have detected that with a plain == nil
// check; the helper exists specifically for the pointer-to-typed-nil pitfall
// that confuses Provider/HTTPClient/Config wiring.
//
// Returns false for any kind that cannot be nil (struct, int, etc.).
func IsTypedNil(value any) bool {
	if value == nil {
		return false
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
