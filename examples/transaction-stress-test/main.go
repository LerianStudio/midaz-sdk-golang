// Package main provides a stress test for transactions using existing org/ledger/accounts.
//
// This example allows you to test transaction throughput with a fixed TPS target.
// Configure via environment variables:
//
//	ORG_ID=<organization-id>
//	LEDGER_ID=<ledger-id>
//	FROM_ACCOUNT_ID=<source-account-id>
//	TO_ACCOUNT_ID=<destination-account-id>
//	TARGET_TPS=100             # Target transactions per second (default: 100)
//	TX_DURATION=60             # Duration in seconds to run the test (default: 60)
//	TX_WORKERS=100             # Number of parallel workers (default: 100)
//	TX_AMOUNT=1                # Amount per transaction in cents (default: 1)
//	TX_ASSET=USD               # Asset code (default: USD)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	client "github.com/LerianStudio/midaz-sdk-golang/v2"
	"github.com/LerianStudio/midaz-sdk-golang/v2/models"
	auth "github.com/LerianStudio/midaz-sdk-golang/v2/pkg/access-manager"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/concurrent"
	"github.com/LerianStudio/midaz-sdk-golang/v2/pkg/config"
)

// StressTestConfig holds the configuration for the stress test
type StressTestConfig struct {
	OrgID         string
	LedgerID      string
	FromAccountID string
	ToAccountID   string
	TargetTPS     int
	Duration      int // seconds
	TxWorkers     int
	TxAmount      string
	TxAsset       string
}

// StressTestMetrics holds the metrics for the stress test
type StressTestMetrics struct {
	SuccessCount   int64
	ErrorCount     int64
	TotalLatencyMs int64
	MinLatencyMs   int64
	MaxLatencyMs   int64
	mu             sync.Mutex
}

func (m *StressTestMetrics) RecordSuccess(latencyMs int64) {
	atomic.AddInt64(&m.SuccessCount, 1)
	atomic.AddInt64(&m.TotalLatencyMs, latencyMs)

	m.mu.Lock()
	if m.MinLatencyMs == 0 || latencyMs < m.MinLatencyMs {
		m.MinLatencyMs = latencyMs
	}
	if latencyMs > m.MaxLatencyMs {
		m.MaxLatencyMs = latencyMs
	}
	m.mu.Unlock()
}

func (m *StressTestMetrics) RecordError() {
	atomic.AddInt64(&m.ErrorCount, 1)
}

func (m *StressTestMetrics) AvgLatencyMs() float64 {
	success := atomic.LoadInt64(&m.SuccessCount)
	if success == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&m.TotalLatencyMs)) / float64(success)
}

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Print configuration
	printConfig(cfg)

	// Create SDK client
	midazClient, err := createClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// Run stress test
	if err := runStressTest(context.Background(), midazClient, cfg); err != nil {
		log.Fatalf("Stress test failed: %v", err)
	}
}

