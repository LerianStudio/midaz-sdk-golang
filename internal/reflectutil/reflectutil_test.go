package reflectutil

import "testing"

type sampleStruct struct{}

func TestIsTypedNil(t *testing.T) {
	var (
		nilPointer   *sampleStruct
		nilSlice     []string
		nilMap       map[string]string
		nilChannel   chan int
		nilFunc      func()
		nilInterface any
	)

	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "literal nil", value: nil, want: false},
		{name: "nil interface", value: nilInterface, want: false},
		{name: "typed nil pointer", value: nilPointer, want: true},
		{name: "typed nil slice", value: nilSlice, want: true},
		{name: "typed nil map", value: nilMap, want: true},
		{name: "typed nil channel", value: nilChannel, want: true},
		{name: "typed nil func", value: nilFunc, want: true},
		{name: "non nil pointer", value: &sampleStruct{}, want: false},
		{name: "non nil slice", value: []string{"value"}, want: false},
		{name: "non nil map", value: map[string]string{"key": "value"}, want: false},
		{name: "struct value", value: sampleStruct{}, want: false},
		{name: "integer value", value: 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTypedNil(tt.value); got != tt.want {
				t.Fatalf("IsTypedNil() = %v, want %v", got, tt.want)
			}
		})
	}
}
