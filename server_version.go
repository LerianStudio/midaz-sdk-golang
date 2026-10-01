package midaz

import (
	"context"
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
)

// The fee modes [ResolveFeeMode] returns.
const (
	FeeModeLegacy = serverversion.FeeModeLegacy
	FeeModeNative = serverversion.FeeModeNative
)

const (
	serverVersionOperation = "midaz.Client.ServerVersion"
	maxServerVersionBody   = 64 << 10 // a real /version body is under 1 KiB
)

// ServerVersion reads the ledger's public GET /version route. Any failure gives
// Source unavailable (legacy fees) and an error to log; an unknown version is no error.
// Callers cache the result and refresh it periodically, never resolving it per request.
func (c *Client) ServerVersion(ctx context.Context) (ServerVersion, error) {
	unavailable := ServerVersion{Source: serverversion.SourceUnavailable}

	if c == nil || c.config == nil || c.GetHTTPClient() == nil {
		return unavailable, sdkerrors.NewConfigurationError(serverVersionOperation, "client is not initialized; build it with midaz.New", nil)
	}

	url := strings.TrimSuffix(c.config.LedgerURL, "/") + "/version"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return unavailable, sdkerrors.NewConfigurationError(serverVersionOperation, "build GET /version request", err)
	}

	resp, err := c.GetHTTPClient().Do(req) // #nosec G704 -- LedgerURL was validated when the client was built
	if err != nil {
		return unavailable, sdkerrors.ClassifyTransportError(serverVersionOperation, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return unavailable, sdkerrors.NewUpstreamHTTPError(serverVersionOperation, "GET /version", resp.StatusCode, nil)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxServerVersionBody))
	if err != nil {
		return unavailable, sdkerrors.ClassifyTransportError(serverVersionOperation, err)
	}

	return serverversion.Parse(body), nil
}

// ResolveFeeMode is the single decision rule: native iff Known && Major >= 4.
func ResolveFeeMode(v ServerVersion) FeeMode {
	return serverversion.ResolveFeeMode(v)
}
