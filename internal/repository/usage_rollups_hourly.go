package repository

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

type hourlyUsageRollupKey struct {
	BucketStart    time.Time
	Provider       string
	Model          string
	AuthType       string
	AuthIndex      string
	APIKeyIdentity string
}

// RebuildUsageRollupsForEvents rebuilds every hourly bucket touched by the given raw events.
func RebuildUsageRollupsForEvents(db *gorm.DB, seedEvents []entities.UsageEvent) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if len(seedEvents) == 0 {
		return nil
	}

	buckets := map[time.Time]struct{}{}
	for _, event := range seedEvents {
		bucket := event.Timestamp.UTC().Truncate(time.Hour)
		buckets[bucket] = struct{}{}
	}
	if len(buckets) == 0 {
		return nil
	}

	bucketList := make([]time.Time, 0, len(buckets))
	for bucket := range buckets {
		bucketList = append(bucketList, bucket)
	}
	sort.Slice(bucketList, func(i, j int) bool {
		return bucketList[i].Before(bucketList[j])
	})

	events := make([]entities.UsageEvent, 0, len(seedEvents))
	for _, bucket := range bucketList {
		var bucketEvents []entities.UsageEvent
		if err := db.
			Where("timestamp >= ? AND timestamp < ?", bucket, bucket.Add(time.Hour)).
			Find(&bucketEvents).Error; err != nil {
			return fmt.Errorf("load usage events for hourly rollup rebuild: %w", err)
		}
		events = append(events, bucketEvents...)
	}

	return rebuildUsageRollupsForBuckets(db, bucketList, events)
}

func RebuildUsageRollupsForBucketRange(db *gorm.DB, startBucket time.Time, endBucket time.Time) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		return rebuildUsageRollupsForBucketRange(tx, startBucket, endBucket)
	})
}

func rebuildUsageRollupsForBucketRange(db *gorm.DB, startBucket time.Time, endBucket time.Time) error {
	start := startBucket.UTC().Truncate(time.Hour)
	end := endBucket.UTC().Truncate(time.Hour)
	if end.Before(start) {
		return nil
	}

	bucketList := hourlyBucketRange(start, end)
	if err := db.Where("bucket_start IN ?", bucketList).Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		return fmt.Errorf("delete hourly usage rollups: %w", err)
	}
	var events []entities.UsageEvent
	if err := db.
		Where("timestamp >= ? AND timestamp < ?", start, end.Add(time.Hour)).
		Find(&events).Error; err != nil {
		return fmt.Errorf("load usage events for hourly rollup range rebuild: %w", err)
	}
	rollups := buildUsageRollupsForBuckets(bucketList, events)
	if len(rollups) == 0 {
		return nil
	}
	if err := db.Create(&rollups).Error; err != nil {
		return fmt.Errorf("create hourly usage rollups: %w", err)
	}
	return nil
}

func rebuildUsageRollupsForBuckets(db *gorm.DB, bucketList []time.Time, events []entities.UsageEvent) error {
	if len(bucketList) == 0 {
		return nil
	}
	rollups := buildUsageRollupsForBuckets(bucketList, events)
	if err := db.Where("bucket_start IN ?", bucketList).Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		return fmt.Errorf("delete hourly usage rollups: %w", err)
	}
	if len(rollups) == 0 {
		return nil
	}
	if err := db.Create(&rollups).Error; err != nil {
		return fmt.Errorf("create hourly usage rollups: %w", err)
	}
	return nil
}

func buildUsageRollupsForBuckets(bucketList []time.Time, events []entities.UsageEvent) []entities.UsageRollupHourly {
	buckets := make(map[time.Time]struct{}, len(bucketList))
	for _, bucket := range bucketList {
		buckets[bucket.UTC().Truncate(time.Hour)] = struct{}{}
	}
	rollupsByKey := map[hourlyUsageRollupKey]*entities.UsageRollupHourly{}
	for _, event := range events {
		bucket := event.Timestamp.UTC().Truncate(time.Hour)
		if _, ok := buckets[bucket]; !ok {
			continue
		}
		key := hourlyUsageRollupKey{
			BucketStart:    bucket,
			Provider:       strings.TrimSpace(event.Provider),
			Model:          strings.TrimSpace(event.Model),
			AuthType:       strings.TrimSpace(event.AuthType),
			AuthIndex:      strings.TrimSpace(event.AuthIndex),
			APIKeyIdentity: usageEventAPIKeyIdentity(event),
		}
		rollup := rollupsByKey[key]
		if rollup == nil {
			rollup = &entities.UsageRollupHourly{
				BucketStart:    key.BucketStart,
				Provider:       key.Provider,
				Model:          key.Model,
				AuthType:       key.AuthType,
				AuthIndex:      key.AuthIndex,
				APIKeyIdentity: key.APIKeyIdentity,
			}
			rollupsByKey[key] = rollup
		}
		applyUsageEventToHourlyRollup(rollup, event)
	}

	if len(rollupsByKey) == 0 {
		return nil
	}
	rollups := make([]entities.UsageRollupHourly, 0, len(rollupsByKey))
	for _, rollup := range rollupsByKey {
		rollups = append(rollups, *rollup)
	}
	return rollups
}

