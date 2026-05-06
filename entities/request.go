package entities

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
)

func requestContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}

	return ctx
}

func newRequestWithContext(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(requestContext(ctx), method, url, body)
}

// prepareServiceBaseURLs returns a normalized clone of the input map. It
// trims surrounding whitespace and trailing slashes off each base URL so
// downstream concatenation is forgiving about input shape. The returned map
// is independent of the caller's map (we use maps.Clone, which mirrors the
// previous "copy then mutate" semantics without a hand-rolled helper).
func prepareServiceBaseURLs(baseURLs map[string]string) map[string]string {
	prepared := maps.Clone(baseURLs)
	for service, serviceURL := range prepared {
		prepared[service] = strings.TrimRight(strings.TrimSpace(serviceURL), "/")
	}

	return prepared
}

func buildLedgerScopedURL(baseURL, organizationID, ledgerID string, parts ...string) string {
	segments := []string{
		"organizations",
		pathSegment(organizationID),
		"ledgers",
		pathSegment(ledgerID),
	}

	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}

		for _, piece := range strings.Split(part, "/") {
			if piece != "" {
				segments = append(segments, pathSegment(piece))
			}
		}
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(baseURL, "/"), strings.Join(segments, "/"))
}
