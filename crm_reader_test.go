package midaz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

const (
	crmTestOrg      = "0190a5f5-2b3c-7d4e-8f90-0123456789ab"
	crmTestHolderID = "0190a5f5-3c4d-7e5f-8a01-123456789abc"
	crmTestDocument = "12345678909"

	// The /version bodies the CRM URL can serve (midaz v3.8.5 crm, v4.0.0 and v4.2.2 ledger).
	crmVersion384    = `{"version":"v3.8.4","requestDate":"2026-10-16T12:00:00Z"}`
	crmVersionUnset  = `{"version":"0.0.0","requestDate":"2026-10-16T12:00:00Z"}`
	ledgerVersion403 = `{"version":"4.0.3","requestDate":"2026-10-16T12:00:00Z","commit":"abc","buildTime":"2026-09-30T00:00:00Z","dirty":false}`
	ledgerVersion422 = `{"schemaVersion":"v1","service":"midaz-ledger","version":"4.2.2","revision":"b6506518f","buildTime":"2026-10-01T00:00:00Z","modified":false,"goVersion":"go1.27.0"}`

	// One instrument and one holder exactly as both APIs serve them (same JSON names on v1 and v2).
	crmGoldenInstrument = `{"id":"0190a5f5-4d5e-7f60-8b12-23456789abcd","document":"12345678909","type":"NATURAL_PERSON",` +
		`"ledgerId":"0190a5f5-5e6f-7071-8c23-3456789abcde","accountId":"0190a5f5-6f70-7182-8d34-456789abcdef",` +
		`"holderId":"0190a5f5-3c4d-7e5f-8a01-123456789abc","metadata":{"midazAccountAlias":"@ana"},` +
		`"bankingDetails":{"branch":"0001","account":"123450","type":"CACC","openingDate":"2025-01-01","bankId":"13935893"},` +
		`"createdAt":"2025-01-01T00:00:00Z","updatedAt":"2025-01-02T00:00:00Z","deletedAt":null}`
	crmGoldenHolder = `{"id":"0190a5f5-3c4d-7e5f-8a01-123456789abc","externalId":"ext-1","type":"NATURAL_PERSON",` +
		`"name":"Ana","document":"12345678909","addresses":{"primary":{"line1":"Rua A","zipCode":"01000-000","city":"Sao Paulo","state":"SP","country":"BR"}},` +
		`"createdAt":"2025-01-01T00:00:00Z","updatedAt":"2025-01-02T00:00:00Z","deletedAt":null}`
)

// crmServer is a CRM that answers /version with the current body and every data route with the
// golden bodies, recording each request.
type crmServer struct {
	*httptest.Server

	version      atomic.Pointer[http.HandlerFunc]
	versionHits  atomic.Int32
	mu           sync.Mutex
	dataRequests []*http.Request
}

func newCRMServer(t *testing.T, version http.HandlerFunc) *crmServer {
	t.Helper()

	s := &crmServer{}
	s.setVersion(version)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			s.versionHits.Add(1)
			(*s.version.Load())(w, r)

			return
		}

		s.mu.Lock()
		s.dataRequests = append(s.dataRequests, r)
		s.mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/aliases"), strings.HasSuffix(r.URL.Path, "/instruments"):
			_, _ = w.Write([]byte(`{"items":[` + crmGoldenInstrument + `],"limit":100,"page":2}`))
		case strings.HasSuffix(r.URL.Path, "/holders"):
			_, _ = w.Write([]byte(`{"items":[` + crmGoldenHolder + `],"limit":100,"page":2}`))
		default:
			_, _ = w.Write([]byte(crmGoldenHolder))
		}
	}))
	t.Cleanup(s.Close)

	return s
}

func (s *crmServer) setVersion(h http.HandlerFunc) { s.version.Store(&h) }

func (s *crmServer) requests() []*http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]*http.Request(nil), s.dataRequests...)
}

func newTestCRMReader(t *testing.T, url string, onResolve func(CRMAPI, ServerVersion, error)) *CRMReader {
	t.Helper()

	r, err := NewCRMReader(CRMReaderConfig{
		URL:           url,
		HTTP:          http.DefaultClient,
		Authorization: func(context.Context) (string, error) { return "Bearer t", nil },
		OnResolve:     onResolve,
	})
	require.NoError(t, err)

	return r
}

