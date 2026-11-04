package midaz

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

const (
	crmReaderOperation  = "midaz.CRMReader"
	crmVersionOperation = "midaz.CRMReader.version"
	crmRefreshInterval  = 5 * time.Minute
	crmProbeTimeout     = 10 * time.Second // bounds a detached probe, so a hung /version cannot hold the flight forever
	maxCRMBody          = 10 << 20
)

// HTTPDoer sends one HTTP request; *http.Client satisfies it.
type HTTPDoer = interface {
	Do(req *http.Request) (*http.Response, error)
}

// CRMReaderConfig configures a [CRMReader].
type CRMReaderConfig struct {
	// URL is the CRM root as configured. One trailing "/v1" is dropped, so "https://crm.example.com"
	// and "https://crm.example.com/v1" name the same CRM. Point it at the ledger for the embedded CRM.
	URL string
	// HTTP sends every request, GET /version included: anything with Do(*http.Request), such as an
	// *http.Client. Required.
	HTTP HTTPDoer
	// Authorization returns the header for challenged metadata and each data read; values may differ after discovery.
	// It is sent verbatim ("" or nil sends none); errors are returned wrapped, unclassified.
	Authorization func(ctx context.Context) (string, error)
	// OnResolve reports the first resolved API, every change, and every failed /version read with
	// the API kept ("" before the first resolution). It runs on the probe's goroutine. Nil reports nothing.
	OnResolve func(api CRMAPI, v ServerVersion, err error)
}

// InstrumentQuery filters [CRMReader.Instruments]. Zero values are not sent.
type InstrumentQuery struct {
	AccountID      string
	Document       string
	BankAccount    string // bankingDetails.account
	BankBranch     string // bankingDetails.branch
	IncludeDeleted bool
	Page           int
	Limit          int
}

// HolderQuery filters [CRMReader.Holders]. Zero values are not sent.
type HolderQuery struct {
	Document string
	Page     int
	Limit    int
}

// CRMReader reads holders and instruments from the standalone CRM 3.x (/v1, instruments are its aliases)
// or the CRM embedded in the ledger 4.x (/v2), picked by [serverversion.ResolveCRMAPI] from GET {URL}/version at first
// use and every 5 minutes. Until /version succeeds, reads fail; an HTTP 401 remains an authentication error. One per CRM URL.
type CRMReader struct {
	root          string
	http          HTTPDoer
	authorization func(context.Context) (string, error)
	onResolve     func(CRMAPI, ServerVersion, error)
	now           func() time.Time

	probes    singleflight.Group
	probeSeq  atomic.Uint64
	resolveMu sync.Mutex
	refusals  map[[32]byte]time.Time
	resolved  atomic.Pointer[crmResolution]
}

type crmResolution struct {
	api        CRMAPI
	expiresAt  time.Time
	generation uint64
}

// NewCRMReader validates cfg. It makes no request.
func NewCRMReader(cfg CRMReaderConfig) (*CRMReader, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, sdkerrors.NewConfigurationError(crmReaderOperation, "CRM URL must be an absolute http(s) URL", nil)
	}

	if cfg.HTTP == nil {
		return nil, sdkerrors.NewConfigurationError(crmReaderOperation, "CRM HTTP client is required", nil)
	}

	onResolve := cfg.OnResolve
	if onResolve == nil {
		onResolve = func(CRMAPI, ServerVersion, error) {}
	}

	return &CRMReader{
		root:          strings.TrimSuffix(strings.TrimSuffix(cfg.URL, "/"), "/v1"),
		http:          cfg.HTTP,
		authorization: cfg.Authorization,
		onResolve:     onResolve,
		now:           time.Now,
		refusals:      make(map[[32]byte]time.Time),
	}, nil
}

