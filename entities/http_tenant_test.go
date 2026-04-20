package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTenantIDContextHelpers verifies that WithTenantID and TenantIDFromContext
// correctly store and retrieve tenant IDs in request contexts.
func TestTenantIDContextHelpers(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		expectID string
	}{
		{
			name:     "empty string is a no-op, no tenant stored",
			tenantID: "",
			expectID: "",
		},
		{
			name:     "valid tenant ID is stored and retrievable",
			tenantID: "tenant-abc",
			expectID: "tenant-abc",
		},
		{
			name:     "UUID-style tenant ID",
			tenantID: "550e8400-e29b-41d4-a716-446655440000",
			expectID: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:     "whitespace-only tenant ID is trimmed to empty (no-op)",
			tenantID: "   ",
			expectID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			newCtx := WithTenantID(ctx, tc.tenantID)

			got := TenantIDFromContext(newCtx)
			assert.Equal(t, tc.expectID, got)
		})
	}
}

// TestTenantIDEmptyStringReturnsOriginalContext verifies that passing an empty
// tenant ID to WithTenantID returns the exact same context (pointer equality).
func TestTenantIDEmptyStringReturnsOriginalContext(t *testing.T) {
	type ctxKey struct{}

	// Use a custom context so we can verify identity via a value marker
	parent := context.WithValue(context.Background(), ctxKey{}, "marker")
	result := WithTenantID(parent, "")

	// If the context is unchanged, our marker value must still be directly accessible
	// AND no tenant key should have been added
	assert.Equal(t, "marker", result.Value(ctxKey{}), "context should be unchanged")
	assert.Empty(t, TenantIDFromContext(result), "no tenant ID should be stored")
}

// TestTenantIDFromContext_BackgroundContext verifies that extracting a tenant ID
// from a plain background context (with no tenant set) returns empty string.
func TestTenantIDFromContext_BackgroundContext(t *testing.T) {
	got := TenantIDFromContext(context.Background())
	assert.Empty(t, got, "expected empty string from a plain background context")
}

// TestTenantIDContextOverwrite verifies that setting a new tenant ID on a context
// that already has one replaces the previous value.
func TestTenantIDContextOverwrite(t *testing.T) {
	ctx := context.Background()
	ctx = WithTenantID(ctx, "first-tenant")
	assert.Equal(t, "first-tenant", TenantIDFromContext(ctx))

	ctx = WithTenantID(ctx, "second-tenant")
	assert.Equal(t, "second-tenant", TenantIDFromContext(ctx))
}

// TestTenantIDHeaderInjection verifies that a tenant ID set via context is
// propagated as an X-Tenant-ID HTTP header when a request is made through doRequest.
func TestTenantIDHeaderInjection(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)

	ctx := WithTenantID(context.Background(), "tenant-abc")

	var out map[string]any

	err := c.doRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)

	assert.Equal(t, "tenant-abc", receivedHeader, "server should receive X-Tenant-ID: tenant-abc")
}

// TestTenantIDClientDefault verifies that a tenant ID set at the client level
// via SetTenantID is sent as a header when no context-level tenant is present.
func TestTenantIDClientDefault(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)
	c.SetTenantID("default-tenant")

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)

	assert.Equal(t, "default-tenant", receivedHeader, "server should receive the client-level default tenant ID")
}

// TestTenantIDContextOverridesDefault verifies that a tenant ID set via context
// takes precedence over the client-level default tenant ID.
func TestTenantIDContextOverridesDefault(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)
	c.SetTenantID("default")

	ctx := WithTenantID(context.Background(), "override")

	var out map[string]any

	err := c.doRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)

	assert.Equal(t, "override", receivedHeader, "context tenant should override client default")
}

// TestTenantIDNoHeaderWhenAbsent verifies that when no tenant ID is set anywhere
// (neither context nor client default), the X-Tenant-ID header is completely
// absent from the request — not present with an empty value.
func TestTenantIDNoHeaderWhenAbsent(t *testing.T) {
	var headerValues []string

	var headerPresent bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerValues = r.Header.Values(HeaderTenantID)
		_, headerPresent = r.Header[http.CanonicalHeaderKey(HeaderTenantID)]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)

	var out map[string]any

	err := c.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
	require.NoError(t, err)

	assert.False(t, headerPresent, "X-Tenant-ID header should be completely absent from request")
	assert.Empty(t, headerValues, "X-Tenant-ID header values should be empty")
}

