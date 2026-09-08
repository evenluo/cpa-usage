package repository

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

const (
	knownStreamingExecutionSQL = "generate = 1 AND stream = 1"
	unknownExecutionSQL        = "(generate IS NULL OR stream IS NULL) AND (generate IS NULL OR generate = 1) AND (stream IS NULL OR stream = 1)"
)

// BuildUsageAttemptPerformanceWithFilter computes fixed-window nearest-rank
// percentiles from bounded raw attempts. Each population is loaded by its own
// statement so result and execution semantics remain explicit and testable.
func BuildUsageAttemptPerformanceWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageDiagnosticFilter) (*dto.UsageAttemptPerformanceRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		return nil, fmt.Errorf("attempt performance requires bounded start and end times")
	}
	var record *dto.UsageAttemptPerformanceRecord
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		record, err = buildUsageAttemptPerformanceSnapshot(ctx, tx, filter)
		return err
	}); err != nil {
		return nil, err
	}
	return record, nil
}

// buildUsageAttemptPerformanceSnapshot issues every statement on one SQLite
// read transaction. Delayed intake may commit while this runs, but it cannot
// split denominators, percentile samples, and Top-N membership across snapshots.
func buildUsageAttemptPerformanceSnapshot(ctx context.Context, db *gorm.DB, filter dto.UsageDiagnosticFilter) (*dto.UsageAttemptPerformanceRecord, error) {
	base := func() *gorm.DB {
		return applyUsageDiagnosticQuery(
			db.WithContext(ctx).Table("usage_events INDEXED BY idx_usage_events_timestamp_id"),
			filter,
		)
	}

	var resultCounts struct {
		Total      int64
		Successful int64
		Failed     int64
	}
	if err := base().Select(`COUNT(*) AS total,
		SUM(CASE WHEN failed = 0 THEN 1 ELSE 0 END) AS successful,
		SUM(CASE WHEN failed = 1 THEN 1 ELSE 0 END) AS failed`).Scan(&resultCounts).Error; err != nil {
		return nil, fmt.Errorf("count attempt performance result populations: %w", err)
	}

	var execution dto.UsageExecutionPopulationRecord
	if resultCounts.Successful > 0 {
		var counts struct {
			GeneratingStreaming int64
			NonGenerating       int64
			NonStreaming        int64
		}
		if err := base().Where("failed = 0").Select(`
			SUM(CASE WHEN generate = 1 AND stream = 1 THEN 1 ELSE 0 END) AS generating_streaming,
			SUM(CASE WHEN generate = 0 THEN 1 ELSE 0 END) AS non_generating,
			SUM(CASE WHEN (generate IS NULL OR generate = 1) AND stream = 0 THEN 1 ELSE 0 END) AS non_streaming`).Scan(&counts).Error; err != nil {
			return nil, fmt.Errorf("count successful execution populations: %w", err)
		}
		execution.GeneratingStreaming = counts.GeneratingStreaming
		execution.NonGenerating = counts.NonGenerating
		execution.NonStreaming = counts.NonStreaming
		execution.Unknown = resultCounts.Successful - counts.GeneratingStreaming - counts.NonGenerating - counts.NonStreaming
	}

	successLatency, err := loadUsageIntegerPercentiles(base().Where("failed = 0 AND latency_ms > 0"), "latency_ms", resultCounts.Successful)
	if err != nil {
		return nil, fmt.Errorf("load successful latency distribution: %w", err)
	}
	failedLatency, err := loadUsageIntegerPercentiles(base().Where("failed = 1 AND latency_ms > 0"), "latency_ms", resultCounts.Failed)
	if err != nil {
		return nil, fmt.Errorf("load failed latency distribution: %w", err)
	}
	streamingTTFT, err := loadUsageIntegerPercentiles(
		base().Where("failed = 0 AND "+knownStreamingExecutionSQL+" AND ttft_ms > 0 AND latency_ms >= ttft_ms"),
		"ttft_ms", execution.GeneratingStreaming,
	)
	if err != nil {
		return nil, fmt.Errorf("load generating streaming TTFT distribution: %w", err)
	}
	unknownTTFT, err := loadUsageIntegerPercentiles(
		base().Where("failed = 0 AND "+unknownExecutionSQL+" AND ttft_ms > 0 AND latency_ms >= ttft_ms"),
		"ttft_ms", execution.Unknown,
	)
	if err != nil {
		return nil, fmt.Errorf("load unknown-execution TTFT distribution: %w", err)
	}
	providerSelected := strings.TrimSpace(filter.Provider) != ""
	streamingTPS := usagePercentileRecord(nil, execution.GeneratingStreaming)
	if providerSelected {
		streamingTPS, err = loadUsageOutputTPSPercentiles(base().Where("failed = 0 AND "+knownStreamingExecutionSQL), execution.GeneratingStreaming)
		if err != nil {
			return nil, fmt.Errorf("load generating streaming Output TPS distribution: %w", err)
		}
	}

	bounds := usagePerformanceHistogramBounds{
		successLatency: histogramUpperBound(successLatency),
		failedLatency:  histogramUpperBound(failedLatency),
		streamingTTFT:  histogramUpperBound(streamingTTFT),
		unknownTTFT:    histogramUpperBound(unknownTTFT),
		streamingTPS:   histogramUpperBound(streamingTPS),
	}
	providers, err := loadUsagePerformanceBreakdown(base, "provider", resultCounts.Total, true, bounds)
	if err != nil {
		return nil, fmt.Errorf("load provider performance breakdown: %w", err)
	}
	models, err := loadUsagePerformanceBreakdown(base, "model", resultCounts.Total, providerSelected, bounds)
	if err != nil {
		return nil, fmt.Errorf("load model performance breakdown: %w", err)
	}
	accounts, err := loadUsagePerformanceBreakdown(base, "auth_index", resultCounts.Total, providerSelected, bounds)
	if err != nil {
		return nil, fmt.Errorf("load account performance breakdown: %w", err)
	}

	return &dto.UsageAttemptPerformanceRecord{
		TotalAttempts:          resultCounts.Total,
		SuccessfulAttempts:     resultCounts.Successful,
		FailedAttempts:         resultCounts.Failed,
		SuccessfulExecution:    execution,
		SuccessfulLatencyMS:    successLatency,
		FailedLatencyMS:        failedLatency,
		StreamingTTFTMS:        streamingTTFT,
		UnknownExecutionTTFTMS: unknownTTFT,
		StreamingOutputTPS:     streamingTPS,
		Providers:              providers,
		Models:                 models,
		Accounts:               accounts,
	}, nil
}

