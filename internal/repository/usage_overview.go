package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func BuildUsageSnapshot(db *gorm.DB) (*dto.StatisticsSnapshot, error) {
	return BuildUsageSnapshotWithFilter(db, dto.UsageTimeScope{})
}

// Snapshot 先读事件，再按时间窗口在内存里汇总。
func BuildUsageSnapshotWithFilter(db *gorm.DB, filter dto.UsageTimeScope) (*dto.StatisticsSnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	events, err := loadUsageOverviewEventsWithFilter(db, filter)
	if err != nil {
		return nil, err
	}

	return buildUsageSnapshotFromEvents(events), nil
}

// Overview 先读事件，再组合窗口、系列和价格信息。
func BuildUsageOverviewWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageOverviewFilter) (*dto.UsageOverviewRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	events, err := loadUsageOverviewEventsWithFilter(db, filter.UsageTimeScope)
	if err != nil {
		return nil, err
	}
	pricingByModel, err := loadPriceSettingsByModel(db)
	if err != nil {
		return nil, err
	}

	return buildUsageOverviewFromEvents(events, filter, pricingByModel), nil
}

func buildUsageOverviewFromEvents(events []entities.UsageEvent, filter dto.UsageOverviewFilter, pricingByModel map[string]entities.ModelPriceSetting) *dto.UsageOverviewRecord {
	windowMinutes := computeWindowMinutes(filter)
	bucketByDay := shouldBucketUsageOverviewByDay(filter, windowMinutes)
	latestHourlyStart := latestHourlySeriesStart(filter)
	overview := &dto.UsageOverviewRecord{
		Usage: &dto.StatisticsSnapshot{
			APIs:           map[string]dto.APISnapshot{},
			RequestsByDay:  map[string]int64{},
			RequestsByHour: map[string]int64{},
			TokensByDay:    map[string]int64{},
			TokensByHour:   map[string]int64{},
		},
		Summary: dto.UsageOverviewSummaryRecord{
			WindowMinutes: windowMinutes,
		},
		Series:       newUsageOverviewSeriesRecord(),
		HourlySeries: newUsageOverviewSeriesRecord(),
		DailySeries:  newUsageOverviewSeriesRecord(),
		Health:       buildUsageOverviewHealth(filter),
	}
	if len(events) == 0 {
		return overview
	}

	var missingPricingEvents int64
	var knownCostAttempts int64
	for _, event := range events {
		_, hasPricing := pricingByModel[strings.TrimSpace(event.Model)]
		if !usageEventHasCompleteAccounting(event) {
			missingPricingEvents++
		} else if !usageEventRequiresPricing(event) || hasPricing {
			knownCostAttempts++
		} else {
			missingPricingEvents++
		}
		applyUsageEventToSnapshot(overview.Usage, event, false)
		applyUsageEventToOverview(overview, event, bucketByDay, latestHourlyStart, pricingByModel)
	}
	overview.Summary.CostAvailable = assessCostCompleteness(missingPricingEvents, knownCostAttempts).Available
	finalizeUsageOverview(overview, false)
	return overview
}

// Overview 第二步：按时间窗口读事件，再交给内存汇总。
func loadUsageOverviewEventsWithFilter(db *gorm.DB, filter dto.UsageTimeScope) ([]entities.UsageEvent, error) {
	query := applyUsageOverviewQuery(db.Model(&entities.UsageEvent{}), filter).Order("timestamp asc")

	var events []entities.UsageEvent
	if err := query.Find(&events).Error; err != nil {
		return nil, fmt.Errorf("load usage events: %w", err)
	}
	return events, nil
}

