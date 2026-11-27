// Copyright 2025 Lerian Studio
// SPDX-License-Identifier: Elastic-2.0

package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPICodeSuffix(t *testing.T) {
	cases := []struct {
		name string
		code string
		want string
	}{
		{name: "prefixed code", code: "CRM-0006", want: "0006"},
		{name: "bare code", code: "0084", want: "0084"},
		{name: "empty", code: "", want: ""},
		{name: "letters only", code: "ABC", want: ""},
		{name: "five digits", code: "12345", want: ""},
		{name: "three digit suffix", code: "CRM-006", want: ""},
		{name: "letter among digits", code: "12a4", want: ""},
		{name: "nothing after dash", code: "CRM-", want: ""},
		{name: "cuts at the last dash", code: "A-B-0006", want: "0006"},
		{name: "trailing space is not trimmed", code: "CRM-0006 ", want: ""},
		{name: "leading space on a bare code is not trimmed", code: " 0084", want: ""},
		{name: "leading space before the dash is cut away", code: " CRM-0006", want: "0006"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, APICodeSuffix(c.code))
		})
	}
}