type usagePerformanceDimensionCount struct {
	Value string
	Count int64
}

type usagePerformanceAttempt struct {
	Dimension                   string  `gorm:"column:dimension"`
	Failed                      bool    `gorm:"column:failed"`
	LatencyMS                   int64   `gorm:"column:latency_ms"`
	TTFTMS                      *int64  `gorm:"column:ttft_ms"`
	Generate                    *bool   `gorm:"column:generate"`
	Stream                      *bool   `gorm:"column:stream"`
	TokenQuality                *string `gorm:"column:token_quality"`
	CanonicalTotalTokens        *int64  `gorm:"column:canonical_total_tokens"`
	CanonicalInputTokens        *int64  `gorm:"column:canonical_input_tokens"`
	CanonicalUncachedTokens     *int64  `gorm:"column:canonical_uncached_tokens"`
	CanonicalCacheReadTokens    *int64  `gorm:"column:canonical_cache_read_tokens"`
	CanonicalCacheWriteTokens   *int64  `gorm:"column:canonical_cache_write_tokens"`
	CanonicalOutputTokens       *int64  `gorm:"column:canonical_output_tokens"`
	CanonicalNonReasoningTokens *int64  `gorm:"column:canonical_non_reasoning_tokens"`
	CanonicalReasoningTokens    *int64  `gorm:"column:canonical_reasoning_tokens"`
	CanonicalUnclassifiedTokens *int64  `gorm:"column:canonical_unclassified_tokens"`
}

const usagePerformanceAttemptColumns = `failed, latency_ms, ttft_ms, generate, stream,
	token_quality,
	canonical_total_tokens, canonical_input_tokens, canonical_uncached_tokens, canonical_cache_read_tokens,
	canonical_cache_write_tokens, canonical_output_tokens, canonical_non_reasoning_tokens,
	canonical_reasoning_tokens, canonical_unclassified_tokens`

type usagePerformanceHistogramBounds struct {
	successLatency float64
	failedLatency  float64
	streamingTTFT  float64
	unknownTTFT    float64
	streamingTPS   float64
}

func histogramUpperBound(distribution dto.UsagePercentileRecord) float64 {
	if distribution.Histogram == nil {
		return 0
	}
	return distribution.Histogram.UpperBound
}

