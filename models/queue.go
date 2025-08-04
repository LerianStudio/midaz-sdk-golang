package models

import (
	"encoding/json"

	"github.com/LerianStudio/midaz/v3/pkg/mmodel"
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
	q.QueueData = append(q.QueueData, QueueData{
		ID:    id,
		Value: value,
	})

	return q
}

// FromMmodelQueue converts an mmodel Queue to an SDK Queue.
// This function is used internally to convert between backend and SDK models.
//
// Parameters:
//   - queue: The mmodel.Queue to convert
//
// Returns:
//   - A models.Queue instance with the same values
func FromMmodelQueue(queue mmodel.Queue) Queue {
	result := Queue{
		OrganizationID: queue.OrganizationID,
		LedgerID:       queue.LedgerID,
		AuditID:        queue.AuditID,
		AccountID:      queue.AccountID,
		QueueData:      make([]QueueData, 0, len(queue.QueueData)),
	}

	for _, data := range queue.QueueData {
		result.QueueData = append(result.QueueData, QueueData{
			ID:    data.ID,
			Value: data.Value,
		})
	}

	return result
}

// ToMmodelQueue converts an SDK Queue to an mmodel Queue.
// This method is used internally to convert between SDK and backend models.
//
// Returns:
//   - An mmodel.Queue instance with the same values
func (q *Queue) ToMmodelQueue() mmodel.Queue {
	if q == nil {
		return mmodel.Queue{}
	}

	result := mmodel.Queue{
		OrganizationID: q.OrganizationID,
		LedgerID:       q.LedgerID,
		AuditID:        q.AuditID,
		AccountID:      q.AccountID,
		QueueData:      make([]mmodel.QueueData, 0, len(q.QueueData)),
	}

	for _, data := range q.QueueData {
		result.QueueData = append(result.QueueData, mmodel.QueueData{
			ID:    data.ID,
			Value: data.Value,
		})
	}

	return result
}