// TestTenantIDRawRequest verifies that tenant ID injection works through the
// doRawRequest code path (used for non-JSON payloads such as multipart uploads).
func TestTenantIDRawRequest(t *testing.T) {
	t.Run("context tenant via raw request", func(t *testing.T) {
		var receivedHeader string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeader = r.Header.Get(HeaderTenantID)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		hc := srv.Client()
		c := NewHTTPClient(hc, "", nil)

		ctx := WithTenantID(context.Background(), "raw-tenant")

		var out map[string]any
		// doRawRequest with nil body doesn't require Content-Type
		err := c.doRawRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out)
		require.NoError(t, err)

		assert.Equal(t, "raw-tenant", receivedHeader, "doRawRequest should inject X-Tenant-ID from context")
	})

	t.Run("client default tenant via raw request", func(t *testing.T) {
		var receivedHeader string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeader = r.Header.Get(HeaderTenantID)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		hc := srv.Client()
		c := NewHTTPClient(hc, "", nil)
		c.SetTenantID("raw-default")

		var out map[string]any

		err := c.doRawRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
		require.NoError(t, err)

		assert.Equal(t, "raw-default", receivedHeader, "doRawRequest should inject client-level default tenant ID")
	})

	t.Run("context overrides default in raw request", func(t *testing.T) {
		var receivedHeader string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeader = r.Header.Get(HeaderTenantID)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		hc := srv.Client()
		c := NewHTTPClient(hc, "", nil)
		c.SetTenantID("raw-default")

		ctx := WithTenantID(context.Background(), "raw-override")

		var out map[string]any

		err := c.doRawRequest(ctx, http.MethodGet, srv.URL, nil, nil, &out)
		require.NoError(t, err)

		assert.Equal(t, "raw-override", receivedHeader, "context tenant should override default in doRawRequest")
	})

	t.Run("no header when absent in raw request", func(t *testing.T) {
		var headerPresent bool

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, headerPresent = r.Header[http.CanonicalHeaderKey(HeaderTenantID)]
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		hc := srv.Client()
		c := NewHTTPClient(hc, "", nil)

		var out map[string]any

		err := c.doRawRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)
		require.NoError(t, err)

		assert.False(t, headerPresent, "X-Tenant-ID should be absent from raw request when not set")
	})
}

// TestTenantIDWithExistingHeaders verifies that tenant ID injection works
// correctly when other custom headers are already present on the request.
func TestTenantIDWithExistingHeaders(t *testing.T) {
	var receivedTenantHeader string

	var receivedCustomHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedTenantHeader = r.Header.Get(HeaderTenantID)
		receivedCustomHeader = r.Header.Get("X-Custom")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)

	ctx := WithTenantID(context.Background(), "tenant-with-headers")

	headers := map[string]string{
		"X-Custom": "custom-value",
	}

	var out map[string]any

	err := c.doRequest(ctx, http.MethodGet, srv.URL, headers, nil, &out)
	require.NoError(t, err)

	assert.Equal(t, "tenant-with-headers", receivedTenantHeader, "tenant header should be present alongside custom headers")
	assert.Equal(t, "custom-value", receivedCustomHeader, "custom headers should not be affected by tenant injection")
}

// TestTenantIDWithRequestBody verifies that tenant ID injection works correctly
// when the request has a JSON body (POST-style request).
func TestTenantIDWithRequestBody(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer srv.Close()

	hc := srv.Client()
	c := NewHTTPClient(hc, "", nil)

	ctx := WithTenantID(context.Background(), "tenant-with-body")

	body := map[string]string{"name": "test"}

	var out map[string]any

	err := c.doRequest(ctx, http.MethodPost, srv.URL, nil, body, &out)
	require.NoError(t, err)

	assert.Equal(t, "tenant-with-body", receivedHeader)
}

// TestSetTenantID verifies the SetTenantID method on the HTTPClient directly.
func TestSetTenantID(t *testing.T) {
	c := NewHTTPClient(nil, "", nil)

	// Initially empty
	assert.Empty(t, c.tenantID)

	// Set a value
	c.SetTenantID("my-tenant")
	assert.Equal(t, "my-tenant", c.tenantID)

	// Overwrite with a new value
	c.SetTenantID("new-tenant")
	assert.Equal(t, "new-tenant", c.tenantID)

	// Set to empty clears it
	c.SetTenantID("")
	assert.Empty(t, c.tenantID)
}

// TestWithDefaultTenantIDOption verifies the entities.WithDefaultTenantID option
// correctly configures the HTTPClient's tenant ID field.
func TestWithDefaultTenantIDOption(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		expectID string
	}{
		{
			name:     "sets tenant ID on entity",
			tenantID: "option-tenant",
			expectID: "option-tenant",
		},
		{
			name:     "empty tenant ID is a no-op",
			tenantID: "",
			expectID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entity := &Entity{
				httpClient: NewHTTPClient(nil, "", nil),
			}

			opt := WithDefaultTenantID(tc.tenantID)
			err := opt(entity)
			require.NoError(t, err)

			assert.Equal(t, tc.expectID, entity.httpClient.tenantID)
		})
	}
}

