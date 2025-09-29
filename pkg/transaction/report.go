package transaction

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ReportEntities contains counts and identifiers for created entities.
type ReportEntities struct {
	Counts ReportEntityCounts `json:"counts"`
	IDs    ReportEntityIDs    `json:"ids,omitempty"`
}

// ReportEntityCounts summarizes how many entities were created.
type ReportEntityCounts struct {
	Organizations int `json:"organizations"`
	Ledgers       int `json:"ledgers"`
	Assets        int `json:"assets"`
	Accounts      int `json:"accounts"`
	Transactions  int `json:"transactions"`
	Portfolios    int `json:"portfolios"`
	Segments      int `json:"segments"`
}

// ReportEntityIDs lists the identifiers for traceability.
type ReportEntityIDs struct {
	OrganizationIDs []string `json:"organizationIds,omitempty"`
	LedgerIDs       []string `json:"ledgerIds,omitempty"`
	AssetIDs        []string `json:"assetIds,omitempty"`
	AccountIDs      []string `json:"accountIds,omitempty"`
	PortfolioIDs    []string `json:"portfolioIds,omitempty"`
	SegmentIDs      []string `json:"segmentIds,omitempty"`
	TransactionIDs  []string `json:"transactionIds,omitempty"`
}

// ReportAPIStats captures minimal API usage information.
type ReportAPIStats struct {
	APICalls int               `json:"apiCalls"`
	Errors   map[string]int    `json:"errors,omitempty"`
	Notes    map[string]string `json:"notes,omitempty"`
}

// ReportDataSummary captures high-level statistics derived from the run.
type ReportDataSummary struct {
	TransactionVolumeByAccount map[string]int            `json:"transactionVolumeByAccount,omitempty"`
	AccountDistributionByType  map[string]int            `json:"accountDistributionByType,omitempty"`
	AssetUsage                 map[string]int            `json:"assetUsage,omitempty"`
	BalanceSummaries           map[string]map[string]any `json:"balanceSummaries,omitempty"`
}

// GenerationReport is a JSON-friendly report for batch transaction runs.
type GenerationReport struct {
	GeneratedAt           time.Time          `json:"generatedAt"`
	Summary               BatchSummary       `json:"summary"`
	Results               []BatchResult      `json:"results"`
	Notes                 string             `json:"notes,omitempty"`
	AdditionalInformation map[string]any     `json:"additionalInformation,omitempty"`
	StepTimings           map[string]string  `json:"stepTimings,omitempty"` // human-friendly durations
	Entities              *ReportEntities    `json:"entities,omitempty"`
	APIStats              *ReportAPIStats    `json:"apiStats,omitempty"`
	DataSummary           *ReportDataSummary `json:"dataSummary,omitempty"`
}

// NewGenerationReport creates a report from batch results.
func NewGenerationReport(results []BatchResult, notes string, additional map[string]any) *GenerationReport {
	return &GenerationReport{
		GeneratedAt:           time.Now().UTC(),
		Summary:               GetBatchSummary(results),
		Results:               results,
		Notes:                 notes,
		AdditionalInformation: additional,
	}
}

// ToJSON returns the JSON-encoded report.
func (r *GenerationReport) ToJSON(pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(r, "", "  ")
	}

	return json.Marshal(r)
}

// SaveJSON writes the report to a file in JSON format.
func (r *GenerationReport) SaveJSON(path string, pretty bool) error {
	data, err := r.ToJSON(pretty)
	if err != nil {
		return err
	}
	// Restrict permissions to owner read/write as report can include IDs.
	return os.WriteFile(path, data, 0o600)
}

