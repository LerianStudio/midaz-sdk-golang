// These flows stay on c.V1.Transactions for the same reason transaction.go
// does: they create through CreateJSON, and the nested send/source/distribute
// creation style exists only on /v1. See the note at the top of transaction.go,
// and examples/03-end-to-end for the /v2 creation path.
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

	"github.com/LerianStudio/midaz-sdk-golang/v6"
	midazmodels "github.com/LerianStudio/midaz-sdk-golang/v6/models"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/concurrent"
	pkgerrors "github.com/LerianStudio/midaz-sdk-golang/v6/pkg/errors"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/observability"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/performance"
	"github.com/LerianStudio/midaz-sdk-golang/v6/pkg/validation"
	"github.com/google/uuid"
)

// ExecuteConcurrentTransactions performs concurrent transactions between accounts to test TPS
//
// Parameters:
//   - ctx: The context for the operation, which can be used for cancellation
//   - midazClient: The initialized Midaz SDK client
//   - orgID: The ID of the organization
//   - ledgerID: The ID of the ledger
//   - customerAccount: The customer account model
//   - merchantAccount: The merchant account model
//   - c2mCount: Number of customer-to-merchant transactions to execute (0 → default).
//   - m2cCount: Number of merchant-to-customer transactions to execute (0 → default).
//
// Returns:
//   - error: Any error encountered during the operation
func ExecuteConcurrentTransactions(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, c2mCount, m2cCount int) error {
	// Create a span for observability
	ctx, span := observability.StartSpan(ctx, "ExecuteConcurrentTransactions")
	defer span.End()

	if c2mCount <= 0 {
		c2mCount = defaultConcurrentCustomerToMerchantTxs
	}

	if m2cCount <= 0 {
		m2cCount = defaultConcurrentMerchantToCustomerTxs
	}

	fmt.Println("\n Executing concurrent transactions for TPS testing...")
	if err := validateConcurrentTransactionAccounts(ctx, customerAccount, merchantAccount); err != nil {
		return err
	}

	addConcurrentTransactionAttributes(ctx, orgID, ledgerID, customerAccount, merchantAccount, c2mCount, m2cCount)

	if err := runConcurrentTransactionBatch(ctx, "customer to merchant", "CustomerToMerchantTransactions", c2mCount, "c2m", "c2m_transactions_failed", func(batchCtx context.Context) error {
		return ExecuteCustomerToMerchantConcurrent(batchCtx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, c2mCount)
	}); err != nil {
		return fmt.Errorf("failed to execute concurrent transactions: %w", err)
	}

	if err := runConcurrentTransactionBatch(ctx, "merchant to customer", "MerchantToCustomerTransactions", m2cCount, "m2c", "m2c_transactions_failed", func(batchCtx context.Context) error {
		return ExecuteMerchantToCustomerConcurrent(batchCtx, midazClient, orgID, ledgerID, customerAccount, merchantAccount, m2cCount)
	}); err != nil {
		return fmt.Errorf("failed to execute concurrent transactions: %w", err)
	}

	return nil
}

func validateConcurrentTransactionAccounts(ctx context.Context, customerAccount, merchantAccount *midazmodels.Account) error {
	if customerAccount == nil || merchantAccount == nil {
		err := errors.New("customer and merchant accounts are required")
		observability.RecordError(ctx, err, "missing_accounts")
		return err
	}

	if !validation.IsValidUUID(customerAccount.ID) || !validation.IsValidUUID(merchantAccount.ID) {
		err := errors.New("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")

		return err
	}

	return nil
}

func addConcurrentTransactionAttributes(ctx context.Context, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, c2mCount, m2cCount int) {
	observability.AddAttribute(ctx, "organization_id", orgID)
	observability.AddAttribute(ctx, "ledger_id", ledgerID)
	observability.AddAttribute(ctx, "customer_account_id", customerAccount.ID)
	observability.AddAttribute(ctx, "merchant_account_id", merchantAccount.ID)
	observability.AddAttribute(ctx, "c2m_tx_count", c2mCount)
	observability.AddAttribute(ctx, "m2c_tx_count", m2cCount)
}

