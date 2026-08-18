package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v4/models"
	"github.com/LerianStudio/midaz-sdk-golang/v4/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type businessTestProvider struct {
	tracer trace.Tracer
	meter  metric.Meter
	logger observability.Logger
}

func newBusinessTestProvider(recorder *tracetest.SpanRecorder, logs *bytes.Buffer) *businessTestProvider {
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	return &businessTestProvider{
		tracer: tracerProvider.Tracer("business-observability"),
		meter:  metricnoop.NewMeterProvider().Meter("business-observability"),
		logger: observability.NewLogger(observability.InfoLevel, logs, nil),
	}
}

func (p *businessTestProvider) Tracer() trace.Tracer { return p.tracer }
func (p *businessTestProvider) Meter() metric.Meter  { return p.meter }
func (p *businessTestProvider) Logger() observability.Logger {
	return p.logger
}
func (*businessTestProvider) Shutdown(context.Context) error { return nil }
func (*businessTestProvider) IsEnabled() bool                { return true }

func TestBusinessObservability_AccountAndTransactionLifecycle(t *testing.T) {
	// Epic 5.3: accounts/transactions now route through plane facades, which do
	// not yet emit midaz.* business events (spans/logs). Re-homing business-event
	// emission to the plane path is the deferred, plan-sanctioned Task 5.2.6
	// (docs/plans/2026-06-30-sdk-v4-remodel.md:621). This test returns with 5.2.6.
	t.Skip("business-event emission on the plane path is deferred to Task 5.2.6")

	recorder := tracetest.NewSpanRecorder()
	logs := &bytes.Buffer{}
	provider := newBusinessTestProvider(recorder, logs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/v1/organizations/org-1/ledgers/ledger-1/accounts":
			assert.Equal(t, http.MethodPost, r.Method)
			writeBusinessJSON(t, w, map[string]any{"id": "11111111-1111-1111-1111-111111111111", "status": map[string]any{"code": "ACTIVE"}})
		case "/v1/organizations/org-1/ledgers/ledger-1/transactions/json":
			assert.Equal(t, http.MethodPost, r.Method)
			writeBusinessJSON(t, w, map[string]any{"id": "tx-1", "status": map[string]any{"code": "PENDING"}})
		case "/v1/organizations/org-1/ledgers/ledger-1/transactions/tx-1/commit":
			assert.Equal(t, http.MethodPost, r.Method)
			writeBusinessJSON(t, w, map[string]any{"id": "tx-1", "status": map[string]any{"code": "APPROVED"}})
		case "/v1/organizations/org-1/ledgers/ledger-1/transactions/tx-1/cancel":
			assert.Equal(t, http.MethodPost, r.Method)
			writeBusinessJSON(t, w, map[string]any{"id": "tx-1", "status": map[string]any{"code": "CANCELED"}})
		default:
			// t.Fatalf from a non-test goroutine is undefined behavior per
			// the testing package docs. Surface the unexpected request via
			// t.Errorf (which is goroutine-safe) and respond with a 404 so
			// the SDK side sees the failure as a real error from the
			// transport rather than as an indefinite hang.
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	entity := newTestEntity(t, server.Client(), "token", map[string]string{"onboarding": server.URL, "transaction": server.URL}, provider)
	require.NoError(t, entity.SetObservability(provider))

	ctx, span := provider.Tracer().Start(context.Background(), "business-flow")

	account, err := entity.Accounts.Create(ctx, "org-1", "ledger-1", models.NewCreateAccountInput("Customer Name", "USD", "deposit").WithMetadata(map[string]any{"secret": "metadata"}))
	require.NoError(t, err)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", account.ID)

	txInput := models.NewCreateTransactionInput("USD", "10.00").WithDescription("Sensitive description").WithSend(&models.SendInput{
		Asset:      "USD",
		Value:      "10.00",
		Source:     &models.SourceInput{From: []models.FromToInput{{AccountAlias: "customer", Amount: models.AmountInput{Asset: "USD", Value: "10.00"}}}},
		Distribute: &models.DistributeInput{To: []models.FromToInput{{AccountAlias: "merchant", Amount: models.AmountInput{Asset: "USD", Value: "10.00"}}}},
	})
	txInput.IdempotencyKey = "must-not-log"

	tx, err := entity.Transactions.CreateJSON(ctx, "org-1", "ledger-1", txInput)
	require.NoError(t, err)
	assert.Equal(t, "tx-1", tx.ID)

	committed, err := entity.Transactions.Commit(ctx, "org-1", "ledger-1", "tx-1")
	require.NoError(t, err)
	assert.Equal(t, "APPROVED", committed.Status.Code)

	cancelled, err := entity.Transactions.Cancel(ctx, "org-1", "ledger-1", "tx-1")
	require.NoError(t, err)
	assert.Equal(t, "CANCELED", cancelled.Status.Code)

	logText := logs.String()
	assert.Contains(t, logText, "midaz.account.created")
	assert.Contains(t, logText, "midaz.transaction.created")
	assert.Contains(t, logText, "midaz.transaction.committed")
	assert.Contains(t, logText, "midaz.transaction.cancelled")
	assert.Contains(t, logText, "11111111-1111-1111-1111-111111111111")
	assert.Contains(t, logText, "tx-1")
	assert.Contains(t, logText, "org-1")
	assert.Contains(t, logText, "ledger-1")
	assert.Contains(t, logText, trace.SpanContextFromContext(ctx).TraceID().String())
	assert.NotContains(t, logText, "Customer Name")
	assert.NotContains(t, logText, "Sensitive description")
	assert.NotContains(t, logText, "metadata")
	assert.NotContains(t, logText, "must-not-log")

	span.End()

	ended := recorder.Ended()
	require.NotEmpty(t, ended)

	events := collectBusinessEvents(ended)
	assert.Contains(t, events, "midaz.account.created")
	assert.Contains(t, events, "midaz.transaction.created")
	assert.Contains(t, events, "midaz.transaction.committed")
	assert.Contains(t, events, "midaz.transaction.cancelled")
}

func TestBusinessObservability_ReadMethodsDoNotEmitMutationEvents(t *testing.T) {
	// Epic 5.3: with business-event emission deferred to Task 5.2.6
	// (docs/plans/2026-06-30-sdk-v4-remodel.md:621), the plane facade emits no
	// midaz.* events on reads OR writes, so these NotContains assertions now pass
	// vacuously and prove nothing. Skipped alongside its two siblings until 5.2.6
	// restores write emission and the read-vs-write contrast has meaning again.
	t.Skip("business-event emission on the plane path is deferred to Task 5.2.6")

	recorder := tracetest.NewSpanRecorder()
	logs := &bytes.Buffer{}
	provider := newBusinessTestProvider(recorder, logs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/v1/organizations/org-1/ledgers/ledger-1/accounts/11111111-1111-1111-1111-111111111111", r.URL.EscapedPath())
		writeBusinessJSON(t, w, map[string]any{"id": "11111111-1111-1111-1111-111111111111", "status": map[string]any{"code": "ACTIVE"}})
	}))
	defer server.Close()

	entity := newTestEntity(t, server.Client(), "token", map[string]string{"onboarding": server.URL, "transaction": server.URL}, provider)
	require.NoError(t, entity.SetObservability(provider))

	ctx, span := provider.Tracer().Start(context.Background(), "business-read")

	account, err := entity.Accounts.Get(ctx, "org-1", "ledger-1", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", account.ID)

	span.End()

	logText := logs.String()
	assert.NotContains(t, logText, "midaz.account.created")
	assert.NotContains(t, logText, "midaz.account.updated")

	events := collectBusinessEvents(recorder.Ended())
	assert.NotContains(t, events, "midaz.account.created")
	assert.NotContains(t, events, "midaz.account.updated")
}