// SaveHTML writes a minimal HTML report for quick viewing.
// This is intentionally dependency-free (no templates) to avoid adding heavy deps.
func (r *GenerationReport) SaveHTML(path string) error {
	b := &strings.Builder{}
	// Simple inline styles for readability
	_, _ = fmt.Fprintf(b, "<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>Mass Demo Report</title>\n")
	_, _ = fmt.Fprintf(b, "<style>body{font-family:Arial,Helvetica,sans-serif;margin:24px;} .k{color:#555;} table{border-collapse:collapse;margin-top:8px;} td,th{border:1px solid #ddd;padding:6px 10px;} th{background:#f6f6f6;text-align:left;} code{background:#f2f2f2;padding:2px 4px;border-radius:4px;} .section{margin-bottom:20px;} .muted{color:#777;}</style></head><body>")
	_, _ = fmt.Fprintf(b, "<h1>Mass Demo Generation Report</h1>")
	_, _ = fmt.Fprintf(b, "<p class=\"muted\">Generated at %s</p>", r.GeneratedAt.Format(time.RFC3339))

	// Summary
	_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>Summary</h2>")
	_, _ = fmt.Fprintf(b, "<table><tbody>")
	_, _ = fmt.Fprintf(b, "<tr><th>Total</th><td>%d</td></tr>", r.Summary.TotalTransactions)
	_, _ = fmt.Fprintf(b, "<tr><th>Success</th><td>%d</td></tr>", r.Summary.SuccessCount)
	_, _ = fmt.Fprintf(b, "<tr><th>Errors</th><td>%d</td></tr>", r.Summary.ErrorCount)
	_, _ = fmt.Fprintf(b, "<tr><th>Success Rate</th><td>%.1f%%</td></tr>", r.Summary.SuccessRate)
	_, _ = fmt.Fprintf(b, "<tr><th>TPS</th><td>%.2f</td></tr>", r.Summary.TransactionsPerSecond)
	_, _ = fmt.Fprintf(b, "</tbody></table></div>")

	// Step timings
	if len(r.StepTimings) > 0 {
		_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>Step Durations</h2><table><tbody>")
		for k, v := range r.StepTimings {
			_, _ = fmt.Fprintf(b, "<tr><th>%s</th><td>%s</td></tr>", k, v)
		}

		_, _ = fmt.Fprintf(b, "</tbody></table></div>")
	}

	// Entities
	if r.Entities != nil {
		_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>Entities</h2>")
		c := r.Entities.Counts
		_, _ = fmt.Fprintf(b, "<table><tbody>")
		_, _ = fmt.Fprintf(b, "<tr><th>Organizations</th><td>%d</td></tr>", c.Organizations)
		_, _ = fmt.Fprintf(b, "<tr><th>Ledgers</th><td>%d</td></tr>", c.Ledgers)
		_, _ = fmt.Fprintf(b, "<tr><th>Assets</th><td>%d</td></tr>", c.Assets)
		_, _ = fmt.Fprintf(b, "<tr><th>Accounts</th><td>%d</td></tr>", c.Accounts)
		_, _ = fmt.Fprintf(b, "<tr><th>Portfolios</th><td>%d</td></tr>", c.Portfolios)
		_, _ = fmt.Fprintf(b, "<tr><th>Segments</th><td>%d</td></tr>", c.Segments)
		_, _ = fmt.Fprintf(b, "<tr><th>Transactions</th><td>%d</td></tr>", c.Transactions)
		_, _ = fmt.Fprintf(b, "</tbody></table>")
		// IDs brief list
		ids := r.Entities.IDs
		if len(ids.OrganizationIDs)+len(ids.LedgerIDs)+len(ids.AssetIDs)+len(ids.AccountIDs)+len(ids.PortfolioIDs)+len(ids.SegmentIDs)+len(ids.TransactionIDs) > 0 {
			_, _ = fmt.Fprintf(b, "<p class=\"muted\">IDs captured (truncated for brevity).</p>")
		}

		_, _ = fmt.Fprintf(b, "</div>")
	}

	// API stats
	if r.APIStats != nil {
		_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>API Stats</h2><table><tbody>")
		_, _ = fmt.Fprintf(b, "<tr><th>Total API Calls</th><td>%d</td></tr>", r.APIStats.APICalls)

		if len(r.APIStats.Errors) > 0 {
			for k, v := range r.APIStats.Errors {
				_, _ = fmt.Fprintf(b, "<tr><th>Error %s</th><td>%d</td></tr>", k, v)
			}
		}

		_, _ = fmt.Fprintf(b, "</tbody></table></div>")
	}

	// Data summary
	if r.DataSummary != nil {
		ds := r.DataSummary
		if len(ds.TransactionVolumeByAccount) > 0 {
			_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>Transaction Volume by Account</h2><table><tbody>")
			for k, v := range ds.TransactionVolumeByAccount {
				_, _ = fmt.Fprintf(b, "<tr><th>%s</th><td>%d</td></tr>", k, v)
			}

			_, _ = fmt.Fprintf(b, "</tbody></table></div>")
		}

		if len(ds.AccountDistributionByType) > 0 {
			_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>Account Distribution by Type</h2><table><tbody>")
			for k, v := range ds.AccountDistributionByType {
				_, _ = fmt.Fprintf(b, "<tr><th>%s</th><td>%d</td></tr>", k, v)
			}

			_, _ = fmt.Fprintf(b, "</tbody></table></div>")
		}

		if len(ds.AssetUsage) > 0 {
			_, _ = fmt.Fprintf(b, "<div class=\"section\"><h2>Asset Usage</h2><table><tbody>")
			for k, v := range ds.AssetUsage {
				_, _ = fmt.Fprintf(b, "<tr><th>%s</th><td>%d</td></tr>", k, v)
			}

			_, _ = fmt.Fprintf(b, "</tbody></table></div>")
		}
	}

	_, _ = fmt.Fprintf(b, "</body></html>")
	// Restrict permissions to owner read/write as report can include IDs.
	return os.WriteFile(path, []byte(b.String()), 0o600)
}
