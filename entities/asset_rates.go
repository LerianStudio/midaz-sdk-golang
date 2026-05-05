package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"strconv"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// AssetRatesListOpts is the typed options struct for
// ListAssetRatesByAssetCode and the ListAll/ListPages iterators.
//
// AssetRatesListOpts is a value type (not a pointer). Sharing across
// goroutines is safe because the entity layer never mutates a caller's
// opts — the SDK reads fields, builds query parameters, and discards
// the snapshot. This eliminates the v2 mutable-shared-options race
// (5.6).
//
// The asset-rates endpoint is cursor-paginated: callers iterate by
// passing back the NextCursor surfaced in the previous response's
// Pagination shape. Page-based fields (Page, Offset) are intentionally
// absent — the endpoint does not honor them on the wire.
//
// Use Validate before calling the entity method to surface limit-cap
// violations as typed errors instead of having them silently capped
// (the v2 footgun, audit finding 5.7).
type AssetRatesListOpts struct {
	// Limit is the maximum number of items per page. Zero falls back
	// to the server default (currently 10). Values above
	// models.MaxLimit (currently 100) cause Validate to return a
	// validation error.
	Limit int

	// Cursor is the server-issued opaque pagination token. Empty
	// string means "first page". Read NextCursor from the previous
	// response's Pagination to fetch the next page.
	Cursor string

	// SortDirection orders results by createdAt. Empty string falls
	// back to the server default (descending). Use models.SortAscending
	// or models.SortDescending; other values are rejected by Validate.
	SortDirection models.SortDirection

	// Filters narrows the result set. Zero value means no narrowing.
	Filters AssetRatesFilters
}

// AssetRatesFilters carries the filter fields valid for the
// ListAssetRatesByAssetCode endpoint. Only the fields meaningful for
// asset-rate filtering are present here — narrower than the v2
// mega-struct ListOptions, which exposed 30+ filter setters
// regardless of which one the endpoint actually honored.
type AssetRatesFilters struct {
	// To restricts results to rates whose target asset matches one
	// of the supplied codes. Empty slice means "no restriction".
	// The wire encoding is a single comma-separated query parameter.
	To []string

	// StartDate filters rates created on or after this date in
	// YYYY-MM-DD format. Empty string means "no lower bound".
	StartDate string

	// EndDate filters rates created on or before this date in
	// YYYY-MM-DD format. Empty string means "no upper bound".
	EndDate string
}

// Validate enforces SDK-side preconditions on AssetRatesListOpts.
//
// Returns a typed validation error when:
//   - Limit is negative
//   - Limit exceeds models.MaxLimit
//   - SortDirection is non-empty and not one of the recognized values
//
// Validate is safe to call on a zero-value AssetRatesListOpts; the
// entity method calls it automatically before issuing the HTTP request.
//
// This replaces the v2 silent-cap behavior, where a Limit of 5000
// was rounded down to 100 with no error returned (audit finding 5.7).
func (o AssetRatesListOpts) Validate() error {
	const operation = "AssetRatesListOpts.Validate"

	if o.Limit < 0 {
		return errors.NewValidationError(operation,
			"limit must be non-negative",
			fmt.Errorf("got %d", o.Limit))
	}

	if o.Limit > models.MaxLimit {
		return errors.NewValidationError(operation,
			"limit exceeds maximum",
			fmt.Errorf("got %d, max %d", o.Limit, models.MaxLimit))
	}

	switch o.SortDirection {
	case "", models.SortAscending, models.SortDescending:
	default:
		return errors.NewValidationError(operation,
			"sort direction must be empty, asc, or desc",
			fmt.Errorf("got %q", o.SortDirection))
	}

	return nil
}

