package workflows

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	midazmodels "github.com/LerianStudio/midaz-sdk-golang/v2/models"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/validation"
	"github.com/google/uuid"
)

// Initialize defaults for transaction volume testing
func init() {
	// Set default values if not already set
	if concurrentCustomerToMerchantTxs == 0 {
		concurrentCustomerToMerchantTxs = 20 // Default number of concurrent C2M transactions to run
	}

	if concurrentMerchantToCustomerTxs == 0 {
		concurrentMerchantToCustomerTxs = 20 // Default number of concurrent M2C transactions to run
	}
}

// ExecuteConcurrentTransactions performs concurrent transactions between accounts to test TPS
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteConcurrentTransactions(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account) error {
	// Create a span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteConcurrentTransactions")
	defer span.End()

	fmt.Println("\n Executing concurrent transactions for TPS testing...")

	// Record transaction parameters in observability
	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)
	observability.AddAttribute(ctx, "customer_account_id", customerAccount.ID)
	observability.AddAttribute(ctx, "merchant_account_id", merchantAccount.ID)
	observability.AddAttribute(ctx, "c2m_tx_count", concurrentCustomerToMerchantTxs)
	observability.AddAttribute(ctx, "m2c_tx_count", concurrentMerchantToCustomerTxs)

	// Validate account IDs
	if !validation.IsValidUUID(customerAccount.ID) || !validation.IsValidUUID(merchantAccount.ID) {
		err := errors.New("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")

		return err
	}

	// Execute concurrent transactions from customer to merchant
	fmt.Printf("Running %d concurrent transactions from customer to merchant...\n", concurrentCustomerToMerchantTxs)

	startTimeC2M := time.Now()

	c2mCtx, c2mSpan := observability.StartSpan(ctx, "CustomerToMerchantTransactions")
	if err := ExecuteCustomerToMerchantConcurrent(c2mCtx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, concurrentCustomerToMerchantTxs); err != nil {
		c2mSpan.End()
		observability.RecordError(ctx, err, "c2m_transactions_failed")

		return fmt.Errorf("failed to execute concurrent transactions: %w", err)
	}

	c2mSpan.End()

	customerToMerchantDuration := time.Since(startTimeC2M)
	customerToMerchantTPS := float64(concurrentCustomerToMerchantTxs) / customerToMerchantDuration.Seconds()

	// Record metrics
	observability.RecordSpanMetric(ctx, "c2m_transaction_duration_seconds", customerToMerchantDuration.Seconds())
	observability.RecordSpanMetric(ctx, "c2m_transactions_per_second", customerToMerchantTPS)

	fmt.Printf(" Completed %d customer to merchant transactions in %.2f seconds (%.2f TPS)\n\n",
		concurrentCustomerToMerchantTxs, customerToMerchantDuration.Seconds(), customerToMerchantTPS)

	// Execute concurrent transactions from merchant to customer
	fmt.Printf("Running %d concurrent transactions from merchant to customer...\n", concurrentMerchantToCustomerTxs)

	startTimeM2C := time.Now()

	m2cCtx, m2cSpan := observability.StartSpan(ctx, "MerchantToCustomerTransactions")
	if err := ExecuteMerchantToCustomerConcurrent(m2cCtx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, concurrentMerchantToCustomerTxs); err != nil {
		m2cSpan.End()
		observability.RecordError(ctx, err, "m2c_transactions_failed")

		return fmt.Errorf("failed to execute concurrent transactions: %w", err)
	}

	m2cSpan.End()

	merchantToCustomerDuration := time.Since(startTimeM2C)
	merchantToCustomerTPS := float64(concurrentMerchantToCustomerTxs) / merchantToCustomerDuration.Seconds()

	// Record metrics
	observability.RecordSpanMetric(ctx, "m2c_transaction_duration_seconds", merchantToCustomerDuration.Seconds())
	observability.RecordSpanMetric(ctx, "m2c_transactions_per_second", merchantToCustomerTPS)

	fmt.Printf(" Completed %d merchant to customer transactions in %.2f seconds (%.2f TPS)\n\n",
		concurrentMerchantToCustomerTxs, merchantToCustomerDuration.Seconds(), merchantToCustomerTPS)

	return nil
}