func hourlyBucketRange(start time.Time, end time.Time) []time.Time {
	if end.Before(start) {
		return nil
	}
	buckets := make([]time.Time, 0, int(end.Sub(start)/time.Hour)+1)
	for bucket := start.UTC().Truncate(time.Hour); !bucket.After(end); bucket = bucket.Add(time.Hour) {
		buckets = append(buckets, bucket)
	}
	return buckets
}

func applyUsageEventToHourlyRollup(rollup *entities.UsageRollupHourly, event entities.UsageEvent) {
	rollup.RequestCount++
	if event.Failed {
		rollup.FailureCount++
	} else {
		rollup.SuccessCount++
	}
	applyUsageAccountingToHourlyRollup(rollup, event)
	if event.LatencyMS > 0 {
		rollup.TotalLatencyMS += event.LatencyMS
		rollup.LatencySampleCount++
	}
	eventTime := event.Timestamp.UTC()
	if rollup.LastEventAt.IsZero() || eventTime.After(rollup.LastEventAt) {
		rollup.LastEventAt = eventTime
	}
}

func applyUsageAccountingToHourlyRollup(rollup *entities.UsageRollupHourly, event entities.UsageEvent) {
	switch event.AccountingState {
	case AccountingAbsent:
		rollup.AccountingAbsentAttempts++
	case AccountingInvalid:
		rollup.AccountingInvalidAttempts++
	case AccountingValid:
		rollup.AccountingValidAttempts++
		switch optionalStringValue(event.TokenQuality) {
		case "complete":
			rollup.AccountingValidCompleteAttempts++
			promptTokens := optionalInt64Value(event.CanonicalUncachedTokens) + optionalInt64Value(event.CanonicalCacheWriteTokens)
			cacheReadTokens := optionalInt64Value(event.CanonicalCacheReadTokens)
			outputTokens := optionalInt64Value(event.CanonicalOutputTokens)
			rollup.CanonicalCompletePromptTokens += promptTokens
			rollup.CanonicalCompleteCacheReadTokens += cacheReadTokens
			rollup.CanonicalCompleteOutputTokens += outputTokens
			if promptTokens == 0 && cacheReadTokens == 0 && outputTokens == 0 {
				rollup.CanonicalCompleteZeroAttempts++
			}
		case "inconsistent":
			rollup.AccountingValidInconsistentAttempts++
		case "unclassified":
			rollup.AccountingValidUnclassifiedAttempts++
		}
		rollup.CanonicalTotalTokens += optionalInt64Value(event.CanonicalTotalTokens)
		rollup.CanonicalInputTokens += optionalInt64Value(event.CanonicalInputTokens)
		rollup.CanonicalUncachedTokens += optionalInt64Value(event.CanonicalUncachedTokens)
		rollup.CanonicalCacheReadTokens += optionalInt64Value(event.CanonicalCacheReadTokens)
		rollup.CanonicalCacheWriteTokens += optionalInt64Value(event.CanonicalCacheWriteTokens)
		rollup.CanonicalOutputTokens += optionalInt64Value(event.CanonicalOutputTokens)
		rollup.CanonicalNonReasoningTokens += optionalInt64Value(event.CanonicalNonReasoningTokens)
		rollup.CanonicalReasoningTokens += optionalInt64Value(event.CanonicalReasoningTokens)
		rollup.CanonicalUnclassifiedTokens += optionalInt64Value(event.CanonicalUnclassifiedTokens)
	}
}

func optionalInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
