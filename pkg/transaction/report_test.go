package transaction

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReportEntitiesFields tests ReportEntities struct fields
func TestReportEntitiesFields(t *testing.T) {
	entities := &ReportEntities{
		Counts: ReportEntityCounts{
			Organizations: 1,
			Ledgers:       2,
			Assets:        5,
			Accounts:      100,
			Transactions:  500,
			Portfolios:    10,
			Segments:      20,
		},
		IDs: ReportEntityIDs{
			OrganizationIDs: []string{"org-1"},
			LedgerIDs:       []string{"ledger-1", "ledger-2"},
			AssetIDs:        []string{"usd", "eur", "btc", "eth", "gbp"},
			AccountIDs:      []string{"acc-1", "acc-2"},
			PortfolioIDs:    []string{"port-1"},
			SegmentIDs:      []string{"seg-1"},
			TransactionIDs:  []string{"tx-1", "tx-2", "tx-3"},
		},
	}

	assert.Equal(t, 1, entities.Counts.Organizations)
	assert.Equal(t, 2, entities.Counts.Ledgers)
	assert.Equal(t, 5, entities.Counts.Assets)
	assert.Equal(t, 100, entities.Counts.Accounts)
	assert.Equal(t, 500, entities.Counts.Transactions)
	assert.Equal(t, 10, entities.Counts.Portfolios)
	assert.Equal(t, 20, entities.Counts.Segments)

	assert.Len(t, entities.IDs.OrganizationIDs, 1)
	assert.Len(t, entities.IDs.LedgerIDs, 2)
	assert.Len(t, entities.IDs.AssetIDs, 5)
	assert.Len(t, entities.IDs.AccountIDs, 2)
}

// TestReportAPIStatsFields tests ReportAPIStats struct fields
func TestReportAPIStatsFields(t *testing.T) {
	stats := &ReportAPIStats{
		APICalls: 1000,
		Errors: map[string]int{
			"validation": 10,
			"network":    5,
		},
		Notes: map[string]string{
			"info": "Some important note",
		},
	}

	assert.Equal(t, 1000, stats.APICalls)
	assert.Equal(t, 10, stats.Errors["validation"])
	assert.Equal(t, 5, stats.Errors["network"])
	assert.Equal(t, "Some important note", stats.Notes["info"])
}

// TestReportDataSummaryFields tests ReportDataSummary struct fields
func TestReportDataSummaryFields(t *testing.T) {
	dataSummary := &ReportDataSummary{
		TransactionVolumeByAccount: map[string]int{
			"acc-1": 100,
			"acc-2": 200,
		},
		AccountDistributionByType: map[string]int{
			"checking": 50,
			"savings":  30,
		},
		AssetUsage: map[string]int{
			"USD": 500,
			"EUR": 200,
		},
		BalanceSummaries: map[string]map[string]any{
			"acc-1": {
				"balance":  1000.50,
				"currency": "USD",
			},
		},
	}

	assert.Equal(t, 100, dataSummary.TransactionVolumeByAccount["acc-1"])
	assert.Equal(t, 200, dataSummary.TransactionVolumeByAccount["acc-2"])
	assert.Equal(t, 50, dataSummary.AccountDistributionByType["checking"])
	assert.Equal(t, 500, dataSummary.AssetUsage["USD"])
	assert.Equal(t, 1000.50, dataSummary.BalanceSummaries["acc-1"]["balance"])
}