// GenerateUniqueIdempotencyKey generates a truly unique idempotency key
// by combining multiple sources of uniqueness
//
// Parameters:
//   - prefix: A prefix to identify the source of the key (e.g., "c2m" for customer to merchant)
//   - index: An index to identify the specific transaction
//
// Returns:
//   - string: A unique idempotency key
func GenerateUniqueIdempotencyKey(prefix string, index int) string {
	// Create a hash of multiple unique components
	h := sha256.New()

	// Add a UUID
	uuidStr := uuid.New().String()
	_, _ = h.Write([]byte(uuidStr))

	// Add the current timestamp with nanosecond precision
	timestamp := time.Now().UnixNano()
	fmt.Fprintf(h, "%d", timestamp)

	// Add the prefix and index
	fmt.Fprintf(h, "%s-%d", prefix, index)

	// Add some random bytes
	randomBytes := make([]byte, 16)
	_, err := cryptorand.Read(randomBytes)
	if err != nil {
		log.Printf("Warning: Failed to generate cryptographically secure random bytes: %v", err)

		// Fallback to a more secure approach than math/rand.Read
		// Use current time and process-specific values to create entropy
		timeNow := time.Now()
		fallbackSource := []byte(fmt.Sprintf("%d-%d-%d-%d-%s-%d",
			timeNow.UnixNano(),
			os.Getpid(),
			os.Getppid(),
			timeNow.Year(),
			timeNow.Location().String(),
			index))

		// Hash the fallback source to get random bytes
		hasher := sha256.New()
		_, _ = hasher.Write(fallbackSource)
		copy(randomBytes, hasher.Sum(nil)[:16])

		log.Printf("Warning: Using fallback method for random bytes generation")
	}

	_, _ = h.Write(randomBytes)

	// Get the hash as a hex string
	hash := hex.EncodeToString(h.Sum(nil))

	// Return a combination of components for maximum uniqueness
	return fmt.Sprintf("%s-%s-%d-%d", prefix, hash[:16], index, timestamp)
}

// handleTransactionError processes errors from transaction creation, categorizing them appropriately
func handleTransactionError(ctx context.Context, err error, index int, operation string) error {
	if err == nil {
		return nil
	}

	// Get detailed error information using the errors package
	errDetails := pkgerrors.GetErrorDetails(err)

	// Record error in observability system
	observability.RecordError(ctx, err, fmt.Sprintf("%s_transaction_error", operation))
	observability.AddAttribute(ctx, "transaction_index", index)
	observability.AddAttribute(ctx, "error_code", errDetails.Code)
	observability.AddAttribute(ctx, "http_status", errDetails.HTTPStatus)

	// Check if this is a cancellation error (context deadline exceeded)
	if pkgerrors.IsCancellationError(err) {
		return fmt.Errorf("%s transaction #%d cancelled: %w", operation, index+1, err)
	}

	// Check if this is a rate limit error that wasn't resolved by retries
	if pkgerrors.IsRateLimitError(err) {
		return fmt.Errorf("%s transaction #%d hit rate limit after retries: %w", operation, index+1, err)
	}

	// Check for insufficient balance errors
	if pkgerrors.IsInsufficientBalanceError(err) {
		return fmt.Errorf("%s transaction #%d failed due to insufficient balance: %w", operation, index+1, err)
	}

	// Check for validation errors
	if pkgerrors.IsValidationError(err) {
		return fmt.Errorf("%s transaction #%d failed validation: %w", operation, index+1, err)
	}

	// Generic error case
	return fmt.Errorf("failed to execute %s transaction #%d: %w", operation, index+1, err)
}

