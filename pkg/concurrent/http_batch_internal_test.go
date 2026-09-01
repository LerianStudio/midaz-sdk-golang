package concurrent

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPBatchProcessorsInstallRedirectPolicy(t *testing.T) {
	previous, err := http.NewRequest(http.MethodPost, "https://api.example/batch", bytes.NewReader([]byte(`[]`)))
	require.NoError(t, err)
	previous.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte(`[]`))), nil
	}

	next, err := http.NewRequest(http.MethodGet, "https://evil.example/batch", nil)
	require.NoError(t, err)

	processor := NewHTTPBatchProcessor(&http.Client{}, "https://api.example", WithBatchRetryCount(0))
	require.NotNil(t, processor.httpClient.CheckRedirect)
	require.ErrorIs(t, processor.httpClient.CheckRedirect(next, []*http.Request{previous}), security.ErrAuthenticatedRedirect)

	retryProcessor, err := NewHTTPBatchProcessorWithRetry(&http.Client{}, "https://api.example", WithBatchRetryCount(0))
	require.NoError(t, err)
	require.NotNil(t, retryProcessor.httpClient.CheckRedirect)
	require.ErrorIs(t, retryProcessor.httpClient.CheckRedirect(next, []*http.Request{previous}), security.ErrAuthenticatedRedirect)
}

func TestHTTPBatchProcessorRedirectPolicyComposesCallerPolicy(t *testing.T) {
	var called bool
	callerClient := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		called = true
		return assert.AnError
	}}

	processor := NewHTTPBatchProcessor(callerClient, "https://api.example", WithBatchRetryCount(0))
	require.NotSame(t, callerClient, processor.httpClient)

	previous, err := http.NewRequest(http.MethodGet, "https://api.example/batch", nil)
	require.NoError(t, err)
	next, err := http.NewRequest(http.MethodGet, "https://api.example/batch?page=2", nil)
	require.NoError(t, err)

	require.ErrorIs(t, processor.httpClient.CheckRedirect(next, []*http.Request{previous}), assert.AnError)
	assert.True(t, called)
}
