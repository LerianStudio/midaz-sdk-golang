package entities

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdkerrors "github.com/LerianStudio/midaz-sdk-golang/v3/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v3/pkg/sdkctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type slice3Provider struct {
	tracer trace.Tracer
	meter  metric.Meter
	logger observability.Logger
}

func newSlice3Provider(recorder *tracetest.SpanRecorder) *slice3Provider {
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	return &slice3Provider{
		tracer: tracerProvider.Tracer("slice3-regression"),
		meter:  metricnoop.NewMeterProvider().Meter("slice3-regression"),
		logger: observability.NewNoopLogger(),
	}
}

func (p *slice3Provider) Tracer() trace.Tracer { return p.tracer }
func (p *slice3Provider) Meter() metric.Meter  { return p.meter }
func (p *slice3Provider) Logger() observability.Logger {
	return p.logger
}
func (*slice3Provider) Shutdown(context.Context) error { return nil }
func (*slice3Provider) IsEnabled() bool                { return true }

func TestHTTPClient_ParseErrorResponse_PreservesStructuredMidazEnvelope(t *testing.T) {
	body := []byte(`{
		"code":"MIDAZ-0042",
		"title":"Validation failed",
		"message":"invalid account payload",
		"entityType":"account",
		"fields":["legalDocument","metadata.taxId"],
		"details":{"reason":"invalid document","document":"12345678900"}
	}`)

	err := (*HTTPClient)(nil).parseErrorResponse(http.StatusBadRequest, body, "req-123")

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, "MIDAZ-0042", sdkErr.APICode)
	assert.Equal(t, "Validation failed", sdkErr.Title)
	assert.Equal(t, "account", sdkErr.EntityType)
	assert.Equal(t, []string{"legalDocument", "metadata.taxId"}, sdkErr.Fields)
	assert.Equal(t, "invalid document", sdkErr.Details["reason"])
	assert.Equal(t, "[REDACTED]", sdkErr.Details["document"])
	assert.Equal(t, "req-123", sdkErr.RequestID)
}

func TestHTTPClient_ParseErrorResponse_PreservesCRMErrPayloadInDetails(t *testing.T) {
	body := []byte(`{
		"code":"CRM-400",
		"title":"CRM validation failed",
		"message":"invalid holder document",
		"err":{"field":"document","reason":"checksum failed"}
	}`)

	err := (*HTTPClient)(nil).parseErrorResponse(http.StatusBadRequest, body, "req-crm")

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, "CRM-400", sdkErr.APICode)
	assert.Equal(t, "CRM validation failed", sdkErr.Title)
	require.Contains(t, sdkErr.Details, "err")
	errPayload, ok := sdkErr.Details["err"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "document", errPayload["field"])
	assert.Equal(t, "checksum failed", errPayload["reason"])
}