// ExecuteCustomerToMerchantConcurrent executes concurrent transactions from customer to merchant
// using the SDK's concurrency helpers
func ExecuteCustomerToMerchantConcurrent(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, count int) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteCustomerToMerchantConcurrent")
	defer span.End()

	rateLimiter := concurrent.NewRateLimiter(20000, 20000)
	defer rateLimiter.Stop()

	indices := make([]int, count)
	for i := range indices {
		indices[i] = i
	}

	applyC2MPerformanceOptions()

	processTransaction := createC2MTransactionProcessor(midazClient, orgID, ledgerID, customerAccount, merchantAccount, rateLimiter)

	startTime := time.Now()
	results := concurrent.WorkerPool(ctx, indices, processTransaction,
		concurrent.WithWorkers(10),
		concurrent.WithBufferSize(count),
		concurrent.WithUnorderedResults(),
	)

	duration := time.Since(startTime)
	successCount, firstError := countC2MResults(results)

	recordC2MMetrics(ctx, duration, successCount, count)

	fmt.Printf("Successfully processed %d/%d concurrent customer to merchant transactions\n", successCount, count)

	return firstError
}

func applyC2MPerformanceOptions() {
	perfOptions := performance.Options{
		BatchSize:       50,
		UseJSONIterator: true,
	}
	performance.ApplyGlobalPerformanceOptions(perfOptions)
}

func createC2MTransactionProcessor(midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, rateLimiter *concurrent.RateLimiter) func(context.Context, int) (string, error) {
	return func(ctx context.Context, index int) (string, error) {
		txCtx, txSpan := observability.StartSpan(ctx, "ProcessCustomerToMerchantTransaction")
		defer txSpan.End()

		observability.AddAttribute(txCtx, "transaction_index", index)

		if err := rateLimiter.Wait(txCtx); err != nil {
			observability.RecordError(txCtx, err, "rate_limiter_wait_error")
			return "", err
		}

		idempotencyKey := GenerateUniqueIdempotencyKey("c2m", index)
		transferInput := buildC2MTransactionInput(index, customerAccount, merchantAccount, idempotencyKey)

		startTime := time.Now()
		tx, err := midazClient.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, transferInput)
		duration := time.Since(startTime)

		observability.RecordSpanMetric(txCtx, "transaction_duration_ms", float64(duration.Milliseconds()))

		if err != nil {
			return "", handleTransactionError(txCtx, err, index, "customer-to-merchant")
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)

		return tx.ID, nil
	}
}

func buildC2MTransactionInput(index int, customerAccount, merchantAccount *midazmodels.Account, idempotencyKey string) *midazmodels.CreateTransactionInput {
	return &midazmodels.CreateTransactionInput{
		ChartOfAccountsGroupName: "default_chart_group",
		Description:              fmt.Sprintf("Concurrent customer to merchant transfer #%d", index+1),
		Amount:                   "0.01",
		AssetCode:                "USD",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
			"index":  index + 1,
		},
		Send: &midazmodels.SendInput{
			Asset: "USD",
			Value: "0.01",
			Source: &midazmodels.SourceInput{
				From: []midazmodels.FromToInput{
					{
						Account: customerAccount.ID,
						Amount:  midazmodels.AmountInput{Asset: "USD", Value: "0.01"},
					},
				},
			},
			Distribute: &midazmodels.DistributeInput{
				To: []midazmodels.FromToInput{
					{
						Account: merchantAccount.ID,
						Amount:  midazmodels.AmountInput{Asset: "USD", Value: "0.01"},
					},
				},
			},
		},
		IdempotencyKey: idempotencyKey,
	}
}

func countC2MResults(results []concurrent.Result[int, string]) (int, error) {
	var successCount int
	var firstError error

	for _, result := range results {
		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
			}
		} else {
			successCount++
		}
	}

	return successCount, firstError
}

