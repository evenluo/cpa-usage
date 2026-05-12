package dto

import "time"

type AnalyticsSummary struct {
	TotalCost     float64
	TotalTokens   int64
	RequestCount  int64
	SuccessCount  int64
	FailureCount  int64
	SuccessRate   float64
	CostAvailable bool
	CostStatus    string
}

type AnalyticsTrendPoint struct {
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
	Summary AnalyticsSummary
	Trend   []AnalyticsTrendPoint
}