func buildUsageSnapshotFromEvents(events []entities.UsageEvent) *dto.StatisticsSnapshot {
	snapshot := &dto.StatisticsSnapshot{
		APIs:           map[string]dto.APISnapshot{},
		RequestsByDay:  map[string]int64{},
		RequestsByHour: map[string]int64{},
		TokensByDay:    map[string]int64{},
		TokensByHour:   map[string]int64{},
	}
	if len(events) == 0 {
		return snapshot
	}

	for _, event := range events {
		applyUsageEventToSnapshot(snapshot, event, true)
	}
	finalizeUsageSnapshot(snapshot, true)
	return snapshot
}

func applyUsageEventToSnapshot(snapshot *dto.StatisticsSnapshot, event entities.UsageEvent, includeDetails bool) {
	apiKey := normalizeUsageOverviewDimension(event.APIGroupKey)
	modelName := normalizeUsageOverviewDimension(event.Model)
	tokens, canonicalAvailable := usageEventCanonicalTokenStats(event)

	apiSnapshot := snapshot.APIs[apiKey]
	if apiSnapshot.Models == nil {
		apiSnapshot.Models = map[string]dto.ModelSnapshot{}
	}

	modelSnapshot := apiSnapshot.Models[modelName]
	if includeDetails {
		detail := dto.RequestDetail{
			Timestamp:                event.Timestamp.UTC(),
			LatencyMS:                event.LatencyMS,
			Source:                   strings.TrimSpace(event.Source),
			AuthIndex:                strings.TrimSpace(event.AuthIndex),
			Failed:                   event.Failed,
			Tokens:                   tokens,
			CanonicalTokensAvailable: canonicalAvailable,
		}
		modelSnapshot.Details = append(modelSnapshot.Details, detail)
	}
	modelSnapshot.TotalRequests++
	modelSnapshot.TotalTokens += tokens.TotalTokens
	apiSnapshot.TotalRequests++
	apiSnapshot.TotalTokens += tokens.TotalTokens
	snapshot.TotalRequests++
	snapshot.TotalTokens += tokens.TotalTokens
	if canonicalAvailable {
		modelSnapshot.CanonicalValidAttempts++
		apiSnapshot.CanonicalValidAttempts++
		snapshot.CanonicalValidAttempts++
	}
	if event.Failed {
		modelSnapshot.FailureCount++
		apiSnapshot.FailureCount++
		snapshot.FailureCount++
	} else {
		modelSnapshot.SuccessCount++
		apiSnapshot.SuccessCount++
		snapshot.SuccessCount++
	}

	dayKey := event.Timestamp.In(time.Local).Format("2006-01-02")
	hourKey := event.Timestamp.UTC().Format("2006-01-02T15:00:00Z")
	snapshot.RequestsByDay[dayKey]++
	snapshot.RequestsByHour[hourKey]++
	snapshot.TokensByDay[dayKey] += tokens.TotalTokens
	snapshot.TokensByHour[hourKey] += tokens.TotalTokens

	apiSnapshot.Models[modelName] = modelSnapshot
	snapshot.APIs[apiKey] = apiSnapshot
}

func finalizeUsageSnapshot(snapshot *dto.StatisticsSnapshot, includeDetails bool) {
	if !includeDetails {
		return
	}
	for apiKey, apiSnapshot := range snapshot.APIs {
		for modelName, modelSnapshot := range apiSnapshot.Models {
			sort.Slice(modelSnapshot.Details, func(i, j int) bool {
				return modelSnapshot.Details[i].Timestamp.Before(modelSnapshot.Details[j].Timestamp)
			})
			apiSnapshot.Models[modelName] = modelSnapshot
		}
		snapshot.APIs[apiKey] = apiSnapshot
	}
}

func newUsageOverviewSeriesRecord() dto.UsageOverviewSeriesRecord {
	return dto.UsageOverviewSeriesRecord{
		Requests:               map[string]int64{},
		Tokens:                 map[string]int64{},
		RPM:                    map[string]float64{},
		TPM:                    map[string]float64{},
		Cost:                   map[string]float64{},
		CostStatus:             map[string]string{},
		InputTokens:            map[string]int64{},
		OutputTokens:           map[string]int64{},
		CachedTokens:           map[string]int64{},
		ReasoningTokens:        map[string]int64{},
		CanonicalValidAttempts: map[string]int64{},
		Models:                 map[string]dto.UsageOverviewSeriesRecord{},
	}
}