func loadConfig() (*StressTestConfig, error) {
	cfg := &StressTestConfig{
		OrgID:         os.Getenv("ORG_ID"),
		LedgerID:      os.Getenv("LEDGER_ID"),
		FromAccountID: os.Getenv("FROM_ACCOUNT_ID"),
		ToAccountID:   os.Getenv("TO_ACCOUNT_ID"),
		TxAmount:      getEnvOrDefault("TX_AMOUNT", "1"),
		TxAsset:       getEnvOrDefault("TX_ASSET", "USD"),
	}

	// Parse numeric values
	var err error
	cfg.TargetTPS, err = strconv.Atoi(getEnvOrDefault("TARGET_TPS", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid TARGET_TPS: %w", err)
	}

	cfg.Duration, err = strconv.Atoi(getEnvOrDefault("TX_DURATION", "60"))
	if err != nil {
		return nil, fmt.Errorf("invalid TX_DURATION: %w", err)
	}

	cfg.TxWorkers, err = strconv.Atoi(getEnvOrDefault("TX_WORKERS", "100"))
	if err != nil {
		return nil, fmt.Errorf("invalid TX_WORKERS: %w", err)
	}

	// Ensure workers >= target TPS for best performance
	if cfg.TxWorkers < cfg.TargetTPS {
		cfg.TxWorkers = cfg.TargetTPS
	}

	// Validate required fields
	if cfg.OrgID == "" {
		return nil, fmt.Errorf("ORG_ID is required")
	}
	if cfg.LedgerID == "" {
		return nil, fmt.Errorf("LEDGER_ID is required")
	}
	if cfg.FromAccountID == "" {
		return nil, fmt.Errorf("FROM_ACCOUNT_ID is required")
	}
	if cfg.ToAccountID == "" {
		return nil, fmt.Errorf("TO_ACCOUNT_ID is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func printConfig(cfg *StressTestConfig) {
	fmt.Println("\n=== Transaction Stress Test ===")
	fmt.Println("================================")
	fmt.Printf("Organization ID:  %s\n", cfg.OrgID)
	fmt.Printf("Ledger ID:        %s\n", cfg.LedgerID)
	fmt.Printf("From Account:     %s\n", cfg.FromAccountID)
	fmt.Printf("To Account:       %s\n", cfg.ToAccountID)
	fmt.Printf("Target TPS:       %d transactions/second\n", cfg.TargetTPS)
	fmt.Printf("Duration:         %d seconds\n", cfg.Duration)
	fmt.Printf("Expected Total:   %d transactions\n", cfg.TargetTPS*cfg.Duration)
	fmt.Printf("Workers:          %d\n", cfg.TxWorkers)
	fmt.Printf("Amount per TX:    %s %s\n", cfg.TxAmount, cfg.TxAsset)
	fmt.Println("================================")
	fmt.Println()
}

func createClient() (*client.Client, error) {
	pluginAuth := auth.AccessManager{
		Enabled:      os.Getenv("PLUGIN_AUTH_ENABLED") == "true",
		Address:      os.Getenv("PLUGIN_AUTH_ADDRESS"),
		ClientID:     os.Getenv("MIDAZ_CLIENT_ID"),
		ClientSecret: os.Getenv("MIDAZ_CLIENT_SECRET"),
	}

	if pluginAuth.Enabled {
		fmt.Printf("Authentication enabled: %s\n", pluginAuth.Address)
	} else {
		fmt.Println("Authentication disabled - calling API directly")
	}

	cfg, err := config.NewConfig(
		config.FromEnvironment(),
		config.WithAccessManager(pluginAuth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	c, err := client.New(
		client.WithConfig(cfg),
		client.UseAllAPIs(),
	)
	if err != nil {
		return nil, err
	}

	if pluginAuth.Enabled {
		fmt.Println("Token obtained successfully")
		fmt.Println("Note: Token is cached and reused for all requests")
	}

	return c, nil
}

func runStressTest(ctx context.Context, c *client.Client, cfg *StressTestConfig) error {
	totalTx := cfg.TargetTPS * cfg.Duration
	fmt.Printf("Starting stress test: %d TPS for %d seconds (%d total transactions)...\n\n",
		cfg.TargetTPS, cfg.Duration, totalTx)

	// Create rate limiter for target TPS
	// Burst size = TPS to allow smooth operation
	rateLimiter := concurrent.NewRateLimiter(cfg.TargetTPS, cfg.TargetTPS)
	defer rateLimiter.Stop()

	// Create metrics tracker
	metrics := &StressTestMetrics{}

	// Create context with exact duration timeout
	// Test stops exactly at the configured duration
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Duration)*time.Second)
	defer cancel()

	// Create indexes for all transactions
	indexes := make([]int, totalTx)
	for i := range indexes {
		indexes[i] = i
	}

	startTime := time.Now()

	// Progress reporter
	done := make(chan struct{})
	go reportProgress(metrics, cfg.TargetTPS, startTime, done)

	// Run transactions with rate limiting
	results := concurrent.WorkerPool(
		ctx,
		indexes,
		func(ctx context.Context, index int) (*models.Transaction, error) {
			// Wait for rate limiter before executing
			if err := rateLimiter.Wait(ctx); err != nil {
				metrics.RecordError()
				return nil, err
			}

			txStart := time.Now()
			tx, err := executeTransaction(ctx, c, cfg, index)
			latencyMs := time.Since(txStart).Milliseconds()

			if err != nil {
				metrics.RecordError()
				return nil, err
			}

			metrics.RecordSuccess(latencyMs)
			return tx, nil
		},
		concurrent.WithWorkers(cfg.TxWorkers),
		concurrent.WithBufferSize(totalTx),
	)

	elapsed := time.Since(startTime)
	close(done)

	// Print results
	printResults(results, metrics, totalTx, elapsed, cfg.TargetTPS)

	return nil
}

func reportProgress(metrics *StressTestMetrics, targetTPS int, startTime time.Time, done <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastSuccess := int64(0)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			elapsed := time.Since(startTime).Seconds()
			currentSuccess := atomic.LoadInt64(&metrics.SuccessCount)
			currentErrors := atomic.LoadInt64(&metrics.ErrorCount)

			// Calculate TPS for last second
			instantTPS := currentSuccess - lastSuccess
			lastSuccess = currentSuccess

			// Calculate overall TPS
			overallTPS := float64(currentSuccess) / elapsed

			fmt.Printf("\r[%5.0fs] Success: %6d | Errors: %4d | Instant TPS: %4d | Avg TPS: %6.1f | Target: %d    ",
				elapsed, currentSuccess, currentErrors, instantTPS, overallTPS, targetTPS)
		}
	}
}

func executeTransaction(ctx context.Context, c *client.Client, cfg *StressTestConfig, index int) (*models.Transaction, error) {
	idempotencyKey := uuid.New().String()

	input := &models.CreateTransactionInput{
		Description:    fmt.Sprintf("Stress test transaction #%d", index+1),
		Pending:        false,
		Amount:         cfg.TxAmount,
		AssetCode:      cfg.TxAsset,
		IdempotencyKey: idempotencyKey,
		Metadata: map[string]any{
			"stress_test":     true,
			"transaction_idx": index,
			"timestamp":       time.Now().Format(time.RFC3339),
		},
		Send: &models.SendInput{
			Asset: cfg.TxAsset,
			Value: cfg.TxAmount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{
						Account: cfg.FromAccountID,
						Amount: models.AmountInput{
							Asset: cfg.TxAsset,
							Value: cfg.TxAmount,
						},
					},
				},
			},
			Distribute: &models.DistributeInput{
				To: []models.FromToInput{
					{
						Account: cfg.ToAccountID,
						Amount: models.AmountInput{
							Asset: cfg.TxAsset,
							Value: cfg.TxAmount,
						},
					},
				},
			},
		},
	}

	return c.Entity.Transactions.CreateTransaction(ctx, cfg.OrgID, cfg.LedgerID, input)
}

