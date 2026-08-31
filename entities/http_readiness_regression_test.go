package entities

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_ErrorBodyExposureDefaultsOff(t *testing.T) {
	const upstreamBody = `{"message":"bad request","client_secret":"raw-secret"}`
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := io.Copy(w, strings.NewReader(upstreamBody))
		writeErrs <- err
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any
	err := client.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)

	requireHandlerNoError(t, writeErrs)
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Empty(t, sdkErr.GetUpstreamBody())
}

func TestHTTPClient_ErrorBodyExposureExplicitOptIn(t *testing.T) {
	const upstreamBody = `{"message":"bad request","client_secret":"raw-secret"}`
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := io.Copy(w, strings.NewReader(upstreamBody))
		writeErrs <- err
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client(), "", nil)
	client.SetExposeErrorBody(true)

	var out map[string]any
	err := client.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)

	requireHandlerNoError(t, writeErrs)
	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.JSONEq(t, upstreamBody, sdkErr.GetUpstreamBody())
}
