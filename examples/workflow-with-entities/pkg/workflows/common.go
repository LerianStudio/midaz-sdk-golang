package workflows

// Default concurrent transaction counts used when callers pass 0.
const (
	defaultConcurrentCustomerToMerchantTxs = 20
	defaultConcurrentMerchantToCustomerTxs = 20
)
