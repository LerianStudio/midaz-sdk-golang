// Package pagination provides utilities for working with paginated API responses
package pagination

import (
	"context"
	"strings"
	"time"
)

// Event represents a pagination operation event for monitoring purposes
type Event struct {
	Operation      string
	EntityType     string
	Limit          int
	Offset         int
	Page           int
	CursorUsed     bool
	TotalItems     int
	ProcessedItems int
	Duration       time.Duration
	HasNextPage    bool
	Error          error
}

// Observer defines the interface for observing pagination events
type Observer interface {
	// RecordEvent records information about a pagination operation
	RecordEvent(ctx context.Context, event *Event)
}

// NilObserver is a no-op observer implementation that discards all events.
// This is the default observer used when no custom observer is provided.
// Use NewObserver to create an instance for explicit no-op behavior.
type NilObserver struct{}

// NewObserver creates a new NilObserver that discards all events.
// To add actual observability, implement the Observer interface with
// custom metrics, logging, or tracing logic.
func NewObserver() Observer {
	return &NilObserver{}
}

// RecordEvent is a no-op implementation that discards the event.
// Implement a custom Observer to capture pagination metrics and events.
func (*NilObserver) RecordEvent(_ context.Context, _ *Event) {
	// No-op: events are discarded by design.
	// Implement Observer interface for custom observability.
}

// GetEntityTypeFromURL extracts the entity type from a URL using a simple heuristic.
//
// Limitations:
//   - Returns the first path segment after the version prefix (e.g., "v1", "v2")
//   - For nested resources like "/v1/organizations/123/ledgers/456", returns "organizations"
//     not "ledgers". This is by design as it identifies the root entity.
//   - Returns "unknown" if no version prefix is found or URL structure is unexpected
//   - Does not handle query parameters or fragments
//
// Example:
//
//	GetEntityTypeFromURL("/v1/accounts/123")           -> "accounts"
//	GetEntityTypeFromURL("/v1/organizations/1/ledgers") -> "organizations"
//	GetEntityTypeFromURL("/api/users")                 -> "unknown"
func GetEntityTypeFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if i > 0 && (part == "v1" || strings.HasPrefix(part, "v")) && i+1 < len(parts) {
			return parts[i+1]
		}
	}

	return "unknown"
}
