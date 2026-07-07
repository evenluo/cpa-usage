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

	if err := db.Where("bucket_start IN ?", bucketList).Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		return fmt.Errorf("delete hourly usage rollups: %w", err)
	}
	if len(rollupsByKey) == 0 {
		return nil
	}
	rollups := make([]entities.UsageRollupHourly, 0, len(rollupsByKey))
	for _, rollup := range rollupsByKey {
		rollups = append(rollups, *rollup)
	}
	if err := db.Create(&rollups).Error; err != nil {
		return fmt.Errorf("create hourly usage rollups: %w", err)
	}
	return nil
}

func applyUsageEventToHourlyRollup(rollup *entities.UsageRollupHourly, event entities.UsageEvent) {
	rollup.RequestCount++
	if event.Failed {
		rollup.FailureCount++
	} else {
		rollup.SuccessCount++
	}
	inputTokens := positiveInt64(event.InputTokens)
	cachedTokens := positiveInt64(event.CachedTokens)
	rollup.InputTokens += inputTokens
	rollup.BillablePromptTokens += maxInt64(inputTokens-cachedTokens, 0)
	rollup.OutputTokens += positiveInt64(event.OutputTokens)
	rollup.ReasoningTokens += positiveInt64(event.ReasoningTokens)
	rollup.CachedTokens += cachedTokens
	rollup.TotalTokens += event.TotalTokens
	if event.LatencyMS > 0 {
		rollup.TotalLatencyMS += event.LatencyMS
		rollup.LatencySampleCount++
	}
	eventTime := event.Timestamp.UTC()
	if rollup.LastEventAt.IsZero() || eventTime.After(rollup.LastEventAt) {
		rollup.LastEventAt = eventTime
	}
}

func positiveInt64(value int64) int64 {
	if value > 0 {
		return value
	}
	return 0
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
