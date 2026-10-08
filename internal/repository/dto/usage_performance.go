package dto

// UsageAttemptPerformanceRecord keeps result and execution populations
// separate so percentile consumers never silently mix unlike attempts.
type UsageAttemptPerformanceRecord struct {
	TotalAttempts          int64
	SuccessfulAttempts     int64
	FailedAttempts         int64
	SuccessfulExecution    UsageExecutionPopulationRecord
	SuccessfulLatencyMS    UsagePercentileRecord
	FailedLatencyMS        UsagePercentileRecord
	StreamingTTFTMS        UsagePercentileRecord
	UnknownExecutionTTFTMS UsagePercentileRecord
	StreamingOutputTPS     UsageOutputTPSDistributionRecord
	Providers              UsagePerformanceBreakdownRecord
	Models                 UsagePerformanceBreakdownRecord
	Accounts               UsagePerformanceBreakdownRecord
}

const UsagePerformanceBreakdownLimit = 8

type UsagePerformanceBreakdownRecord struct {
	Items      []UsagePerformanceBreakdownItemRecord
	OtherCount int64
}

type UsagePerformanceBreakdownItemRecord struct {
	Value                  string
	AttemptCount           int64
	SuccessfulAttempts     int64
	FailedAttempts         int64
	SuccessfulExecution    UsageExecutionPopulationRecord
	SuccessfulLatencyMS    UsagePercentileRecord
	FailedLatencyMS        UsagePercentileRecord
	StreamingTTFTMS        UsagePercentileRecord
	UnknownExecutionTTFTMS UsagePercentileRecord
	StreamingOutputTPS     UsageOutputTPSDistributionRecord
}

// UsageExecutionPopulationRecord is an exhaustive, non-overlapping partition
// of successful attempts. Explicit false flags take precedence over unknowns.
type UsageExecutionPopulationRecord struct {
	GeneratingStreaming int64
	NonGenerating       int64
	NonStreaming        int64
	Unknown             int64
}

// UsagePercentileRecord uses nearest-rank percentiles over valid samples.
// Coverage is nil only when PopulationCount is zero.
type UsagePercentileRecord struct {
	PopulationCount int64
	SampleCount     int64
	Coverage        *float64
	P50             *float64
	P95             *float64
	Histogram       *UsageHistogramRecord
}

// UsageHistogramRecord counts the same valid samples used by the exact
// percentiles. Bins start at zero and have equal width; the last includes
// UpperBound. Comparable breakdowns share their overall population's bound.
type UsageHistogramRecord struct {
	UpperBound float64
	Counts     []int64
}

const UsagePerformanceHistogramBins = 24

// UsageOutputTPSDistributionRecord describes generating/streaming throughput
// where the slow tail is the low end: P10 is the nearest-rank 10th percentile,
// so 90% of valid samples are at least that fast. Bands are fixed perceptual
// intervals rather than a data-dependent equal-width histogram, so a slow model
// keeps its resolution next to a fast one and bands compare across snapshots.
type UsageOutputTPSDistributionRecord struct {
	PopulationCount int64
	SampleCount     int64
	Coverage        *float64
	P50             *float64
	P10             *float64
	// Bands counts the same valid samples per UsageOutputTPSBandEdges interval.
	// It is nil only when SampleCount is zero.
	Bands []int64
}

// UsageOutputTPSBandEdges are the lower edges, in tokens per second, of the
// fixed Output TPS bands. Band i covers [edge[i], edge[i+1]); the last band is
// open-ended. Resolution is 10 tok/s up to 100 where perceived speed differs
// most, then coarser bands that only record that faster output exists.
var UsageOutputTPSBandEdges = []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 150, 200, 300}
