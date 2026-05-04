package security

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlice8ValidateOutboundRequestRejectsUserinfo(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://user:pass@example.com", nil)
	require.NoError(t, err)
	require.ErrorContains(t, ValidateOutboundRequest(req), "user information")
}
