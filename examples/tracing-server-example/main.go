package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// sanitizeLogInput removes control characters from strings to prevent log injection attacks.
func sanitizeLogInput(input string) string {
	sanitized := strings.ReplaceAll(input, "\n", "\\n")
	sanitized = strings.ReplaceAll(sanitized, "\r", "\\r")
	sanitized = strings.ReplaceAll(sanitized, "\t", "\\t")

	return sanitized
}

// This example demonstrates server-side tracing propagation with incoming HTTP requests
func main() {
	// Create observability provider
	provider, err := observability.New(context.Background(),
		observability.WithServiceName("midaz-server"),
		observability.WithServiceVersion("1.0.0"),
		observability.WithEnvironment("development"),
		observability.WithComponentEnabled(true, true, true),
		observability.WithFullTracingSampling(),
	)
	if err != nil {
		log.Fatalf("Failed to create observability provider: %v", err)
	}
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down observability provider: %v", err)
		}
	}()

	// Create Midaz client for downstream calls
	// Set auth token via environment variable or replace "your-api-token" with actual token
	err = os.Setenv("MIDAZ_AUTH_TOKEN", "your-api-token")
	if err != nil {
		log.Fatalf("Failed to set MIDAZ_AUTH_TOKEN environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MIDAZ_AUTH_TOKEN"); err != nil {
			log.Printf("Warning: Failed to unset MIDAZ_AUTH_TOKEN: %v", err)
		}
	}()

	midazClient, err := client.New(
		client.WithBaseURL("https://api.midaz.com"),
		client.WithObservabilityProvider(provider),
	)
	if err != nil {
		log.Fatalf("Failed to create Midaz client: %v", err)
	}

	// Create HTTP server with tracing middleware
	server := &Server{
		provider:    provider,
		midazClient: midazClient,
	}

	// Set up routes with tracing middleware
	mux := http.NewServeMux()

	// Wrap each handler with tracing middleware
	mux.Handle("/api/organizations", server.tracingMiddleware(http.HandlerFunc(server.handleOrganizations)))
	mux.Handle("/api/ledgers", server.tracingMiddleware(http.HandlerFunc(server.handleLedgers)))
	mux.Handle("/api/health", server.tracingMiddleware(http.HandlerFunc(server.handleHealth)))

	fmt.Println("Server starting on :8080")
	fmt.Println("Test with: curl -H 'traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01' http://localhost:8080/api/organizations")

	// Create HTTP server with proper timeouts for security
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// Server represents our HTTP server with tracing capabilities
type Server struct {
	provider    observability.Provider
	midazClient *client.Client
}

// tracingMiddleware extracts tracing context from incoming requests and creates spans
func (s *Server) tracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip tracing for health checks if desired
		if r.URL.Path == "/api/health" && r.Header.Get("traceparent") == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract tracing context from incoming request headers
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}

		// Extract trace context
		ctx := observability.ExtractContext(r.Context(), headers)

		// Start a new span for this request
		tracer := s.provider.Tracer()
		operationName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := tracer.Start(ctx, operationName,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Add request attributes
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.scheme", r.URL.Scheme),
			attribute.String("http.host", r.Host),
			attribute.String("http.target", r.URL.Path),
			attribute.String("user_agent.original", r.UserAgent()),
		)

		// Extract correlation IDs from headers for baggage
		if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
			ctx, _ = observability.WithBaggageItem(ctx, "request-id", requestID) //nolint:errcheck // baggage errors are non-fatal
			span.SetAttributes(attribute.String("request.id", requestID))
		}

		if userID := r.Header.Get("X-User-ID"); userID != "" {
			ctx, _ = observability.WithBaggageItem(ctx, "user-id", userID) //nolint:errcheck // baggage errors are non-fatal
			span.SetAttributes(attribute.String("user.id", userID))
		}

		// Create response writer wrapper to capture status code
		wrapper := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
		}

		// Update request context and call next handler
		r = r.WithContext(ctx)
		next.ServeHTTP(wrapper, r)

		// Set response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", wrapper.statusCode),
		)

		// Set span status based on HTTP status code
		if wrapper.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrapper.statusCode))
		} else {
			span.SetStatus(codes.Ok, "Request completed successfully")
		}
	})
}

// handleOrganizations demonstrates handling incoming requests with tracing
func (s *Server) handleOrganizations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := s.provider.Logger()

	// Log with trace context
	traceID := observability.TraceID(ctx)
	spanID := observability.SpanID(ctx)
	logger.Info("Handling organizations request",
		"trace_id", traceID,
		"span_id", spanID,
		"method", r.Method,
	)

	switch r.Method {
	case http.MethodGet:
		s.listOrganizations(ctx, w, r)
	case http.MethodPost:
		s.createOrganization(ctx, w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listOrganizations lists organizations with downstream tracing
func (s *Server) listOrganizations(ctx context.Context, w http.ResponseWriter, _ *http.Request) {
	// Start child span for this operation
	tracer := s.provider.Tracer()
	ctx, span := tracer.Start(ctx, "list_organizations")
	defer span.End()

	logger := s.provider.Logger()
	logger.Info("Listing organizations")

	// Call Midaz API - tracing context will be automatically propagated
	organizations, err := s.midazClient.Entity.Organizations.ListOrganizations(ctx, nil)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)

		logger.Error("Failed to list organizations", "error", err)
		http.Error(w, "Failed to list organizations", http.StatusInternalServerError)
		return
	}

	// Add attributes about the response
	span.SetAttributes(
		attribute.Int("organizations.count", len(organizations.Items)),
	)

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", observability.TraceID(ctx))

	response := map[string]any{
		"organizations": organizations.Items,
		"count":         len(organizations.Items),
		"trace_id":      observability.TraceID(ctx),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		span.RecordError(err)
		logger.Error("Failed to encode response", "error", err)
		return
	}

	span.SetStatus(codes.Ok, "Organizations listed successfully")
	logger.Info("Organizations listed successfully", "count", len(organizations.Items))
}

