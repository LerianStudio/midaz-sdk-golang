package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name string
		uuid string
		want bool
	}{
		{
			name: "Valid UUID lowercase",
			uuid: "123e4567-e89b-12d3-a456-426614174000",
			want: true,
		},
		{
			name: "Valid UUID uppercase",
			uuid: "123E4567-E89B-12D3-A456-426614174000",
			want: true,
		},
		{
			name: "Valid UUID mixed case",
			uuid: "123E4567-e89b-12D3-a456-426614174000",
			want: true,
		},
		{
			name: "Valid UUID all zeros",
			uuid: "00000000-0000-0000-0000-000000000000",
			want: true,
		},
		{
			name: "Valid UUID all F's",
			uuid: "ffffffff-ffff-ffff-ffff-ffffffffffff",
			want: true,
		},
		{
			name: "Empty string",
			uuid: "",
			want: false,
		},
		{
			name: "Invalid - missing dashes",
			uuid: "123e4567e89b12d3a456426614174000",
			want: false,
		},
		{
			name: "Invalid - wrong length first segment",
			uuid: "123e456-e89b-12d3-a456-426614174000",
			want: false,
		},
		{
			name: "Invalid - wrong length second segment",
			uuid: "123e4567-e89-12d3-a456-426614174000",
			want: false,
		},
		{
			name: "Invalid - wrong length third segment",
			uuid: "123e4567-e89b-12d-a456-426614174000",
			want: false,
		},
		{
			name: "Invalid - wrong length fourth segment",
			uuid: "123e4567-e89b-12d3-a45-426614174000",
			want: false,
		},
		{
			name: "Invalid - wrong length fifth segment",
			uuid: "123e4567-e89b-12d3-a456-42661417400",
			want: false,
		},
		{
			name: "Invalid - contains non-hex characters",
			uuid: "123g4567-e89b-12d3-a456-426614174000",
			want: false,
		},
		{
			name: "Invalid - extra characters",
			uuid: "123e4567-e89b-12d3-a456-426614174000x",
			want: false,
		},
		{
			name: "Invalid - spaces",
			uuid: " 123e4567-e89b-12d3-a456-426614174000",
			want: false,
		},
		{
			name: "Invalid - completely wrong format",
			uuid: "not-a-uuid",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidUUID(tt.uuid)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		scale  int64
		want   bool
	}{
		{
			name:   "Valid amount with scale 2",
			amount: 1000,
			scale:  2,
			want:   true,
		},
		{
			name:   "Valid amount with scale 0",
			amount: 1000,
			scale:  0,
			want:   true,
		},
		{
			name:   "Valid amount with scale 18",
			amount: 1000,
			scale:  18,
			want:   true,
		},
		{
			name:   "Valid zero amount",
			amount: 0,
			scale:  2,
			want:   true,
		},
		{
			name:   "Invalid negative amount",
			amount: -1000,
			scale:  2,
			want:   false,
		},
		{
			name:   "Invalid negative scale",
			amount: 1000,
			scale:  -1,
			want:   false,
		},
		{
			name:   "Invalid scale above 18",
			amount: 1000,
			scale:  19,
			want:   false,
		},
		{
			name:   "Invalid scale way above limit",
			amount: 1000,
			scale:  100,
			want:   false,
		},
		{
			name:   "Large valid amount",
			amount: 9223372036854775807, // max int64
			scale:  8,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidAmount(tt.amount, tt.scale)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidExternalAccountID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{
			name: "Valid external account USD",
			id:   "@external/USD",
			want: true,
		},
		{
			name: "Valid external account EUR",
			id:   "@external/EUR",
			want: true,
		},
		{
			name: "Valid external account BTC",
			id:   "@external/BTC",
			want: true,
		},
		{
			name: "Valid external account with 4 letters",
			id:   "@external/USDT",
			want: true,
		},
		{
			name: "Valid external account with lowercase (format only checks prefix)",
			id:   "@external/usd",
			want: true,
		},
		{
			name: "Empty string",
			id:   "",
			want: false,
		},
		{
			name: "Missing @ prefix",
			id:   "external/USD",
			want: false,
		},
		{
			name: "Different prefix",
			id:   "@internal/USD",
			want: false,
		},
		{
			name: "Regular account ID",
			id:   "acc_12345",
			want: false,
		},
		{
			name: "UUID account ID",
			id:   "123e4567-e89b-12d3-a456-426614174000",
			want: false,
		},
		{
			name: "External account without slash",
			id:   "@externalUSD",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidExternalAccountID(tt.id)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidAuthToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{
			name:  "Valid token - exactly 8 chars",
			token: "12345678",
			want:  true,
		},
		{
			name:  "Valid token - long token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0",
			want:  true,
		},
		{
			name:  "Valid token - alphanumeric",
			token: "abc12345def67890",
			want:  true,
		},
		{
			name:  "Empty string",
			token: "",
			want:  false,
		},
		{
			name:  "Too short - 7 chars",
			token: "1234567",
			want:  false,
		},
		{
			name:  "Too short - 1 char",
			token: "a",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidAuthToken(tt.token)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidMetadataValueType(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{
			name:  "Valid string",
			value: "test",
			want:  true,
		},
		{
			name:  "Valid empty string",
			value: "",
			want:  true,
		},
		{
			name:  "Valid bool true",
			value: true,
			want:  true,
		},
		{
			name:  "Valid bool false",
			value: false,
			want:  true,
		},
		{
			name:  "Valid int",
			value: 123,
			want:  true,
		},
		{
			name:  "Valid negative int",
			value: -123,
			want:  true,
		},
		{
			name:  "Valid zero int",
			value: 0,
			want:  true,
		},
		{
			name:  "Valid float64",
			value: 123.45,
			want:  true,
		},
		{
			name:  "Valid negative float64",
			value: -123.45,
			want:  true,
		},
		{
			name:  "Valid zero float64",
			value: 0.0,
			want:  true,
		},
		{
			name:  "Valid nil",
			value: nil,
			want:  true,
		},
		{
			name:  "Invalid int32",
			value: int32(123),
			want:  false,
		},
		{
			name:  "Invalid int64",
			value: int64(123),
			want:  false,
		},
		{
			name:  "Invalid float32",
			value: float32(123.45),
			want:  false,
		},
		{
			name:  "Invalid slice",
			value: []string{"a", "b"},
			want:  false,
		},
		{
			name:  "Invalid map",
			value: map[string]any{"key": "value"},
			want:  false,
		},
		{
			name:  "Invalid struct",
			value: struct{ Name string }{Name: "test"},
			want:  false,
		},
		{
			name:  "Invalid complex",
			value: complex(1, 2),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidMetadataValueType(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateMetadataSize(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		wantErr  bool
	}{
		{
			name:     "Nil metadata",
			metadata: nil,
			wantErr:  false,
		},
		{
			name:     "Empty metadata",
			metadata: map[string]any{},
			wantErr:  false,
		},
		{
			name: "Small metadata",
			metadata: map[string]any{
				"key": "value",
			},
			wantErr: false,
		},
		{
			name: "Metadata with various types",
			metadata: map[string]any{
				"string":  "value",
				"bool":    true,
				"int":     123,
				"float64": 123.45,
			},
			wantErr: false,
		},
		{
			name: "Metadata at boundary - just under 4KB",
			metadata: func() map[string]any {
				m := make(map[string]any)
				// Create metadata with total size just under 4096
				for i := 0; i < 50; i++ {
					key := string(make([]byte, 20)) // 20 bytes per key
					m[key+string(rune(i))] = string(make([]byte, 50))
				}

				return m
			}(),
			wantErr: false,
		},
		{
			name: "Metadata exceeds 4KB",
			metadata: func() map[string]any {
				m := make(map[string]any)
				// Create metadata that exceeds 4096 bytes
				for i := 0; i < 100; i++ {
					key := string(make([]byte, 20))
					m[key+string(rune(i))] = string(make([]byte, 50))
				}

				return m
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMetadataSize(tt.metadata)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrMetadataSizeExceeded)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	t.Run("Error returns message", func(t *testing.T) {
		err := &Error{Message: "test error message"}
		assert.Equal(t, "test error message", err.Error())
	})

	t.Run("ErrMetadataSizeExceeded has correct message", func(t *testing.T) {
		assert.Contains(t, ErrMetadataSizeExceeded.Error(), "metadata size")
		assert.Contains(t, ErrMetadataSizeExceeded.Error(), "4KB")
	})
}