func recordC2MMetrics(ctx context.Context, duration time.Duration, successCount, count int) {
	observability.RecordSpanMetric(ctx, "c2m_batch_duration_seconds", duration.Seconds())
	observability.RecordSpanMetric(ctx, "c2m_batch_success_count", float64(successCount))
	observability.RecordSpanMetric(ctx, "c2m_batch_error_count", float64(count-successCount))

	if duration.Seconds() > 0 {
		observability.RecordSpanMetric(ctx, "c2m_batch_transactions_per_second", float64(successCount)/duration.Seconds())
	}
}

// ExecuteMerchantToCustomerConcurrent executes concurrent transactions from merchant to customer
// using the SDK's concurrency helpers
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - entity: The initialized Midaz SDK entity client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - count: The number of concurrent transactions to execute
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteMerchantToCustomerConcurrent(ctx context.Context, midazClient *client.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, count int) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteMerchantToCustomerConcurrent")
	defer span.End()

	transactionInputs, err := buildM2CTransactionInputs(ctx, merchantAccount, customerAccount, count)
	if err != nil {
		return err
	}

	applyM2CPerformanceOptions()

	batchSize := performance.GetOptimalBatchSize(count, 2000)
	observability.AddAttribute(ctx, "batch_size", batchSize)

	processTransactionBatch := createM2CBatchProcessor(midazClient, orgID, ledgerID)

	fmt.Println("Processing transactions in batches...")

	batchOpStartTime := time.Now()
	batchResults := concurrent.Batch(ctx, transactionInputs, batchSize, processTransactionBatch, concurrent.WithWorkers(2))
	batchOpDuration := time.Since(batchOpStartTime)

	observability.RecordSpanMetric(ctx, "m2c_batch_operation_duration_seconds", batchOpDuration.Seconds())

	successCount, firstError := countM2CResults(batchResults)
	recordM2CMetrics(ctx, batchOpDuration, successCount, count)

	fmt.Printf("Successfully processed %d/%d concurrent merchant to customer transactions\n", successCount, count)

	return firstError
}

func buildM2CTransactionInputs(ctx context.Context, merchantAccount, customerAccount *midazmodels.Account, count int) ([]*midazmodels.CreateTransactionInput, error) {
	if !validation.IsValidUUID(merchantAccount.ID) || !validation.IsValidUUID(customerAccount.ID) {
		err := errors.New("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")
		return nil, err
	}

	inputs := make([]*midazmodels.CreateTransactionInput, count)
	for i := 0; i < count; i++ {
		inputs[i] = buildM2CTransactionInput(i, merchantAccount, customerAccount, GenerateUniqueIdempotencyKey("m2c", i))
	}

	return inputs, nil
}

func buildM2CTransactionInput(index int, merchantAccount, customerAccount *midazmodels.Account, idempotencyKey string) *midazmodels.CreateTransactionInput {
	return &midazmodels.CreateTransactionInput{
		ChartOfAccountsGroupName: "default_chart_group",
		Description:              fmt.Sprintf("Concurrent merchant to customer transfer #%d", index+1),
		Amount:                   "0.01",
		AssetCode:                "USD",
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
			"index":  index + 1,
		},
		Send: &midazmodels.SendInput{
			Asset: "USD",
			Value: "0.01",
			Source: &midazmodels.SourceInput{
				From: []midazmodels.FromToInput{
					{
						Account: merchantAccount.ID,
						Amount:  midazmodels.AmountInput{Asset: "USD", Value: "0.01"},
					},
				},
			},
			Distribute: &midazmodels.DistributeInput{
				To: []midazmodels.FromToInput{
					{
						Account: customerAccount.ID,
						Amount:  midazmodels.AmountInput{Asset: "USD", Value: "0.01"},
					},
				},
			},
		},
		IdempotencyKey: idempotencyKey,
	}
}

