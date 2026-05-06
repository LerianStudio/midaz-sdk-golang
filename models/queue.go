package models

import (
	"encoding/json"

	"github.com/google/uuid"
)

// Queue represents a transaction queue in the Midaz system.
// Queues are used to temporarily store transaction data before processing,
// allowing for batched or asynchronous transaction handling.
type Queue struct {
	// OrganizationID is the unique identifier of the organization that owns this queue
	OrganizationID uuid.UUID `json:"organizationId"`

	// LedgerID is the identifier of the ledger associated with this queue
	LedgerID uuid.UUID `json:"ledgerId"`

	// AuditID is the identifier for audit tracking purposes
	AuditID uuid.UUID `json:"auditId"`

	// AccountID is the identifier of the account associated with this queue
	AccountID uuid.UUID `json:"accountId"`

	// QueueData contains the collection of data items in this queue
	QueueData []QueueData `json:"queueData"`
}

// QueueData represents a single data item in a queue.
// Each item has a unique identifier and contains arbitrary JSON data.
type QueueData struct {
	// ID is the unique identifier for this queue data item
	ID uuid.UUID `json:"id"`

	// Value contains the actual data as raw JSON
	Value json.RawMessage `json:"value"`
}

// AddQueueData adds a new data item to the queue.
// This method appends a new data item with the provided ID and value.
//
// Parameters:
//   - id: Unique identifier for the new queue data item
//   - value: The data to store, as raw JSON
//
// Returns:
//   - A pointer to the modified Queue for method chaining
func (q *Queue) AddQueueData(id uuid.UUID, value json.RawMessage) *Queue {
	if q == nil {
		return nil
	}

	q.QueueData = append(q.QueueData, QueueData{
		ID:    id,
		Value: append(json.RawMessage(nil), value...),
	})

	return q
}

// NOTE: FromMmodelQueue and ToMmodelQueue were retired in Track 7E. Queue
// is fully SDK-owned with field shapes and JSON tags identical to the wire
// format; the conversion adapters were no-ops.
