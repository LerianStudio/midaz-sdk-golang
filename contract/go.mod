// Nested module: isolates the Midaz server dependency (and its transitive
// graph) from the SDK's published go.mod. See drift_test.go for rationale.
module github.com/LerianStudio/midaz-sdk-golang/v6/contract

go 1.27.0

require (
	github.com/LerianStudio/midaz-sdk-golang/v6 v6.0.0
	github.com/LerianStudio/midaz/v4 v4.2.0
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/LerianStudio/lib-observability/v3 v3.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

replace github.com/LerianStudio/midaz-sdk-golang/v6 => ../
