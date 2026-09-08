package dto

// UsageModelMappingSummaryRecord contains only the fixed-window facts needed
// by the collapsed model-mapping view.
type UsageModelMappingSummaryRecord struct {
	TotalAttempts         int64
	ObservedAliasAttempts int64
	MissingAliasAttempts  int64
	DisplayedMappings     int64
}
