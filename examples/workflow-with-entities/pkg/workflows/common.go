package workflows

// Global variables for concurrent transaction counts
var (
	concurrentCustomerToMerchantTxs int
	concurrentMerchantToCustomerTxs int
)
