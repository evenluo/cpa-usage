package repository

import (
	"context"
	"fmt"
	"strings"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type usageModelMappingSummaryRow struct {
	TotalAttempts          int64
	ObservedAliasAttempts  int64
	CanonicalValidAttempts int64
	ObservedTotalCost      float64
	MissingPricingEvents   int64
	PricedBillableEvents   int64
}

type usageModelMappingAggregateRow struct {
	ModelAlias             string
	Model                  string
	Provider               string
	AttemptCount           int64
	FailureCount           int64
	CanonicalValidAttempts int64
	TotalLatencyMS         int64
	LatencySampleCount     int64
	TotalCost              float64
	MissingPricingEvents   int64
	PricedBillableEvents   int64
}

// BuildUsageModelMappingsWithFilter returns a bounded fixed-window projection
// of observed CPA alias labels to actual model/provider pairs. Blank aliases
// stay outside the mapping rows and are reported as a coverage gap.
func BuildUsageModelMappingsWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageDiagnosticFilter) (*dto.UsageModelMappingDistributionRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		return nil, fmt.Errorf("model mappings require bounded start and end times")
	}

	var record *dto.UsageModelMappingDistributionRecord
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		source := analyticsEventsAggregateSource()
		observedAliasPredicate := "model_alias IS NOT NULL AND TRIM(model_alias) <> ''"
		base := func() *gorm.DB {
			return applyUsageDiagnosticQuery(
				tx.WithContext(ctx).Table("usage_events INDEXED BY idx_usage_events_timestamp_id"),
				filter,
			).Joins("LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_events.model)")
		}

		var summary usageModelMappingSummaryRow
		if err := base().Select(`
		COUNT(*) AS total_attempts,
			COALESCE(SUM(CASE WHEN ` + observedAliasPredicate + ` THEN 1 ELSE 0 END), 0) AS observed_alias_attempts,
			COALESCE(SUM(` + source.accounting.stateAttemptsExpr(AccountingValid) + `), 0) AS canonical_valid_attempts,
		COALESCE(SUM(CASE WHEN ` + observedAliasPredicate + ` THEN ` + analyticsSourceCostSQLExpression(source) + ` ELSE 0 END), 0) AS observed_total_cost,
		COALESCE(SUM(CASE WHEN ` + observedAliasPredicate + ` THEN ` + analyticsSourceMissingPricingSQLExpression(source) + ` ELSE 0 END), 0) AS missing_pricing_events,
		COALESCE(SUM(CASE WHEN ` + observedAliasPredicate + ` THEN ` + analyticsSourcePricedBillableSQLExpression(source) + ` ELSE 0 END), 0) AS priced_billable_events`).
			Scan(&summary).Error; err != nil {
			return fmt.Errorf("summarize usage model mapping population: %w", err)
		}

		var rows []usageModelMappingAggregateRow
		if err := base().Select(`
		TRIM(model_alias) AS model_alias,
		TRIM(usage_events.model) AS model,
		TRIM(usage_events.provider) AS provider,
		COUNT(*) AS attempt_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(` + source.accounting.stateAttemptsExpr(AccountingValid) + `), 0) AS canonical_valid_attempts,
		COALESCE(SUM(` + source.latencySumExpr + `), 0) AS total_latency_ms,
		COALESCE(SUM(` + source.latencyCountExpr + `), 0) AS latency_sample_count,
		COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
		COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
		COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`).
			Where(observedAliasPredicate).
			Group("TRIM(model_alias), TRIM(usage_events.model), TRIM(usage_events.provider)").
			Order("attempt_count DESC, model_alias ASC, model ASC, provider ASC").
			Limit(dto.UsageModelMappingLimit).
			Scan(&rows).Error; err != nil {
			return fmt.Errorf("load usage model mappings: %w", err)
		}

		record = &dto.UsageModelMappingDistributionRecord{
			TotalAttempts:          summary.TotalAttempts,
			ObservedAliasAttempts:  summary.ObservedAliasAttempts,
			CanonicalValidAttempts: summary.CanonicalValidAttempts,
			MissingAliasAttempts:   max(summary.TotalAttempts-summary.ObservedAliasAttempts, 0),
			ObservedTotalCost:      summary.ObservedTotalCost,
			Mappings:               make([]dto.UsageModelMappingRecord, 0, len(rows)),
		}
		cost := assessCostCompleteness(summary.MissingPricingEvents, summary.PricedBillableEvents)
		record.ObservedCostAvailable, record.ObservedCostStatus = cost.Available, cost.Status
		visibleAttempts := int64(0)
		for _, row := range rows {
			mapping := dto.UsageModelMappingRecord{
				ModelAlias:             strings.TrimSpace(row.ModelAlias),
				Model:                  strings.TrimSpace(row.Model),
				Provider:               strings.TrimSpace(row.Provider),
				AttemptCount:           row.AttemptCount,
				FailureCount:           row.FailureCount,
				CanonicalValidAttempts: row.CanonicalValidAttempts,
				TotalLatencyMS:         row.TotalLatencyMS,
				LatencySampleCount:     row.LatencySampleCount,
				TotalCost:              row.TotalCost,
			}
			if row.AttemptCount > 0 {
				mapping.FailureShare = float64(row.FailureCount) / float64(row.AttemptCount) * 100
			}
			if row.LatencySampleCount > 0 {
				mapping.MeanLatencyMS = float64(row.TotalLatencyMS) / float64(row.LatencySampleCount)
			}
			mappingCost := assessCostCompleteness(row.MissingPricingEvents, row.PricedBillableEvents)
			mapping.CostAvailable, mapping.CostStatus = mappingCost.Available, mappingCost.Status
			record.Mappings = append(record.Mappings, mapping)
			visibleAttempts += row.AttemptCount
		}
		record.OtherAttempts = max(record.ObservedAliasAttempts-visibleAttempts, 0)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}
