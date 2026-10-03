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

var errNotVersionBody = errors.New("body is not a /version response")

// ServerVersion reads the ledger's public GET /version route. The error is non-nil iff Source is
// unavailable (legacy fees): transport, non-2xx, or a 2xx body that is not a /version response.
// A placeholder version is Known=false with no error. Cache the result; never resolve per request.
func (c *Client) ServerVersion(ctx context.Context) (ServerVersion, error) {
	unavailable := ServerVersion{Source: serverversion.SourceUnavailable}

	if c == nil || c.config == nil {
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

	if v := serverversion.Parse(body); v.Source != serverversion.SourceUnavailable {
		return v, nil
	}

	return unavailable, sdkerrors.NewResponseDecodeError(serverVersionOperation, resp.StatusCode, errNotVersionBody)
}

// ResolveFeeMode is the single decision rule: native iff Known && version >= 4.1.0.
func ResolveFeeMode(v ServerVersion) FeeMode {
	return serverversion.ResolveFeeMode(v)
}
