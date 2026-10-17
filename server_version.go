package midaz

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/serverversion"
)

type (
	// ServerVersion is what the connected Midaz ledger says it is.
	ServerVersion = serverversion.ServerVersion
	// FeeMode is who owns fee application against the connected ledger.
	FeeMode = serverversion.FeeMode
	// CRMAPI is which CRM API answers at a CRM URL.
	CRMAPI = serverversion.CRMAPI
)

// The fee modes [ResolveFeeMode] returns.
const (
	FeeModeLegacy = serverversion.FeeModeLegacy
	FeeModeNative = serverversion.FeeModeNative
)

// The CRM APIs [ResolveCRMAPI] returns.
const (
	CRMAPIV1 = serverversion.CRMAPIV1
	CRMAPIV2 = serverversion.CRMAPIV2
)

const (
	serverVersionOperation = "midaz.Client.ServerVersion"
	maxServerVersionBody   = 64 << 10 // a real /version body is under 1 KiB
)

var errNotVersionBody = errors.New("body is not a /version response")

// ServerVersion reads the ledger's public GET /version route. The error is non-nil iff Source is
// unavailable (legacy fees): transport, non-2xx, or a 2xx body that is not a /version response.
// A placeholder version is Known=false with no error. Cache the result; never resolve per request.
func (c *Client) ServerVersion(ctx context.Context) (ServerVersion, error) {
	if c == nil || c.config == nil {
		return ServerVersion{Source: serverversion.SourceUnavailable},
			sdkerrors.NewConfigurationError(serverVersionOperation, "client is not initialized; build it with midaz.New", nil)
	}

	return readServerVersion(ctx, c.GetHTTPClient(), c.config.LedgerURL, serverVersionOperation)
}

// readServerVersion GETs baseURL+"/version" through doer, with no credential, and parses the body.
// The error is non-nil iff Source is unavailable.
func readServerVersion(ctx context.Context, doer httpDoer, baseURL, operation string) (ServerVersion, error) {
	unavailable := ServerVersion{Source: serverversion.SourceUnavailable}
	url := strings.TrimSuffix(baseURL, "/") + "/version"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return unavailable, sdkerrors.NewConfigurationError(operation, "build GET /version request", err)
	}

	resp, err := doer.Do(req) // #nosec G704 -- baseURL was validated when the client or reader was built
	if err != nil {
		return unavailable, sdkerrors.ClassifyTransportError(operation, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return unavailable, sdkerrors.NewUpstreamHTTPError(operation, "GET /version", resp.StatusCode, nil)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxServerVersionBody))
	if err != nil {
		return unavailable, sdkerrors.ClassifyTransportError(operation, err)
	}

	if v := serverversion.Parse(body); v.Source != serverversion.SourceUnavailable {
		return v, nil
	}

	return unavailable, sdkerrors.NewResponseDecodeError(operation, resp.StatusCode, errNotVersionBody)
}

// ResolveFeeMode is the single decision rule: native iff Known && version >= 4.1.0.
func ResolveFeeMode(v ServerVersion) FeeMode {
	return serverversion.ResolveFeeMode(v)
}

// ResolveCRMAPI decides the CRM API from the /version served at the CRM URL: CRMAPIV2 iff Known && major >= 4.
func ResolveCRMAPI(v ServerVersion) CRMAPI {
	return serverversion.ResolveCRMAPI(v)
}
