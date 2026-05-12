// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Apache-2.0

package errors

import "reflect"

// IsNilInterfaceValue reports whether value is either an untyped nil or a
// typed nil whose underlying value is nil — i.e. one of the cases where
// `value == nil` returns false but the value is operationally
// unusable.
//
// Go's typed-nil trap is the motivation:
//
//	var e *Error           // (e *Error)(nil), a typed nil
//	var i any = e          // i carries the type *Error and a nil pointer
//	fmt.Println(i == nil)  // false — the empty-interface comparison checks
//	                       //         that both the dynamic type AND value
//	                       //         are nil
//
// Code that branches on `err != nil` after assigning a typed-nil pointer
// to an `error` interface follows the wrong branch and eventually panics
// when it dereferences. IsNilInterfaceValue is the single canonical
// check used across pkg/errors, entities/http, and pkg/retry to detect
// both cases.
//
// The fast `value == nil` short-circuit means the hot path (most error
// returns are already nil-checked at the call site) skips the reflect
// machinery entirely. The reflect branch only runs on values where the
// interface header has a non-nil type pointer — typically wrapped errors
// that callers forgot to unwrap.
//
// pkg/retry maintains a private copy of this helper rather than
// importing pkg/errors to keep the retry package free of cross-package
// dependencies. If you change the semantics here, update
// pkg/retry/retry.go:isNilInterfaceValue in lockstep.
func IsNilInterfaceValue(value any) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