func runConcurrentTransactionBatch(ctx context.Context, label, spanName string, count int, metricPrefix, errorEvent string, execute func(context.Context) error) error {
	fmt.Printf("Running %d concurrent transactions from %s...\n", count, label)

	startTime := time.Now()
	batchCtx, batchSpan := observability.StartSpan(ctx, spanName)
	defer batchSpan.End()

	if err := execute(batchCtx); err != nil {
		observability.RecordError(batchCtx, err, errorEvent)
		return err
	}

	duration := time.Since(startTime)
	tps := float64(count) / duration.Seconds()

	observability.RecordSpanMetric(batchCtx, metricPrefix+"_transaction_duration_seconds", duration.Seconds())
	observability.RecordSpanMetric(batchCtx, metricPrefix+"_transactions_per_second", tps)

	fmt.Printf(" Completed %d %s transactions in %.2f seconds (%.2f TPS)\n\n", count, label, duration.Seconds(), tps)

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
func ExecuteCustomerToMerchantConcurrent(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, count int) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteCustomerToMerchantConcurrent")
	defer span.End()
	if customerAccount == nil || merchantAccount == nil {
		return errors.New("customer and merchant accounts are required")
	}

	// Transaction legs address accounts by alias; the ledger does not resolve
	// account IDs there.
	if midazmodels.GetAccountAlias(*customerAccount) == "" || midazmodels.GetAccountAlias(*merchantAccount) == "" {
		return errors.New("customer and merchant accounts must have aliases: transaction legs address accounts by alias")
	}

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
		BatchSize: 50,
	}
	performance.ApplyGlobalPerformanceOptions(perfOptions)
}

func createC2MTransactionProcessor(midazClient *midaz.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, rateLimiter *concurrent.RateLimiter) func(context.Context, int) (string, error) {
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
		tx, err := midazClient.V1.Transactions.CreateJSON(txCtx, orgID, ledgerID, transferInput)
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
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
			"index":  index + 1,
		},
		Send: &midazmodels.SendInput{
			Asset: "USD",
			Value: 0.01,
			Source: &midazmodels.SourceInput{
				From: []midazmodels.FromToInput{
					{
						AccountAlias: midazmodels.GetAccountAlias(*customerAccount),
						Amount:       midazmodels.AmountInput{Asset: "USD", Value: 0.01},
					},
				},
			},
			Distribute: &midazmodels.DistributeInput{
				To: []midazmodels.FromToInput{
					{
						AccountAlias: midazmodels.GetAccountAlias(*merchantAccount),
						Amount:       midazmodels.AmountInput{Asset: "USD", Value: 0.01},
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
func ExecuteMerchantToCustomerConcurrent(ctx context.Context, midazClient *midaz.Client, orgID, ledgerID string, customerAccount, merchantAccount *midazmodels.Account, count int) error {
	ctx, span := observability.StartSpan(ctx, "ExecuteMerchantToCustomerConcurrent")
	defer span.End()
	if customerAccount == nil || merchantAccount == nil {
		return errors.New("customer and merchant accounts are required")
	}

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
	if merchantAccount == nil || customerAccount == nil {
		return nil, errors.New("merchant and customer accounts are required")
	}
	if !validation.IsValidUUID(merchantAccount.ID) || !validation.IsValidUUID(customerAccount.ID) {
		err := errors.New("invalid account IDs")
		observability.RecordError(ctx, err, "invalid_account_ids")
		return nil, err
	}

	// Transaction legs address accounts by alias; the ledger does not resolve
	// account IDs there.
	if midazmodels.GetAccountAlias(*merchantAccount) == "" || midazmodels.GetAccountAlias(*customerAccount) == "" {
		err := errors.New("merchant and customer accounts must have aliases: transaction legs address accounts by alias")
		observability.RecordError(ctx, err, "missing_account_aliases")
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
		Metadata: map[string]any{
			"source": "go-sdk-example",
			"type":   "transfer",
			"index":  index + 1,
		},
		Send: &midazmodels.SendInput{
			Asset: "USD",
			Value: 0.01,
			Source: &midazmodels.SourceInput{
				From: []midazmodels.FromToInput{
					{
						AccountAlias: midazmodels.GetAccountAlias(*merchantAccount),
						Amount:       midazmodels.AmountInput{Asset: "USD", Value: 0.01},
					},
				},
			},
			Distribute: &midazmodels.DistributeInput{
				To: []midazmodels.FromToInput{
					{
						AccountAlias: midazmodels.GetAccountAlias(*customerAccount),
						Amount:       midazmodels.AmountInput{Asset: "USD", Value: 0.01},
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
	}
	performance.ApplyBatchingOptions(perfOptions)
}

func createM2CBatchProcessor(midazClient *midaz.Client, orgID, ledgerID string) func(context.Context, []*midazmodels.CreateTransactionInput) ([]*midazmodels.Transaction, error) {
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

func createM2CSingleTransactionProcessor(midazClient *midaz.Client, orgID, ledgerID string, results *[]*midazmodels.Transaction, resultsMutex *sync.Mutex) func(context.Context, *midazmodels.CreateTransactionInput) error {
	return func(ctx context.Context, input *midazmodels.CreateTransactionInput) error {
		txCtx, txSpan := observability.StartSpan(ctx, "ProcessSingleTransaction")
		defer txSpan.End()

		index := extractTransactionIndex(txCtx, input)
		txStartTime := time.Now()

		tx, err := midazClient.V1.Transactions.CreateJSON(txCtx, orgID, ledgerID, input)
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