// createOrganization creates an organization with tracing
func (s *Server) createOrganization(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	tracer := s.provider.Tracer()
	ctx, span := tracer.Start(ctx, "create_organization")
	defer span.End()

	logger := s.provider.Logger()

	// Parse request body
	var reqBody struct {
		LegalName string         `json:"legal_name"`
		Metadata  map[string]any `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		span.SetStatus(codes.Error, "Invalid request body")
		span.RecordError(err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("organization.legal_name", reqBody.LegalName),
	)

	logger.Info("Creating organization", "legal_name", sanitizeLogInput(reqBody.LegalName))

	// Create organization through Midaz client
	orgInput := models.NewCreateOrganizationInput(reqBody.LegalName)
	if reqBody.Metadata != nil {
		orgInput = orgInput.WithMetadata(reqBody.Metadata)
	}

	organization, err := s.midazClient.Entity.Organizations.CreateOrganization(ctx, orgInput)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)

		logger.Error("Failed to create organization", "error", err)
		http.Error(w, "Failed to create organization", http.StatusInternalServerError)
		return
	}

	span.SetAttributes(
		attribute.String("organization.id", organization.ID),
	)

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", observability.TraceID(ctx))
	w.WriteHeader(http.StatusCreated)

	response := map[string]any{
		"organization": organization,
		"trace_id":     observability.TraceID(ctx),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		span.RecordError(err)
		logger.Error("Failed to encode response", "error", err)
		return
	}

	span.SetStatus(codes.Ok, "Organization created successfully")
	logger.Info("Organization created successfully",
		"org_id", organization.ID,
		"legal_name", organization.LegalName,
	)
}

// handleLedgers demonstrates nested operations with tracing
func (s *Server) handleLedgers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// This will inherit the trace context from the middleware
	tracer := s.provider.Tracer()
	ctx, span := tracer.Start(ctx, "handle_ledgers")
	defer span.End()

	logger := s.provider.Logger()
	logger.Info("Handling ledgers request")

	// Simulate complex business logic with multiple operations
	if err := s.performLedgerOperations(ctx); err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	span.SetStatus(codes.Ok, "Ledger operations completed")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Trace-ID", observability.TraceID(ctx))
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":   "success",
		"trace_id": observability.TraceID(ctx),
	}); err != nil {
		span.RecordError(err)
		logger.Error("Failed to encode response", "error", err)
	}
}

// performLedgerOperations demonstrates nested spans and operations
func (s *Server) performLedgerOperations(ctx context.Context) error {
	tracer := s.provider.Tracer()
	logger := s.provider.Logger()

	// Operation 1: List organizations
	ctx, span1 := tracer.Start(ctx, "ledger_ops_list_orgs")
	logger.Info("Step 1: Listing organizations for ledger operations")

	organizations, err := s.midazClient.Entity.Organizations.ListOrganizations(ctx, nil)
	if err != nil {
		span1.SetStatus(codes.Error, err.Error())
		span1.RecordError(err)
		span1.End()
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	span1.SetAttributes(attribute.Int("organizations.found", len(organizations.Items)))
	span1.SetStatus(codes.Ok, "Organizations listed")
	span1.End()

	if len(organizations.Items) == 0 {
		return errors.New("no organizations available")
	}

	orgID := organizations.Items[0].ID

	// Operation 2: List ledgers for the organization
	ctx, span2 := tracer.Start(ctx, "ledger_ops_list_ledgers")
	span2.SetAttributes(attribute.String("organization.id", orgID))
	logger.Info("Step 2: Listing ledgers", "org_id", orgID)

	ledgers, err := s.midazClient.Entity.Ledgers.ListLedgers(ctx, orgID, nil)
	if err != nil {
		span2.SetStatus(codes.Error, err.Error())
		span2.RecordError(err)
		span2.End()
		return fmt.Errorf("failed to list ledgers: %w", err)
	}

	span2.SetAttributes(attribute.Int("ledgers.found", len(ledgers.Items)))
	span2.SetStatus(codes.Ok, "Ledgers listed")
	span2.End()

	logger.Info("Ledger operations completed successfully",
		"org_id", orgID,
		"ledgers_count", len(ledgers.Items),
	)

	return nil
}

// handleHealth simple health check endpoint
func (*Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	w.Header().Set("Content-Type", "application/json")

	response := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	}

	// Add trace ID if available
	if traceID := observability.TraceID(ctx); traceID != "" {
		response["trace_id"] = traceID
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode health response: %v", err)
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = 200
	}
	return rw.ResponseWriter.Write(b)
}
