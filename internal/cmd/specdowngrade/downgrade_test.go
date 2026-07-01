package main

import (
	"strings"
	"testing"
)

func TestDowngrade(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		mustContain []string
		mustNotHave []string
	}{
		{
			name: "nullable string type-array becomes type+nullable",
			in: "openapi: 3.1.0\ncomponents:\n  schemas:\n    X:\n      properties:\n        a:\n          type:\n            - string\n            - \"null\"\n",
			mustContain: []string{"type: string", "nullable: true"},
			mustNotHave: []string{`"null"`},
		},
		{
			name: "nullable array type-array becomes type+nullable",
			in: "openapi: 3.1.0\nx:\n  type:\n    - array\n    - \"null\"\n",
			mustContain: []string{"type: array", "nullable: true"},
			mustNotHave: []string{`"null"`},
		},
		{
			name:        "format boolean on boolean is stripped",
			in:          "openapi: 3.1.0\nx:\n  format: boolean\n  type: boolean\n",
			mustContain: []string{"type: boolean"},
			mustNotHave: []string{"format: boolean"},
		},
		{
			name:        "format uuid on string is preserved",
			in:          "openapi: 3.1.0\nx:\n  format: uuid\n  type: string\n",
			mustContain: []string{"format: uuid", "type: string"},
		},
		{
			name:        "format int64 on integer is preserved",
			in:          "openapi: 3.1.0\nx:\n  format: int64\n  type: integer\n",
			mustContain: []string{"format: int64", "type: integer"},
		},
		{
			name:        "format on nullable string type-array is preserved after collapse",
			in:          "openapi: 3.1.0\nx:\n  format: uuid\n  type:\n    - string\n    - \"null\"\n",
			mustContain: []string{"format: uuid", "type: string", "nullable: true"},
			mustNotHave: []string{`"null"`},
		},
		{
			name:        "openapi version flipped to 3.0.3",
			in:          "openapi: 3.1.0\nx: 1\n",
			mustContain: []string{"openapi: 3.0.3"},
			mustNotHave: []string{"3.1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Downgrade([]byte(tt.in))
			if err != nil {
				t.Fatalf("Downgrade returned error: %v", err)
			}
			out := string(got)
			for _, want := range tt.mustContain {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, out)
				}
			}
			for _, bad := range tt.mustNotHave {
				if strings.Contains(out, bad) {
					t.Errorf("output should not contain %q\ngot:\n%s", bad, out)
				}
			}
		})
	}
}

func TestDowngradeDeterministic(t *testing.T) {
	in := []byte("openapi: 3.1.0\ncomponents:\n  schemas:\n    X:\n      properties:\n        a:\n          format: uuid\n          type:\n            - string\n            - \"null\"\n        b:\n          format: boolean\n          type: boolean\n")

	first, err := Downgrade(in)
	if err != nil {
		t.Fatalf("first Downgrade error: %v", err)
	}
	// Idempotent: transforming twice equals transforming once.
	second, err := Downgrade(in)
	if err != nil {
		t.Fatalf("second Downgrade error: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("non-deterministic output:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