// toQueryParams renders an AssetRatesListOpts into the wire query
// parameter map consumed by the asset-rates endpoint.
//
// Empty/zero fields are omitted so the server applies its own
// defaults rather than the SDK forcing a value. This matters for
// limit (server default differs from SDK default) and sort_order
// (server may pick a non-descending default for some endpoints).
func (o AssetRatesListOpts) toQueryParams() map[string]string {
	params := make(map[string]string)

	if len(o.Filters.To) > 0 {
		params["to"] = strings.Join(o.Filters.To, ",")
	}

	if o.Limit > 0 {
		params["limit"] = strconv.Itoa(o.Limit)
	}

	if o.Filters.StartDate != "" {
		params["start_date"] = o.Filters.StartDate
	}

	if o.Filters.EndDate != "" {
		params["end_date"] = o.Filters.EndDate
	}

	if o.SortDirection != "" {
		params["sort_order"] = string(o.SortDirection)
	}

	if o.Cursor != "" {
		params["cursor"] = o.Cursor
	}

	return params
}

// AssetRatesService defines the interface for asset rate operations.
// It provides methods to create, update, and retrieve asset conversion rates.
type AssetRatesService interface {
	// CreateOrUpdateAssetRate creates a new asset rate or updates an existing one.
	//
	// This method uses PUT semantics - if an asset rate with the same from/to asset
	// codes exists, it will be updated; otherwise, a new one will be created.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger.
	//   - ledgerID: The ID of the ledger where the asset rate will be stored.
	//   - input: The asset rate details including source/target assets and conversion rate.
	//
	// Returns:
	//   - *models.AssetRate: The created or updated asset rate if successful.
	//   - error: An error if the operation fails.
	//
	// Example:
	//
	//	rate, err := assetRatesService.CreateOrUpdateAssetRate(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    models.NewCreateAssetRateInput("USD", "BRL", 500).
	//	        WithScale(2).
	//	        WithSource("Central Bank"),
	//	)
	CreateOrUpdateAssetRate(ctx context.Context, organizationID, ledgerID string, input *models.CreateAssetRateInput) (*models.AssetRate, error)

	// GetAssetRate retrieves an asset rate by its external ID.
	//
	// Parameters:
	//   - ctx: Context for the request, which can be used for cancellation and timeout.
	//   - organizationID: The ID of the organization that owns the ledger.
	//   - ledgerID: The ID of the ledger containing the asset rate.
	//   - externalID: The external identifier of the asset rate to retrieve.
	//
	// Returns:
	//   - *models.AssetRate: The asset rate if found.
	//   - error: An error if the operation fails or the asset rate doesn't exist.
	//
	// Example:
	//
	//	rate, err := assetRatesService.GetAssetRate(
	//	    context.Background(),
	//	    "org-123",
	//	    "ledger-456",
	//	    "external-id-789",
	//	)
	GetAssetRate(ctx context.Context, organizationID, ledgerID, externalID string) (*models.AssetRate, error)

	// ListAssetRatesByAssetCode retrieves one page of asset rates for a
	// specific source asset code. The endpoint is cursor-paginated;
	// the returned response carries a NextCursor (in
	// response.Pagination) that callers feed back via opts.Cursor to
	// fetch the next page.
	//
	// For most consumers, prefer ListAssetRatesByAssetCodeAll, which
	// handles the cursor-fed loop transparently and yields one item
	// at a time via iter.Seq2.
	//
	// Validates opts before issuing the HTTP request. A failed
	// validation returns a typed *errors.Error with category
	// validation; the request is not sent.
	//
	// Parameters:
	//   - ctx: cancellable request context
	//   - organizationID: organization owning the ledger
	//   - ledgerID: ledger containing the asset rates
	//   - assetCode: source asset code filter (e.g. "USD")
	//   - opts: typed options; pass AssetRatesListOpts{} for defaults
	//
	// Example:
	//
	//	resp, err := client.AssetRates.ListAssetRatesByAssetCode(
	//	    ctx, "org-123", "ledger-456", "USD",
	//	    entities.AssetRatesListOpts{
	//	        Limit:   50,
	//	        Filters: entities.AssetRatesFilters{To: []string{"BRL", "EUR"}},
	//	    },
	//	)
	ListAssetRatesByAssetCode(ctx context.Context, organizationID, ledgerID, assetCode string, opts AssetRatesListOpts) (*models.ListResponse[models.AssetRate], error)

	// ListAssetRatesByAssetCodeAll yields every asset rate matching
	// the source asset code, transparently following pagination
	// cursors. The returned iter.Seq2 short-circuits on the first
	// transport or validation error; callers must check err on every
	// iteration.
	//
	// Pair with entities.Collect to materialize a bounded slice or
	// entities.CollectAll for an unbounded drain (use the latter
	// only when domain knowledge guarantees the result set is small).
	//
	// Example:
	//
	//	for rate, err := range client.AssetRates.ListAssetRatesByAssetCodeAll(
	//	    ctx, "org-123", "ledger-456", "USD",
	//	    entities.AssetRatesListOpts{Limit: 100},
	//	) {
	//	    if err != nil {
	//	        return fmt.Errorf("asset rates iteration failed: %w", err)
	//	    }
	//	    process(rate)
	//	}
	ListAssetRatesByAssetCodeAll(ctx context.Context, organizationID, ledgerID, assetCode string, opts AssetRatesListOpts) iter.Seq2[models.AssetRate, error]

	// ListAssetRatesByAssetCodePages yields one full *ListResponse
	// per page, transparently following pagination cursors. Use this
	// when the caller needs page-level metadata (Pagination shape,
	// per-page item counts, or cursor checkpointing for resumable
	// jobs).
	//
	// For item-level iteration prefer ListAssetRatesByAssetCodeAll.
	ListAssetRatesByAssetCodePages(ctx context.Context, organizationID, ledgerID, assetCode string, opts AssetRatesListOpts) iter.Seq2[*models.ListResponse[models.AssetRate], error]
}

