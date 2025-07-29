package models

// MetricsCount represents the count metrics for various entities in the Midaz system.
// It provides aggregated counts of different resource types within the system.
// This structure is used across various models to report on system usage and statistics.
type MetricsCount struct {
	// OrganizationsCount is the total number of organizations in the system
	OrganizationsCount int `json:"organizationsCount"`

	// LedgersCount is the total number of ledgers in the system
	LedgersCount int `json:"ledgersCount"`

	// AssetsCount is the total number of assets in the system
	AssetsCount int `json:"assetsCount"`

	// SegmentsCount is the total number of segments in the system
	SegmentsCount int `json:"segmentsCount"`

	// PortfoliosCount is the total number of portfolios in the system
	PortfoliosCount int `json:"portfoliosCount"`

	// AccountsCount is the total number of accounts in the system
	AccountsCount int `json:"accountsCount"`

	// TransactionsCount is the total number of transactions in the system
	TransactionsCount int `json:"transactionsCount"`

	// OperationsCount is the total number of operations in the system
	OperationsCount int `json:"operationsCount"`
}

// IsEmpty returns true if all count values are zero.
// This can be useful to check if the metrics response contains any data.
//
// Returns:
//   - true if all metrics are zero, false otherwise
func (m MetricsCount) IsEmpty() bool {
	return m.OrganizationsCount == 0 &&
		m.LedgersCount == 0 &&
		m.AssetsCount == 0 &&
		m.SegmentsCount == 0 &&
		m.PortfoliosCount == 0 &&
		m.AccountsCount == 0 &&
		m.TransactionsCount == 0 &&
		m.OperationsCount == 0
}