// requireUnavailable pins the fail-closed answer: retryable, never a 404 the engines read as
// "account not found".
func requireUnavailable(t *testing.T, err error) {
	t.Helper()

	var sdkErr *sdkerrors.Error

	require.ErrorAs(t, err, &sdkErr)
	assert.True(t, sdkErr.Retryable(), "want retryable, got %v", err)
	require.NotErrorIs(t, err, sdkerrors.ErrNotFound, "a failed /version must never read as not found")

	_, upstream := sdkerrors.ActualHTTPStatus(err)
	assert.False(t, upstream, "a failed /version must not hand the caller an upstream status: %v", err)
}

// TestCRMReaderResolvesAPIFromVersion pins the rule end to end: the /version served at the CRM URL
// picks the route, and an unreadable one fails closed with nothing cached.
func TestCRMReaderResolvesAPIFromVersion(t *testing.T) {
	tests := []struct {
		name    string
		version http.HandlerFunc
		timeout time.Duration
		want    string // data path prefix; "" for unavailable
	}{
		{name: "CRM 3.8.4", version: writeBody(crmVersion384), want: "/v1/aliases"},
		{name: "CRM 3.x without VERSION serves 0.0.0", version: writeBody(crmVersionUnset), want: "/v1/aliases"},
		{name: "ledger 4.0.3 legacy body", version: writeBody(ledgerVersion403), want: "/v2/organizations/" + crmTestOrg + "/instruments"},
		{name: "ledger 4.2.2 buildinfo v1", version: writeBody(ledgerVersion422), want: "/v2/organizations/" + crmTestOrg + "/instruments"},
		{name: "404", version: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }},
		{name: "non-JSON", version: writeBody(`<html>Welcome to nginx!</html>`)},
		{
			name:    "timeout",
			version: func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() },
			timeout: 50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newCRMServer(t, tt.version)
			defer srv.CloseClientConnections() // releases a hung /version before Close waits on it

			reader := newTestCRMReader(t, srv.URL, nil)

			ctx := context.Background()
			if tt.timeout > 0 {
				var cancel context.CancelFunc

				ctx, cancel = context.WithTimeout(ctx, tt.timeout)
				defer cancel()
			}

			_, err := reader.Instruments(ctx, crmTestOrg, InstrumentQuery{AccountID: "acc"})

			if tt.want != "" {
				require.NoError(t, err)
				require.Len(t, srv.requests(), 1)
				assert.Equal(t, tt.want, srv.requests()[0].URL.Path)

				return
			}

			requireUnavailable(t, err)
			assert.Empty(t, srv.requests(), "no data read without a resolved API")
			assert.Nil(t, reader.resolved.Load(), "a failed first read caches nothing")

			if tt.timeout == 0 {
				_, err = reader.Instruments(context.Background(), crmTestOrg, InstrumentQuery{})
				requireUnavailable(t, err)
				assert.EqualValues(t, 2, srv.versionHits.Load(), "the next call probes again")
			}
		})
	}
}

