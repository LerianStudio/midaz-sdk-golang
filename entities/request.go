package entities

import (
	"context"
	"io"
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

func prepareServiceBaseURLs(baseURLs map[string]string) map[string]string {
	prepared := copyBaseURLs(baseURLs)
	for service, serviceURL := range prepared {
		prepared[service] = strings.TrimRight(strings.TrimSpace(serviceURL), "/")
	}

	return prepared
}
