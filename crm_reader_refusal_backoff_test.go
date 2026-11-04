package midaz

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
	"github.com/stretchr/testify/require"
)

type refusalContextKey struct{}

func TestCRMReaderRefreshRefusalBackoffPerHeader(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) { testCRMRefusalBackoff(t, status) })
	}
}

func testCRMRefusalBackoff(t *testing.T, status int) {
	t.Helper()
	var refresh atomic.Bool
	var callbacks atomic.Int64
	srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !refresh.Load() {
			_, _ = w.Write([]byte(crmVersion384))
			return
		}
		if r.Header.Get("Authorization") != "Bearer healthy" {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	reader, err := NewCRMReader(CRMReaderConfig{URL: srv.URL, HTTP: http.DefaultClient,
		Authorization: func(ctx context.Context) (string, error) {
			value, _ := ctx.Value(refusalContextKey{}).(string)
			return value, nil
		},
		OnResolve: func(CRMAPI, ServerVersion, error) { callbacks.Add(1) },
	})
	require.NoError(t, err)
	var clock atomic.Int64
	start := time.Now()
	reader.now = func() time.Time { return start.Add(time.Duration(clock.Load())) }
	read := func(header string) {
		t.Helper()
		_, err := reader.Holders(context.WithValue(context.Background(), refusalContextKey{}, header), crmTestOrg, HolderQuery{})
		require.NoError(t, err)
		flight := reader.probes.DoChan("refresh:"+crmVersionOperation+"\x00"+header, func() (any, error) { return struct{}{}, nil })
		select {
		case <-flight:
		case <-time.After(2 * time.Second):
			t.Fatal("refresh did not complete")
		}
	}
	read("")
	refresh.Store(true)
	clock.Store(int64(crmRefreshInterval))
	read("")
	read("Bearer refused")
	require.EqualValues(t, 3, srv.versionHits.Load())
	require.EqualValues(t, 3, callbacks.Load())
	read("")
	read("Bearer refused")
	require.EqualValues(t, 3, srv.versionHits.Load(), "completed refusals should be suppressed per header")
	require.EqualValues(t, 3, callbacks.Load(), "suppressed refresh must not report")
	clock.Store(int64(2 * crmRefreshInterval))
	read("")
	read("Bearer refused")
	require.EqualValues(t, 5, srv.versionHits.Load(), "refusals retry at the interval")
	read("Bearer healthy")
	require.EqualValues(t, 6, srv.versionHits.Load(), "healthy credentials refresh immediately")
	require.Equal(t, serverversion.CRMAPIV2, reader.resolved.Load().api)
}
