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

// defaultObserver is the default implementation of Observer
type defaultObserver struct {
	// Optional fields for future extension (e.g., metrics client)
}

// NewObserver creates a new pagination observer
func NewObserver() Observer {
	return &defaultObserver{}
}

// RecordEvent records information about a pagination operation
func (o *defaultObserver) RecordEvent(ctx context.Context, event *Event) {
	// This is a stub implementation
	// In a real implementation, this would record metrics, structured logs, etc.

	// Example of what could be implemented:
	// 1. Record timing metrics for pagination operations
	// 2. Count pagination requests by entity type
	// 3. Track cursor vs. offset pagination usage
	// 4. Log slow pagination operations
	// 5. Record error rates for pagination operations
}

// GetEntityTypeFromURL extracts the entity type from a URL
func GetEntityTypeFromURL(url string) string {
	// Simple heuristic to extract entity type from URL
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if i > 0 && (part == "v1" || strings.HasPrefix(part, "v")) && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return "unknown"
}
