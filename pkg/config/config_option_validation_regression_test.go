package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfig_WithNilOption_ReturnsError(t *testing.T) {
	_, err := NewConfig(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "option cannot be nil")
}
