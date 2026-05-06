package entities

//go:generate mockgen -source=asset_rates.go -destination=mocks/mock_asset_rates.go -package=mocks AssetRatesService

import (
	"bytes"
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"strings"

	"github.com/LerianStudio/midaz-sdk-golang/v3/models"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
)

// AssetRatesService defines the interface for asset rate operations.
//
// AssetRates intentionally has no separate CreateAssetRate / UpdateAssetRate
// pair (audit 7.17 — Track 7F). The Midaz server endpoint is upsert-style
// (PUT-semantics keyed on the from/to asset code tuple), so the SDK
// surfaces a single CreateOrUpdateAssetRate method that mirrors the wire
// contract truthfully. Splitting it into a fictitious Create + Update
// pair would be DX theater that diverges from server reality.
type AssetRatesService interface {
	// CreateOrUpdateAssetRate creates a new asset rate or updates an existing one.
	//
	// This method uses PUT semantics — if an asset rate with the same from/to asset
	// codes exists, it will be updated; otherwise, a new one will be created.
	// There is no separate Update endpoint; this method is the canonical mutation path.
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
	//   - opts: typed options; pass models.AssetRatesListOpts{} for defaults
	//
	// Example:
	//
	//	resp, err := client.AssetRates.ListAssetRatesByAssetCode(
	//	    ctx, "org-123", "ledger-456", "USD",
	//	    models.AssetRatesListOpts{
	//	        CursorListOpts: models.CursorListOpts{Limit: 50},
	//	        Filters:        models.AssetRatesFilters{To: []string{"BRL", "EUR"}},
	//	    },
	//	)
	ListAssetRatesByAssetCode(ctx context.Context, organizationID, ledgerID, assetCode string, opts models.AssetRatesListOpts) (*models.ListResponse[models.AssetRate], error)

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
	//	    models.AssetRatesListOpts{CursorListOpts: models.CursorListOpts{Limit: 100}},
	//	) {
	//	    if err != nil {
	//	        return fmt.Errorf("asset rates iteration failed: %w", err)
	//	    }
	//	    process(rate)
	//	}
	ListAssetRatesByAssetCodeAll(ctx context.Context, organizationID, ledgerID, assetCode string, opts models.AssetRatesListOpts) iter.Seq2[models.AssetRate, error]

	// ListAssetRatesByAssetCodePages yields one full *ListResponse
	// per page, transparently following pagination cursors. Use this
	// when the caller needs page-level metadata (Pagination shape,
	// per-page item counts, or cursor checkpointing for resumable
	// jobs).
	//
	// For item-level iteration prefer ListAssetRatesByAssetCodeAll.
	ListAssetRatesByAssetCodePages(ctx context.Context, organizationID, ledgerID, assetCode string, opts models.AssetRatesListOpts) iter.Seq2[*models.ListResponse[models.AssetRate], error]
}

// assetRatesEntity implements the AssetRatesService interface.
type assetRatesEntity struct {
	serviceEntity
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
	return &assetRatesEntity{serviceEntity: newServiceEntity(client, authToken, baseURLs)}
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
	opts models.AssetRatesListOpts,
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

	if params := opts.ToQueryParams(); len(params) > 0 {
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
	opts models.AssetRatesListOpts,
) iter.Seq2[models.AssetRate, error] {
	return flattenPages(e.ListAssetRatesByAssetCodePages(ctx, organizationID, ledgerID, assetCode, opts))
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
	opts models.AssetRatesListOpts,
) iter.Seq2[*models.ListResponse[models.AssetRate], error] {
	ctx = requestContext(ctx)

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
