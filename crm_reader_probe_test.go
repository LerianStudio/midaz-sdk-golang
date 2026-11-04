package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

func TestCRMReaderProbe_InitialUnauthorizedPreservesStatus(t *testing.T) {
	t.Parallel()
	srv := newCRMServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	reader := newTestCRMReader(t, srv.URL, nil)
	_, err := reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.Error(t, err)
	status, ok := sdkerrors.ActualHTTPStatus(err)
	require.True(t, ok)
	require.Equal(t, http.StatusUnauthorized, status)
	require.True(t, sdkerrors.IsAuthenticationError(err))
}

func TestCRMReaderProbe_HolderNotFoundRemainsNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(ledgerVersion422))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	reader := newTestCRMReader(t, srv.URL, nil)
	_, err := reader.Holder(context.Background(), crmTestOrg, crmTestHolderID)
	require.ErrorIs(t, err, sdkerrors.ErrNotFound)
}

func TestCRMReaderProbe_CallbackCanReadCachedResolution(t *testing.T) {
	t.Parallel()
	srv := newCRMServer(t, writeBody(ledgerVersion422))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	var reader *CRMReader
	reader = newTestCRMReader(t, srv.URL, func(_ CRMAPI, _ ServerVersion, _ error) {
		_, err := reader.Holders(ctx, crmTestOrg, HolderQuery{})
		result <- err
	})
	_, err := reader.Holders(ctx, crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("callback read blocked")
	}
}

func TestCRMReaderProbe_NotificationsRemainOrdered(t *testing.T) {
	t.Parallel()
	failedCallback := make(chan struct{})
	goodHTTP := make(chan struct{})
	release := make(chan struct{})
	var selected atomic.Bool
	srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "bad" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		close(goodHTTP)
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	reader := newTestCRMReader(t, srv.URL, func(api CRMAPI, _ ServerVersion, err error) {
		if err != nil {
			close(failedCallback)
			<-release
		}
		selected.Store(api == serverversion.CRMAPIV2)
	})
	badDone := make(chan error, 1)
	go func() { badDone <- reader.probe("bad") }()
	select {
	case <-failedCallback:
	case <-time.After(5 * time.Second):
		t.Fatal("failed callback did not start")
	}
	goodDone := make(chan error, 1)
	go func() { goodDone <- reader.probe("good") }()
	select {
	case <-goodHTTP:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("successful probe did not reach HTTP")
	}
	select {
	case <-goodDone:
		close(release)
		<-badDone
		t.Fatal("successful notification overtook failed callback")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	require.Error(t, <-badDone)
	require.NoError(t, <-goodDone)
	require.True(t, selected.Load())
	_, err := reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	require.Contains(t, srv.requests()[0].URL.Path, "/v2/organizations/")
}