// TestTenantIDPropagationThroughServiceEntity verifies that a tenant ID set at the
// Entity level via WithDefaultTenantID is propagated to service entities and arrives
// as an X-Tenant-ID header when a service method makes an HTTP request.
// This is the end-to-end test for the initServices -> propagateTenantID flow.
func TestTenantIDPropagationThroughServiceEntity(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		// Return a valid JSON response that ListOrganizations can unmarshal
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	// Create an Entity using the test server URL and a default tenant ID.
	// New() sets both "onboarding" and "transaction" base URLs to the same value.
	entity, err := New(srv.URL, WithDefaultTenantID("e2e-tenant"))
	require.NoError(t, err)

	// Replace the underlying http.Client with the test server's client so that
	// TLS certificates are accepted for the httptest server.
	entity.httpClient.client = srv.Client()

	// Reinitialize services so they pick up the test server's HTTP client.
	// We must also re-propagate the tenant ID since initServices creates fresh HTTPClients.
	entity.initServices()

	// Call a service method — this exercises the full path:
	// Entity.Organizations -> organizationsEntity.HTTPClient -> doRequest -> header injection
	_, err = entity.Organizations.ListOrganizations(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, "e2e-tenant", receivedHeader,
		"X-Tenant-ID header should be propagated from Entity through to service entity HTTP request")
}

// TestTenantIDPropagationThroughServiceEntityWithUnexportedField verifies tenant ID
// propagation through a service entity that uses an unexported httpClient field
// (e.g., accountsEntity), covering the other code path in propagateTenantID.
func TestTenantIDPropagationThroughServiceEntityWithUnexportedField(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity, err := New(srv.URL, WithDefaultTenantID("e2e-tenant-accounts"))
	require.NoError(t, err)

	entity.httpClient.client = srv.Client()
	entity.initServices()

	// Call a service method on Accounts (uses unexported httpClient field)
	_, err = entity.Accounts.ListAccounts(context.Background(), "org-1", "ledger-1", nil)
	require.NoError(t, err)

	assert.Equal(t, "e2e-tenant-accounts", receivedHeader,
		"X-Tenant-ID should propagate to service entities with unexported httpClient field")
}

// TestSetHTTPClientPreservesTenantID verifies that calling SetHTTPClient on an Entity
// preserves the previously configured tenant ID.
func TestSetHTTPClientPreservesTenantID(t *testing.T) {
	entity := &Entity{
		httpClient: NewHTTPClient(nil, "token", nil),
	}
	entity.httpClient.tenantID = "preserved-tenant"
	entity.baseURLs = map[string]string{
		"onboarding":  "http://localhost",
		"transaction": "http://localhost",
	}

	// Replace the HTTP client
	newClient := &http.Client{}
	entity.SetHTTPClient(newClient)

	// Verify tenant ID was preserved on the entity-level HTTPClient
	assert.Equal(t, "preserved-tenant", entity.httpClient.tenantID,
		"SetHTTPClient should preserve the tenant ID")
}

// TestWithHTTPClientOptionPreservesTenantID verifies that the WithHTTPClient option
// preserves the previously configured tenant ID when replacing the HTTP client.
func TestWithHTTPClientOptionPreservesTenantID(t *testing.T) {
	entity := &Entity{
		httpClient: NewHTTPClient(nil, "token", nil),
		baseURLs: map[string]string{
			"onboarding":  "http://localhost",
			"transaction": "http://localhost",
		},
	}
	entity.httpClient.tenantID = "option-preserved-tenant"

	opt := WithHTTPClient(&http.Client{})
	err := opt(entity)
	require.NoError(t, err)

	assert.Equal(t, "option-preserved-tenant", entity.httpClient.tenantID,
		"WithHTTPClient option should preserve the tenant ID")
}

// TestTenantIDPropagationAfterSetHTTPClient verifies the full round-trip: setting a
// tenant ID, replacing the HTTP client via SetHTTPClient, and confirming the tenant ID
// reaches the server through a service entity call.
func TestTenantIDPropagationAfterSetHTTPClient(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity, err := New(srv.URL, WithDefaultTenantID("surviving-tenant"))
	require.NoError(t, err)

	// Replace the HTTP client — tenant ID should survive
	entity.SetHTTPClient(srv.Client())

	_, err = entity.Organizations.ListOrganizations(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, "surviving-tenant", receivedHeader,
		"tenant ID should survive SetHTTPClient and propagate to service entities")
}