func loadUsagePerformanceBreakdown(base func() *gorm.DB, dimension string, total int64, includeOutputTPS bool, bounds usagePerformanceHistogramBounds) (dto.UsagePerformanceBreakdownRecord, error) {
	expression := "TRIM(" + dimension + ")"
	var ranked []usagePerformanceDimensionCount
	if err := base().Select(expression + " AS value, COUNT(*) AS count").
		Where(expression + " <> ''").
		Group(expression).
		Order("count DESC, value ASC").
		Limit(dto.UsagePerformanceBreakdownLimit).
		Scan(&ranked).Error; err != nil {
		return dto.UsagePerformanceBreakdownRecord{}, err
	}
	if len(ranked) == 0 {
		return dto.UsagePerformanceBreakdownRecord{Items: []dto.UsagePerformanceBreakdownItemRecord{}, OtherCount: total}, nil
	}
	values := make([]string, 0, len(ranked))
	for _, row := range ranked {
		values = append(values, strings.TrimSpace(row.Value))
	}
	var attempts []usagePerformanceAttempt
	if err := base().Select(expression+" AS dimension, "+usagePerformanceAttemptColumns).
		Where(expression+" IN ?", values).
		Scan(&attempts).Error; err != nil {
		return dto.UsagePerformanceBreakdownRecord{}, err
	}
	groups := make(map[string][]usagePerformanceAttempt, len(values))
	for _, attempt := range attempts {
		value := strings.TrimSpace(attempt.Dimension)
		groups[value] = append(groups[value], attempt)
	}
	items := make([]dto.UsagePerformanceBreakdownItemRecord, 0, len(ranked))
	visibleCount := int64(0)
	for _, row := range ranked {
		value := strings.TrimSpace(row.Value)
		item := summarizeUsagePerformanceGroup(value, groups[value], includeOutputTPS, bounds)
		visibleCount += item.AttemptCount
		items = append(items, item)
	}
	return dto.UsagePerformanceBreakdownRecord{Items: items, OtherCount: total - visibleCount}, nil
}

func (attempt usagePerformanceAttempt) entity() entities.UsageEvent {
	return entities.UsageEvent{
		Failed:    attempt.Failed,
		LatencyMS: attempt.LatencyMS,
		TTFTMS:    attempt.TTFTMS,
		Generate:  attempt.Generate,
		Stream:    attempt.Stream,
		UsageAccounting: entities.UsageAccounting{
			TokenQuality:                attempt.TokenQuality,
			CanonicalTotalTokens:        attempt.CanonicalTotalTokens,
			CanonicalInputTokens:        attempt.CanonicalInputTokens,
			CanonicalUncachedTokens:     attempt.CanonicalUncachedTokens,
			CanonicalCacheReadTokens:    attempt.CanonicalCacheReadTokens,
			CanonicalCacheWriteTokens:   attempt.CanonicalCacheWriteTokens,
			CanonicalOutputTokens:       attempt.CanonicalOutputTokens,
			CanonicalNonReasoningTokens: attempt.CanonicalNonReasoningTokens,
			CanonicalReasoningTokens:    attempt.CanonicalReasoningTokens,
			CanonicalUnclassifiedTokens: attempt.CanonicalUnclassifiedTokens,
		},
	}
}

