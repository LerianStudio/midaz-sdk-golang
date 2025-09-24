package entities

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestIdempotencyHeaderInjection(t *testing.T) {
    var seen string

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        seen = r.Header.Get("X-Idempotency")
        w.Header().Set("Content-Type", "application/json")
        _, _ = w.Write([]byte(`{}`))
    }))
    defer srv.Close()

    hc := srv.Client()
    c := NewHTTPClient(hc, "", nil)

    ctx := WithIdempotencyKey(context.Background(), "abc123")

    var out map[string]any
    if err := c.doRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out); err != nil {
        t.Fatalf("doRequest failed: %v", err)
    }

    if seen != "abc123" {
        t.Fatalf("expected X-Idempotency header 'abc123', got '%s'", seen)
    }
}