// TestCRMReaderRoutesAndDecodesBothAPIs pins each API's routes and organization placement, the
// identical query on both, the configured "/v1" suffix being dropped, no Authorization on /version,
// and one golden body decoding to the same models through both APIs.
func TestCRMReaderRoutesAndDecodesBothAPIs(t *testing.T) {
	type decoded struct {
		instruments *models.ListResponse[models.Instrument]
		holders     *models.ListResponse[models.Holder]
		holder      *models.Holder
	}

	tests := []struct {
		name      string
		version   string
		urlSuffix string
		paths     []string
		orgHeader string
	}{
		{
			name: "v1", version: crmVersion384, urlSuffix: "/v1/", orgHeader: crmTestOrg,
			paths: []string{"/v1/aliases", "/v1/holders", "/v1/holders/" + crmTestHolderID},
		},
		{
			name: "v2", version: ledgerVersion422,
			paths: []string{
				"/v2/organizations/" + crmTestOrg + "/instruments",
				"/v2/organizations/" + crmTestOrg + "/holders",
				"/v2/organizations/" + crmTestOrg + "/holders/" + crmTestHolderID,
			},
		},
	}

	results := map[string]decoded{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var versionAuth string

			srv := newCRMServer(t, func(w http.ResponseWriter, r *http.Request) {
				versionAuth = r.Header.Get("Authorization")
				_, _ = w.Write([]byte(tt.version))
			})
			reader := newTestCRMReader(t, srv.URL+tt.urlSuffix, nil)
			ctx := context.Background()

			var (
				got decoded
				err error
			)

			got.instruments, err = reader.Instruments(ctx, crmTestOrg, InstrumentQuery{
				AccountID: "acc", Document: crmTestDocument, BankAccount: "123450", IncludeDeleted: true, Page: 2, Limit: 100,
			})
			require.NoError(t, err)

			got.holders, err = reader.Holders(ctx, crmTestOrg, HolderQuery{Document: crmTestDocument, Page: 2, Limit: 100})
			require.NoError(t, err)

			got.holder, err = reader.Holder(ctx, crmTestOrg, crmTestHolderID)
			require.NoError(t, err)

			results[tt.name] = got

			reqs := srv.requests()
			require.Len(t, reqs, len(tt.paths))

			for i, path := range tt.paths {
				assert.Equal(t, path, reqs[i].URL.Path)
				assert.Equal(t, tt.orgHeader, reqs[i].Header.Get("X-Organization-Id"))
				assert.Equal(t, "Bearer t", reqs[i].Header.Get("Authorization"))
			}

			assert.Equal(t, "account_id=acc&banking_details_account=123450&document=12345678909&include_deleted=true&limit=100&page=2", reqs[0].URL.RawQuery)
			assert.Equal(t, "document=12345678909&limit=100&page=2", reqs[1].URL.RawQuery)
			assert.Empty(t, reqs[2].URL.RawQuery)
			assert.Empty(t, versionAuth, "/version is public; never send the credential there")
		})
	}

	require.Len(t, results, 2)
	assert.Equal(t, results["v1"], results["v2"], "one body, one decode, whichever API served it")

	instrument := results["v2"].instruments.Items[0]
	assert.Equal(t, 100, results["v2"].instruments.Pagination.Limit)
	assert.Equal(t, 100, results["v2"].holders.Pagination.Limit)
	assert.Equal(t, crmTestHolderID, instrument.HolderID.String())
	assert.Equal(t, "13935893", *instrument.BankingDetails.BankID)
	assert.Equal(t, "0001", *instrument.BankingDetails.Branch)
	assert.Equal(t, "Ana", *results["v2"].holder.Name)
	assert.Equal(t, "Sao Paulo", results["v2"].holder.Addresses.Primary.City)
}

// TestCRMReaderRefresh pins the refresh: past 5 minutes a call answers the cached API and starts one
// re-read; a failed re-read keeps the last API, a changed answer swaps it, OnResolve reports both.
func TestCRMReaderRefresh(t *testing.T) {
	type report struct {
		api CRMAPI
		err error
	}

	reports := make(chan report, 4)
	srv := newCRMServer(t, writeBody(crmVersion384))
	reader := newTestCRMReader(t, srv.URL, func(api CRMAPI, _ ServerVersion, err error) { reports <- report{api, err} })

	var clock atomic.Int64

	start := time.Now()
	reader.now = func() time.Time { return start.Add(time.Duration(clock.Load())) }

	read := func() string {
		t.Helper()

		_, err := reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
		require.NoError(t, err)

		reqs := srv.requests()

		return reqs[len(reqs)-1].URL.Path
	}
	next := func() report {
		t.Helper()

		select {
		case r := <-reports:
			return r
		case <-time.After(5 * time.Second):
			t.Fatal("no OnResolve report")

			return report{}
		}
	}

	assert.Equal(t, "/v1/holders", read())
	assert.Equal(t, report{api: serverversion.CRMAPIV1}, next(), "first resolution is reported")

	clock.Store(int64(crmRefreshInterval))
	srv.setVersion(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	assert.Equal(t, "/v1/holders", read(), "an expired API still answers while the re-read runs")

	failed := next()
	assert.Equal(t, serverversion.CRMAPIV1, failed.api, "a failed re-read keeps the last API")
	require.Error(t, failed.err)
	assert.Equal(t, "/v1/holders", read())

	clock.Store(int64(3 * crmRefreshInterval))
	srv.setVersion(writeBody(ledgerVersion422))
	assert.Equal(t, "/v1/holders", read())
	assert.Equal(t, report{api: serverversion.CRMAPIV2}, next(), "a changed answer swaps the API")
	assert.Equal(t, "/v2/organizations/"+crmTestOrg+"/holders", read())
	assert.EqualValues(t, 3, srv.versionHits.Load(), "one probe per expiry")
}

// TestCRMReaderSharesFirstProbe pins that concurrent first calls share one GET /version.
func TestCRMReaderSharesFirstProbe(t *testing.T) {
	release := make(chan struct{})
	srv := newCRMServer(t, func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte(ledgerVersion422))
	})
	reader := newTestCRMReader(t, srv.URL, nil)

	const callers = 20

	var wg sync.WaitGroup

	errs := make(chan error, callers)

	for range callers {
		wg.Go(func() {
			_, err := reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
			errs <- err
		})
	}

	time.Sleep(100 * time.Millisecond) // let every caller join the in-flight probe
	close(release)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	assert.EqualValues(t, 1, srv.versionHits.Load())
	assert.Len(t, srv.requests(), callers)
}