// TestGenerationReportFields tests GenerationReport struct fields
func TestGenerationReportFields(t *testing.T) {
	now := time.Now().UTC()
	results := []BatchResult{
		{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
		{Index: 1, TransactionID: "tx-2", Duration: 100 * time.Millisecond},
	}

	report := &GenerationReport{
		GeneratedAt: now,
		Summary: BatchSummary{
			TotalTransactions: 2,
			SuccessCount:      2,
		},
		Results: results,
		Notes:   "Test report",
		AdditionalInformation: map[string]any{
			"key": "value",
		},
		StepTimings: map[string]string{
			"step1": "100ms",
			"step2": "200ms",
		},
		Entities: &ReportEntities{
			Counts: ReportEntityCounts{
				Transactions: 2,
			},
		},
		APIStats: &ReportAPIStats{
			APICalls: 10,
		},
		DataSummary: &ReportDataSummary{
			AssetUsage: map[string]int{"USD": 100},
		},
	}

	assert.Equal(t, now, report.GeneratedAt)
	assert.Equal(t, 2, report.Summary.TotalTransactions)
	assert.Len(t, report.Results, 2)
	assert.Equal(t, "Test report", report.Notes)
	assert.Equal(t, "value", report.AdditionalInformation["key"])
	assert.Equal(t, "100ms", report.StepTimings["step1"])
	assert.Equal(t, 2, report.Entities.Counts.Transactions)
	assert.Equal(t, 10, report.APIStats.APICalls)
	assert.Equal(t, 100, report.DataSummary.AssetUsage["USD"])
}

// TestNewGenerationReport tests the NewGenerationReport constructor
func TestNewGenerationReport(t *testing.T) {
	t.Run("with results and notes", func(t *testing.T) {
		results := []BatchResult{
			{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
			{Index: 1, Error: errors.New("error"), Duration: 50 * time.Millisecond},
		}

		report := NewGenerationReport(results, "Test notes", map[string]any{"extra": "data"})

		require.NotNil(t, report)
		assert.False(t, report.GeneratedAt.IsZero())
		assert.Equal(t, 2, report.Summary.TotalTransactions)
		assert.Equal(t, 1, report.Summary.SuccessCount)
		assert.Equal(t, 1, report.Summary.ErrorCount)
		assert.Len(t, report.Results, 2)
		assert.Equal(t, "Test notes", report.Notes)
		assert.Equal(t, "data", report.AdditionalInformation["extra"])
	})

	t.Run("with empty results", func(t *testing.T) {
		report := NewGenerationReport([]BatchResult{}, "", nil)

		require.NotNil(t, report)
		assert.Equal(t, 0, report.Summary.TotalTransactions)
		assert.Empty(t, report.Results)
		assert.Empty(t, report.Notes)
		assert.Nil(t, report.AdditionalInformation)
	})

	t.Run("with nil additional information", func(t *testing.T) {
		results := []BatchResult{{Index: 0, TransactionID: "tx-1"}}
		report := NewGenerationReport(results, "notes", nil)

		require.NotNil(t, report)
		assert.Nil(t, report.AdditionalInformation)
	})
}

// TestGenerationReportToJSON tests the ToJSON method
func TestGenerationReportToJSON(t *testing.T) {
	t.Run("compact JSON", func(t *testing.T) {
		report := NewGenerationReport(
			[]BatchResult{{Index: 0, TransactionID: "tx-1"}},
			"notes",
			nil,
		)

		data, err := report.ToJSON(false)

		require.NoError(t, err)
		require.NotEmpty(t, data)
		assert.False(t, strings.Contains(string(data), "\n"))

		// Verify it's valid JSON
		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)
		assert.Equal(t, "notes", parsed["notes"])
	})

	t.Run("pretty JSON", func(t *testing.T) {
		report := NewGenerationReport(
			[]BatchResult{{Index: 0, TransactionID: "tx-1"}},
			"notes",
			nil,
		)

		data, err := report.ToJSON(true)

		require.NoError(t, err)
		require.NotEmpty(t, data)
		assert.True(t, strings.Contains(string(data), "\n"))
		assert.True(t, strings.Contains(string(data), "  ")) // Indentation
	})

	t.Run("full report serializes correctly", func(t *testing.T) {
		report := &GenerationReport{
			GeneratedAt: time.Now().UTC(),
			Summary: BatchSummary{
				TotalTransactions:     10,
				SuccessCount:          8,
				ErrorCount:            2,
				SuccessRate:           80.0,
				TransactionsPerSecond: 5.0,
			},
			Results: []BatchResult{
				{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
			},
			Notes: "Full report test",
			AdditionalInformation: map[string]any{
				"version": "1.0",
			},
			StepTimings: map[string]string{
				"setup": "50ms",
			},
			Entities: &ReportEntities{
				Counts: ReportEntityCounts{
					Organizations: 1,
					Ledgers:       2,
				},
				IDs: ReportEntityIDs{
					OrganizationIDs: []string{"org-1"},
				},
			},
			APIStats: &ReportAPIStats{
				APICalls: 100,
				Errors:   map[string]int{"timeout": 2},
			},
			DataSummary: &ReportDataSummary{
				AssetUsage: map[string]int{"USD": 50},
			},
		}

		data, err := report.ToJSON(true)
		require.NoError(t, err)

		var parsed GenerationReport
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, 10, parsed.Summary.TotalTransactions)
		assert.Equal(t, "Full report test", parsed.Notes)
		assert.Equal(t, 1, parsed.Entities.Counts.Organizations)
		assert.Equal(t, 100, parsed.APIStats.APICalls)
		assert.Equal(t, 50, parsed.DataSummary.AssetUsage["USD"])
	})
}

// TestGenerationReportSaveJSON tests the SaveJSON method
func TestGenerationReportSaveJSON(t *testing.T) {
	t.Run("saves file successfully", func(t *testing.T) {
		report := NewGenerationReport(
			[]BatchResult{{Index: 0, TransactionID: "tx-1"}},
			"test notes",
			nil,
		)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report.json")

		err := report.SaveJSON(filePath, false)
		require.NoError(t, err)

		// Verify file exists
		_, err = os.Stat(filePath)
		require.NoError(t, err)

		// Verify content
		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var parsed GenerationReport
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)
		assert.Equal(t, "test notes", parsed.Notes)
	})

	t.Run("saves pretty JSON", func(t *testing.T) {
		report := NewGenerationReport(
			[]BatchResult{{Index: 0, TransactionID: "tx-1"}},
			"pretty test",
			nil,
		)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-pretty.json")

		err := report.SaveJSON(filePath, true)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.True(t, strings.Contains(string(data), "\n"))
	})

	t.Run("file permissions are restrictive", func(t *testing.T) {
		report := NewGenerationReport(
			[]BatchResult{{Index: 0, TransactionID: "tx-1"}},
			"permissions test",
			nil,
		)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-perms.json")

		err := report.SaveJSON(filePath, false)
		require.NoError(t, err)

		info, err := os.Stat(filePath)
		require.NoError(t, err)

		// Check that the file is not world-readable (0600 = owner read/write only)
		mode := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0o600), mode)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		report := NewGenerationReport([]BatchResult{}, "", nil)

		err := report.SaveJSON("/nonexistent/path/report.json", false)
		assert.Error(t, err)
	})
}

