package transaction

import (
	"encoding/json"
	"os"
	"time"
)

// GenerationReport is a JSON-friendly report for batch transaction runs.
type GenerationReport struct {
	GeneratedAt           time.Time      `json:"generatedAt"`
	Summary               BatchSummary   `json:"summary"`
	Results               []BatchResult  `json:"results"`
	Notes                 string         `json:"notes,omitempty"`
	AdditionalInformation map[string]any `json:"additionalInformation,omitempty"`
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
	return os.WriteFile(path, data, 0o644)
}
