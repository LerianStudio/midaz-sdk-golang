package midaz

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
	"github.com/stretchr/testify/require"
)

func TestCRMReaderPublicDiscoveryBeforeAuthorization(t *testing.T) {
	var resolved atomic.Bool
	srv := newCRMServer(t, writeBody(ledgerVersion422))
	reader, err := NewCRMReader(CRMReaderConfig{
		URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(context.Context) (string, error) {
			if !resolved.Load() {
				return "", errors.New("unused CRM credential unavailable")
			}
			return "Bearer ledger", nil
		},
		OnResolve: func(_ CRMAPI, _ ServerVersion, err error) {
			if err == nil {
				resolved.Store(true)
			}
		},
	})
	require.NoError(t, err)
	_, err = reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 1, srv.versionHits.Load())
	require.Equal(t, "Bearer ledger", srv.requests()[0].Header.Get("Authorization"))
}

func TestCRMReaderPublicMetadataDoesNotWaitForUnusedCredential(t *testing.T) {
	var resolved atomic.Bool
	blocked := make(chan struct{})
	defer close(blocked)
	srv := newCRMServer(t, writeBody(ledgerVersion422))
	reader, err := NewCRMReader(CRMReaderConfig{
		URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(ctx context.Context) (string, error) {
			if !resolved.Load() {
				select {
				case <-blocked:
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			return "Bearer ledger", nil
		},
		OnResolve: func(_ CRMAPI, _ ServerVersion, err error) {
			if err == nil {
				resolved.Store(true)
			}
		},
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = reader.Holders(ctx, crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	require.Len(t, srv.requests(), 1)
}

func TestCRMReaderProtectedMetadataUsesCallerContext(t *testing.T) {
	type key struct{}
	var hits atomic.Int32
	srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer tenant" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	reader, err := NewCRMReader(CRMReaderConfig{
		URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(ctx context.Context) (string, error) {
			if ctx.Value(key{}) != "tenant" {
				return "", errors.New("missing tenant")
			}
			return "Bearer tenant", nil
		},
	})
	require.NoError(t, err)
	_, err = reader.Holders(context.WithValue(context.Background(), key{}, "tenant"), crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 2, hits.Load())
	require.Equal(t, "Bearer tenant", srv.requests()[0].Header.Get("Authorization"))
}

func TestCRMReaderPublicFailureCallbackRetriesSameFlight(t *testing.T) {
	var hits atomic.Int32
	var renewed atomic.Bool
	srv := newCRMServer(t, func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	inner := make(chan error, 1)
	var reader *CRMReader
	reader, err := NewCRMReader(CRMReaderConfig{URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(context.Context) (string, error) {
			if renewed.Load() {
				return "renewed", nil
			}
			return "old", nil
		},
		OnResolve: func(api CRMAPI, _ ServerVersion, err error) {
			if api == "" && err != nil {
				renewed.Store(true)
				_, err := reader.Holders(ctx, crmTestOrg, HolderQuery{})
				inner <- err
			}
		},
	})
	require.NoError(t, err)
	outer := make(chan error, 1)
	go func() { _, err := reader.Holders(ctx, crmTestOrg, HolderQuery{}); outer <- err }()
	select {
	case err := <-inner:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("callback retry blocked")
	}
	select {
	case err := <-outer:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("initial read blocked")
	}
	require.EqualValues(t, 2, hits.Load())
	require.Equal(t, "renewed", srv.requests()[0].Header.Get("Authorization"))
}

func TestCRMReaderProtectedMetadataWithoutAuthorization(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil", true: "empty"}[empty], func(t *testing.T) {
			srv := newCRMServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
			var callbacks atomic.Int32
			cfg := CRMReaderConfig{URL: srv.URL, HTTP: http.DefaultClient, OnResolve: func(_ CRMAPI, _ ServerVersion, err error) {
				if err != nil {
					callbacks.Add(1)
				}
			}}
			if empty {
				cfg.Authorization = func(context.Context) (string, error) { return "", nil }
			}
			reader, err := NewCRMReader(cfg)
			require.NoError(t, err)
			_, err = reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
			require.True(t, sdkerrors.IsAuthenticationError(err))
			status, ok := sdkerrors.ActualHTTPStatus(err)
			require.True(t, ok)
			require.Equal(t, http.StatusUnauthorized, status)
			require.EqualValues(t, 1, callbacks.Load())
			require.EqualValues(t, 1, srv.versionHits.Load())
			require.Empty(t, srv.requests())
		})
	}
}

func TestCRMReaderRefreshIsolatedByCredential(t *testing.T) {
	type contextKey struct{}
	var refresh atomic.Bool
	badStarted := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !refresh.Load() {
			_, _ = w.Write([]byte(crmVersion384))
			return
		}
		if r.Header.Get("Authorization") == "" {
			close(badStarted)
			select {
			case <-release:
			case <-r.Context().Done():
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	reports := make(chan CRMAPI, 2)
	reader, err := NewCRMReader(CRMReaderConfig{URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(ctx context.Context) (string, error) {
			if ctx.Value(contextKey{}) != nil {
				return "", errors.New("credential unavailable")
			}
			return "Bearer valid", nil
		},
		OnResolve: func(api CRMAPI, _ ServerVersion, err error) {
			if err == nil {
				reports <- api
			}
		},
	})
	require.NoError(t, err)
	var clock atomic.Int64
	start := time.Now()
	reader.now = func() time.Time { return start.Add(time.Duration(clock.Load())) }
	_, err = reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	select {
	case api := <-reports:
		require.Equal(t, serverversion.CRMAPIV1, api)
	case <-time.After(2 * time.Second):
		t.Fatal("initial resolution did not complete")
	}
	refresh.Store(true)
	clock.Store(int64(crmRefreshInterval))
	_, err = reader.Holders(context.WithValue(context.Background(), contextKey{}, true), crmTestOrg, HolderQuery{})
	require.Error(t, err)
	select {
	case <-badStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("empty-credential refresh did not start")
	}
	_, err = reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	select {
	case api := <-reports:
		require.Equal(t, serverversion.CRMAPIV2, api)
	case <-time.After(2 * time.Second):
		t.Fatal("valid credential refresh was blocked by failed credential")
	}
}

func TestCRMReaderPrivateRefreshAfterCallerCanceled(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n > 2 {
			<-release
		}
		if r.Header.Get("Authorization") != "Bearer tenant" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if n <= 2 {
			_, _ = w.Write([]byte(crmVersion384))
			return
		}
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	reports := make(chan CRMAPI, 3)
	reader, err := NewCRMReader(CRMReaderConfig{URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(ctx context.Context) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			return "Bearer tenant", nil
		},
		OnResolve: func(api CRMAPI, _ ServerVersion, err error) {
			if err == nil {
				reports <- api
			}
		},
	})
	require.NoError(t, err)
	var clock atomic.Int64
	start := time.Now()
	reader.now = func() time.Time { return start.Add(time.Duration(clock.Load())) }
	_, err = reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	select {
	case api := <-reports:
		require.Equal(t, serverversion.CRMAPIV1, api)
	case <-time.After(2 * time.Second):
		t.Fatal("initial resolution did not complete")
	}
	clock.Store(int64(crmRefreshInterval))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, err = reader.Holders(ctx, crmTestOrg, HolderQuery{})
	require.NoError(t, err)
	cancel()
	close(release)
	select {
	case api := <-reports:
		require.Equal(t, serverversion.CRMAPIV2, api)
	case <-time.After(2 * time.Second):
		t.Fatal("private refresh did not complete after caller cancellation")
	}
}