// TestGenerationReportSaveHTML tests the SaveHTML method
func TestGenerationReportSaveHTML(t *testing.T) {
	t.Run("saves basic HTML", func(t *testing.T) {
		report := NewGenerationReport(
			[]BatchResult{
				{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
				{Index: 1, Error: errors.New("test error"), Duration: 50 * time.Millisecond},
			},
			"HTML test notes",
			nil,
		)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report.html")

		err := report.SaveHTML(filePath)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		html := string(data)

		assert.True(t, strings.Contains(html, "<!DOCTYPE html>"))
		assert.True(t, strings.Contains(html, "Mass Demo Generation Report"))
		assert.True(t, strings.Contains(html, "Summary"))
	})

	t.Run("includes step timings when present", func(t *testing.T) {
		report := &GenerationReport{
			GeneratedAt: time.Now().UTC(),
			Summary:     BatchSummary{TotalTransactions: 1},
			Results:     []BatchResult{{Index: 0, TransactionID: "tx-1"}},
			StepTimings: map[string]string{
				"setup":    "100ms",
				"execute":  "500ms",
				"teardown": "50ms",
			},
		}

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-timings.html")

		err := report.SaveHTML(filePath)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		html := string(data)

		assert.True(t, strings.Contains(html, "Step Durations"))
		assert.True(t, strings.Contains(html, "setup"))
		assert.True(t, strings.Contains(html, "100ms"))
	})

	t.Run("includes entities when present", func(t *testing.T) {
		report := &GenerationReport{
			GeneratedAt: time.Now().UTC(),
			Summary:     BatchSummary{TotalTransactions: 1},
			Results:     []BatchResult{{Index: 0, TransactionID: "tx-1"}},
			Entities: &ReportEntities{
				Counts: ReportEntityCounts{
					Organizations: 1,
					Ledgers:       2,
					Assets:        5,
					Accounts:      100,
					Portfolios:    10,
					Segments:      20,
					Transactions:  500,
				},
				IDs: ReportEntityIDs{
					OrganizationIDs: []string{"org-1"},
				},
			},
		}

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-entities.html")

		err := report.SaveHTML(filePath)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		html := string(data)

		assert.True(t, strings.Contains(html, "Entities"))
		assert.True(t, strings.Contains(html, "Organizations"))
		assert.True(t, strings.Contains(html, "Ledgers"))
		assert.True(t, strings.Contains(html, "Assets"))
		assert.True(t, strings.Contains(html, "IDs captured"))
	})

	t.Run("includes API stats when present", func(t *testing.T) {
		report := &GenerationReport{
			GeneratedAt: time.Now().UTC(),
			Summary:     BatchSummary{TotalTransactions: 1},
			Results:     []BatchResult{{Index: 0, TransactionID: "tx-1"}},
			APIStats: &ReportAPIStats{
				APICalls: 500,
				Errors: map[string]int{
					"timeout": 5,
					"network": 3,
				},
			},
		}

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-api.html")

		err := report.SaveHTML(filePath)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		html := string(data)

		assert.True(t, strings.Contains(html, "API Stats"))
		assert.True(t, strings.Contains(html, "Total API Calls"))
		assert.True(t, strings.Contains(html, "500"))
		assert.True(t, strings.Contains(html, "Error timeout"))
		assert.True(t, strings.Contains(html, "Error network"))
	})

	t.Run("includes data summary when present", func(t *testing.T) {
		report := &GenerationReport{
			GeneratedAt: time.Now().UTC(),
			Summary:     BatchSummary{TotalTransactions: 1},
			Results:     []BatchResult{{Index: 0, TransactionID: "tx-1"}},
			DataSummary: &ReportDataSummary{
				TransactionVolumeByAccount: map[string]int{
					"acc-1": 100,
					"acc-2": 200,
				},
				AccountDistributionByType: map[string]int{
					"checking": 50,
					"savings":  30,
				},
				AssetUsage: map[string]int{
					"USD": 500,
					"EUR": 200,
				},
			},
		}

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-data.html")

		err := report.SaveHTML(filePath)
		require.NoError(t, err)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)
		html := string(data)

		assert.True(t, strings.Contains(html, "Transaction Volume by Account"))
		assert.True(t, strings.Contains(html, "acc-1"))
		assert.True(t, strings.Contains(html, "Account Distribution by Type"))
		assert.True(t, strings.Contains(html, "checking"))
		assert.True(t, strings.Contains(html, "Asset Usage"))
		assert.True(t, strings.Contains(html, "USD"))
	})

	t.Run("file permissions are restrictive", func(t *testing.T) {
		report := NewGenerationReport([]BatchResult{}, "", nil)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "report-perms.html")

		err := report.SaveHTML(filePath)
		require.NoError(t, err)

		info, err := os.Stat(filePath)
		require.NoError(t, err)

		mode := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0o600), mode)
	})

	t.Run("returns error for invalid path", func(t *testing.T) {
		report := NewGenerationReport([]BatchResult{}, "", nil)

		err := report.SaveHTML("/nonexistent/path/report.html")
		assert.Error(t, err)
	})
}

