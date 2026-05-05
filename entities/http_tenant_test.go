package entities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: standalone tests of the context helpers (storage, retrieval, pointer
// identity for empty/whitespace inputs, overwrite semantics) live in
// pkg/sdkctx/sdkctx_test.go since v3 — that package owns the canonical
// helpers. This file focuses exclusively on integration: the X-Tenant-ID
// HTTP header injection that takes the sdkctx-stored value and propagates
// it into the request line.

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
		ctxTenant      string // tenant set via sdkctx.WithRequestTenantID on context; empty = no context tenant
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
					ctx = sdkctx.WithRequestTenantID(ctx, tc.ctxTenant)
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

	ctx := sdkctx.WithRequestTenantID(context.Background(), "tenant-with-headers")

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

	ctx := sdkctx.WithRequestTenantID(context.Background(), "tenant-with-body")

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
	// We pass srv.Client() up front so TLS certificates are accepted for the httptest server.
	entity := newTestEntity(t, srv.Client(), "", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
		"crm":         srv.URL,
	}, nil, WithDefaultTenantID("e2e-tenant"))

	// Call a service method — this exercises the full path:
	// Entity.Organizations -> organizationsEntity.HTTPClient -> doRequest -> header injection
	_, err := entity.Organizations.ListOrganizations(context.Background(), nil)
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

	entity := newTestEntity(t, srv.Client(), "", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
		"crm":         srv.URL,
	}, nil, WithDefaultTenantID("e2e-tenant-accounts"))

	// Call a service method on Accounts (uses unexported httpClient field)
	_, err := entity.Accounts.ListAccounts(context.Background(), "org-1", "ledger-1", nil)
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

	entity := newTestEntity(t, &http.Client{Timeout: 30 * time.Second}, "", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
		"crm":         srv.URL,
	}, nil, WithDefaultTenantID("option-preserved-tenant"))

	// Verify root HTTPClient has the tenant
	assert.Equal(t, "option-preserved-tenant", entity.httpClient.tenantID,
		"root HTTPClient should have the tenant ID")

	// Replace the HTTP client via option
	opt := WithHTTPClient(srv.Client())
	err := opt(entity)
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

	entity := newTestEntity(t, &http.Client{Timeout: 30 * time.Second}, "", map[string]string{
		"onboarding":  srv.URL,
		"transaction": srv.URL,
		"crm":         srv.URL,
	}, nil, WithDefaultTenantID("surviving-tenant"))

	// Replace the HTTP client — tenant ID should survive
	entity.SetHTTPClient(srv.Client())

	_, err := entity.Organizations.ListOrganizations(context.Background(), nil)
	require.NoError(t, err)

	assert.Equal(t, "surviving-tenant", receivedHeader,
		"tenant ID should survive SetHTTPClient and propagate to service entities")
}
