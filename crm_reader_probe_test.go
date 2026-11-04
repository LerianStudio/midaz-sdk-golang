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
	var notifications atomic.Int32
	reader := newTestCRMReader(t, srv.URL, func(_ CRMAPI, _ ServerVersion, _ error) { notifications.Add(1) })
	_, err := reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.EqualValues(t, 1, notifications.Load())
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

func TestCRMReaderProbe_InitialFailureCallbackCanRetry(t *testing.T) {
	t.Parallel()
	var renewed atomic.Bool
	srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "refused" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	inner := make(chan error, 1)
	var reader *CRMReader
	var err error
	reader, err = NewCRMReader(CRMReaderConfig{
		URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(context.Context) (string, error) {
			if renewed.Load() {
				return "renewed", nil
			}
			return "refused", nil
		},
		OnResolve: func(api CRMAPI, _ ServerVersion, err error) {
			if api == "" && err != nil {
				renewed.Store(true)
				_, innerErr := reader.Holders(ctx, crmTestOrg, HolderQuery{})
				inner <- innerErr
			}
		},
	})
	require.NoError(t, err)
	outer := make(chan error, 1)
	go func() { _, callErr := reader.Holders(ctx, crmTestOrg, HolderQuery{}); outer <- callErr }()
	select {
	case callErr := <-inner:
		require.NoError(t, callErr)
	case <-ctx.Done():
		t.Fatal("callback retry blocked")
	}
	select {
	case callErr := <-outer:
		require.NoError(t, callErr)
	case <-ctx.Done():
		t.Fatal("initial read blocked")
	}
	require.Contains(t, srv.requests()[0].URL.Path, "/v2/organizations/")
}

func TestCRMReaderProbe_NotificationsRemainOrdered(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		failures int
	}{{"one failure", 1}, {"two failures", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			started := make(chan struct{}, tc.failures)
			release := make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			var selected atomic.Bool
			srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "good" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = w.Write([]byte(ledgerVersion422))
			})
			reader := newTestCRMReader(t, srv.URL, func(api CRMAPI, _ ServerVersion, err error) {
				if api == "" && err != nil {
					started <- struct{}{}
					select {
					case <-release:
					case <-ctx.Done():
						return
					}
				}
				selected.Store(api == serverversion.CRMAPIV2)
			})
			done := make(chan error, tc.failures)
			for _, credential := range []string{"bad-one", "bad-two"}[:tc.failures] {
				go func() { done <- reader.probe(credential) }()
			}
			for range tc.failures {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("initial callbacks did not start")
				}
			}
			good := make(chan error, 1)
			go func() { good <- reader.probe("good") }()
			select {
			case err := <-good:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal("successful probe did not finish while callbacks were held")
			}
			close(release)
			for range tc.failures {
				select {
				case err := <-done:
					require.NoError(t, err)
				case <-ctx.Done():
					t.Fatal("initial probes did not finish")
				}
			}
			require.True(t, selected.Load())
			_, err := reader.Holders(ctx, crmTestOrg, HolderQuery{})
			require.NoError(t, err)
			require.Contains(t, srv.requests()[0].URL.Path, "/v2/organizations/")
		})
	}
}