// TestReportEntityCountsZeroValues tests default zero values
func TestReportEntityCountsZeroValues(t *testing.T) {
	counts := ReportEntityCounts{}

	assert.Equal(t, 0, counts.Organizations)
	assert.Equal(t, 0, counts.Ledgers)
	assert.Equal(t, 0, counts.Assets)
	assert.Equal(t, 0, counts.Accounts)
	assert.Equal(t, 0, counts.Transactions)
	assert.Equal(t, 0, counts.Portfolios)
	assert.Equal(t, 0, counts.Segments)
}

// TestReportEntityIDsEmptySlices tests empty slices
func TestReportEntityIDsEmptySlices(t *testing.T) {
	ids := ReportEntityIDs{}

	assert.Nil(t, ids.OrganizationIDs)
	assert.Nil(t, ids.LedgerIDs)
	assert.Nil(t, ids.AssetIDs)
	assert.Nil(t, ids.AccountIDs)
	assert.Nil(t, ids.PortfolioIDs)
	assert.Nil(t, ids.SegmentIDs)
	assert.Nil(t, ids.TransactionIDs)
}

// TestReportJSONSerialization tests JSON serialization/deserialization
func TestReportJSONSerialization(t *testing.T) {
	t.Run("round-trip serialization", func(t *testing.T) {
		original := &GenerationReport{
			GeneratedAt: time.Now().UTC().Truncate(time.Second),
			Summary: BatchSummary{
				TotalTransactions:     100,
				SuccessCount:          95,
				ErrorCount:            5,
				SuccessRate:           95.0,
				TotalDuration:         10 * time.Second,
				AverageDuration:       100 * time.Millisecond,
				TransactionsPerSecond: 9.5,
				ErrorCategories: map[string]int{
					"validation": 3,
					"network":    2,
				},
			},
			Results: []BatchResult{
				{Index: 0, TransactionID: "tx-1", Duration: 100 * time.Millisecond},
			},
			Notes: "Round-trip test",
			AdditionalInformation: map[string]any{
				"key1": "value1",
				"key2": float64(123), // JSON numbers are float64
			},
			StepTimings: map[string]string{
				"step1": "100ms",
			},
		}

		data, err := original.ToJSON(false)
		require.NoError(t, err)

		var parsed GenerationReport
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		assert.Equal(t, original.Summary.TotalTransactions, parsed.Summary.TotalTransactions)
		assert.Equal(t, original.Summary.SuccessCount, parsed.Summary.SuccessCount)
		assert.Equal(t, original.Notes, parsed.Notes)
		assert.Equal(t, original.StepTimings["step1"], parsed.StepTimings["step1"])
	})

	t.Run("nil optional fields serialize correctly", func(t *testing.T) {
		report := &GenerationReport{
			GeneratedAt: time.Now().UTC(),
			Summary:     BatchSummary{TotalTransactions: 1},
			Results:     []BatchResult{},
			Entities:    nil,
			APIStats:    nil,
			DataSummary: nil,
		}

		data, err := report.ToJSON(false)
		require.NoError(t, err)

		// Check that nil fields are omitted
		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		_, hasEntities := parsed["entities"]
		_, hasAPIStats := parsed["apiStats"]
		_, hasDataSummary := parsed["dataSummary"]

		assert.False(t, hasEntities, "nil entities should be omitted")
		assert.False(t, hasAPIStats, "nil apiStats should be omitted")
		assert.False(t, hasDataSummary, "nil dataSummary should be omitted")
	})
}

