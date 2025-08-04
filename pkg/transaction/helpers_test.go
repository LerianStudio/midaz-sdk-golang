package transaction

import (
	"testing"

	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/stretchr/testify/assert"
)

// TestTransactionStatus tests the transaction status helper functions
func TestTransactionStatus(t *testing.T) {
	// Test IsTransactionSuccessful
	completedTx := &models.Transaction{
		Status: models.Status{Code: "COMPLETED"},
	}
	pendingTx := &models.Transaction{
		Status: models.Status{Code: "PENDING"},
	}
	failedTx := &models.Transaction{
		Status: models.Status{Code: "FAILED"},
	}

	assert.True(t, IsTransactionSuccessful(completedTx))
	assert.False(t, IsTransactionSuccessful(pendingTx))
	assert.False(t, IsTransactionSuccessful(failedTx))
	assert.False(t, IsTransactionSuccessful(nil))

	// Test GetTransactionStatus
	assert.Equal(t, "Completed", GetTransactionStatus(completedTx))
	assert.Equal(t, "Pending", GetTransactionStatus(pendingTx))
	assert.Equal(t, "Failed", GetTransactionStatus(failedTx))
	assert.Equal(t, "Unknown", GetTransactionStatus(nil))
}