func applyM2CPerformanceOptions() {
	perfOptions := performance.Options{
		BatchSize:           100,
		EnableHTTPPooling:   true,
		MaxIdleConnsPerHost: 10,
		UseJSONIterator:     true,
	}
	performance.ApplyBatchingOptions(perfOptions)
}

func createM2CBatchProcessor(midazClient *client.Client, orgID, ledgerID string) func(context.Context, []*midazmodels.CreateTransactionInput) ([]*midazmodels.Transaction, error) {
	return func(ctx context.Context, batch []*midazmodels.CreateTransactionInput) ([]*midazmodels.Transaction, error) {
		batchCtx, batchSpan := observability.StartSpan(ctx, "ProcessTransactionBatch")
		defer batchSpan.End()

		observability.AddAttribute(batchCtx, "batch_size", len(batch))

		results := make([]*midazmodels.Transaction, 0, len(batch))
		resultsMutex := &sync.Mutex{}
		batchStartTime := time.Now()

		err := concurrent.ForEach(batchCtx, batch,
			createM2CSingleTransactionProcessor(midazClient, orgID, ledgerID, &results, resultsMutex),
			concurrent.WithWorkers(3),
			concurrent.WithRateLimit(10),
		)

		batchDuration := time.Since(batchStartTime)
		observability.RecordSpanMetric(batchCtx, "batch_duration_seconds", batchDuration.Seconds())

		if batchDuration.Seconds() > 0 && err == nil {
			observability.RecordSpanMetric(batchCtx, "batch_transactions_per_second", float64(len(results))/batchDuration.Seconds())
		}

		if err != nil {
			observability.RecordError(batchCtx, err, "batch_processing_error")
			return nil, err
		}

		return results, nil
	}
}

func createM2CSingleTransactionProcessor(midazClient *client.Client, orgID, ledgerID string, results *[]*midazmodels.Transaction, resultsMutex *sync.Mutex) func(context.Context, *midazmodels.CreateTransactionInput) error {
	return func(ctx context.Context, input *midazmodels.CreateTransactionInput) error {
		txCtx, txSpan := observability.StartSpan(ctx, "ProcessSingleTransaction")
		defer txSpan.End()

		index := extractTransactionIndex(txCtx, input)
		txStartTime := time.Now()

		tx, err := midazClient.Entity.Transactions.CreateTransaction(txCtx, orgID, ledgerID, input)
		txDuration := time.Since(txStartTime)
		observability.RecordSpanMetric(txCtx, "transaction_duration_ms", float64(txDuration.Milliseconds()))

		if err != nil {
			return handleTransactionError(txCtx, err, index, "merchant-to-customer")
		}

		observability.AddAttribute(txCtx, "transaction_id", tx.ID)

		resultsMutex.Lock()
		*results = append(*results, tx)
		resultsMutex.Unlock()

		return nil
	}
}

func extractTransactionIndex(ctx context.Context, input *midazmodels.CreateTransactionInput) int {
	var index int
	if idx, ok := input.Metadata["index"]; ok {
		if idxInt, ok := idx.(int); ok {
			index = idxInt - 1
			observability.AddAttribute(ctx, "transaction_index", index)
		}
	}

	return index
}

func countM2CResults(batchResults []concurrent.Result[*midazmodels.CreateTransactionInput, *midazmodels.Transaction]) (int, error) {
	var successCount int
	var firstError error

	for _, result := range batchResults {
		if result.Error != nil {
			if firstError == nil {
				firstError = result.Error
			}
		} else {
			successCount++
		}
	}

	return successCount, firstError
}

func recordM2CMetrics(ctx context.Context, duration time.Duration, successCount, count int) {
	observability.RecordSpanMetric(ctx, "m2c_batch_success_count", float64(successCount))
	observability.RecordSpanMetric(ctx, "m2c_batch_error_count", float64(count-successCount))

	if duration.Seconds() > 0 {
		observability.RecordSpanMetric(ctx, "m2c_batch_transactions_per_second", float64(successCount)/duration.Seconds())
	}
}
