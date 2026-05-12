package dto

import "time"

const (
	AnalyticsCostStatusAvailable   = "available"
	AnalyticsCostStatusPartial     = "partial"
	AnalyticsCostStatusUnavailable = "unavailable"
)

type AnalyticsSummaryRecord struct {
	TotalCost     float64
	TotalTokens   int64
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	SuccessRate   float64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsTrendPointRecord struct {
	Label         string
	BucketStart   time.Time
	BucketEnd     time.Time
	TotalCost     float64
	TotalTokens   int64
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsSummarySnapshot struct {
	Summary AnalyticsSummaryRecord
	Trend   []AnalyticsTrendPointRecord
}