func summarizeUsagePerformanceGroup(value string, attempts []usagePerformanceAttempt, includeOutputTPS bool, bounds usagePerformanceHistogramBounds) dto.UsagePerformanceBreakdownItemRecord {
	item := dto.UsagePerformanceBreakdownItemRecord{Value: value, AttemptCount: int64(len(attempts))}
	successLatency := make([]float64, 0, len(attempts))
	failedLatency := make([]float64, 0, len(attempts))
	streamingTTFT := make([]float64, 0, len(attempts))
	unknownTTFT := make([]float64, 0, len(attempts))
	streamingTPS := make([]float64, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt.Failed {
			item.FailedAttempts++
			if attempt.LatencyMS > 0 {
				failedLatency = append(failedLatency, float64(attempt.LatencyMS))
			}
			continue
		}
		item.SuccessfulAttempts++
		if attempt.LatencyMS > 0 {
			successLatency = append(successLatency, float64(attempt.LatencyMS))
		}
		cohort := usageExecutionCohort(attempt.Generate, attempt.Stream)
		switch cohort {
		case "generating_streaming":
			item.SuccessfulExecution.GeneratingStreaming++
			if attempt.validTTFT() {
				streamingTTFT = append(streamingTTFT, float64(*attempt.TTFTMS))
			}
			if includeOutputTPS {
				if outputTPS := InterpretUsageAttempt(attempt.entity()).OutputTPS; outputTPS != nil {
					streamingTPS = append(streamingTPS, *outputTPS)
				}
			}
		case "non_generating":
			item.SuccessfulExecution.NonGenerating++
		case "non_streaming":
			item.SuccessfulExecution.NonStreaming++
		case "unknown":
			item.SuccessfulExecution.Unknown++
			if attempt.validTTFT() {
				unknownTTFT = append(unknownTTFT, float64(*attempt.TTFTMS))
			}
		}
	}
	item.SuccessfulLatencyMS = usagePercentileRecordWithHistogram(successLatency, item.SuccessfulAttempts, bounds.successLatency)
	item.FailedLatencyMS = usagePercentileRecordWithHistogram(failedLatency, item.FailedAttempts, bounds.failedLatency)
	item.StreamingTTFTMS = usagePercentileRecordWithHistogram(streamingTTFT, item.SuccessfulExecution.GeneratingStreaming, bounds.streamingTTFT)
	item.UnknownExecutionTTFTMS = usagePercentileRecordWithHistogram(unknownTTFT, item.SuccessfulExecution.Unknown, bounds.unknownTTFT)
	item.StreamingOutputTPS = usagePercentileRecordWithHistogram(streamingTPS, item.SuccessfulExecution.GeneratingStreaming, bounds.streamingTPS)
	return item
}

func usageExecutionCohort(generate, stream *bool) string {
	if generate != nil && !*generate {
		return "non_generating"
	}
	if stream != nil && !*stream {
		return "non_streaming"
	}
	if generate != nil && *generate && stream != nil && *stream {
		return "generating_streaming"
	}
	return "unknown"
}

func (attempt usagePerformanceAttempt) validTTFT() bool {
	return attempt.TTFTMS != nil && *attempt.TTFTMS > 0 && attempt.LatencyMS >= *attempt.TTFTMS
}

func loadUsageIntegerPercentiles(query *gorm.DB, column string, populationCount int64) (dto.UsagePercentileRecord, error) {
	var raw []int64
	if err := query.Pluck(column, &raw).Error; err != nil {
		return dto.UsagePercentileRecord{}, err
	}
	values := make([]float64, len(raw))
	for index, value := range raw {
		values[index] = float64(value)
	}
	return usagePercentileRecord(values, populationCount), nil
}

func loadUsageOutputTPSPercentiles(query *gorm.DB, populationCount int64) (dto.UsagePercentileRecord, error) {
	var attempts []usagePerformanceAttempt
	if err := query.Select("'' AS dimension, " + usagePerformanceAttemptColumns).Scan(&attempts).Error; err != nil {
		return dto.UsagePercentileRecord{}, err
	}
	values := make([]float64, 0, len(attempts))
	for _, attempt := range attempts {
		event := attempt.entity()
		if outputTPS := InterpretUsageAttempt(event).OutputTPS; outputTPS != nil {
			values = append(values, *outputTPS)
		}
	}
	return usagePercentileRecord(values, populationCount), nil
}

func usagePercentileRecord(values []float64, populationCount int64) dto.UsagePercentileRecord {
	return usagePercentileRecordWithHistogram(values, populationCount, 0)
}

func usagePercentileRecordWithHistogram(values []float64, populationCount int64, upperBound float64) dto.UsagePercentileRecord {
	record := dto.UsagePercentileRecord{PopulationCount: populationCount, SampleCount: int64(len(values))}
	if populationCount > 0 {
		coverage := float64(len(values)) / float64(populationCount)
		record.Coverage = &coverage
	}
	if len(values) == 0 {
		return record
	}
	sort.Float64s(values)
	p50 := nearestRankPercentile(values, 0.50)
	p95 := nearestRankPercentile(values, 0.95)
	record.P50 = &p50
	record.P95 = &p95
	if upperBound == 0 {
		upperBound = values[len(values)-1]
	}
	counts := make([]int64, dto.UsagePerformanceHistogramBins)
	for _, value := range values {
		index := 0
		if upperBound > 0 {
			index = min(int(value/upperBound*float64(len(counts))), len(counts)-1)
		}
		counts[index]++
	}
	record.Histogram = &dto.UsageHistogramRecord{UpperBound: upperBound, Counts: counts}
	return record
}

func nearestRankPercentile(sortedValues []float64, percentile float64) float64 {
	index := int(math.Ceil(percentile*float64(len(sortedValues)))) - 1
	index = max(index, 0)
	return sortedValues[index]
}
