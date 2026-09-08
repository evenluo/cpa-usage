package dto

// UsagePerformanceProviderOptionsRecord is the complete fixed-window provider
// catalog used to choose one scoped attempt-performance population.
type UsagePerformanceProviderOptionsRecord struct {
	ProviderOptions []UsagePerformanceProviderOptionRecord
}

type UsagePerformanceProviderOptionRecord struct {
	Provider     string
	RequestCount int64
}