// TestCRMReaderTransportErrorHidesRequest pins that a refused connection never renders the query (a
// document, an account number) or the holder id in the path.
func TestCRMReaderTransportErrorHidesRequest(t *testing.T) {
	srv := newCRMServer(t, writeBody(crmVersion384))
	reader := newTestCRMReader(t, srv.URL, nil)

	_, err := reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
	require.NoError(t, err)

	srv.Close()

	_, err = reader.Instruments(context.Background(), crmTestOrg, InstrumentQuery{Document: crmTestDocument, BankAccount: "98765432"})
	require.Error(t, err)
	assert.True(t, sdkerrors.IsNetworkError(err), "want network error, got %v", err)

	for _, rendered := range []string{err.Error(), errors.Unwrap(err).Error()} {
		assert.NotContains(t, rendered, crmTestDocument)
		assert.NotContains(t, rendered, "98765432")
		assert.NotContains(t, rendered, "/aliases", "the request URL is replaced by the CRM root")
	}

	_, err = reader.Holder(context.Background(), crmTestOrg, crmTestHolderID)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), crmTestHolderID)
	assert.NotContains(t, errors.Unwrap(err).Error(), crmTestHolderID)
}

// TestCRMReaderRequestFailures pins the non-transport failures of a data read.
func TestCRMReaderRequestFailures(t *testing.T) {
	t.Run("non-2xx decodes the problem body and keeps its code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/version" {
				_, _ = w.Write([]byte(ledgerVersion422))

				return
			}

			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"status":403,"title":"Forbidden","code":"0042","detail":"insufficient privileges"}`))
		}))
		defer srv.Close()

		_, err := newTestCRMReader(t, srv.URL, nil).Holders(context.Background(), crmTestOrg, HolderQuery{})

		status, ok := sdkerrors.ActualHTTPStatus(err)
		assert.True(t, ok)
		assert.Equal(t, http.StatusForbidden, status)

		var sdkErr *sdkerrors.Error

		require.ErrorAs(t, err, &sdkErr)
		assert.Equal(t, "0042", sdkErr.APICode)
	})

	t.Run("an Authorization error is wrapped unclassified", func(t *testing.T) {
		srv := newCRMServer(t, writeBody(ledgerVersion422))
		tokenErr := errors.New("token refused")

		reader, err := NewCRMReader(CRMReaderConfig{
			URL:           srv.URL,
			HTTP:          http.DefaultClient,
			Authorization: func(context.Context) (string, error) { return "", tokenErr },
		})
		require.NoError(t, err)

		_, err = reader.Holders(context.Background(), crmTestOrg, HolderQuery{})
		require.ErrorIs(t, err, tokenErr)

		var sdkErr *sdkerrors.Error

		assert.NotErrorAs(t, err, &sdkErr, "the caller keeps its own token codes")
		assert.Empty(t, srv.requests())
	})

	t.Run("ids that are not UUIDs never reach the wire", func(t *testing.T) {
		srv := newCRMServer(t, writeBody(ledgerVersion422))
		reader := newTestCRMReader(t, srv.URL, nil)

		_, err := reader.Holder(context.Background(), crmTestOrg, "")
		assert.True(t, sdkerrors.IsValidationError(err), "want validation error, got %v", err)

		_, err = reader.Holders(context.Background(), "../x", HolderQuery{})
		assert.True(t, sdkerrors.IsValidationError(err), "want validation error, got %v", err)
		assert.Zero(t, srv.versionHits.Load())
	})
}

func TestNewCRMReaderRefusesBadConfig(t *testing.T) {
	for name, cfg := range map[string]CRMReaderConfig{
		"no URL":         {HTTP: http.DefaultClient},
		"relative URL":   {URL: "crm.example.com/v1", HTTP: http.DefaultClient},
		"no HTTP client": {URL: "https://crm.example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewCRMReader(cfg)
			assert.True(t, sdkerrors.IsConfigurationError(err), "want configuration error, got %v", err)
		})
	}
}