func printResults(results []concurrent.Result[int, *models.Transaction], metrics *StressTestMetrics, total int, elapsed time.Duration, targetTPS int) {
	success := atomic.LoadInt64(&metrics.SuccessCount)
	errors := atomic.LoadInt64(&metrics.ErrorCount)
	actualTPS := float64(success) / elapsed.Seconds()

	fmt.Println("\n\n=== Stress Test Results ===")
	fmt.Println("===========================")
	fmt.Printf("Target TPS:         %d\n", targetTPS)
	fmt.Printf("Actual TPS:         %.2f\n", actualTPS)
	fmt.Printf("TPS Achievement:    %.1f%%\n", (actualTPS/float64(targetTPS))*100)
	fmt.Println("---------------------------")
	fmt.Printf("Total Planned:      %d\n", total)
	fmt.Printf("Successful:         %d (%.1f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("Failed:             %d (%.1f%%)\n", errors, float64(errors)/float64(total)*100)
	fmt.Println("---------------------------")
	fmt.Printf("Duration:           %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf("Avg Latency:        %.2f ms\n", metrics.AvgLatencyMs())
	fmt.Printf("Min Latency:        %d ms\n", metrics.MinLatencyMs)
	fmt.Printf("Max Latency:        %d ms\n", metrics.MaxLatencyMs)
	fmt.Println("===========================")

	// Group errors by type and count them
	errorTypes := make(map[string]int)
	var allErrors []string

	for _, result := range results {
		if result.Error != nil {
			errMsg := result.Error.Error()
			errorTypes[errMsg]++
			allErrors = append(allErrors, fmt.Sprintf("TX #%d: %v", result.Item+1, result.Error))
		}
	}

	// Print error summary to console
	if len(errorTypes) > 0 {
		fmt.Println("\n=== Error Summary ===")
		for errType, count := range errorTypes {
			// Truncate long error messages for console
			displayErr := errType
			if len(displayErr) > 100 {
				displayErr = displayErr[:100] + "..."
			}
			fmt.Printf("  [%d occurrences] %s\n", count, displayErr)
		}

		// Write all errors to file
		errorFile := fmt.Sprintf("errors_%s.log", time.Now().Format("20060102_150405"))
		if err := writeErrorsToFile(errorFile, errorTypes, allErrors, targetTPS, actualTPS, success, errors, elapsed); err != nil {
			fmt.Printf("\nWarning: Could not write errors to file: %v\n", err)
		} else {
			fmt.Printf("\nAll errors written to: %s\n", errorFile)
		}
	}
}

func writeErrorsToFile(filename string, errorTypes map[string]int, allErrors []string, targetTPS int, actualTPS float64, success, errors int64, elapsed time.Duration) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	file.WriteString("=== Transaction Stress Test Error Report ===\n")
	file.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339)))
	file.WriteString(fmt.Sprintf("Target TPS: %d\n", targetTPS))
	file.WriteString(fmt.Sprintf("Actual TPS: %.2f\n", actualTPS))
	file.WriteString(fmt.Sprintf("Duration: %v\n", elapsed.Round(time.Millisecond)))
	file.WriteString(fmt.Sprintf("Successful: %d\n", success))
	file.WriteString(fmt.Sprintf("Failed: %d\n", errors))
	file.WriteString("\n")

	// Write error summary
	file.WriteString("=== Error Summary by Type ===\n")
	for errType, count := range errorTypes {
		file.WriteString(fmt.Sprintf("[%d occurrences] %s\n", count, errType))
	}
	file.WriteString("\n")

	// Write all errors
	file.WriteString("=== All Errors ===\n")
	for _, errLine := range allErrors {
		file.WriteString(errLine + "\n")
	}

	return nil
}