func TestHTTPClient_CountRequest_ValidatesTotalCountHeader(t *testing.T) {
	tests := []struct {
		name      string
		header    *string
		want      int
		wantError string
	}{
		{name: "missing header", wantError: "missing X-Total-Count header"},
		{name: "whitespace header", header: stringPtr("   \t"), wantError: "missing X-Total-Count header"},
		{name: "nonnumeric header", header: stringPtr("not-a-number"), wantError: "invalid X-Total-Count header"},
		{name: "negative header", header: stringPtr("-1"), wantError: "invalid X-Total-Count header"},
		{name: "overflow header", header: stringPtr("999999999999999999999999999999"), wantError: "invalid X-Total-Count header"},
		{name: "valid header with spaces", header: stringPtr(" 42 "), want: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeErrs := make(chan error, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.header != nil {
					w.Header().Set(HeaderTotalCount, *tt.header)
				}

				writeErrs <- nil
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.Client(), "", nil)
			got, err := client.doCountRequest(context.Background(), countRequestMethod(), srv.URL, countRequestHeaders())

			requireHandlerNoError(t, writeErrs)

			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				if tt.header != nil {
					assert.NotContains(t, err.Error(), *tt.header)
				}

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHTTPClient_ContextIdempotencyKey_SkipsSafeMethods(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		useCount   bool
		wantHeader string
	}{
		{name: "GET omits context idempotency", method: http.MethodGet},
		{name: "HEAD count omits context idempotency", method: http.MethodHead, useCount: true},
		{name: "POST includes context idempotency", method: http.MethodPost, wantHeader: "idem-safe-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(chan string, 1)
			writeErrs := make(chan error, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen <- r.Header.Get("X-Idempotency")

				w.Header().Set(HeaderTotalCount, "7")
				w.Header().Set("Content-Type", "application/json")

				if tt.method != http.MethodHead {
					_, err := w.Write([]byte(`{}`))
					writeErrs <- err

					return
				}

				writeErrs <- nil
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.Client(), "", nil)
			ctx := sdkctx.WithIdempotencyKey(context.Background(), "idem-safe-test")

			if tt.useCount {
				_, err := client.doCountRequest(ctx, tt.method, srv.URL, nil)
				require.NoError(t, err)
			} else {
				var out map[string]any

				err := client.doRequest(ctx, tt.method, srv.URL, nil, map[string]string{"ok": "true"}, &out)
				require.NoError(t, err)
			}

			requireHandlerNoError(t, writeErrs)
			assert.Equal(t, tt.wantHeader, <-seen)
		})
	}
}

func TestHTTPClient_CountEnvelopeFailure_RecordsErrorSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := newSlice3Provider(recorder)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(HeaderTotalCount, "not-a-number")
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client(), "", provider)
	_, err := client.doCountRequest(context.Background(), countRequestMethod(), srv.URL, nil)
	require.Error(t, err)

	spans := recorder.Ended()
	require.NotEmpty(t, spans)
	assert.True(t, spanHasErrorStatus(spans[len(spans)-1]), "malformed count envelope should mark the request span as failed")
}

func TestHTTPClient_ProcessResponse_EmptyNullAndMalformedBodiesReturnErrors(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantError string
	}{
		{name: "empty response", body: nil, wantError: "empty response body"},
		{name: "null response", body: []byte("null"), wantError: "null response body"},
		{name: "malformed response", body: []byte(`{"id":`), wantError: "failed to unmarshal response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(&http.Client{}, "", nil)

			var out map[string]any

			err := client.processResponse(&out, tt.body)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestHTTPClient_ErrorResponse_RequestIDPropagates(t *testing.T) {
	writeErrs := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-ID", "req-observable")
		w.WriteHeader(http.StatusBadRequest)

		writeErrs <- json.NewEncoder(w).Encode(map[string]string{"message": "bad request"})
	}))
	defer srv.Close()

	client := NewHTTPClient(srv.Client(), "", nil)

	var out map[string]any

	err := client.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)

	requireHandlerNoError(t, writeErrs)

	var sdkErr *sdkerrors.Error
	require.ErrorAs(t, err, &sdkErr)
	assert.Equal(t, "req-observable", sdkErr.RequestID)
}

func TestHTTPClient_ErrorResponse_EmptyNullAndMalformedBodiesReturnStructuredErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty error body", body: ""},
		{name: "null error body", body: "null"},
		{name: "malformed error body", body: `{"message":`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeErrs := make(chan error, 1)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)

				_, err := io.Copy(w, strings.NewReader(tt.body))
				writeErrs <- err
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.Client(), "", nil)

			var out map[string]any

			err := client.doRequest(context.Background(), http.MethodGet, srv.URL, nil, nil, &out)

			requireHandlerNoError(t, writeErrs)

			var sdkErr *sdkerrors.Error
			require.ErrorAs(t, err, &sdkErr)
			assert.Equal(t, http.StatusBadRequest, sdkErr.StatusCode)
			assert.NotEmpty(t, sdkErr.Message)
		})
	}
}

func stringPtr(value string) *string { return &value }

func spanHasErrorStatus(span sdktrace.ReadOnlySpan) bool {
	return span.Status().Code.String() == "Error"
}