// TestHTMLOutputStructure tests the structure of generated HTML
func TestHTMLOutputStructure(t *testing.T) {
	report := &GenerationReport{
		GeneratedAt: time.Now().UTC(),
		Summary: BatchSummary{
			TotalTransactions:     50,
			SuccessCount:          45,
			ErrorCount:            5,
			SuccessRate:           90.0,
			TransactionsPerSecond: 10.0,
		},
		Results: []BatchResult{},
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "structure.html")

	err := report.SaveHTML(filePath)
	require.NoError(t, err)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	html := string(data)

	// Check HTML structure
	assert.True(t, strings.Contains(html, "<!DOCTYPE html>"))
	assert.True(t, strings.Contains(html, "<html>"))
	assert.True(t, strings.Contains(html, "</html>"))
	assert.True(t, strings.Contains(html, "<head>"))
	assert.True(t, strings.Contains(html, "</head>"))
	assert.True(t, strings.Contains(html, "<body>"))
	assert.True(t, strings.Contains(html, "</body>"))
	assert.True(t, strings.Contains(html, "<style>"))
	assert.True(t, strings.Contains(html, "charset=\"utf-8\""))

	// Check summary values
	assert.True(t, strings.Contains(html, "50"))    // Total
	assert.True(t, strings.Contains(html, "45"))    // Success
	assert.True(t, strings.Contains(html, "5"))     // Errors
	assert.True(t, strings.Contains(html, "90.0%")) // Success rate
	assert.True(t, strings.Contains(html, "10.00")) // TPS
}

// TestReportWithEmptyDataSummaryMaps tests data summary with empty maps
func TestReportWithEmptyDataSummaryMaps(t *testing.T) {
	report := &GenerationReport{
		GeneratedAt: time.Now().UTC(),
		Summary:     BatchSummary{TotalTransactions: 1},
		Results:     []BatchResult{},
		DataSummary: &ReportDataSummary{
			TransactionVolumeByAccount: map[string]int{},
			AccountDistributionByType:  map[string]int{},
			AssetUsage:                 map[string]int{},
		},
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty-maps.html")

	err := report.SaveHTML(filePath)
	require.NoError(t, err)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	html := string(data)

	// Empty maps should not generate their sections
	assert.False(t, strings.Contains(html, "Transaction Volume by Account"))
	assert.False(t, strings.Contains(html, "Account Distribution by Type"))
	assert.False(t, strings.Contains(html, "Asset Usage"))
}

// TestReportWithNoIDs tests entities with counts but no IDs
func TestReportWithNoIDs(t *testing.T) {
	report := &GenerationReport{
		GeneratedAt: time.Now().UTC(),
		Summary:     BatchSummary{TotalTransactions: 1},
		Results:     []BatchResult{},
		Entities: &ReportEntities{
			Counts: ReportEntityCounts{
				Organizations: 1,
				Ledgers:       2,
			},
			IDs: ReportEntityIDs{}, // No IDs
		},
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "no-ids.html")

	err := report.SaveHTML(filePath)
	require.NoError(t, err)

	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	html := string(data)

	// Should show entities section but not the IDs note
	assert.True(t, strings.Contains(html, "Entities"))
	assert.False(t, strings.Contains(html, "IDs captured"))
}

// TestGenerationReportGeneratedAtUTC tests that GeneratedAt is in UTC
func TestGenerationReportGeneratedAtUTC(t *testing.T) {
	report := NewGenerationReport([]BatchResult{}, "", nil)

	// The time should be very recent
	assert.WithinDuration(t, time.Now().UTC(), report.GeneratedAt, 1*time.Second)

	// Should be in UTC
	_, offset := report.GeneratedAt.Zone()
	assert.Equal(t, 0, offset)
}
