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

	// Pointer identity: WithTenantID must return the exact same context for empty input
	assert.Same(t, parent, result, "WithTenantID should return the original context for empty input")

	// If the context is unchanged, our marker value must still be directly accessible
	// AND no tenant key should have been added
	assert.Equal(t, "marker", result.Value(ctxKey{}), "context should be unchanged")
	assert.Empty(t, TenantIDFromContext(result), "no tenant ID should be stored")
}

// TestTenantIDWhitespaceOnlyReturnsOriginalContext verifies that passing a
// whitespace-only tenant ID returns the exact same context (pointer equality).
func TestTenantIDWhitespaceOnlyReturnsOriginalContext(t *testing.T) {
	type ctxKey struct{}

	parent := context.WithValue(context.Background(), ctxKey{}, "marker")
	result := WithTenantID(parent, "   ")

	assert.Same(t, parent, result, "WithTenantID should return the original context for whitespace-only input")
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

// requestRunner abstracts doRequest and doRawRequest so tenant header tests
// can exercise both code paths through a single table-driven matrix.
type requestRunner func(ctx context.Context, c *HTTPClient, url string, headers map[string]string, body any, out any) error

func doRequestRunner(ctx context.Context, c *HTTPClient, url string, headers map[string]string, body any, out any) error {
	return c.doRequest(ctx, http.MethodGet, url, headers, body, out)
}

func doRawRequestRunner(ctx context.Context, c *HTTPClient, url string, headers map[string]string, _ any, out any) error {
	return c.doRawRequest(ctx, http.MethodGet, url, headers, nil, out)
}

// TestTenantIDHeaderMatrix exercises tenant header injection across both doRequest
// and doRawRequest with a shared table of precedence cases.
func TestTenantIDHeaderMatrix(t *testing.T) {
	runners := map[string]requestRunner{
		"doRequest":    doRequestRunner,
		"doRawRequest": doRawRequestRunner,
	}

	cases := []struct {
		name           string
		ctxTenant      string // tenant set via WithTenantID on context; empty = no context tenant
		clientTenant   string // tenant set via SetTenantID on client; empty = no client default
		expectedHeader string // expected X-Tenant-ID value; empty = header absent
	}{
		{
			name:           "context tenant injected",
			ctxTenant:      "tenant-abc",
			expectedHeader: "tenant-abc",
		},
		{
			name:           "client default tenant",
			clientTenant:   "default-tenant",
			expectedHeader: "default-tenant",
		},
		{
			name:           "context overrides client default",
			ctxTenant:      "override",
			clientTenant:   "default",
			expectedHeader: "override",
		},
		{
			name:           "no header when absent",
			expectedHeader: "",
		},
	}

	for runnerName, run := range runners {
		for _, tc := range cases {
			t.Run(runnerName+"/"+tc.name, func(t *testing.T) {
				var receivedHeader string

				var headerPresent bool

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					receivedHeader = r.Header.Get(HeaderTenantID)
					_, headerPresent = r.Header[http.CanonicalHeaderKey(HeaderTenantID)]
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{}`))
				}))
				defer srv.Close()

				hc := srv.Client()
				c := NewHTTPClient(hc, "", nil)

				if tc.clientTenant != "" {
					c.SetTenantID(tc.clientTenant)
				}

				ctx := context.Background()
				if tc.ctxTenant != "" {
					ctx = WithTenantID(ctx, tc.ctxTenant)
				}

				var out map[string]any

				err := run(ctx, c, srv.URL, nil, nil, &out)
				require.NoError(t, err)

				if tc.expectedHeader == "" {
					assert.False(t, headerPresent, "X-Tenant-ID should be absent")
				} else {
					assert.Equal(t, tc.expectedHeader, receivedHeader)
				}
			})
		}
	}
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
// preserves the previously configured tenant ID when replacing the HTTP client,
// and that the tenant ID is propagated end-to-end through actual service requests.
func TestWithHTTPClientOptionPreservesTenantID(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get(HeaderTenantID)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	entity, err := New(srv.URL, WithDefaultTenantID("option-preserved-tenant"))
	require.NoError(t, err)

	// Verify root HTTPClient has the tenant
	assert.Equal(t, "option-preserved-tenant", entity.httpClient.tenantID,
		"root HTTPClient should have the tenant ID")

	// Replace the HTTP client via option
	opt := WithHTTPClient(srv.Client())
	err = opt(entity)
	require.NoError(t, err)

	// Verify root field survived
	assert.Equal(t, "option-preserved-tenant", entity.httpClient.tenantID,
		"WithHTTPClient option should preserve the tenant ID")

	// End-to-end: verify the tenant header reaches the server
	_, err = entity.Organizations.ListOrganizations(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, "option-preserved-tenant", receivedHeader,
		"tenant ID should propagate to service entities after WithHTTPClient")
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