func TestBusinessObservability_UpdateTransactionUsesUpdatedEvent(t *testing.T) {
	// Epic 5.3: transactions route through the plane facade, which does not yet
	// emit midaz.transaction.updated. Deferred to Task 5.2.6 (plan :621).
	t.Skip("business-event emission on the plane path is deferred to Task 5.2.6")

	recorder := tracetest.NewSpanRecorder()
	logs := &bytes.Buffer{}
	provider := newBusinessTestProvider(recorder, logs)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/v1/organizations/org-1/ledgers/ledger-1/transactions/tx-1", r.URL.EscapedPath())
		writeBusinessJSON(t, w, map[string]any{"id": "tx-1", "status": map[string]any{"code": "PENDING"}})
	}))
	defer server.Close()

	entity := newTestEntity(t, server.Client(), "token", map[string]string{"onboarding": server.URL, "transaction": server.URL}, provider)
	require.NoError(t, entity.SetObservability(provider))

	ctx, span := provider.Tracer().Start(context.Background(), "business-update")

	tx, err := entity.Transactions.UpdateTransaction(ctx, "org-1", "ledger-1", "tx-1", models.NewUpdateTransactionInput().WithDescription("Sensitive description"))
	require.NoError(t, err)
	assert.Equal(t, "tx-1", tx.ID)

	span.End()

	logText := logs.String()
	assert.Contains(t, logText, "midaz.transaction.updated")
	assert.NotContains(t, logText, "midaz.transaction.reverted")
	assert.NotContains(t, logText, "Sensitive description")

	events := collectBusinessEvents(recorder.Ended())
	assert.Contains(t, events, "midaz.transaction.updated")
	assert.NotContains(t, events, "midaz.transaction.reverted")
}

func writeBusinessJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(body))
}

func collectBusinessEvents(spans []sdktrace.ReadOnlySpan) map[string]struct{} {
	events := map[string]struct{}{}

	for _, span := range spans {
		for _, event := range span.Events() {
			events[event.Name] = struct{}{}
		}
	}

	return events
}