func applyUsageEventToOverviewSeries(series *dto.UsageOverviewSeriesRecord, event entities.UsageEvent, tokens dto.TokenStats, canonicalAvailable bool, cost float64, costStatus string, bucketKey string, bucketMinutes int64) {
	series.CostStatus[bucketKey] = mergeUsageOverviewCostStatus(series.Requests[bucketKey], series.CostStatus[bucketKey], costStatus)
	series.Requests[bucketKey]++
	series.Tokens[bucketKey] += tokens.TotalTokens
	series.Cost[bucketKey] += cost
	series.InputTokens[bucketKey] += tokens.InputTokens
	series.OutputTokens[bucketKey] += tokens.OutputTokens
	series.CachedTokens[bucketKey] += tokens.CachedTokens
	series.ReasoningTokens[bucketKey] += tokens.ReasoningTokens
	if canonicalAvailable {
		series.CanonicalValidAttempts[bucketKey]++
	}
	series.RPM[bucketKey] = float64(series.Requests[bucketKey]) / float64(bucketMinutes)
	series.TPM[bucketKey] = float64(series.Tokens[bucketKey]) / float64(bucketMinutes)

	modelName := normalizeUsageOverviewDimension(event.Model)
	modelSeries := series.Models[modelName]
	if modelSeries.Requests == nil {
		modelSeries = newUsageOverviewSeriesRecord()
	}
	modelSeries.CostStatus[bucketKey] = mergeUsageOverviewCostStatus(modelSeries.Requests[bucketKey], modelSeries.CostStatus[bucketKey], costStatus)
	modelSeries.Requests[bucketKey]++
	modelSeries.Tokens[bucketKey] += tokens.TotalTokens
	modelSeries.Cost[bucketKey] += cost
	modelSeries.InputTokens[bucketKey] += tokens.InputTokens
	modelSeries.OutputTokens[bucketKey] += tokens.OutputTokens
	modelSeries.CachedTokens[bucketKey] += tokens.CachedTokens
	modelSeries.ReasoningTokens[bucketKey] += tokens.ReasoningTokens
	if canonicalAvailable {
		modelSeries.CanonicalValidAttempts[bucketKey]++
	}
	modelSeries.RPM[bucketKey] = float64(modelSeries.Requests[bucketKey]) / float64(bucketMinutes)
	modelSeries.TPM[bucketKey] = float64(modelSeries.Tokens[bucketKey]) / float64(bucketMinutes)
	series.Models[modelName] = modelSeries
}

func mergeUsageOverviewCostStatus(existingRequests int64, existingStatus string, incomingStatus string) string {
	if existingRequests == 0 {
		return incomingStatus
	}
	if existingStatus == incomingStatus {
		return existingStatus
	}
	return dto.CostStatusPartial
}

func usageEventRequiresPricing(event entities.UsageEvent) bool {
	if !usageEventHasCompleteAccounting(event) {
		return false
	}
	facts := InterpretUsageAttempt(event).Accounting
	return optionalInt64Value(facts.Input.UncachedTokens) > 0 || optionalInt64Value(facts.Input.CacheWriteTokens) > 0 ||
		optionalInt64Value(facts.Input.CacheReadTokens) > 0 || optionalInt64Value(facts.Output.TotalTokens) > 0
}

