package midaz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TestFacadesPropagateCallerTraceContext proves ledger and tracer facade calls carry
// the caller's trace context through the OTel global propagator, and add nothing
// when the host installed none. It swaps process-global state, so it is not parallel.
func TestFacadesPropagateCallerTraceContext(t *testing.T) {
	original := otel.GetTextMapPropagator()
	t.Cleanup(func() { otel.SetTextMapPropagator(original) })

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)

	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)

	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true,
	}))

	var (
		mu     sync.Mutex
		traces = map[string]string{}
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/v1/login/oauth/access_token" {
			_, _ = w.Write([]byte(`{"accessToken":"` + transportTestToken + `","expiresAt":"` +
				time.Now().Add(time.Hour).Format(time.RFC3339) + `"}`))

			return
		}

		mu.Lock()
		traces[r.URL.Path] = r.Header.Get("traceparent")
		mu.Unlock()

		_, _ = w.Write([]byte(`{"items":[],"limit":10}`))
	}))
	t.Cleanup(srv.Close)

	c, err := New(
		WithEnvironment(config.EnvironmentLocal),
		WithAccessManager(AccessManager{Address: srv.URL, ClientID: "id", ClientSecret: "secret"}),
		WithLedgerURL(srv.URL),
		WithTracerURL(srv.URL+"/v1"),
	)
	require.NoError(t, err)

	call := func() map[string]string {
		mu.Lock()
		traces = map[string]string{}
		mu.Unlock()

		_, err := c.V1.Organizations.List(ctx, models.OrganizationsListOpts{})
		require.NoError(t, err)
		_, err = c.Rules.List(ctx, models.RulesListOpts{})
		require.NoError(t, err)

		mu.Lock()
		defer mu.Unlock()

		return map[string]string{"ledger": traces["/v1/organizations"], "tracer": traces["/v1/rules"]}
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	for plane, header := range call() {
		assert.Contains(t, header, traceID.String(), "%s plane must carry the caller's trace id", plane)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	for plane, header := range call() {
		assert.Empty(t, header, "%s plane must add no header without a propagator", plane)
	}
}
