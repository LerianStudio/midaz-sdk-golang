package concurrent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/concurrent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBatchProcessor_RedactsErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"error":"upstream failed access_token=secret-token"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()

	processor := concurrent.NewHTTPBatchProcessor(server.Client(), server.URL)
	_, err := processor.ExecuteBatch(context.Background(), []concurrent.HTTPBatchRequest{{
		Method: http.MethodGet,
		Path:   "/accounts",
	}})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-token")
	assert.Contains(t, err.Error(), "[REDACTED]")
}
