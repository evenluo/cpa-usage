package dto

// UsageAttemptPerformanceRecord keeps result and execution populations
// separate so percentile consumers never silently mix unlike attempts.
type UsageAttemptPerformanceRecord struct {
	TotalAttempts             int64
	SuccessfulAttempts        int64
	FailedAttempts            int64
	SuccessfulExecution       UsageExecutionPopulationRecord
	SuccessfulLatencyMS       UsagePercentileRecord
	FailedLatencyMS           UsagePercentileRecord
	StreamingTTFTMS           UsagePercentileRecord
	UnknownExecutionTTFTMS    UsagePercentileRecord
	StreamingOutputTPS        UsagePercentileRecord
	UnknownExecutionOutputTPS UsagePercentileRecord
	Providers                 UsagePerformanceBreakdownRecord
	Models                    UsagePerformanceBreakdownRecord
	Accounts                  UsagePerformanceBreakdownRecord
}

const UsagePerformanceBreakdownLimit = 8

type UsagePerformanceBreakdownRecord struct {
	Items      []UsagePerformanceBreakdownItemRecord
	OtherCount int64
}

type UsagePerformanceBreakdownItemRecord struct {
	Value                     string
	AttemptCount              int64
	SuccessfulAttempts        int64
	FailedAttempts            int64
	SuccessfulExecution       UsageExecutionPopulationRecord
	SuccessfulLatencyMS       UsagePercentileRecord
	FailedLatencyMS           UsagePercentileRecord
	StreamingTTFTMS           UsagePercentileRecord
	UnknownExecutionTTFTMS    UsagePercentileRecord
	StreamingOutputTPS        UsagePercentileRecord
	UnknownExecutionOutputTPS UsagePercentileRecord
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
}