// assetRatesEntity implements the AssetRatesService interface.
type assetRatesEntity struct {
	httpClient *HTTPClient
	baseURLs   map[string]string
}

func (e *assetRatesEntity) setDefaultTenantID(tenantID string) {
	e.httpClient.setTenantIDLocked(tenantID)
}

// newAssetRatesEntity creates a new asset rates entity.
//
// Parameters:
//   - client: The HTTP client used for API requests.
//   - authToken: The authentication token for API authorization.
//   - baseURLs: Map of service names to base URLs.
//
// Returns:
//   - AssetRatesService: An implementation of the AssetRatesService interface.
func newAssetRatesEntity(client *http.Client, authToken string, baseURLs map[string]string) AssetRatesService {
	httpClient := NewHTTPClient(client, authToken, nil)

	return &assetRatesEntity{
		httpClient: httpClient,
		baseURLs:   prepareServiceBaseURLs(baseURLs),
	}
}

// CreateOrUpdateAssetRate creates a new asset rate or updates an existing one.
func (e *assetRatesEntity) CreateOrUpdateAssetRate(
	ctx context.Context,
	organizationID, ledgerID string,
	input *models.CreateAssetRateInput,
) (*models.AssetRate, error) {
	const operation = "CreateOrUpdateAssetRate"

	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if input == nil {
		return nil, errors.NewMissingParameterError(operation, "input")
	}

	if err := input.Validate(); err != nil {
		return nil, errors.NewValidationError(operation, "invalid asset rate input", err)
	}

	url := e.buildURL(organizationID, ledgerID, "")

	body, err := json.Marshal(input)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	req, err := newRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var assetRate models.AssetRate
	if err := e.httpClient.sendRequest(req, &assetRate); err != nil {
		return nil, err
	}

	normalizeAssetRate(&assetRate)

	return &assetRate, nil
}

// GetAssetRate retrieves an asset rate by its external ID.
func (e *assetRatesEntity) GetAssetRate(
	ctx context.Context,
	organizationID, ledgerID, externalID string,
) (*models.AssetRate, error) {
	const operation = "GetAssetRate"

	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if strings.TrimSpace(externalID) == "" {
		return nil, errors.NewMissingParameterError(operation, "externalID")
	}

	url := e.buildURL(organizationID, ledgerID, externalID)

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	var assetRate models.AssetRate
	if err := e.httpClient.sendRequest(req, &assetRate); err != nil {
		return nil, err
	}

	normalizeAssetRate(&assetRate)

	return &assetRate, nil
}