func applyUsageEventToOverview(overview *dto.UsageOverviewRecord, event entities.UsageEvent, bucketByDay bool, latestHourlyStart *time.Time, pricingByModel map[string]entities.ModelPriceSetting) {
	tokens, canonicalAvailable := usageEventCanonicalTokenStats(event)
	overview.Summary.CachedTokens += tokens.CachedTokens
	overview.Summary.ReasoningTokens += tokens.ReasoningTokens
	if canonicalAvailable {
		overview.Summary.CanonicalValidAttempts++
	}
	if event.Failed {
		overview.Health.TotalFailure++
	} else {
		overview.Health.TotalSuccess++
	}
	pricing, hasPricing := pricingByModel[strings.TrimSpace(event.Model)]
	cost := calculateUsageEventCost(event, pricing)
	costStatus := dto.CostStatusUnavailable
	if usageEventHasCompleteAccounting(event) && (!usageEventRequiresPricing(event) || hasPricing) {
		costStatus = dto.CostStatusAvailable
	}
	overview.Summary.TotalCost += cost

	bucketKey, bucketMinutes := usageOverviewBucket(event.Timestamp.UTC(), bucketByDay)
	applyUsageEventToOverviewSeries(&overview.Series, event, tokens, canonicalAvailable, cost, costStatus, bucketKey, bucketMinutes)

	hourKey, hourMinutes := usageOverviewBucket(event.Timestamp.UTC(), false)
	if latestHourlyStart == nil || !event.Timestamp.UTC().Before(*latestHourlyStart) {
		applyUsageEventToOverviewSeries(&overview.HourlySeries, event, tokens, canonicalAvailable, cost, costStatus, hourKey, hourMinutes)
	}

	dayKey, dayMinutes := usageOverviewBucket(event.Timestamp.UTC(), true)
	applyUsageEventToOverviewSeries(&overview.DailySeries, event, tokens, canonicalAvailable, cost, costStatus, dayKey, dayMinutes)
	updateUsageOverviewHealthBlock(overview.Health.BlockDetails, event)
}

func finalizeUsageOverview(overview *dto.UsageOverviewRecord, includeDetails bool) {
	finalizeUsageSnapshot(overview.Usage, includeDetails)
	overview.Summary.RequestCount = overview.Usage.TotalRequests
	overview.Summary.TokenCount = overview.Usage.TotalTokens
	if overview.Summary.WindowMinutes > 0 {
		overview.Summary.RPM = float64(overview.Summary.RequestCount) / float64(overview.Summary.WindowMinutes)
		overview.Summary.TPM = float64(overview.Summary.TokenCount) / float64(overview.Summary.WindowMinutes)
	}
	if total := overview.Health.TotalSuccess + overview.Health.TotalFailure; total > 0 {
		overview.Health.SuccessRate = (float64(overview.Health.TotalSuccess) / float64(total)) * 100
	}
}

func normalizeUsageOverviewDimension(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}

const usageOverviewDailyBucketThresholdMinutes int64 = 7 * 24 * 60

func computeWindowMinutes(filter dto.UsageOverviewFilter) int64 {
	if filter.StartTime == nil || filter.EndTime == nil {
		return 0
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	if end.Before(start) {
		return 0
	}
	minutes := int64(end.Sub(start) / time.Minute)
	if end.Sub(start)%time.Minute != 0 {
		minutes++
	}
	if minutes < 1 {
		return 1
	}
	return minutes
}

func shouldBucketUsageOverviewByDay(filter dto.UsageOverviewFilter, windowMinutes int64) bool {
	if filter.Range == "all" || filter.Range == "7d" {
		return true
	}
	return windowMinutes >= usageOverviewDailyBucketThresholdMinutes
}

func usageOverviewBucket(timestamp time.Time, byDay bool) (string, int64) {
	if byDay {
		return timestamp.In(time.Local).Format("2006-01-02"), 24 * 60
	}
	return timestamp.UTC().Format("2006-01-02T15:00:00Z"), 60
}

func latestHourlySeriesStart(filter dto.UsageOverviewFilter) *time.Time {
	if filter.EndTime == nil {
		return nil
	}
	currentHour := filter.EndTime.UTC().Truncate(time.Hour)
	start := currentHour.Add(-23 * time.Hour)
	return &start
}
