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
	return BuildUsageSnapshotWithFilter(db, dto.UsageQueryFilter{})
}

// Snapshot 先读事件，再按时间窗口在内存里汇总。
func BuildUsageSnapshotWithFilter(db *gorm.DB, filter dto.UsageQueryFilter) (*dto.StatisticsSnapshot, error) {
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
func BuildUsageOverviewWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) (*dto.UsageOverviewRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	events, err := loadUsageOverviewEventsWithFilter(db, filter)
	if err != nil {
		return nil, err
	}
	pricingByModel, err := loadPriceSettingsByModel(db)
	if err != nil {
		return nil, err
	}

	return buildUsageOverviewFromEvents(events, filter, pricingByModel), nil
}

func buildUsageOverviewFromEvents(events []entities.UsageEvent, filter dto.UsageQueryFilter, pricingByModel map[string]entities.ModelPriceSetting) *dto.UsageOverviewRecord {
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
			CostAvailable: true,
		},
		Series:       newUsageOverviewSeriesRecord(),
		HourlySeries: newUsageOverviewSeriesRecord(),
		DailySeries:  newUsageOverviewSeriesRecord(),
		Health:       buildUsageOverviewHealth(filter),
	}
	if len(events) == 0 {
		return overview
	}

	for _, event := range events {
		applyUsageEventToSnapshot(overview.Usage, event, false)
		applyUsageEventToOverview(overview, event, bucketByDay, latestHourlyStart, pricingByModel)
	}
	finalizeUsageOverview(overview, false)
	return overview
}

// Overview 第二步：按时间窗口读事件，再交给内存汇总。
func loadUsageOverviewEventsWithFilter(db *gorm.DB, filter dto.UsageQueryFilter) ([]entities.UsageEvent, error) {
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

	apiSnapshot := snapshot.APIs[apiKey]
	if apiSnapshot.Models == nil {
		apiSnapshot.Models = map[string]dto.ModelSnapshot{}
	}

	modelSnapshot := apiSnapshot.Models[modelName]
	if includeDetails {
		detail := dto.RequestDetail{
			Timestamp: event.Timestamp.UTC(),
			LatencyMS: event.LatencyMS,
			Source:    strings.TrimSpace(event.Source),
			AuthIndex: strings.TrimSpace(event.AuthIndex),
			Failed:    event.Failed,
			Tokens: dto.TokenStats{
				InputTokens:     event.InputTokens,
				OutputTokens:    event.OutputTokens,
				ReasoningTokens: event.ReasoningTokens,
				CachedTokens:    event.CachedTokens,
				TotalTokens:     event.TotalTokens,
			},
		}
		modelSnapshot.Details = append(modelSnapshot.Details, detail)
	}
	modelSnapshot.TotalRequests++
	modelSnapshot.TotalTokens += event.TotalTokens
	apiSnapshot.TotalRequests++
	apiSnapshot.TotalTokens += event.TotalTokens
	snapshot.TotalRequests++
	snapshot.TotalTokens += event.TotalTokens
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
	snapshot.TokensByDay[dayKey] += event.TotalTokens
	snapshot.TokensByHour[hourKey] += event.TotalTokens

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
		Requests:        map[string]int64{},
		Tokens:          map[string]int64{},
		RPM:             map[string]float64{},
		TPM:             map[string]float64{},
		Cost:            map[string]float64{},
		InputTokens:     map[string]int64{},
		OutputTokens:    map[string]int64{},
		CachedTokens:    map[string]int64{},
		ReasoningTokens: map[string]int64{},
		Models:          map[string]dto.UsageOverviewSeriesRecord{},
	}
}

func applyUsageEventToOverviewSeries(series *dto.UsageOverviewSeriesRecord, event entities.UsageEvent, cost float64, bucketKey string, bucketMinutes int64) {
	series.Requests[bucketKey]++
	series.Tokens[bucketKey] += event.TotalTokens
	series.Cost[bucketKey] += cost
	series.InputTokens[bucketKey] += event.InputTokens
	series.OutputTokens[bucketKey] += event.OutputTokens
	series.CachedTokens[bucketKey] += event.CachedTokens
	series.ReasoningTokens[bucketKey] += event.ReasoningTokens
	series.RPM[bucketKey] = float64(series.Requests[bucketKey]) / float64(bucketMinutes)
	series.TPM[bucketKey] = float64(series.Tokens[bucketKey]) / float64(bucketMinutes)

	modelName := normalizeUsageOverviewDimension(event.Model)
	modelSeries := series.Models[modelName]
	if modelSeries.Requests == nil {
		modelSeries = newUsageOverviewSeriesRecord()
	}
	modelSeries.Requests[bucketKey]++
	modelSeries.Tokens[bucketKey] += event.TotalTokens
	modelSeries.Cost[bucketKey] += cost
	modelSeries.InputTokens[bucketKey] += event.InputTokens
	modelSeries.OutputTokens[bucketKey] += event.OutputTokens
	modelSeries.CachedTokens[bucketKey] += event.CachedTokens
	modelSeries.ReasoningTokens[bucketKey] += event.ReasoningTokens
	modelSeries.RPM[bucketKey] = float64(modelSeries.Requests[bucketKey]) / float64(bucketMinutes)
	modelSeries.TPM[bucketKey] = float64(modelSeries.Tokens[bucketKey]) / float64(bucketMinutes)
	series.Models[modelName] = modelSeries
}

func usageEventRequiresPricing(event entities.UsageEvent) bool {
	return event.InputTokens > 0 || event.OutputTokens > 0 || event.CachedTokens > 0
}

func applyUsageEventToOverview(overview *dto.UsageOverviewRecord, event entities.UsageEvent, bucketByDay bool, latestHourlyStart *time.Time, pricingByModel map[string]entities.ModelPriceSetting) {
	overview.Summary.CachedTokens += event.CachedTokens
	overview.Summary.ReasoningTokens += event.ReasoningTokens
	if event.Failed {
		overview.Health.TotalFailure++
	} else {
		overview.Health.TotalSuccess++
	}
	pricing, ok := pricingByModel[strings.TrimSpace(event.Model)]
	if !ok && usageEventRequiresPricing(event) {
		overview.Summary.CostAvailable = false
	}
	cost := calculateUsageEventCost(event, pricing)
	overview.Summary.TotalCost += cost

	bucketKey, bucketMinutes := usageOverviewBucket(event.Timestamp.UTC(), bucketByDay)
	applyUsageEventToOverviewSeries(&overview.Series, event, cost, bucketKey, bucketMinutes)

	hourKey, hourMinutes := usageOverviewBucket(event.Timestamp.UTC(), false)
	if latestHourlyStart == nil || !event.Timestamp.UTC().Before(*latestHourlyStart) {
		applyUsageEventToOverviewSeries(&overview.HourlySeries, event, cost, hourKey, hourMinutes)
	}

	dayKey, dayMinutes := usageOverviewBucket(event.Timestamp.UTC(), true)
	applyUsageEventToOverviewSeries(&overview.DailySeries, event, cost, dayKey, dayMinutes)
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

func computeWindowMinutes(filter dto.UsageQueryFilter) int64 {
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

func shouldBucketUsageOverviewByDay(filter dto.UsageQueryFilter, windowMinutes int64) bool {
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

func latestHourlySeriesStart(filter dto.UsageQueryFilter) *time.Time {
	if filter.EndTime == nil {
		return nil
	}
	currentHour := filter.EndTime.UTC().Truncate(time.Hour)
	start := currentHour.Add(-23 * time.Hour)
	return &start
}