// ListAssetRatesByAssetCode retrieves one page of asset rates.
func (e *assetRatesEntity) ListAssetRatesByAssetCode(
	ctx context.Context,
	organizationID, ledgerID, assetCode string,
	opts AssetRatesListOpts,
) (*models.ListResponse[models.AssetRate], error) {
	const operation = "ListAssetRatesByAssetCode"

	if strings.TrimSpace(organizationID) == "" {
		return nil, errors.NewMissingParameterError(operation, "organizationID")
	}

	if strings.TrimSpace(ledgerID) == "" {
		return nil, errors.NewMissingParameterError(operation, "ledgerID")
	}

	if strings.TrimSpace(assetCode) == "" {
		return nil, errors.NewMissingParameterError(operation, "assetCode")
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	url := e.buildFromAssetURL(organizationID, ledgerID, assetCode)

	req, err := newRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.NewInternalError(operation, err)
	}

	if params := opts.toQueryParams(); len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}

		req.URL.RawQuery = q.Encode()
	}

	var response models.ListResponse[models.AssetRate]
	if err := e.httpClient.sendRequest(req, &response); err != nil {
		return nil, err
	}

	if response.Items == nil {
		response.Items = []models.AssetRate{}
	}

	for i := range response.Items {
		normalizeAssetRate(&response.Items[i])
	}

	return &response, nil
}

// ListAssetRatesByAssetCodeAll returns an iter.Seq2 that drains every
// asset rate matching the asset code, advancing pagination cursors
// transparently.
//
// The iterator yields (zero-value, error) and stops on:
//   - validation failure of opts (yielded once before any HTTP)
//   - any transport or HTTP error
//   - context cancellation
//
// The iterator stops normally when the server returns a page with no
// NextCursor — there is no third "done" signal; the absence of a
// further yield IS the done signal.
func (e *assetRatesEntity) ListAssetRatesByAssetCodeAll(
	ctx context.Context,
	organizationID, ledgerID, assetCode string,
	opts AssetRatesListOpts,
) iter.Seq2[models.AssetRate, error] {
	return func(yield func(models.AssetRate, error) bool) {
		for page, err := range e.ListAssetRatesByAssetCodePages(ctx, organizationID, ledgerID, assetCode, opts) {
			if err != nil {
				var zero models.AssetRate
				yield(zero, err)
				return
			}

			for _, rate := range page.Items {
				if !yield(rate, nil) {
					return
				}
			}
		}
	}
}

// ListAssetRatesByAssetCodePages returns an iter.Seq2 that yields one
// full *ListResponse[AssetRate] per page, advancing pagination cursors
// transparently.
//
// Each yielded *ListResponse carries the full page including its
// Pagination shape, so callers can drive page-level UIs (progress
// bars), checkpoint cursors for resumable jobs, or short-circuit
// based on per-page metadata.
//
// On error the iterator yields (nil, err) and stops.
func (e *assetRatesEntity) ListAssetRatesByAssetCodePages(
	ctx context.Context,
	organizationID, ledgerID, assetCode string,
	opts AssetRatesListOpts,
) iter.Seq2[*models.ListResponse[models.AssetRate], error] {
	return func(yield func(*models.ListResponse[models.AssetRate], error) bool) {
		current := opts

		for {
			if ctx.Err() != nil {
				yield(nil, ctx.Err())
				return
			}

			page, err := e.ListAssetRatesByAssetCode(ctx, organizationID, ledgerID, assetCode, current)
			if err != nil {
				yield(nil, err)
				return
			}

			if !yield(page, nil) {
				return
			}

			next := page.Pagination.NextCursor
			if next == "" {
				return
			}

			current.Cursor = next
		}
	}
}

// buildURL builds the URL for asset rates API calls.
func (e *assetRatesEntity) buildURL(organizationID, ledgerID, externalID string) string {
	if externalID == "" {
		return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "asset-rates")
	}

	return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "asset-rates", externalID)
}

// buildFromAssetURL builds the URL for listing asset rates by source asset code.
func (e *assetRatesEntity) buildFromAssetURL(organizationID, ledgerID, assetCode string) string {
	return buildLedgerScopedURL(e.baseURLs["transaction"], organizationID, ledgerID, "asset-rates", "from", assetCode)
}

func normalizeAssetRate(assetRate *models.AssetRate) {
	if assetRate != nil && assetRate.Metadata == nil {
		assetRate.Metadata = map[string]any{}
	}
}
