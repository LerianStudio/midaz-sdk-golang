package midaz

import (
	stderrors "errors"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/config"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewReturnsTypedConfigurationError verifies that midaz.New() returns a
// typed *errors.Error (Category=CategoryConfiguration) when construction fails.
// This is the v3 contract: setup mistakes are distinguishable from runtime
// API failures via errors.Is / errors.As.
func TestNewReturnsTypedConfigurationError(t *testing.T) {
	tests := []struct {
		name             string
		options          []Option
		wantUnwrapPhrase string
	}{
		{
			name:             "nil option at index 0",
			options:          []Option{nil},
			wantUnwrapPhrase: "", // nil option has no underlying err to unwrap
		},
		{
			name:             "nil option at index 1",
			options:          []Option{WithDebug(true), nil},
			wantUnwrapPhrase: "",
		},
		{
			name:             "WithConfig(nil)",
			options:          []Option{WithConfig(nil)},
			wantUnwrapPhrase: "config cannot be nil",
		},
		{
			name:             "WithBaseURL invalid",
			options:          []Option{WithBaseURL("://bad-url")},
			wantUnwrapPhrase: "invalid base URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.options...)
			require.Error(t, err)

			// All construction errors are typed configuration errors.
			assert.True(t, sdkerrors.IsConfigurationError(err),
				"expected ErrConfiguration, got %T: %v", err, err)
			require.ErrorIs(t, err, sdkerrors.ErrConfiguration,
				"errors.Is(err, ErrConfiguration) must return true")

			// Operation context is set so users know where it came from.
			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr, "should be *errors.Error")
			assert.Equal(t, "midaz.New", sdkErr.Operation,
				"operation must be midaz.New for construction errors")

			// Underlying option errors are reachable via Unwrap.
			if tt.wantUnwrapPhrase != "" {
				inner := stderrors.Unwrap(err)
				require.Error(t, inner, "wrapped error must be reachable via Unwrap")
				assert.Contains(t, inner.Error(), tt.wantUnwrapPhrase)
			}
		})
	}
}

// TestNewIndexesNilOptionInError verifies the nil-option index appears in the
// error message so callers can identify which option in their slice is nil.
func TestNewIndexesNilOptionInError(t *testing.T) {
	tests := []struct {
		name     string
		options  []Option
		wantText string
	}{
		{name: "nil at 0", options: []Option{nil}, wantText: "index 0"},
		{name: "nil at 2", options: []Option{WithDebug(true), WithDebug(false), nil}, wantText: "index 2"},
		{name: "nil at 5", options: []Option{
			WithDebug(true), WithDebug(false), WithDebug(true), WithDebug(false), WithDebug(true), nil,
		}, wantText: "index 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.options...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantText,
				"error message should identify the nil option's index")
		})
	}
}

// TestPublicConfigValidate exposes the validation rules as a method on Config.
// Advanced callers can use this to validate a Config they constructed via
// DefaultConfig() and mutated directly.
func TestPublicConfigValidate(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		cfg := createTestConfig(t)
		require.NoError(t, cfg.Validate())
	})

	t.Run("missing onboarding URL fails", func(t *testing.T) {
		cfg := createTestConfig(t)
		delete(cfg.ServiceURLs, config.ServiceOnboarding)
		require.Error(t, cfg.Validate())
	})

	t.Run("missing transaction URL fails", func(t *testing.T) {
		cfg := createTestConfig(t)
		delete(cfg.ServiceURLs, config.ServiceTransaction)
		require.Error(t, cfg.Validate())
	})
}

// TestNewWithValidConfigSucceeds is the happy path: a properly-configured
// client constructs cleanly with no error.
func TestNewWithValidConfigSucceeds(t *testing.T) {
	c, err := New(WithConfig(createTestConfig(t)))
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Entity)
	require.NotNil(t, c.Accounts, "Accounts service must be initialized via embedded Entity")
}