// Instruments lists the organization's instruments matching q; Pagination.Limit is the limit the CRM applied.
func (r *CRMReader) Instruments(ctx context.Context, organizationID string, q InstrumentQuery) (*models.ListResponse[models.Instrument], error) {
	query := pageQuery(q.Page, q.Limit)
	setIfNotEmpty(query, "account_id", q.AccountID)
	setIfNotEmpty(query, "document", q.Document)
	setIfNotEmpty(query, "banking_details_account", q.BankAccount)
	setIfNotEmpty(query, "banking_details_branch", q.BankBranch)

	if q.IncludeDeleted {
		query.Set("include_deleted", "true")
	}

	var page models.ListResponse[models.Instrument]
	if err := r.get(ctx, "midaz.CRMReader.Instruments", organizationID, "aliases", "instruments", query, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// Holders lists the organization's holders matching q; Pagination.Limit is the limit the CRM applied.
func (r *CRMReader) Holders(ctx context.Context, organizationID string, q HolderQuery) (*models.ListResponse[models.Holder], error) {
	query := pageQuery(q.Page, q.Limit)
	setIfNotEmpty(query, "document", q.Document)

	var page models.ListResponse[models.Holder]
	if err := r.get(ctx, "midaz.CRMReader.Holders", organizationID, "holders", "holders", query, &page); err != nil {
		return nil, err
	}

	return &page, nil
}

// Holder reads one holder of the organization.
func (r *CRMReader) Holder(ctx context.Context, organizationID, holderID string) (*models.Holder, error) {
	const operation = "midaz.CRMReader.Holder"

	id, err := uuid.Parse(holderID)
	if err != nil {
		return nil, sdkerrors.NewValidationError(operation, "holderID must be a UUID", err)
	}

	var holder models.Holder
	if err := r.get(ctx, operation, organizationID, "holders/"+id.String(), "holders/"+id.String(), nil, &holder); err != nil {
		return nil, err
	}

	return &holder, nil
}

// get reads {root}/v1/{v1Path} with the organization in X-Organization-Id, or
// {root}/v2/organizations/{org}/{v2Path}, and decodes a 2xx body into out.
func (r *CRMReader) get(ctx context.Context, operation, organizationID, v1Path, v2Path string, query url.Values, out any) error {
	org, err := uuid.Parse(organizationID)
	if err != nil {
		return sdkerrors.NewValidationError(operation, "organizationID must be a UUID", err)
	}

	api, err := r.api(ctx)
	if err != nil {
		return err
	}

	var authorization string
	if r.authorization != nil {
		if authorization, err = r.authorization(ctx); err != nil {
			return fmt.Errorf("crm authorization: %w", err)
		}
	}

	endpoint := r.root + "/v1/" + v1Path
	if api == serverversion.CRMAPIV2 {
		endpoint = r.root + "/v2/organizations/" + org.String() + "/" + v2Path
	}

	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return sdkerrors.NewConfigurationError(operation, "build CRM request", nil) // its error would render the URL
	}

	req.Header.Set("Accept", "application/json")

	if api == serverversion.CRMAPIV1 {
		req.Header.Set("X-Organization-Id", org.String())
	}

	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := r.http.Do(req) // #nosec G704 -- root was validated by NewCRMReader; ids are parsed UUIDs
	if err != nil {
		return sdkerrors.ClassifyTransportError(operation, r.withoutRequestURL(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCRMBody))
	if err != nil {
		return sdkerrors.ClassifyTransportError(operation, r.withoutRequestURL(err))
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return sdkerrors.DecodeProblemJSON(resp.StatusCode, body, resp.Header.Get("X-Request-Id"))
	}

	if err := json.Unmarshal(body, out); err != nil {
		return sdkerrors.NewResponseDecodeError(operation, resp.StatusCode, err)
	}

	return nil
}

// withoutRequestURL replaces the request URL a transport error renders with the CRM root: the query
// carries a document and the path a holder id.
func (r *CRMReader) withoutRequestURL(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}

	return &url.Error{Op: urlErr.Op, URL: r.root, Err: urlErr.Err}
}

// api returns cached metadata immediately, refreshing expired metadata in the background.
func (r *CRMReader) api(ctx context.Context) (CRMAPI, error) {
	resolved := r.resolved.Load()
	if resolved == nil {
		if err := r.discover(ctx); err != nil {
			return "", err
		}
		return r.resolved.Load().api, nil
	}
	if r.now().Before(resolved.expiresAt) {
		return resolved.api, nil
	}
	var authorization string
	if r.authorization != nil {
		var err error
		authorization, err = r.authorization(ctx)
		if err != nil {
			authorization = ""
		}
	}
	key := "refresh:" + crmVersionOperation + "\x00" + authorization
	r.probes.DoChan(key, func() (any, error) {
		r.resolveMu.Lock()
		now := r.now()
		for header, deadline := range r.refusals {
			if !now.Before(deadline) {
				delete(r.refusals, header)
			}
		}
		_, refused := r.refusals[sha256.Sum256([]byte(authorization))]
		r.resolveMu.Unlock()
		if refused {
			return struct{}{}, nil
		}
		return nil, r.probe(authorization, key)
	})
	return resolved.api, nil
}

func crmAuthRefusal(err error) bool {
	status, ok := sdkerrors.ActualHTTPStatus(err)
	return ok && (status == http.StatusUnauthorized || status == http.StatusForbidden)
}

func (r *CRMReader) discover(ctx context.Context) error {
	public := r.probes.DoChan(crmVersionOperation, func() (any, error) {
		generation := r.probeSeq.Add(1)
		probeCtx, cancel := context.WithTimeout(context.Background(), crmProbeTimeout)
		defer cancel()
		v, err := readServerVersion(probeCtx, r.http, r.root, crmVersionOperation, "")
		if crmAuthRefusal(err) && r.authorization != nil {
			return v, err
		}
		if err != nil {
			r.probes.Forget(crmVersionOperation)
		}
		return v, r.commitProbe(generation, v, err, "", false)
	})
	var result singleflight.Result
	select {
	case result = <-public:
	case <-ctx.Done():
		return sdkerrors.ClassifyTransportError(crmReaderOperation, ctx.Err())
	}
	if !crmAuthRefusal(result.Err) || r.authorization == nil {
		return result.Err
	}
	authorization, err := r.authorization(ctx)
	if err != nil {
		return fmt.Errorf("crm authorization: %w", err)
	}
	key := crmVersionOperation + "\x00" + authorization
	private := r.probes.DoChan(key, func() (any, error) {
		if authorization != "" {
			return nil, r.probe(authorization, key)
		}
		r.probes.Forget(key)
		return nil, r.commitProbe(0, ServerVersion{Source: serverversion.SourceUnavailable}, result.Err, "", false)
	})
	select {
	case privateResult := <-private:
		return privateResult.Err
	case <-ctx.Done():
		return sdkerrors.ClassifyTransportError(crmReaderOperation, ctx.Err())
	}
}

// probe reads /version on a fresh context: of the caller that started it, only the Authorization value is sent. A failure
// keeps the last API; with none yet it caches nothing and returns the initial failure.
func (r *CRMReader) probe(authorization, key string) error {
	generation := r.probeSeq.Add(1)
	ctx, cancel := context.WithTimeout(context.Background(), crmProbeTimeout)
	defer cancel()

	v, err := readServerVersion(ctx, r.http, r.root, crmVersionOperation, authorization)
	if err != nil && r.resolved.Load() == nil {
		r.probes.Forget(key)
	}
	return r.commitProbe(generation, v, err, authorization, strings.HasPrefix(key, "refresh:"))
}

func (r *CRMReader) commitProbe(generation uint64, v ServerVersion, err error, authorization string, refresh bool) error {
	if err != nil && r.resolved.Load() == nil {
		r.onResolve("", v, err)
	}

	r.resolveMu.Lock()
	defer r.resolveMu.Unlock()
	last := r.resolved.Load()

	if err != nil && last == nil {
		if status, ok := sdkerrors.ActualHTTPStatus(err); ok && status == http.StatusUnauthorized {
			return err
		}

		// The cause's text only: a /version 404 in the chain would match ErrNotFound, "account not found".
		return sdkerrors.NewNetworkError(crmReaderOperation, errors.New("CRM API unresolved: "+err.Error()))
	}
	if err != nil {
		if crmAuthRefusal(err) && refresh {
			r.refusals[sha256.Sum256([]byte(authorization))] = r.now().Add(crmRefreshInterval)
		} else if !crmAuthRefusal(err) {
			r.resolved.Store(&crmResolution{api: last.api, expiresAt: r.now().Add(crmRefreshInterval), generation: last.generation})
		}
		r.onResolve(last.api, v, err)

		return nil
	}

	if last != nil && generation < last.generation {
		return nil
	}
	api := serverversion.ResolveCRMAPI(v)
	r.resolved.Store(&crmResolution{api: api, expiresAt: r.now().Add(crmRefreshInterval), generation: generation})

	if last == nil || last.api != api {
		r.onResolve(api, v, nil)
	}

	return nil
}

func pageQuery(page, limit int) url.Values {
	query := url.Values{}

	if page > 0 {
		query.Set("page", strconv.Itoa(page))
	}

	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	return query
}

func setIfNotEmpty(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}
